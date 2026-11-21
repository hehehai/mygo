//go:build !js

package mygo

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/update"
	"github.com/egoist/mygo/internal/wait"
)

// updateServer serves a signed update whose app holds main with content,
// and with deltaFrom, a delta of it from that version of installedApp.
// requests counts the requests of each file.
func updateServer(t *testing.T, version, content, deltaFrom string) (srv *httptest.Server, key string, m update.Manifest, requests func(string) int) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(nil)
	src := t.TempDir()
	app := "app"
	if runtime.GOOS == "darwin" {
		app = "Test.app"
	}
	main := filepath.Join(src, app, "main")
	if err := os.MkdirAll(filepath.Dir(main), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, app, "resource"), []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, app, "code"), newCode, 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []string{app}
	if runtime.GOOS != "darwin" {
		// Elsewhere the archive holds the files of the app directory.
		src, entries = filepath.Join(src, app), []string{"code", "main", "resource"}
	}
	var archive bytes.Buffer
	if err := update.WriteArchive(&archive, src, entries); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive.Bytes())
	m = update.Manifest{Version: version, Notes: "- Faster", Date: "2026-09-27T10:00:00Z", Size: int64(archive.Len()), Signature: update.Sign(priv, sum[:]),
		Release: update.SignRelease(priv, version, "", sum[:])}
	var mu sync.Mutex
	counts := map[string]int{}
	mux := http.NewServeMux()
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counts[r.URL.Path]++
		mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	m.URL = srv.URL + "/app.tar.gz"
	mux.HandleFunc("/app.tar.gz", func(w http.ResponseWriter, r *http.Request) { w.Write(archive.Bytes()) })
	if deltaFrom != "" {
		old, _ := installedApp(t)
		var delta bytes.Buffer
		if err := update.WriteDelta(&delta, deltaFrom, version, old, filepath.Join(src, entries[0]), nil); runtime.GOOS != "darwin" {
			delta.Reset()
			err = update.WriteDelta(&delta, deltaFrom, version, old, src, entries)
		} else if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(delta.Bytes())
		m.Deltas = []update.Delta{{From: deltaFrom, URL: srv.URL + "/app.delta", Size: int64(delta.Len()), Signature: update.Sign(priv, sum[:]),
			Release: update.SignRelease(priv, version, deltaFrom, sum[:])}}
		mux.HandleFunc("/app.delta", func(w http.ResponseWriter, r *http.Request) { w.Write(delta.Bytes()) })
	}
	mux.HandleFunc("/update.json", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(m) })
	return srv, base64.StdEncoding.EncodeToString(pub), m, func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return counts[path]
	}
}

// installedApp creates what an update replaces, holding main with "v1".
func installedApp(t *testing.T) (target, main string) {
	t.Helper()
	dir := t.TempDir()
	target = dir
	if runtime.GOOS == "darwin" {
		target = filepath.Join(dir, "My App.app")
	}
	main = filepath.Join(target, "main")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "resource"), []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "code"), oldCode, 0o644); err != nil {
		t.Fatal(err)
	}
	return target, main
}

// oldCode is a file of installedApp, which newCode, of the update, changes
// a little: a delta patches it.
var oldCode, newCode = func() ([]byte, []byte) {
	r := rand.New(rand.NewPCG(1, 2))
	old := make([]byte, 64<<10)
	for i := range old {
		old[i] = byte(r.Uint32())
	}
	new := bytes.Clone(old)
	for range 20 {
		new[r.IntN(len(new))]++
	}
	return old, new
}()

