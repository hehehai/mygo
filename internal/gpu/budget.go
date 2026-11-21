package gpu

import "sort"

// ImageBudget bounds what a renderer keeps and uploads of images' textures
// (ui.SetTextureBudget). Renderers keep a texture for every image a frame
// drew until it has not been drawn for a while (240 frames): a grid of
// thousands of thumbnails scrolled fast passes more images in that time than
// memory holds, and a frame showing many new ones uploads them all at
// once. The zero value bounds nothing, as before.
type ImageBudget struct {
	// Bytes bounds the bytes of the images' textures kept: past it, Trim
	// releases the textures of the images drawn longest ago, never one
	// drawn in the last KeepFrames frames. 0 for no bound.
	Bytes int64
	// Uploads bounds how many textures of images not uploaded before a
	// frame makes: Admit refuses the others, which that frame does not
	// draw, and Deferred asks for another. 0 for no bound.
	Uploads int

	kept     int64 // bytes of the textures kept
	uploads  int   // made this frame
	deferred bool
	// all: this frame admits every image (AdmitAll asked the last one).
	all, allNext bool
}

// KeepFrames is how many recent frames' images TrimImages never releases.
const KeepFrames = 3

// Set changes the bounds.
func (b *ImageBudget) Set(bytes int64, uploads int) {
	b.Bytes, b.Uploads = max(bytes, 0), max(uploads, 0)
}

// BeginFrame starts a frame's count of uploads.
func (b *ImageBudget) BeginFrame() {
	b.uploads, b.deferred = 0, false
	b.all, b.allNext = b.allNext, false
}

// AdmitAll has the next frame admit every image it draws, past Uploads:
// the renderer let go of its textures while the window drew nothing
// (Metal's idle trim), and what that frame draws again is mostly what the
// window showed, not new images. Left out, they would be missing from it
// and come back a few a frame: the window would flash.
func (b *ImageBudget) AdmitAll() { b.allNext = true }

// Admit reports whether the frame may upload the texture of an image it
// has none of (a new image, not one changed or resized): false past
// Uploads, and the image is not drawn this frame.
func (b *ImageBudget) Admit() bool {
	if b.Uploads > 0 && b.uploads >= b.Uploads && !b.all {
		b.deferred = true
		return false
	}
	b.uploads++
	return true
}

// Deferred reports whether the frame left images out (Admit refused one):
// the window draws another.
func (b *ImageBudget) Deferred() bool { return b.deferred }

// Add counts bytes of textures made (negative: released).
func (b *ImageBudget) Add(bytes int64) { b.kept += bytes }

// Kept is the bytes of the textures kept.
func (b *ImageBudget) Kept() int64 { return b.kept }

// Reset counts none kept (the renderer released them all).
func (b *ImageBudget) Reset() { b.kept = 0 }

// TrimImages releases, while b keeps more than b.Bytes, the textures of
// images[key] drawn longest ago (by lastFrame), none drawn since frame -
// KeepFrames and none that counts no bytes (bytes 0: not the budget's, as
// a texture a GPU source gives). drop releases one, counts its bytes off b
// (Add) and takes it out of images.
func TrimImages[T any](b *ImageBudget, images map[uint64]T, frame uint64, lastFrame func(T) uint64, bytes func(T) int64, drop func(key uint64, t T)) {
	if b.Bytes <= 0 || b.kept <= b.Bytes {
		return
	}
	type old struct {
		key  uint64
		last uint64
	}
	var cand []old
	for key, t := range images {
		if last := lastFrame(t); frame-last > KeepFrames && bytes(t) > 0 {
			cand = append(cand, old{key, last})
		}
	}
	sort.Slice(cand, func(i, j int) bool {
		if cand[i].last != cand[j].last {
			return cand[i].last < cand[j].last
		}
		return cand[i].key < cand[j].key
	})
	for _, c := range cand {
		if b.kept <= b.Bytes {
			break
		}
		drop(c.key, images[c.key])
	}
}
