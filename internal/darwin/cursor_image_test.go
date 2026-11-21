//go:build darwin && !ios

package darwin

import (
	"image"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestImageCursor: a picture's pointer is an NSCursor of its points' size
// with its hot spot, made once per picture; an empty picture is the arrow.
func TestImageCursor(t *testing.T) {
	load()
	withPool(func() {
		c := &platform.CursorImage{Image: image.NewRGBA(image.Rect(0, 0, 48, 40)), Scale: 2, HotX: 2, HotY: 18}
		cur := imageCursor(c)
		if cur == 0 || cur == send(class("NSCursor"), "arrowCursor") {
			t.Fatal("no cursor of the picture")
		}
		if sz := msgSize(send(cur, "image"), sel("size")); sz.Width != 24 || sz.Height != 20 {
			t.Errorf("image %v, want 24×20 points", sz)
		}
		if hs := msgPoint(cur, sel("hotSpot")); hs.X != 2 || hs.Y != 18 {
			t.Errorf("hot spot %v", hs)
		}
		if imageCursor(c) != cur {
			t.Error("made again")
		}
		if imageCursor(&platform.CursorImage{Image: image.NewRGBA(image.Rect(0, 0, 0, 0))}) != send(class("NSCursor"), "arrowCursor") {
			t.Error("an empty picture is not the arrow")
		}
	})
}
