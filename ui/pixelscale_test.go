package ui

import "testing"

// TestPixelScale: a view reads the window's pixels per DIP, zoomed with
// its content.
func TestPixelScale(t *testing.T) {
	var got float32
	zoom := float32(1)
	tt := NewTester(func(c *Context) { c.SetZoom(zoom); got = c.PixelScale() }, 100, 100)
	tt.SetScale(2)
	tt.Frame()
	if got != 2 {
		t.Errorf("scale 2: %v", got)
	}
	tt.SetScale(1)
	tt.Frame()
	if got != 1 {
		t.Errorf("scale 1: %v", got)
	}
	zoom = 1.5
	tt.Frame()
	tt.Frame()
	if got != 1.5 {
		t.Errorf("scale 1 zoomed 1.5: %v", got)
	}
}