func TestUpdater(t *testing.T) {
	ctx := context.Background()
	srv, key, m, _ := updateServer(t, "1.2.0", "v2", "")

	if _, err := Updater.Check(ctx); err != ErrUpdatesDisabled {
		t.Errorf("Check without updates: %v", err)
	}
	up, err := checkUpdate(ctx, feedSource(srv.URL+"/update.json"), key, "1.1.9", false)
	if err != nil || up == nil || up.Version != "1.2.0" || up.Notes != "- Faster" || up.Date.Year() != 2026 {
		t.Fatalf("checkUpdate = %+v, %v", up, err)
	}
	// An app on a prerelease, whose user turned prereleases off, takes no
	// older version, and the release of its version once there is one.
	for _, current := range []string{"1.2.0", "1.10.0", "1.3.0-beta.1"} {
		if up, err := checkUpdate(ctx, feedSource(srv.URL+"/update.json"), key, current, false); up != nil || err != nil {
			t.Errorf("checkUpdate from %s = %+v, %v", current, up, err)
		}
	}
	if up, err := checkUpdate(ctx, feedSource(srv.URL+"/update.json"), key, "1.2.0-rc.1", false); err != nil || up == nil || up.Version != "1.2.0" {
		t.Errorf("checkUpdate from 1.2.0-rc.1 = %+v, %v", up, err)
	}
	// Feeds of updates.url are the same with prereleases on.
	if up, err := checkUpdate(ctx, feedSource(srv.URL+"/update.json"), key, "1.1.9", true); err != nil || up == nil || up.Version != "1.2.0" {
		t.Errorf("checkUpdate with prereleases = %+v, %v", up, err)
	}
	if _, err := checkUpdate(ctx, feedSource("http://example.com/update.json"), key, "1.0.0", false); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("plain HTTP: %v", err)
	}
	// A feed with nothing published yet: up to date, not an error, and so
	// is a source that says so; other failures stay errors.
	if up, err := checkUpdate(ctx, feedSource(srv.URL+"/missing/update.json"), key, "1.0.0", false); up != nil || err != nil {
		t.Errorf("checkUpdate of a feed with no manifest = %+v, %v", up, err)
	}
	for _, e := range []error{fmt.Errorf("nothing at %s: %w", srv.URL, fs.ErrNotExist), fmt.Errorf("%w", ErrNotPublished)} {
		if up, err := checkUpdate(ctx, errSource{e}, key, "1.0.0", false); up != nil || err != nil {
			t.Errorf("checkUpdate of a source that has nothing (%v) = %+v, %v", e, up, err)
		}
	}
	if _, err := checkUpdate(ctx, errSource{errors.New("offline")}, key, "1.0.0", false); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Errorf("checkUpdate of a failing source: %v", err)
	}

	target, main := installedApp(t)
	quit := runningApp(t)
	var progress [2]int64
	err = installUpdate(ctx, httpGet, up.manifest, key, up.current, target, func(n, total int64) { progress = [2]int64{n, total} })
	if err != nil {
		t.Fatal(err)
	}
	quit()
	if b, _ := os.ReadFile(main); string(b) != "v2" {
		t.Errorf("after the update main = %q", b)
	}
	if progress != [2]int64{m.Size, m.Size} {
		t.Errorf("progress = %v, want %d", progress, m.Size)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".*")); runtime.GOOS == "darwin" && len(leftovers) > 0 {
		t.Errorf("left behind: %q", leftovers)
	}

	// A tampered archive, or one signed with another key, is refused and
	// changes nothing.
	target, main = installedApp(t)
	bad := up.manifest
	bad.Release = base64.StdEncoding.EncodeToString(make([]byte, 64))
	if err := installUpdate(ctx, httpGet, bad, key, up.current, target, nil); err == nil || !strings.Contains(err.Error(), "signed") {
		t.Errorf("bad signature: %v", err)
	}
	// The archive's own signature alone (a manifest of before release
	// signatures) is not enough: it does not say what version it is.
	old := up.manifest
	old.Release = ""
	if err := installUpdate(ctx, httpGet, old, key, up.current, target, nil); err == nil || !strings.Contains(err.Error(), "signed") {
		t.Errorf("no release signature: %v", err)
	}
	// A signed archive offered as another version (an old one as newer, a
	// beta as a release) is refused.
	for _, v := range []string{"9.9.9", "1.2.0-beta.1"} {
		replay := up.manifest
		replay.Version = v
		if err := installUpdate(ctx, httpGet, replay, key, up.current, target, nil); err == nil || !strings.Contains(err.Error(), "signed") {
			t.Errorf("the archive of 1.2.0 as %s: %v", v, err)
		}
	}
	otherKey, _, _ := ed25519.GenerateKey(nil)
	if err := installUpdate(ctx, httpGet, up.manifest, base64.StdEncoding.EncodeToString(otherKey), up.current, target, nil); err == nil {
		t.Error("an update signed with another key was installed")
	}
	short := up.manifest
	short.Size++
	if err := installUpdate(ctx, httpGet, short, key, up.current, target, nil); err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Errorf("wrong size: %v", err)
	}
	if b, _ := os.ReadFile(main); string(b) != "v1" {
		t.Errorf("a refused update changed main to %q", b)
	}
}

