//go:build linux && !android && (amd64 || arm64)

package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// threads returns how many threads the process has.
func threads(t *testing.T) int {
	t.Helper()
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Skip(err)
	}
	for l := range strings.SplitSeq(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "Threads:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
	}
	t.Skip("no thread count")
	return 0
}

// TestRecordersKeepThreadsFlat: a Recorder made for each still, as
// plugins/render's Still makes one, takes the text engine the one before
// released as it closed. Pango's font map starts a thread of its own,
// which a new engine for each grew by one a still, with fontconfig's set
// of the app's fonts, until loading a font took minutes.
func TestRecordersKeepThreadsFlat(t *testing.T) {
	still := func(i int) {
		r := coreNewRecorder(func(c *context) {
			coreColumn(c).Children(func() {
				coreText(c, fmt.Sprintf("Still %d 字形", i)).FontSize(17)
			})
		}, RecorderOptions{Width: 200, Height: 80})
		r.Frame(0)
		r.Close()
	}
	for i := range 5 {
		still(i)
	}
	before := threads(t)
	const stills = 60
	for i := range stills {
		still(i)
	}
	if grew := threads(t) - before; grew > stills/3 {
		t.Errorf("%d stills grew %d threads", stills, grew)
	}
}
