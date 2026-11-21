package ui

import (
	"image"
	"testing"
)

// TestCursorImage: over an element with CursorImage the host shows its
// picture; over others their shapes; a picture's element inside a
// clickable one shows it.
func TestCursorImage(t *testing.T) {
	pin := NewImageCursor(image.NewRGBA(image.Rect(0, 0, 48, 48)), 2, 2, 22)
	tt := coreNewTester(func(c *context) {
		coreRow(c).Fill().Gap(10).Padding(10).Children(func() {
			coreBox(c).Size(40, 40).Label("canvas").CursorImage(pin)
			coreBox(c).Size(40, 40).Label("plain").Cursor(CursorCrosshair)
			coreButtonBase(c).Size(40, 40).Label("button").CursorImage(pin)
		})
	}, 200, 60)
	for _, want := range []struct {
		name   string
		cursor Cursor
		img    *ImageCursor
	}{{"canvas", cursorImage, pin}, {"plain", CursorCrosshair, nil}, {"button", cursorImage, pin}, {"canvas", cursorImage, pin}} {
		r, ok := tt.Find(want.name)
		if !ok {
			t.Fatalf("no %q", want.name)
		}
		if tt.Move(center(r)); tt.Cursor() != want.cursor || tt.CursorImage() != want.img {
			t.Errorf("over %q: %v %p, want %v %p", want.name, tt.Cursor(), tt.CursorImage(), want.cursor, want.img)
		}
	}
	if pin.c.Image.Rect.Dx() != 48 || pin.c.Scale != 2 || pin.c.HotY != 22 {
		t.Errorf("the picture %+v", pin.c)
	}
}