// Channels: an app of a release is offered no beta unless its user chose
// prereleases; an app of a beta takes betas and the release after them.
// Redirects go to HTTPS only.
func TestUpdaterChannelsAndRedirects(t *testing.T) {
	ctx := context.Background()
	srv, key, _, _ := updateServer(t, "1.3.0-beta.1", "v2", "")
	feed := feedSource(srv.URL + "/update.json")
	if up, err := checkUpdate(ctx, feed, key, "1.2.0", false); up != nil || err != nil {
		t.Errorf("an app of a release offered a beta: %+v, %v", up, err)
	}
	if up, err := checkUpdate(ctx, feed, key, "1.2.0", true); up == nil || err != nil {
		t.Errorf("an app of a release that takes prereleases not offered the beta: %+v, %v", up, err)
	}
	if up, err := checkUpdate(ctx, feed, key, "1.3.0-alpha.2", false); up == nil || err != nil {
		t.Errorf("an app of a beta not offered a newer beta: %+v, %v", up, err)
	}
	srv2, key2, _, _ := updateServer(t, "1.3.0", "v2", "")
	if up, err := checkUpdate(ctx, feedSource(srv2.URL+"/update.json"), key2, "1.3.0-beta.4", false); up == nil || err != nil {
		t.Errorf("an app of a beta not offered the release: %+v, %v", up, err)
	}
	for _, c := range []struct {
		current, offered string
		pre, takes       bool
	}{
		{"0.1.0-beta.2", "0.1.0-beta.3", false, true},
		{"0.1.0", "0.1.1", false, true},
		{"0.1.0", "0.2.0-beta.1", false, false},
		{"0.1.0", "0.2.0-beta.1", true, true},
		{"0.2.0-beta.1", "0.1.1", true, false},
	} {
		if got := update.Takes(c.current, c.offered, c.pre); got != c.takes {
			t.Errorf("Takes(%s, %s, %t) = %t", c.current, c.offered, c.pre, got)
		}
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/update.json", http.StatusFound)
	}))
	defer redirect.Close()
	if _, err := checkUpdate(ctx, feedSource(redirect.URL+"/update.json"), key, "1.0.0", false); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("a redirect to plain HTTP: %v", err)
	}
}

func TestUpdaterDelta(t *testing.T) {
	ctx := context.Background()
	srv, key, m, requests := updateServer(t, "1.2.0", "v2, a bigger version", "1.1.9")
	up, err := checkUpdate(ctx, feedSource(srv.URL+"/update.json"), key, "1.1.9", false)
	if err != nil || len(up.manifest.Deltas) != 1 {
		t.Fatalf("checkUpdate = %+v, %v", up, err)
	}
	install := func(m update.Manifest, current string, corrupt bool) (progress [][2]int64) {
		t.Helper()
		target, main := installedApp(t)
		quit := runningApp(t)
		if corrupt {
			os.WriteFile(filepath.Join(target, "code"), newCode[:1000], 0o644)
		}
		err := installUpdate(ctx, httpGet, m, key, current, target, func(n, total int64) { progress = append(progress, [2]int64{n, total}) })
		if err != nil {
			t.Fatal(err)
		}
		quit()
		if b, _ := os.ReadFile(main); string(b) != "v2, a bigger version" {
			t.Errorf("after the update main = %q", b)
		}
		if b, _ := os.ReadFile(filepath.Join(target, "resource")); string(b) != "unchanged" {
			t.Errorf("after the update resource = %q", b)
		}
		if b, _ := os.ReadFile(filepath.Join(target, "code")); !bytes.Equal(b, newCode) {
			t.Error("after the update code is not the new one")
		}
		if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".*")); runtime.GOOS == "darwin" && len(leftovers) > 0 {
			t.Errorf("left behind: %q", leftovers)
		}
		return progress
	}
	delta := m.Deltas[0]

	// The delta of the running version is all it downloads.
	if delta.Size > m.Size/10 {
		t.Errorf("the delta takes %d bytes, the archive %d", delta.Size, m.Size)
	}
	if progress := install(up.manifest, up.current, false); progress[len(progress)-1] != [2]int64{delta.Size, delta.Size} {
		t.Errorf("progress = %v, want the delta's %d bytes", progress, delta.Size)
	}
	if requests("/app.delta") != 1 || requests("/app.tar.gz") != 0 {
		t.Errorf("downloaded the delta %d times and the archive %d", requests("/app.delta"), requests("/app.tar.gz"))
	}

	// An app that the delta was not made from gets the archive, and so do
	// other versions and deltas that are not signed.
	if progress := install(up.manifest, up.current, true); progress[len(progress)-1] != [2]int64{m.Size, m.Size} {
		t.Errorf("progress = %v, want the archive's %d bytes last", progress, m.Size)
	}
	if requests("/app.delta") != 2 || requests("/app.tar.gz") != 1 {
		t.Errorf("downloaded the delta %d times and the archive %d", requests("/app.delta"), requests("/app.tar.gz"))
	}
	install(up.manifest, "1.1.8", false)
	if requests("/app.delta") != 2 || requests("/app.tar.gz") != 2 {
		t.Errorf("downloaded the delta %d times and the archive %d", requests("/app.delta"), requests("/app.tar.gz"))
	}
	unsigned := up.manifest
	unsigned.Deltas = []update.Delta{delta}
	unsigned.Deltas[0].Signature, unsigned.Deltas[0].Release = m.Signature, m.Release
	install(unsigned, up.current, false)
	if requests("/app.delta") != 3 || requests("/app.tar.gz") != 3 {
		t.Errorf("downloaded the delta %d times and the archive %d", requests("/app.delta"), requests("/app.tar.gz"))
	}
}

