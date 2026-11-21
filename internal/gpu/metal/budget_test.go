//go:build darwin && !ios

package metal

import (
	"bytes"
	"image"
	"testing"

	"github.com/egoist/mygo/internal/scene"
)

// imagesScene draws n images of 64×64 pixels of their own.
func imagesScene(n int, seed byte) *scene.Scene {
	s := &scene.Scene{Width: 640, Height: 128, Clear: scene.Color{A: 255}}
	for i := range n {
		pix := make([]byte, 64*64*4)
		for j := range pix {
			pix[j] = seed + byte(i)
		}
		img := scene.NewImageRGBA(64, 64, pix)
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpImage, Rect: scene.Rect{X: float32(i%10) * 64, Y: float32(i/10) * 64, W: 64, H: 64},
			Image: img, Src: scene.Rect{W: 64, H: 64}})
	}
	return s
}

// TestImageBudget: past UploadsPerFrame a frame leaves the new images out
// and says so; past Bytes, the textures of images not drawn lately go,
// the oldest first, never those of the last frames. Unset, nothing is
// bounded.
func TestImageBudget(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		unavailable(t, err)
	}
	defer r.Release()
	const one = 64 * 64 * 4
	a := imagesScene(10, 10)

	// No budget: one frame uploads all ten.
	renderRaw(t, r, a, false)
	if len(r.images) != 10 || r.Deferred() || r.ImageBytes() != 10*one {
		t.Fatalf("no budget: %d images, deferred %v, %d bytes", len(r.images), r.Deferred(), r.ImageBytes())
	}
	pool(r.releaseTextures)
	if r.ImageBytes() != 0 {
		t.Fatal("released textures still counted")
	}

	// Three uploads a frame: four frames to show them all.
	r.SetImageBudget(0, 3)
	for i, want := range []int{3, 6, 9, 10} {
		renderRaw(t, r, a, false)
		if len(r.images) != want || r.Deferred() != (want < 10) {
			t.Fatalf("frame %d: %d images, deferred %v", i+1, len(r.images), r.Deferred())
		}
	}

	// A budget of five images: once another ten have been drawn for more
	// than three frames, the first ten go; the new ten stay, being recent.
	r.SetImageBudget(5*one, 0)
	b := imagesScene(10, 100)
	for range 5 {
		renderRaw(t, r, b, false)
		pool(r.trimImages)
	}
	if len(r.images) != 10 || r.ImageBytes() != 10*one {
		t.Fatalf("after trimming: %d images, %d bytes", len(r.images), r.ImageBytes())
	}
	for _, op := range b.Ops {
		if r.images[op.Image.ID()] == nil {
			t.Fatal("a recent image's texture was released")
		}
	}
	// Drawing nothing of them for a few frames lets them go to the budget.
	empty := &scene.Scene{Width: 640, Height: 128, Clear: scene.Color{A: 255}}
	for range 5 {
		renderRaw(t, r, empty, false)
		pool(r.trimImages)
	}
	if r.ImageBytes() > 5*one || len(r.images) != 5 {
		t.Fatalf("idle images past the budget: %d images, %d bytes", len(r.images), r.ImageBytes())
	}
}

// TestLeftOutRedrawn: the frame after one that left images out draws where
// they are, though the scene did not change (the frames compared find
// nothing new there); an image left out that no OpImage draws has the
// next frame draw whole.
func TestLeftOutRedrawn(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		unavailable(t, err)
	}
	defer r.Release()
	r.SetImageBudget(0, 3)
	a := imagesScene(10, 10)
	frame := func() ([]image.Rectangle, bool) {
		damage, known := r.diff.Diff(a)
		damage, known = r.takeLeftOut(damage, known)
		out := append([]image.Rectangle(nil), damage...)
		renderRaw(t, r, a, false)
		r.noteLeftOut(a, r.diff.Bounds())
		r.diff.Remember(a)
		return out, known
	}
	if _, known := frame(); known {
		t.Fatal("the first frame is known")
	}
	damage, known := frame()
	if !known || len(damage) != 7 {
		t.Fatalf("after 3 of 10 images: known %v, damage %v", known, damage)
	}
	for i, d := range damage {
		if want := image.Rect((3+i)*64, 0, (4+i)*64, 64); !want.In(d) {
			t.Errorf("damage %d: %v, not around %v", i, d, want)
		}
	}
	frame()
	frame()
	if damage, known := frame(); !known || len(damage) != 0 || r.Deferred() {
		t.Fatalf("all drawn: known %v, damage %v, deferred %v", known, damage, r.Deferred())
	}
	// Left out but drawn by no OpImage: the next frame is drawn whole.
	r.img.leftOut = append(r.img.leftOut[:0], 1<<60)
	r.noteLeftOut(a, r.diff.Bounds())
	if _, known := r.takeLeftOut(nil, true); known {
		t.Fatal("an image of no OpImage left out: the next frame is known")
	}
}

