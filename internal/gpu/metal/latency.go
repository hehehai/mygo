//go:build darwin

package metal

import (
	"os"
	"strconv"

	"github.com/egoist/mygo/internal/pacing"
)

// Latency experiments (Onda M2 WP4: an edit shows on the display about
// 3.5 refreshes after its frame presents while a video plays, the
// drawables queued behind each other; docs/v2/m2-perf.md in Onda). Unset,
// nothing changes:
//
//	MYGO_METAL_DRAWABLES=2      the layer's maximumDrawableCount (2 or 3;
//	                            default 3): one frame less queued
//	MYGO_METAL_NO_PREFETCH=1    no drawable fetched ahead after a present:
//	                            each frame takes its own at its refresh
//	MYGO_METAL_DISPLAY_SYNC=0   displaySyncEnabled off with frames still
//	                            paced by the display link: a drawable goes
//	                            to the window server as soon as drawn, which
//	                            composites the newest at each refresh
var (
	latencyDrawables  = envInt("MYGO_METAL_DRAWABLES", 3, 2, 3)
	latencyNoPrefetch = os.Getenv("MYGO_METAL_NO_PREFETCH") == "1"
	latencyNoSync     = os.Getenv("MYGO_METAL_DISPLAY_SYNC") == "0"
)

func envInt(name string, def, lo, hi int) int {
	n, err := strconv.Atoi(os.Getenv(name))
	if err != nil || n < lo || n > hi {
		return def
	}
	return n
}

// followLowLatency turns the layer's display sync off while
// pacing.LowLatency is set (ui.SetLowLatency), on again after: the app
// chooses, as a video editor wants its edits shown at once while paused
// and its playback even.
func (r *Renderer) followLowLatency() {
	if pacing.Uncapped || latencyNoSync {
		return // always off
	}
	want := pacing.LowLatency.Load()
	if want == r.lowLatency || !respondsTo(r.layer, "setDisplaySyncEnabled:") {
		return
	}
	on := uintptr(1)
	if want {
		on = 0
	}
	send(r.layer, "setDisplaySyncEnabled:", on)
	r.lowLatency = want
}