// runningApp stands in for the app that installUpdate updates on macOS,
// where the update waits for the app to exit: quit ends it and returns once
// the update is in place.
func runningApp(t *testing.T) (quit func()) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		return func() {}
	}
	app := exec.Command("/bin/sleep", "60")
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	old := appPid
	appPid = app.Process.Pid
	t.Cleanup(func() { appPid = old })
	return func() {
		t.Helper()
		app.Process.Kill()
		app.Wait()
		p := pendingUpdate.Swap(nil)
		if p == nil {
			t.Fatal("no update staged")
		}
		if err := p.helper.Wait(); err != nil {
			t.Fatal(err)
		}
	}
}

// On macOS the bundle of the running app stays in place until the app has
// exited: moving it away took the app's Dock tile with it, and the
// relaunched version got a second tile.
func TestUpdaterWaitsForTheAppToExit(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only macOS waits for the app to exit")
	}
	ctx := context.Background()
	srv, key, _, _ := updateServer(t, "1.2.0", "v2", "")
	up, err := checkUpdate(ctx, feedSource(srv.URL+"/update.json"), key, "1.1.9", false)
	if err != nil {
		t.Fatal(err)
	}
	target, main := installedApp(t)
	quit := runningApp(t)
	if err := installUpdate(ctx, httpGet, up.manifest, key, up.current, target, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if b, _ := os.ReadFile(main); string(b) != "v1" {
		t.Errorf("while the app runs main = %q", b)
	}
	quit()
	if b, _ := os.ReadFile(main); string(b) != "v2" {
		t.Errorf("after the app exited main = %q", b)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".*")); len(leftovers) > 0 {
		t.Errorf("left behind: %q", leftovers)
	}
}

func TestUpdaterWaitsForOpenFiles(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows keeps open files from being renamed")
	}
	ctx := context.Background()
	srv, key, _, _ := updateServer(t, "1.2.0", "v2", "")
	up, err := checkUpdate(ctx, feedSource(srv.URL+"/update.json"), key, "1.1.9", false)
	if err != nil {
		t.Fatal(err)
	}
	target, main := installedApp(t)
	// Open main as antivirus software does to scan it: Go opens files
	// without sharing their deletion, which renaming needs.
	f, err := os.Open(main)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	time.AfterFunc(500*time.Millisecond, func() { f.Close() })
	if err := installUpdate(ctx, httpGet, up.manifest, key, up.current, target, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(main); string(b) != "v2" {
		t.Errorf("after the update main = %q", b)
	}
}

func TestUpdaterPrereleases(t *testing.T) {
	t.Cleanup(func() { Updater.SetPrereleases(false) })
	if Updater.Prereleases() {
		t.Error("prereleases are on at launch")
	}
	// Check may run on another goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			Updater.Prereleases()
		}
	}()
	Updater.SetPrereleases(true)
	<-done
	if !Updater.Prereleases() {
		t.Error("SetPrereleases(true) did not turn them on")
	}
	Updater.SetPrereleases(false)
	if Updater.Prereleases() {
		t.Error("SetPrereleases(false) did not turn them off")
	}
}