// TestCanvasImagesUnbounded: a canvas is drawn once a version, so the
// images it draws are uploaded whatever the uploads a frame makes.
func TestCanvasImagesUnbounded(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		unavailable(t, err)
	}
	defer r.Release()
	r.SetImageBudget(0, 2)
	inner := imagesScene(10, 30)
	c := scene.NewCanvas(640, 128)
	c.Scene.Ops = inner.Ops
	s := &scene.Scene{Width: 640, Height: 128, Clear: scene.Color{A: 255}, Ops: []scene.Op{
		{Kind: scene.OpImage, Rect: scene.Rect{W: 640, H: 128}, Image: c, Src: scene.Rect{W: 640, H: 128}},
	}}
	renderRaw(t, r, s, false)
	if len(r.images) != 10 || r.Deferred() {
		t.Fatalf("the canvas drew %d of its 10 images, deferred %v", len(r.images), r.Deferred())
	}
}

// TestIdleTrimRedrawsAtOnce: after the idle trim let go of every texture,
// the next frame draws all its images again, past UploadsPerFrame (left
// out, the window flashed as it came back to the front); new images
// after it wait their turn again.
func TestIdleTrimRedrawsAtOnce(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		unavailable(t, err)
	}
	defer r.Release()
	r.SetImageBudget(0, 3)
	a := imagesScene(10, 10)
	for range 4 {
		renderRaw(t, r, a, false)
	}
	if len(r.images) != 10 {
		t.Fatalf("before: %d images", len(r.images))
	}
	pool(r.releaseIdle)
	if len(r.images) != 0 || r.ImageBytes() != 0 {
		t.Fatalf("idle: %d images kept", len(r.images))
	}
	renderRaw(t, r, a, false)
	if len(r.images) != 10 || r.Deferred() {
		t.Fatalf("after the trim: %d images, deferred %v", len(r.images), r.Deferred())
	}
	renderRaw(t, r, imagesScene(10, 100), false)
	if len(r.images) != 13 || !r.Deferred() {
		t.Fatalf("new images after: %d images, deferred %v", len(r.images), r.Deferred())
	}
}

// TestIdleTrimKeepsShown: with an idle keep, the trim keeps the textures
// of the images the last frame drew, up to that many bytes, and the next
// frame uploads only the others; an image drawn before the last frame
// goes.
func TestIdleTrimKeepsShown(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		unavailable(t, err)
	}
	defer r.Release()
	const one = 64 * 64 * 4
	r.SetIdleKeep(7 * one)
	old := imagesScene(3, 200)
	renderRaw(t, r, old, false)
	a := imagesScene(10, 10)
	renderRaw(t, r, a, false)
	if len(r.images) != 13 {
		t.Fatalf("before: %d images", len(r.images))
	}
	pool(r.releaseIdle)
	if len(r.images) != 7 || r.ImageBytes() != 7*one {
		t.Fatalf("idle: %d images kept (%d bytes), want 7", len(r.images), r.ImageBytes())
	}
	if r.mask.tex == 0 && a.MaskAtlas != nil {
		t.Fatal("idle: the glyphs' atlas went")
	}
	pix := renderRaw(t, r, a, false)
	if len(r.images) != 10 || r.img.uploads != 3 {
		t.Fatalf("after the trim: %d images, %d uploaded, want 10 and 3", len(r.images), r.img.uploads)
	}
	r.SetIdleKeep(0)
	want := renderRaw(t, r, a, false)
	if !bytes.Equal(pix, want) {
		t.Fatal("the images kept draw other pixels")
	}
	pool(r.releaseIdle)
	if len(r.images) != 0 {
		t.Fatalf("no idle keep: %d images kept", len(r.images))
	}
}
