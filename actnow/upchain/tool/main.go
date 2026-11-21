// Command upchain is the helper of the update chain check (../README.md):
// a local HTTPS update feed laid out as actnow.dev's, the site's publishing
// and manifest checks, and the faults the old clients are tried with.
//
//	upchain serve -root DIR -ca CA.pem -port PORT.txt -log REQ.log -mode MODE
//	upchain publish -feed DIR/updates/APP FILE...
//	upchain sitecheck -site URL -app APP -version V -pub KEY -dir BUILD [-release]
//
// It uses the standard library only, and is not part of MyGo (its own
// module).
package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("upchain: ")
	if len(os.Args) < 2 {
		log.Fatal("usage: upchain serve|publish|sitecheck …")
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:])
	case "publish":
		err = publish(os.Args[2:])
	case "sitecheck":
		err = sitecheck(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

// serve serves root over HTTPS on 127.0.0.1 with a certificate of a CA it
// makes (written to -ca, which the probes and build tools trust; nothing
// goes in a keychain), with Range requests, logging each request. The mode
// file says what fault to serve (one word, read at each request):
//
//	normal     the files as they are
//	nodelta    manifests without their deltas (the archive is downloaded)
//	baddelta   deltas with a byte changed (the delta fails its signature;
//	           the client falls back to the archive)
//	norelease  manifests without their release signatures
//	relabel    manifests that say 99.0.0 (a signed archive offered as
//	           another version)
func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	root := fs.String("root", "", "directory served")
	caOut := fs.String("ca", "", "where to write the CA certificate (PEM)")
	portOut := fs.String("port", "", "where to write the port")
	logPath := fs.String("log", "", "request log")
	modePath := fs.String("mode", "", "mode file")
	fs.Parse(args)
	cert, caPEM, err := makeCert()
	if err != nil {
		return err
	}
	if err := os.WriteFile(*caOut, caPEM, 0o644); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*portOut, []byte(strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)+"\n"), 0o644); err != nil {
		return err
	}
	logf, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	var mu sync.Mutex
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := "normal"
		if b, err := os.ReadFile(*modePath); err == nil && strings.TrimSpace(string(b)) != "" {
			mode = strings.TrimSpace(string(b))
		}
		rec := &statusWriter{ResponseWriter: w, code: 200}
		serveFile(rec, r, *root, mode)
		mu.Lock()
		fmt.Fprintf(logf, "%s %s %d %d %s range=%q\n", r.Method, r.URL.Path, rec.code, rec.n, mode, r.Header.Get("Range"))
		mu.Unlock()
	})
	srv := &http.Server{Handler: h, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}, ReadHeaderTimeout: 30 * time.Second}
	return srv.ServeTLS(ln, "", "")
}

type statusWriter struct {
	http.ResponseWriter
	code int
	n    int64
}

func (w *statusWriter) WriteHeader(code int) { w.code = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.n += int64(n)
	return n, err
}

func serveFile(w http.ResponseWriter, r *http.Request, root, mode string) {
	p := path.Clean("/" + r.URL.Path)
	file := filepath.Join(root, filepath.FromSlash(p))
	data, err := os.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name := path.Base(p)
	switch {
	case strings.HasPrefix(name, "update-") && strings.HasSuffix(name, ".json"):
		if data, err = fault(data, mode); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
	case strings.HasSuffix(name, ".delta") && mode == "baddelta" && len(data) > 100:
		data = bytes.Clone(data)
		data[len(data)/2] ^= 0xff
	}
	st, _ := os.Stat(file)
	http.ServeContent(w, r, name, st.ModTime(), bytes.NewReader(data))
}

// fault changes a manifest as mode says.
func fault(data []byte, mode string) ([]byte, error) {
	if mode == "normal" || mode == "baddelta" {
		return data, nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	switch mode {
	case "nodelta":
		delete(m, "deltas")
	case "norelease":
		delete(m, "release")
		if ds, ok := m["deltas"].([]any); ok {
			for _, d := range ds {
				delete(d.(map[string]any), "release")
			}
		}
	case "relabel":
		m["version"] = "99.0.0"
	default:
		return nil, fmt.Errorf("unknown mode %q", mode)
	}
	return json.MarshalIndent(m, "", "  ")
}

// makeCert makes a CA and a certificate of 127.0.0.1 and localhost signed
// by it, for one run.
func makeCert() (tls.Certificate, []byte, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	now := time.Now()
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "upchain test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(48 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	ca, _ = x509.ParseCertificate(caDER)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(48 * time.Hour),
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, DNSNames: []string{"localhost"},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key},
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), nil
}

