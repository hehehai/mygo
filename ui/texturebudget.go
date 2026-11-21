package ui

import "sync"

// TextureBudget bounds what windows keep on the GPU for images (Bitmap,
// Image): a renderer keeps a texture for every image a frame drew until
// no frame has drawn it for a while (about 4 seconds), which a grid of
// thousands of thumbnails scrolled fast turns into gigabytes, and a
// frame showing many new images uploads them all before it shows.
//
// The zero TextureBudget, the default, bounds nothing. Metal's renderer
// (macOS) follows it; the others ignore it.
type TextureBudget struct {
	// Bytes bounds the textures of images kept (4 bytes a pixel): past
	// it, after each frame, the textures of the images drawn longest ago
	// are released, never one drawn in the last 3 frames; an image drawn
	// again is uploaded again. 0 for no bound.
	Bytes int64
	// UploadsPerFrame bounds how many images a frame draws that have no
	// texture yet; it leaves the others out (what is under them shows)
	// and the window draws another frame for them at once, painting its
	// elements again without building the view. 0 for no bound. An app
	// showing a placeholder of its own meanwhile bounds uploads itself,
	// as it knows which images are new. The frame after a window rested
	// (Metal lets go of its textures once it has drawn nothing for 2
	// seconds) draws every image it shows at once: they are not new.
	UploadsPerFrame int
	// IdleKeep is how many bytes of the textures of the images it last
	// showed a window keeps once it has drawn nothing for a while (Metal
	// lets go of its drawables and textures after 2 seconds): the
	// smallest first, and the glyphs' atlases with them. Its next frame,
	// when something moves again, then has none of those to upload on
	// the main thread first (a grid of thumbnails: 10 ms and more). 0,
	// the default, keeps none.
	IdleKeep int64
}

var textureBudget struct {
	sync.Mutex
	b TextureBudget
}

// SetTextureBudget sets the budget of every window's renderer, from its
// next frame; renderers drawing in memory ignore it.
func SetTextureBudget(b TextureBudget) {
	textureBudget.Lock()
	textureBudget.b = TextureBudget{Bytes: max(b.Bytes, 0), UploadsPerFrame: max(b.UploadsPerFrame, 0), IdleKeep: max(b.IdleKeep, 0)}
	textureBudget.Unlock()
}

// CurrentTextureBudget is the budget SetTextureBudget set.
func CurrentTextureBudget() TextureBudget {
	textureBudget.Lock()
	defer textureBudget.Unlock()
	return textureBudget.b
}

// budgeted is a GPU renderer that bounds its images' textures.
type budgeted interface {
	SetImageBudget(bytes int64, uploads int)
	SetIdleKeep(bytes int64)
	// Deferred reports whether the last frame left out images it had no
	// texture of, past the uploads a frame makes.
	Deferred() bool
}

// applyTextureBudget gives r the budget before it draws.
func applyTextureBudget(r gpuRenderer) {
	if b, ok := r.(budgeted); ok {
		tb := CurrentTextureBudget()
		b.SetImageBudget(tb.Bytes, tb.UploadsPerFrame)
		b.SetIdleKeep(tb.IdleKeep)
	}
}

// drawLeftOut asks, after a frame whose renderer left out images past the
// uploads a frame makes, for another drawing them: the last frame's
// elements painted again, unless a frame built anew is asked for.
func (h *windowHost) drawLeftOut() {
	b, ok := h.gpu.(budgeted)
	if !ok || !b.Deferred() {
		return
	}
	if rt := h.rt; rt != nil && !rt.buildAsked {
		rt.lookDue = true
	}
	h.requestFrame()
}
