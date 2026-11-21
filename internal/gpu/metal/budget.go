//go:build darwin

package metal

import (
	"cmp"
	"image"
	"slices"

	"github.com/egoist/mygo/internal/gpu"
	"github.com/egoist/mygo/internal/scene"
)

// budgetState bounds the images' textures (ui.SetTextureBudget): their
// bytes, the uploads a frame makes, and what the idle trim keeps.
type budgetState struct {
	budget gpu.ImageBudget
	// idleKeep is the bytes of the textures of the images the last frame
	// drew that the idle trim keeps (ui.TextureBudget.IdleKeep).
	idleKeep int64
	// unbounded admits every image while set: a canvas is drawn once a
	// version, so an image it left out would stay out.
	unbounded bool
	// leftOut are the images the frame being built left out (Admit
	// refused them), and redraw where the next frame draws them, as
	// what it compares with the last frame does not see they are new:
	// nil and whole when an image left out is not an OpImage's.
	leftOut []uint64
	redraw  []image.Rectangle
	whole   bool
	// uploads is how many images' textures the frame uploaded.
	uploads int
}

// SetImageBudget bounds the bytes of images' textures kept and how many
// a frame uploads (ui.SetTextureBudget); 0 for no bound.
func (r *Renderer) SetImageBudget(bytes int64, uploads int) { r.img.budget.Set(bytes, uploads) }

// SetIdleKeep is how many bytes of the textures of the images the last
// frame drew the idle trim keeps (ui.TextureBudget.IdleKeep); 0: none.
func (r *Renderer) SetIdleKeep(bytes int64) { r.img.idleKeep = max(bytes, 0) }

// Deferred reports whether the last frame left out images whose textures
// the budget kept it from uploading: the window draws another.
func (r *Renderer) Deferred() bool { return r.img.budget.Deferred() }

// ImageBytes is the bytes of the images' textures kept.
func (r *Renderer) ImageBytes() int64 { return r.img.budget.Kept() }

// beginBudget starts a frame's count of uploads.
func (r *Renderer) beginBudget() {
	r.img.budget.BeginFrame()
	r.img.leftOut = r.img.leftOut[:0]
	r.img.uploads = 0
}

// admit reports whether the frame may upload the texture of img, which
// has none: false past the uploads a frame makes, and img is left out of
// the frame.
func (r *Renderer) admit(img *scene.Image) bool {
	if r.img.unbounded {
		return true
	}
	if !r.img.budget.Admit() {
		r.img.leftOut = append(r.img.leftOut, img.ID())
		return false
	}
	return true
}

// count has t, an image's texture of its pixels, count n bytes against
// the budget (0 as it goes).
func (r *Renderer) count(t *texture, n int64) {
	r.img.budget.Add(n - t.bytes)
	t.bytes = n
}

// noteLeftOut finds, after the frame s was built, where the images it left
// out draw, for the next frame to draw there (takeLeftOut). Bounds are
// those diff found for s, when it compared it.
func (r *Renderer) noteLeftOut(s *scene.Scene, bounds []image.Rectangle) {
	r.img.redraw, r.img.whole = r.img.redraw[:0], false
	if len(r.img.leftOut) == 0 {
		return
	}
	if len(bounds) != len(s.Ops) {
		r.img.whole = true
		return
	}
	// drawn tells, of each image left out, whether an OpImage draws it.
	drawn := make(map[uint64]bool, len(r.img.leftOut))
	for _, id := range r.img.leftOut {
		drawn[id] = false
	}
	for i := range s.Ops {
		op := &s.Ops[i]
		if op.Kind != scene.OpImage || op.Image == nil {
			continue
		}
		if _, ok := drawn[op.Image.ID()]; ok {
			r.img.redraw = append(r.img.redraw, bounds[i])
			drawn[op.Image.ID()] = true
		}
	}
	// An image left out that no OpImage draws (an effect's, a mask's):
	// the next frame draws whole.
	for _, ok := range drawn {
		if !ok {
			r.img.redraw, r.img.whole = r.img.redraw[:0], true
			return
		}
	}
}

// takeLeftOut adds to damage, what changed since the last frame drawn
// (known), where the last frame left out images, and reports whether it
// is still known.
func (r *Renderer) takeLeftOut(damage []image.Rectangle, known bool) ([]image.Rectangle, bool) {
	if r.img.whole {
		r.img.whole = false
		r.img.redraw = r.img.redraw[:0]
		return damage, false
	}
	if known {
		damage = append(damage, r.img.redraw...)
	}
	r.img.redraw = r.img.redraw[:0]
	return damage, known
}

// trimImages forgets, past the budget, the textures of the images drawn
// longest ago.
func (r *Renderer) trimImages() {
	gpu.TrimImages(&r.img.budget, r.images, r.frame,
		func(t *texture) uint64 { return t.lastFrame },
		func(t *texture) int64 { return t.bytes },
		func(key uint64, t *texture) {
			r.dropTexture(t)
			delete(r.images, key)
		})
}

// releaseIdle lets go of the textures of a window that draws nothing
// (trim). Its next frame mostly draws again what it showed: it uploads
// those images at once, past the budget's uploads a frame (which are for
// new images), rather than leave them out and show them missing.
//
// With an idle keep (ui.TextureBudget.IdleKeep), the textures of the
// images the last frame drew stay, the smallest first up to that many
// bytes, and the glyphs' atlases: the next frame (the window coming back
// to life, as a key opens a panel over it) would upload them all again
// on the main thread (a grid's 46 thumbnails, 22 MB: 10 ms on mac-air).
func (r *Renderer) releaseIdle() {
	r.img.budget.AdmitAll()
	if r.img.idleKeep <= 0 {
		r.releaseTextures()
		return
	}
	type kept struct {
		key uint64
		t   *texture
	}
	var shown []kept
	for key, t := range r.images {
		if t.lastFrame == r.frame && t.tex != 0 && t.bytes > 0 {
			shown = append(shown, kept{key, t})
		}
	}
	slices.SortFunc(shown, func(a, b kept) int { return cmp.Or(cmp.Compare(a.t.bytes, b.t.bytes), cmp.Compare(a.key, b.key)) })
	keep := map[uint64]*texture{}
	var bytes int64
	for _, k := range shown {
		if bytes+k.t.bytes > r.img.idleKeep {
			break
		}
		bytes += k.t.bytes
		keep[k.key] = k.t
		delete(r.images, k.key)
	}
	mask, color := r.mask, r.color
	r.mask, r.color = texture{}, texture{}
	r.releaseTextures()
	r.mask, r.color = mask, color
	r.images = keep
	r.img.budget.Reset()
	r.img.budget.Add(bytes)
}