type manifest struct {
	Version   string  `json:"version"`
	URL       string  `json:"url"`
	Size      int64   `json:"size"`
	Signature string  `json:"signature"`
	Release   string  `json:"release"`
	Deltas    []delta `json:"deltas"`
}

type delta struct {
	From      string `json:"from"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
	Release   string `json:"release"`
}

// publish puts the files of a build in the feed directory of an app as
// actnow.dev publishes a release: a prerelease's in beta/; a release's in
// the app's directory, and its manifests in beta/ too unless a newer
// version is published there (the beta channel's manifest is the newest
// version of both channels).
func publish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	feed := fs.String("feed", "", "the app's feed directory (…/updates/<app>)")
	fs.Parse(args)
	var version string
	var manifests []string
	for _, f := range fs.Args() {
		if strings.HasPrefix(filepath.Base(f), "update-") {
			manifests = append(manifests, f)
			var m manifest
			b, err := os.ReadFile(f)
			if err != nil || json.Unmarshal(b, &m) != nil {
				return fmt.Errorf("reading %s: %v", f, err)
			}
			version = m.Version
		}
	}
	if version == "" {
		return errors.New("no manifest among the files")
	}
	dir := *feed
	beta := isPrerelease(version)
	if beta {
		dir = filepath.Join(dir, "beta")
	}
	if err := os.MkdirAll(filepath.Join(*feed, "beta"), 0o755); err != nil {
		return err
	}
	// The files first, the manifests last.
	for _, f := range append(slicesWithout(fs.Args(), manifests), manifests...) {
		if err := copyFile(f, filepath.Join(dir, filepath.Base(f))); err != nil {
			return err
		}
	}
	if !beta {
		for _, f := range manifests {
			to := filepath.Join(*feed, "beta", filepath.Base(f))
			var old manifest
			if b, err := os.ReadFile(to); err == nil && json.Unmarshal(b, &old) == nil && compare(old.Version, version) > 0 {
				continue
			}
			if err := copyFile(f, to); err != nil {
				return err
			}
		}
	}
	fmt.Printf("published %s in %s\n", version, dir)
	return nil
}

func slicesWithout(all, drop []string) []string {
	var out []string
	for _, a := range all {
		keep := true
		for _, d := range drop {
			keep = keep && a != d
		}
		if keep {
			out = append(out, a)
		}
	}
	return out
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}

// sitecheck checks the manifests of a build in dir as actnow.dev's
// release pipeline does (packages/api/src/services/releases/pipeline.ts,
// checkManifest and checkArtifactName): the version, the archive's URL in
// the channel's directory of the site and its name, size and signature,
// each delta's URL, name, origin, size and signature; with -release also
// the release signatures MyGo's fork writes (and Refto's and Dicta's
// published versions require).
func sitecheck(args []string) error {
	fs := flag.NewFlagSet("sitecheck", flag.ExitOnError)
	site := fs.String("site", "", "the site, https://host[:port]")
	app := fs.String("app", "", "the app's feed name (pik)")
	version := fs.String("version", "", "the version built")
	pub := fs.String("pub", "", "the update public key (base64)")
	dir := fs.String("dir", "", "the build's output directory of the target")
	release := fs.Bool("release", false, "require release signatures")
	fs.Parse(args)
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(*pub))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("bad -pub")
	}
	channelDir := *app
	if isPrerelease(*version) {
		channelDir += "/beta"
	}
	files, _ := filepath.Glob(filepath.Join(*dir, "update-*.json"))
	if len(files) == 0 {
		return fmt.Errorf("no manifest in %s", *dir)
	}
	var problems []string
	bad := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	for _, f := range files {
		name := filepath.Base(f)
		platform := strings.TrimSuffix(strings.TrimPrefix(name, "update-"), ".json")
		var m manifest
		b, _ := os.ReadFile(f)
		if err := json.Unmarshal(b, &m); err != nil {
			bad("%s: not JSON", name)
			continue
		}
		if m.Version != *version {
			bad("%s: version %s is not %s", name, m.Version, *version)
		}
		v := regexp.QuoteMeta(*version)
		archiveName, ok := siteFile(*site, channelDir, m.URL)
		switch {
		case !ok:
			bad("%s: url %s is not %s/updates/%s/<archive>", name, m.URL, *site, channelDir)
		case !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,199}$`).MatchString(archiveName) ||
			!regexp.MustCompile(`-`+v+`-`+regexp.QuoteMeta(platform)+`\.(tar\.gz|tgz|zip)$`).MatchString(archiveName):
			bad("%s: archive %s is not named <name>-%s-%s.tar.gz", name, archiveName, *version, platform)
		default:
			sum, size, err := digest(filepath.Join(*dir, archiveName))
			switch {
			case err != nil:
				bad("%s: %v", name, err)
			case size != m.Size:
				bad("%s: size %d is not the archive's %d", name, m.Size, size)
			case !verify(key, sum, m.Signature):
				bad("%s: %s is not signed with the update key", name, archiveName)
			case *release && !verify(key, releaseMessage(*version, "", sum), m.Release):
				bad("%s: no valid release signature", name)
			}
		}
		for _, d := range m.Deltas {
			deltaName, ok := siteFile(*site, channelDir, d.URL)
			if !ok {
				bad("%s: delta %s is not a file of this release's channel", name, d.URL)
				continue
			}
			mm := regexp.MustCompile(`-(.+)-to-` + v + `-` + regexp.QuoteMeta(platform) + `\.delta$`).FindStringSubmatch(deltaName)
			if mm == nil || !strings.Contains(deltaName, "-"+d.From+"-to-") || compare(d.From, *version) >= 0 {
				bad("%s: delta %s is not named <name>-%s-to-%s-%s.delta from an older version", name, deltaName, d.From, *version, platform)
				continue
			}
			sum, size, err := digest(filepath.Join(*dir, deltaName))
			switch {
			case err != nil:
				bad("%s: %v", name, err)
			case size != d.Size:
				bad("%s: delta %s size %d is not %d", name, deltaName, d.Size, size)
			case !verify(key, sum, d.Signature):
				bad("%s: delta %s has a bad signature", name, deltaName)
			case *release && !verify(key, releaseMessage(*version, d.From, sum), d.Release):
				bad("%s: delta %s has no valid release signature", name, deltaName)
			}
		}
		fmt.Printf("sitecheck %s: version %s, archive %s, %d deltas\n", name, m.Version, path.Base(m.URL), len(m.Deltas))
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// siteFile returns the file name of u when it is a file of the channel's
// directory of the site.
func siteFile(site, channelDir, u string) (string, bool) {
	p, err := url.Parse(u)
	s, err2 := url.Parse(site)
	if err != nil || err2 != nil || p.Scheme+"://"+p.Host != s.Scheme+"://"+s.Host {
		return "", false
	}
	rest, ok := strings.CutPrefix(p.Path, "/updates/"+channelDir+"/")
	return rest, ok && rest != "" && !strings.Contains(rest, "/")
}

