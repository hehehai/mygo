//go:build darwin && !ios

package metal

import (
	"testing"

	"github.com/egoist/mygo/internal/pacing"
	"github.com/egoist/mygo/internal/scene"
)

// TestLowLatency renders frames into a layer while pacing.LowLatency
// (ui.SetLowLatency) changes: the layer's displaySyncEnabled is on by
// default, as upstream leaves it, turns off at the first frame after
// LowLatency is set, on again at the first after it is cleared, back and
// forth, and a frame that finds it unchanged sends nothing.
func TestLowLatency(t *testing.T) {
	if err := load(); err != nil {
		t.Skip("no Metal:", err)
	}
	if pacing.Uncapped || latencyNoSync {
		t.Skip("display sync off for the whole run (MYGO_UNCAPPED, MYGO_METAL_DISPLAY_SYNC)")
	}
	var layer id
	pool(func() { layer = send(class("CALayer"), "new") })
	if layer == 0 {
		t.Skip("no Core Animation")
	}
	defer pool(func() { release(&layer) })
	r, err := New(uintptr(layer))
	if err != nil {
		unavailable(t, err)
	}
	defer r.Release()
	if !respondsTo(r.layer, "displaySyncEnabled") {
		t.Skip("CAMetalLayer has no displaySyncEnabled")
	}
	defer pacing.LowLatency.Store(false)
	synced := func() (on bool) {
		pool(func() { on = send(r.layer, "displaySyncEnabled")&0xff != 0 })
		return on
	}
	if !synced() {
		t.Fatal("a new layer's display sync is off")
	}
	if n := send(r.layer, "maximumDrawableCount"); latencyDrawables == 3 && n != 3 {
		t.Errorf("maximumDrawableCount %d, want 3 with MYGO_METAL_DRAWABLES unset", n)
	}
	i := 0
	frame := func() {
		t.Helper()
		i++
		r.pre.wait()
		s := &scene.Scene{}
		s.Reset(64, 32, scene.Color{A: 255})
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: float32(i % 50), W: 8, H: 8}, Color: scene.Color{R: 255, A: 255}})
		if err := r.Render(s); err != nil {
			t.Fatal(err)
		}
	}
	frame()
	if !synced() || r.lowLatency {
		t.Fatalf("the first frame: display sync %v, low latency %v", synced(), r.lowLatency)
	}
	for round, on := range []bool{true, true, false, true, false, false, true, false} {
		pacing.LowLatency.Store(on)
		frame()
		if synced() == on || r.lowLatency != on {
			t.Fatalf("round %d, LowLatency %v: display sync %v, renderer's lowLatency %v", round, on, synced(), r.lowLatency)
		}
		frame() // a frame after, LowLatency unchanged
		if synced() == on {
			t.Fatalf("round %d, LowLatency %v: display sync %v on the frame after", round, on, synced())
		}
	}
	// A layer whose display sync is off, the app having set it, sees it
	// changed elsewhere only at its next frame: none came, so none is sent.
	pacing.LowLatency.Store(true)
	if !synced() {
		t.Fatal("display sync changed with no frame drawn")
	}
	frame()
	if synced() {
		t.Fatal("display sync on after the frame")
	}
}

// TestLatencyEnv checks the diagnostics' parsing: unset or out of range,
// MYGO_METAL_DRAWABLES keeps the default.
func TestLatencyEnv(t *testing.T) {
	for v, want := range map[string]int{"": 3, "2": 2, "3": 3, "1": 3, "4": 3, "x": 3} {
		t.Setenv("MYGO_TEST_DRAWABLES", v)
		if got := envInt("MYGO_TEST_DRAWABLES", 3, 2, 3); got != want {
			t.Errorf("%q: %d, want %d", v, got, want)
		}
	}
}
