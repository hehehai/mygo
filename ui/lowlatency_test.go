package ui

import (
	"testing"

	"github.com/egoist/mygo/internal/pacing"
	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/surface"
)

// TestSetLowLatency checks that SetLowLatency sets and clears
// pacing.LowLatency, which the Metal renderer follows (TestLowLatency in
// internal/gpu/metal checks the layer), and that it changes nothing of
// how the window host paces frames: unlike MYGO_UNCAPPED, a paced frame
// drawn in memory still waits for the display's refresh, as in
// TestFramesInMemoryWaitForTheRefresh, so that elsewhere than macOS it
// is a no-op.
func TestSetLowLatency(t *testing.T) {
	t.Cleanup(func() { SetLowLatency(false) })
	if pacing.LowLatency.Load() {
		t.Fatal("low latency by default")
	}
	for _, on := range []bool{true, false, true, true, false} {
		SetLowLatency(on)
		if got := pacing.LowLatency.Load(); got != on {
			t.Fatalf("SetLowLatency(%v): pacing.LowLatency %v", on, got)
		}
	}

	SetLowLatency(true)
	t.Setenv("MYGO_GPU", "0")
	s := &refreshSurface{refresh: make(chan struct{})}
	t.Cleanup(func() { close(s.refresh) })
	h := &windowHost{conn: &surface.Conn{Surface: s}}
	h.rt = newRuntime(func(c *context) { coreText(c, "Hello") }, h)
	s.h = h
	h.conn.Post = func(func()) {}
	h.event(platform.SurfaceEvent{Kind: platform.SurfaceFrame})
	h.event(platform.SurfaceEvent{Kind: platform.SurfaceFrame, Paced: true})
	if s.pixels != 1 || !h.waiting || h.framesAtOnce() {
		t.Fatalf("low latency: %d frames, waiting %v, frames at once %v; want a paced frame to wait for the refresh", s.pixels, h.waiting, h.framesAtOnce())
	}
}