func digest(file string) ([]byte, int64, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return h.Sum(nil), n, err
}

func verify(key ed25519.PublicKey, msg []byte, sig string) bool {
	b, err := base64.StdEncoding.DecodeString(sig)
	return err == nil && ed25519.Verify(key, msg, b)
}

// releaseMessage is MyGo's update.ReleaseMessage.
func releaseMessage(version, from string, digest []byte) []byte {
	channel := "stable"
	if isPrerelease(version) {
		channel = "beta"
	}
	return fmt.Appendf(nil, "mygo-update-release\nversion=%s\nchannel=%s\nfrom=%s\nsha256=%x\n",
		strings.TrimPrefix(version, "v"), channel, strings.TrimPrefix(from, "v"), digest)
}

func isPrerelease(v string) bool {
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "+")
	return strings.Contains(v, "-")
}

// compare is MyGo's update.Compare.
func compare(a, b string) int {
	a, b = strings.TrimPrefix(a, "v"), strings.TrimPrefix(b, "v")
	a, _, _ = strings.Cut(a, "+")
	b, _, _ = strings.Cut(b, "+")
	ar, apre, _ := strings.Cut(a, "-")
	br, bpre, _ := strings.Cut(b, "-")
	if c := fields(ar, br); c != 0 {
		return c
	}
	switch {
	case apre == bpre:
		return 0
	case apre == "":
		return 1
	case bpre == "":
		return -1
	}
	return fields(apre, bpre)
}

func fields(a, b string) int {
	af, bf := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(af) || i < len(bf); i++ {
		var x, y string
		if i < len(af) {
			x = af[i]
		}
		if i < len(bf) {
			y = bf[i]
		}
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case x == y:
			continue
		case x == "":
			return -1
		case y == "":
			return 1
		case xerr == nil && yerr == nil:
			if xn != yn {
				if xn < yn {
					return -1
				}
				return 1
			}
		case xerr == nil:
			return -1
		case yerr == nil:
			return 1
		default:
			return strings.Compare(x, y)
		}
	}
	return 0
}
