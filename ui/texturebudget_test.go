package ui

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/surface"
)

// budgetGPU is a GPU renderer that takes a texture budget and leaves
// images out while defer_ is set.
type budgetGPU struct {
	testGPU
	bytes, keep int64
	uploads     int
	set         int
	defer_      bool
}

func (g *budgetGPU) SetImageBudget(bytes int64, uploads int) {
	g.bytes, g.uploads, g.set = bytes, uploads, g.set+1
}
func (g *budgetGPU) SetIdleKeep(bytes int64) { g.keep = bytes }
func (g *budgetGPU) Deferred() bool          { return g.defer_ }

// countingSurface counts the frames asked for.
type countingSurface struct {
	testSurface
	requests int
}

func (s *countingSurface) RequestFrame() { s.requests++ }

// TestTextureBudget: the window hands the budget to its renderer, and a
// frame that left images out asks for another, which paints the last
// frame's elements again without building the view.
func TestTextureBudget(t *testing.T) {
	t.Cleanup(func() { SetTextureBudget(TextureBudget{}) })
	if (CurrentTextureBudget() != TextureBudget{}) {
		t.Fatal("a budget by default")
	}
	g := &budgetGPU{}
	t.Setenv("MYGO_GPU", "")
	newGPU = func(platform.SurfaceNative) (gpuRenderer, error) { return g, nil }
	t.Cleanup(func() { newGPU = newGPURenderer })
	s := &countingSurface{}
	h := &windowHost{conn: &surface.Conn{Surface: s}}
	builds := 0
	h.rt = newRuntime(func(c *context) { builds++; coreText(c, "Hello") }, h)
	frame := func() { h.event(platform.SurfaceEvent{Kind: platform.SurfaceFrame}) }

	// Unset, the renderer draws without bounds, as before.
	frame()
	if g.frames != 1 || g.set != 1 || g.bytes != 0 || g.uploads != 0 || g.keep != 0 || s.requests != 0 {
		t.Fatalf("no budget: %+v, %d requests", g, s.requests)
	}
	SetTextureBudget(TextureBudget{Bytes: 128 << 20, UploadsPerFrame: 4, IdleKeep: 32 << 20})
	frame()
	if g.bytes != 128<<20 || g.uploads != 4 || g.keep != 32<<20 || s.requests != 0 {
		t.Fatalf("budget: %+v", g)
	}
	// A frame that left images out asks for another, painting again.
	g.defer_ = true
	frame()
	if s.requests != 1 || !h.rt.lookDue {
		t.Fatalf("deferred images: %d requests, look due %v", s.requests, h.rt.lookDue)
	}
	g.defer_ = false
	before := builds
	frame()
	if s.requests != 1 || builds != before || g.frames != 4 {
		t.Fatalf("after: %d requests, %d builds, %d frames", s.requests, builds-before, g.frames)
	}
	SetTextureBudget(TextureBudget{Bytes: -1, UploadsPerFrame: -3, IdleKeep: -2})
	if (CurrentTextureBudget() != TextureBudget{}) {
		t.Errorf("negative budget: %+v", CurrentTextureBudget())
	}
}
