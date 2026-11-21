//go:build linux || darwin

package mygo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// fileWatchTestOpenUnder lists the files the process holds open in dir or
// under it (Linux: /proc/self/fd; macOS: lsof).
func fileWatchTestOpenUnder(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	if runtime.GOOS == "linux" {
		fds, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Skip(err)
		}
		for _, fd := range fds {
			if p, err := os.Readlink(filepath.Join("/proc/self/fd", fd.Name())); err == nil {
				paths = append(paths, p)
			}
		}
	} else {
		out, err := exec.Command("lsof", "-n", "-P", "-Fn", "-p", strconv.Itoa(os.Getpid())).Output()
		if err != nil {
			t.Skip(err)
		}
		for _, l := range strings.Split(string(out), "\n") {
			if p, ok := strings.CutPrefix(l, "n"); ok {
				paths = append(paths, p)
			}
		}
	}
	var under []string
	for _, p := range paths {
		if p == dir || strings.HasPrefix(p, dir+"/") {
			under = append(under, p)
		}
	}
	return under
}

// A watch does not hold its directory open between two looks (a disk with
// a watched folder could not be ejected), yet still reports what changed,
// and still ends when its directory is replaced meanwhile.
func TestFileWatchHoldsNoDirectory(t *testing.T) {
	for _, hints := range []bool{false, true} {
		t.Run("hints="+strconv.FormatBool(hints), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "root")
			if err := os.MkdirAll(filepath.Join(dir, "a"), 0700); err != nil {
				t.Fatal(err)
			}
			w, err := WatchFiles(context.Background(), dir, FileWatchOptions{Recursive: true, Debounce: -1, HintsOnly: hints})
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			if open := fileWatchTestOpenUnder(t, w.root); len(open) > 0 {
				t.Fatalf("held open after the watch started: %v", open)
			}
			if err := os.WriteFile(filepath.Join(dir, "a", "f"), []byte("x"), 0600); err != nil {
				t.Fatal(err)
			}
			fileWatchTestNotice(t, w, "a", platform.FileWatchNotice{Name: "f"})
			want := FileEvent{Op: FileCreate, Path: "a/f"}
			if hints {
				want = FileEvent{Op: FileHint, Path: "a", IsDir: true}
			}
			if e := fileWatchTestNext(t, w); e != want {
				t.Fatalf("event %+v, want %+v", e, want)
			}
			if open := fileWatchTestOpenUnder(t, w.root); len(open) > 0 {
				t.Fatalf("held open after a look: %v", open)
			}
			// Replaced between two looks: the watch ends.
			if err := os.Rename(dir, dir+"-old"); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, "a"), 0700); err != nil {
				t.Fatal(err)
			}
			fileWatchTestNotice(t, w, "a", platform.FileWatchNotice{Name: "g"})
			ctx := context.Background()
			for {
				batch, err := w.Next(ctx)
				if err != nil {
					break // ended
				}
				for _, e := range batch {
					if e.Path != "." || e.Op != FileRemove {
						t.Fatalf("after the replacement: %+v", batch)
					}
				}
			}
		})
	}
}
