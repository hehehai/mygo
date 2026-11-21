package ui

import "testing"

// TestPlaceholderEllipsis: a single-line input's placeholder too long for
// its content box ends in "…" there, short of its padding, and the text
// typed in it is cut there too.
func TestPlaceholderEllipsis(t *testing.T) {
	text := ""
	tt := NewTester(func(c *Context) {
		TextInput(c.Key("q"), &text).Placeholder("Search the whole library by name").Width(140).Padding(4, 60, 4, 8)
	}, 300, 80)
	tt.Frame()
	inPadding := func(what string) {
		t.Helper()
		img := tt.Image()
		// Nothing in the right padding (60 DIPs): the pixels there are the
		// input's background.
		bg := img.RGBAAt(140-8, 13)
		for x := 140 - 58; x < 140-8; x++ {
			for y := 8; y < 20; y++ {
				if img.RGBAAt(x, y) != bg {
					t.Fatalf("%s drawn at %d,%d in the padding", what, x, y)
				}
			}
		}
	}
	inPadding("placeholder")
	text = "Some text long enough to run under the buttons"
	tt.Frame()
	inPadding("text")
}
