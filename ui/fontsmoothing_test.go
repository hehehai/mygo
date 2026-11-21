package ui

import (
	"bytes"
	"testing"

	"github.com/egoist/mygo/internal/text"
)

// TestFontSmoothingPerWindow: a window without font smoothing paints its
// text's glyphs Flat, whatever their color, and another window's text is
// as before; turning it back on paints them as before. Pixels differ only
// where the text engine smooths fonts (Core Text); elsewhere Flat draws
// as the shades.
func TestFontSmoothingPerWindow(t *testing.T) {
	view := func(c *Context) {
		Column(c).Background(RGB(30, 30, 34)).Padding(8).Children(func() {
			Text(c, "Translate 翻译").FontSize(15).FontWeight(600).TextColor(RGB(255, 255, 255))
			Text(c, "Dark text").TextColor(RGB(20, 20, 20))
		})
	}
	smooth := NewTester(view, 200, 80)
	smooth.SetScale(2)
	want := bytes.Clone(smooth.Image().Pix)
	flat := NewTester(view, 200, 80)
	flat.SetScale(2)
	flat.SetFontSmoothing(false)
	shades := map[text.Shade]bool{}
	p := &Painter{rt: flat.rt}
	for _, c := range []Color{RGB(255, 255, 255), RGB(20, 20, 20)} {
		shades[p.shadeOf(c)] = true
	}
	if len(shades) != 1 || !shades[text.Flat] {
		t.Fatalf("unsmoothed window's shades: %v", shades)
	}
	unsmoothed := bytes.Clone(flat.Image().Pix)
	// The other window's text is unchanged.
	smooth.Frame()
	if !bytes.Equal(smooth.Image().Pix, want) {
		t.Fatal("a window with font smoothing drew otherwise after another without")
	}
	flat.SetFontSmoothing(true)
	if !bytes.Equal(flat.Image().Pix, want) {
		t.Fatal("turned back on, the window draws otherwise than one never turned off")
	}
	t.Logf("pixels differ without smoothing: %v", !bytes.Equal(unsmoothed, want))
}
