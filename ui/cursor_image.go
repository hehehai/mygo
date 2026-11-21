package ui

import (
	"image"
	"image/draw"

	"github.com/egoist/mygo/internal/platform"
)

// ImageCursor is a pointer of a picture (Element.CursorImage).
type ImageCursor struct{ c platform.CursorImage }

// NewImageCursor makes a pointer of img, scale of its pixels a DIP (2 for
// a picture drawn at twice its size, sharp on Retina screens), its hot
// spot — the point that points — at (hotX, hotY) DIPs from its top left.
// Make it once and keep it: the platform makes its pointer once for each.
func NewImageCursor(img image.Image, scale, hotX, hotY float32) *ImageCursor {
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Rect.Min != (image.Point{}) {
		b := img.Bounds()
		rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Rect, img, b.Min, draw.Src)
	}
	if scale <= 0 {
		scale = 1
	}
	return &ImageCursor{c: platform.CursorImage{Image: rgba, Scale: float64(scale), HotX: float64(hotX), HotY: float64(hotY)}}
}

// cursorImage is the Cursor of an element showing an ImageCursor.
const cursorImage Cursor = 254

// CursorImage shows c as the pointer over the element, as Cursor shows a
// shape, picked as Cursor is (the innermost element under the pointer that
// sets one). Where the platform has no pictures for pointers (all but
// macOS), the default pointer shows. nil sets none.
func (e *node) CursorImage(c *ImageCursor) *node {
	if c == nil {
		e.cursor, e.cursorImg = 0, nil
		return e
	}
	e.cursor, e.cursorImg = cursorImage+1, c
	return e
}