// TestUpdaterBetaChannel checks the releases' feed and, with prereleases,
// the beta channel's of updates.prereleases too, offering the newer.
func TestUpdaterBetaChannel(t *testing.T) {
	ctx := context.Background()
	key := base64.StdEncoding.EncodeToString(make([]byte, ed25519.PublicKeySize))
	// The version of each channel's manifest, "" for none (404); a status
	// other than 200 fails it.
	var mu sync.Mutex
	versions := map[string]string{}
	status := map[string]int{}
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		asked = append(asked, r.URL.Path)
		if s := status[r.URL.Path]; s != 0 && s != http.StatusOK {
			http.Error(w, "down", s)
			return
		}
		v := versions[r.URL.Path]
		if v == "" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"version": %q, "url": "https://example.com/app-%s.tar.gz", "size": 1, "signature": "sig"}`, v, v)
	}))
	defer srv.Close()
	defer func(r []time.Duration) { httpRetries = r }(httpRetries)
	httpRetries = []time.Duration{time.Millisecond, time.Millisecond}
	feeds := channelFeeds{srv.URL + "/app/update.json", srv.URL + "/app/beta/update.json"}
	set := func(release, beta string, betaStatus int) {
		mu.Lock()
		versions["/app/update.json"], versions["/app/beta/update.json"] = release, beta
		status["/app/beta/update.json"] = betaStatus
		asked = nil
		mu.Unlock()
	}
	for _, c := range []struct {
		release, beta string
		betaStatus    int
		current       string
		pre           bool
		want          string // a version, "" for up to date, or "error: …"
	}{
		// A fix published after a beta of the next version.
		{"1.2.1", "1.3.0-beta.2", 200, "1.2.0", false, "1.2.1"},
		{"1.2.1", "1.3.0-beta.2", 200, "1.2.0", true, "1.3.0-beta.2"},
		{"1.2.1", "1.3.0-beta.2", 200, "1.3.0-beta.1", true, "1.3.0-beta.2"},
		{"1.2.1", "1.3.0-beta.2", 200, "1.3.0-beta.2", true, ""},
		// Never back to 1.2.1.
		{"1.2.1", "1.3.0-beta.2", 200, "1.3.0-beta.1", false, ""},
		// The newer of the two, whichever channel's manifest names it.
		{"1.3.0", "1.3.0-beta.2", 200, "1.3.0-beta.1", true, "1.3.0"},
		{"1.3.0", "1.3.0", 200, "1.3.0-beta.2", false, "1.3.0"},
		// The beta channel down or empty: the releases'.
		{"1.3.0", "", 200, "1.2.0", true, "1.3.0"},
		{"1.3.0", "1.3.0", 502, "1.2.0", true, "1.3.0"},
		// Only betas yet: the releases' feed has nothing published, which
		// is up to date without prereleases.
		{"", "0.1.0-beta.2", 200, "0.1.0-beta.1", true, "0.1.0-beta.2"},
		{"", "0.1.0-beta.2", 200, "0.1.0-beta.1", false, ""},
		{"", "", 200, "0.1.0-beta.1", true, ""},
		// Nothing published and the beta channel down: its error.
		{"", "", 502, "0.1.0-beta.1", true, "error: 502"},
	} {
		set(c.release, c.beta, c.betaStatus)
		up, err := checkUpdate(ctx, feeds, key, c.current, c.pre)
		got := ""
		switch {
		case err != nil:
			got = "error: " + err.Error()
		case up != nil:
			got = up.Version
		}
		if got != c.want && !(strings.HasPrefix(c.want, "error: ") && strings.HasPrefix(got, "error: ") && strings.Contains(got, strings.TrimPrefix(c.want, "error: "))) {
			t.Errorf("releases %q, beta %q (%d), from %s, prereleases %t: %s, want %s", c.release, c.beta, c.betaStatus, c.current, c.pre, got, c.want)
		}
		mu.Lock()
		if !c.pre && slices.ContainsFunc(asked, func(p string) bool { return strings.Contains(p, "/beta/") }) {
			t.Errorf("read the beta channel without prereleases: %v", asked)
		}
		mu.Unlock()
	}
}

func TestUpdaterConfigured(t *testing.T) {
	feed, key := packageUpdateFeed, packageUpdateKey
	defer func() { packageUpdateFeed, packageUpdateKey = feed, key }()
	packageUpdateFeed, packageUpdateKey = "", ""
	if Updater.Configured() {
		t.Error("configured without a feed")
	}
	packageUpdateFeed = "https://example.com/update.json"
	if Updater.Configured() {
		t.Error("configured without a key")
	}
	packageUpdateKey = "k"
	if !Updater.Configured() {
		t.Error("not configured with a feed and a key")
	}
}

// errSource fails with err.
type errSource struct{ err error }

func (s errSource) Manifest(context.Context, bool) (io.ReadCloser, error) { return nil, s.err }
func (s errSource) Open(context.Context, string) (io.ReadCloser, error)   { return nil, s.err }

// testSource serves the update of updateServer as an update service of the
// app's own could: its manifest names the files by ids that only Open
// knows, and Open reads them from srv with a client of its own.
type testSource struct {
	srv *httptest.Server
	m   update.Manifest
	// pre holds the prereleases argument of each call of Manifest, and
	// opened the URL of each call of Open.
	pre    []bool
	opened []string
}

func (s *testSource) Manifest(ctx context.Context, pre bool) (io.ReadCloser, error) {
	s.pre = append(s.pre, pre)
	b, err := json.Marshal(s.m)
	return io.NopCloser(bytes.NewReader(b)), err
}

func (s *testSource) Open(ctx context.Context, url string) (io.ReadCloser, error) {
	s.opened = append(s.opened, url)
	name, ok := strings.CutPrefix(url, "service:")
	if !ok {
		return nil, fmt.Errorf("%s is not a file of the service", url)
	}
	resp, err := s.srv.Client().Get(s.srv.URL + "/" + name)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func TestUpdaterSource(t *testing.T) {
	ctx := context.Background()
	srv, key, m, requests := updateServer(t, "1.2.0", "v2", "")
	// httpGet refuses this URL: only the source can download it.
	m.URL = "service:app.tar.gz"
	src := &testSource{srv: srv, m: m}
	up, err := checkUpdate(ctx, src, key, "1.1.9", true)
	if err != nil || up == nil || up.Version != "1.2.0" || up.Notes != "- Faster" {
		t.Fatalf("checkUpdate = %+v, %v", up, err)
	}
	if len(src.pre) != 1 || !src.pre[0] || requests("/update.json") != 0 {
		t.Errorf("Manifest got prereleases %v, and the feed was read %d times", src.pre, requests("/update.json"))
	}

	target, main := installedApp(t)
	quit := runningApp(t)
	if err := installUpdate(ctx, up.source.Open, up.manifest, key, up.current, target, nil); err != nil {
		t.Fatal(err)
	}
	quit()
	if b, _ := os.ReadFile(main); string(b) != "v2" {
		t.Errorf("after the update main = %q", b)
	}
	if len(src.opened) != 1 || src.opened[0] != "service:app.tar.gz" || requests("/app.tar.gz") != 1 {
		t.Errorf("opened %q, and the archive was downloaded %d times", src.opened, requests("/app.tar.gz"))
	}

	// A source's updates are checked as the feed's: Check refuses one that
	// is not signed, and Install one that is signed badly, which changes
	// nothing.
	src.m.Signature = ""
	if up, err := checkUpdate(ctx, src, key, "1.1.9", false); err == nil {
		t.Errorf("an unsigned update was offered: %+v", up)
	}
	src.m.Signature = m.Signature
	src.m.Release = base64.StdEncoding.EncodeToString(make([]byte, 64))
	up, err = checkUpdate(ctx, src, key, "1.1.9", false)
	if err != nil || up == nil {
		t.Fatalf("checkUpdate = %+v, %v", up, err)
	}
	target, main = installedApp(t)
	if err := installUpdate(ctx, up.source.Open, up.manifest, key, up.current, target, nil); err == nil || !strings.Contains(err.Error(), "signed") {
		t.Errorf("bad signature: %v", err)
	}
	if b, _ := os.ReadFile(main); string(b) != "v1" {
		t.Errorf("a refused update changed main to %q", b)
	}

	// SetSource sets the source that Check reads, and nil the feed again.
	t.Cleanup(func() { Updater.SetSource(nil) })
	Updater.SetSource(src)
	if s := updateSource.Load(); s == nil || *s != UpdateSource(src) {
		t.Error("SetSource did not set the source")
	}
	Updater.SetSource(nil)
	if updateSource.Load() != nil {
		t.Error("SetSource(nil) did not restore the feed")
	}
}

// A gateway failing for a moment is asked again, and an error that stays
// is returned.
func TestHTTPGetRetries(t *testing.T) {
	defer func(r []time.Duration) { httpRetries = r }(httpRetries)
	httpRetries = []time.Duration{time.Millisecond, time.Millisecond}
	var tries atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/missing":
			tries.Add(1)
			http.NotFound(w, r)
		case r.URL.Path == "/down":
			tries.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		case tries.Add(1) < 3:
			w.WriteHeader(http.StatusGatewayTimeout)
		default:
			io.WriteString(w, "ok")
		}
	}))
	defer srv.Close()
	body, err := httpGet(t.Context(), srv.URL+"/file")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(body)
	body.Close()
	if string(b) != "ok" || tries.Load() != 3 {
		t.Errorf("read %q in %d tries", b, tries.Load())
	}
	tries.Store(0)
	if _, err := httpGet(t.Context(), srv.URL+"/down"); err == nil || !strings.Contains(err.Error(), "503") || tries.Load() != 3 {
		t.Errorf("down: %v in %d tries", err, tries.Load())
	}
	tries.Store(0)
	if _, err := httpGet(t.Context(), srv.URL+"/missing"); err == nil || tries.Load() != 1 {
		t.Errorf("missing: %v in %d tries", err, tries.Load())
	}
	if got := retryAfter(" 5 "); got != 5*time.Second {
		t.Errorf("Retry-After 5: %v", got)
	}
}

// A download whose connection drops part way goes on with a Range request
// from where it stopped, through the transport of SetTransport; a server
// that answers the whole file again, or a file that changed, fails it.
func TestHTTPGetResumes(t *testing.T) {
	defer func(d time.Duration) { httpResumeWait = d }(httpResumeWait)
	httpResumeWait = time.Millisecond
	data := bytes.Repeat([]byte("0123456789abcdef"), 40_000)
	var drops, ranges atomic.Int32
	var ignoreRange atomic.Bool
	var ifRange atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		if rg := r.Header.Get("Range"); rg != "" && !ignoreRange.Load() {
			ranges.Add(1)
			ifRange.Store(r.Header.Get("If-Range"))
			from, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, len(data)-1, len(data)))
			w.Header().Set("Content-Length", strconv.Itoa(len(data)-from))
			w.WriteHeader(http.StatusPartialContent)
			end := len(data)
			if drops.Add(1) == 2 { // the first resume dies too, after some bytes
				end = from + 1000
			}
			w.Write(data[from:end])
			if end < len(data) {
				hijackClose(w)
			}
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		if r.Header.Get("Range") == "" && drops.Load() == 0 {
			drops.Add(1)
			w.Write(data[:len(data)/3]) // the first connection dies a third of the way
			hijackClose(w)
			return
		}
		w.Write(data)
	}))
	defer srv.Close()
	var through atomic.Int32
	Updater.SetTransport(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		through.Add(1)
		return http.DefaultTransport.RoundTrip(r)
	}))
	defer Updater.SetTransport(nil)

	pub, priv, _ := ed25519.GenerateKey(nil)
	sum := sha256.Sum256(data)
	path := filepath.Join(t.TempDir(), "app.tar.gz")
	var last int64
	err := download(t.Context(), httpGet, srv.URL+"/app.tar.gz", int64(len(data)), signed{update.SignRelease(priv, "1.2.0", "", sum[:]), "1.2.0", ""}, pub, path, func(n, _ int64) { last = n })
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, data) || last != int64(len(data)) {
		t.Fatalf("got %d bytes (progress %d), want %d", len(got), last, len(data))
	}
	if ranges.Load() != 2 || through.Load() != 3 || ifRange.Load() != `"v1"` {
		t.Errorf("%d Range requests (If-Range %v), %d requests through the transport", ranges.Load(), ifRange.Load(), through.Load())
	}

	// A server that sends the whole file again is not joined to the part.
	drops.Store(0)
	ignoreRange.Store(true)
	body, err := httpGet(t.Context(), srv.URL+"/app.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(body)
	body.Close()
	if err == nil || !strings.Contains(err.Error(), "not the bytes from") {
		t.Errorf("a server that does not resume: %v", err)
	}
}

// hijackClose ends the connection of a response under way.
func hijackClose(w http.ResponseWriter) {
	w.(http.Flusher).Flush()
	if c, _, err := w.(http.Hijacker).Hijack(); err == nil {
		c.Close()
	}
}

// SetTransport: the checks go through the app's transport, redirects
// still only to HTTPS, and back to the default once it is nil.
func TestUpdaterSetTransport(t *testing.T) {
	ctx := context.Background()
	srv, key, _, _ := updateServer(t, "1.3.0", "v2", "")
	var through atomic.Int32
	Updater.SetTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		through.Add(1)
		return http.DefaultTransport.RoundTrip(req)
	}))
	defer Updater.SetTransport(nil)
	if up, err := checkUpdate(ctx, feedSource(srv.URL+"/update.json"), key, "1.2.0", false); up == nil || err != nil {
		t.Fatalf("checkUpdate = %+v, %v", up, err)
	}
	if through.Load() == 0 {
		t.Error("the check did not go through the transport set")
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/update.json", http.StatusFound)
	}))
	defer redirect.Close()
	if _, err := checkUpdate(ctx, feedSource(redirect.URL+"/update.json"), key, "1.0.0", false); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("a redirect to plain HTTP through the transport set: %v", err)
	}
	Updater.SetTransport(nil)
	n := through.Load()
	if up, err := checkUpdate(ctx, feedSource(srv.URL+"/update.json"), key, "1.2.0", false); up == nil || err != nil {
		t.Fatalf("checkUpdate after SetTransport(nil) = %+v, %v", up, err)
	}
	if through.Load() != n {
		t.Error("SetTransport(nil) kept the transport")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// A server that accepts the connection and never answers is given up on,
// rather than holding a check open until the socket times out.
func TestHTTPGetHeaderTimeout(t *testing.T) {
	transport := httpClient().Transport.(*http.Transport)
	defer func(r []time.Duration, d time.Duration) {
		httpRetries, transport.ResponseHeaderTimeout = r, d
	}(httpRetries, transport.ResponseHeaderTimeout)
	httpRetries = []time.Duration{time.Millisecond}
	transport.ResponseHeaderTimeout = 50 * time.Millisecond

	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	start := time.Now()
	if _, err := httpGet(t.Context(), srv.URL+"/file"); err == nil {
		t.Fatal("a server that never answers returned no error")
	}
	if waited := time.Since(start); waited > 5*time.Second {
		t.Fatalf("waited %v for the headers of a response", waited)
	}
}

// A body that stops arriving is given up on, rather than blocking the read
// of a download until the socket times out.
func TestHTTPGetStallGivesUp(t *testing.T) {
	defer func(d time.Duration) { httpStallTimeout = d }(httpStallTimeout)
	httpStallTimeout = 100 * time.Millisecond

	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		io.WriteString(w, "start")
		w.(http.Flusher).Flush()
		<-block
	}))
	defer srv.Close()
	defer close(block)

	body, err := httpGet(t.Context(), srv.URL+"/file")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if _, err := io.ReadAll(body); err == nil || !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("reading a body that stopped ended with %v", err)
	}
}

// Even an immediately firing callback sees a fully initialized body.
func TestStalledBodyShortTimeout(t *testing.T) {
	defer func(d time.Duration) { httpStallTimeout = d }(httpStallTimeout)
	httpStallTimeout = time.Nanosecond
	for range 100 {
		ctx, cancel := context.WithCancel(t.Context())
		body := newStalledBody(io.NopCloser(strings.NewReader("")), cancel)
		wait.For(t, "the stalled body to cancel its request", ctx.Done())
		body.Close()
		if !body.stalled.Load() {
			t.Fatal("the body did not report a stall")
		}
	}
}

// A body that keeps arriving is not given up, however slowly it sends.
func TestHTTPGetKeepsASlowBody(t *testing.T) {
	defer func(d time.Duration) { httpStallTimeout = d }(httpStallTimeout)
	// Each byte comes well within the stall timeout, even on a loaded
	// runner whose sleeps overrun, and all of them take longer than it.
	httpStallTimeout = 400 * time.Millisecond

	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 10; i++ {
			io.WriteString(w, "x")
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
		<-block
	}))
	defer srv.Close()
	defer close(block)

	body, err := httpGet(t.Context(), srv.URL+"/file")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	b, err := io.ReadAll(io.LimitReader(body, 10))
	if err != nil || len(b) != 10 {
		t.Fatalf("read %d bytes of a slow body: %v", len(b), err)
	}
}

// TestWorkPattern checks that an app at the root of a drive or share, as
// D:\, unpacks its update next to it (#1529).
func TestWorkPattern(t *testing.T) {
	roots := []string{string(filepath.Separator), "."}
	if runtime.GOOS == "windows" {
		roots = append(roots, `D:\`, `\\server\share\`)
	}
	for _, target := range roots {
		pattern := workPattern(target)
		if strings.ContainsFunc(pattern, func(r rune) bool { return r < 0x80 && os.IsPathSeparator(uint8(r)) }) {
			t.Errorf("workPattern(%q) = %q, which holds a separator", target, pattern)
		}
	}
	if got, want := workPattern(filepath.Join(t.TempDir(), "MyApp")), ".MyApp.update-"; got != want {
		t.Errorf("workPattern of an app in a folder = %q, want %q", got, want)
	}
	// The pattern makes a directory next to a root.
	dir, err := os.MkdirTemp(t.TempDir(), workPattern(string(filepath.Separator)))
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(dir)
}

// TestRelaunchArguments: Relaunch's arguments replace those the app
// started with, and reach the new instance however it is started: by
// itself, through Launch Services, after a staged update on macOS, and
// under mygo dev.
func TestRelaunchArguments(t *testing.T) {
	args := []string{"--reason", "it's an update", "a b", `$HOME "quoted" \ back`, "line\nbreak", ""}
	onMain(func() {
		if got := relaunchArguments(); !slices.Equal(got, os.Args[1:]) {
			t.Errorf("without arguments: %q, want the app's %q", got, os.Args[1:])
		}
		App.relaunchArgs = args
		defer func() { App.relaunchArgs = nil }()
		if got := relaunchArguments(); !slices.Equal(got, args) {
			t.Errorf("with arguments: %q", got)
		}
	})

	if got := openArgs("/A.app", nil); !slices.Equal(got, []string{"/A.app"}) {
		t.Errorf("open without arguments: %q", got)
	}
	if got := openArgs("/A.app", args); !slices.Equal(got, append([]string{"/A.app", "--args"}, args...)) {
		t.Errorf("open with arguments: %q", got)
	}

	dev := strings.Split(string(devRelaunchData(args)), "\x00")
	if !slices.Equal(dev[:len(dev)-1], args) {
		t.Errorf("for mygo dev: %q", dev)
	}

	if runtime.GOOS == "windows" {
		return
	}
	// The script that installs a staged update opens the app with the
	// arguments of open that the relaunch file sets, as Relaunch wrote
	// them once the update was staged. It prints them here.
	dir := t.TempDir()
	target, bundle, work := filepath.Join(dir, "A.app"), filepath.Join(dir, "new", "A.app"), filepath.Join(dir, "work")
	for _, d := range []string{target, bundle, work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(work, "relaunch"), []byte(setArgs(openArgs(target, args))), 0o644); err != nil {
		t.Fatal(err)
	}
	exited := exec.Command("/bin/sh", "-c", "exit 0")
	if err := exited.Run(); err != nil {
		t.Fatal(err)
	}
	script := strings.Replace(installAfterExit, "exec /usr/bin/open", `printf '%s\0'`, 1)
	out, err := exec.Command("/bin/sh", "-c", script, "mygo-update", strconv.Itoa(exited.Process.Pid), target, bundle, work).Output()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(string(out), "\x00")
	if want := openArgs(target, args); !slices.Equal(got[:len(got)-1], want) {
		t.Errorf("the update opened the app with %q, want %q", got, want)
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Error("the update left its work")
	}
}
