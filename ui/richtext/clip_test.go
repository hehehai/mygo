package richtext

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestEditorClipsToBox: an editor scrolled into a long paragraph paints
// nothing outside its box, above or below it (an element's Clip clips
// its children, not what its own Draw paints), styled or not.
func TestEditorClipsToBox(t *testing.T) {
	for _, styled := range []bool{false, true} {
		doc := NewDocument(NewText(strings.Repeat("A line of text that wraps in the box, again and again. ", 60)))
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Background(ui.Hex("#ffffff")).Children(func() {
				ui.Box(c).Height(80)
				ui.Box(c).Height(100).Padding(0, 20).Children(func() {
					var e ui.Element
					if styled {
						e = Editor(c.Key("ed"), doc)
					} else {
						e = EditorBase(c.Key("ed"), doc)
					}
					e.Fill().MinHeight(0)
				})
			})
		}, 400, 300)
		tt.Frame()
		for range 5 {
			tt.Scroll(200, 130, 0, -40)
			tt.Frame()
		}
		img := tt.Image()
		for _, band := range [][2]int{{0, 79}, {181, 300}} {
			for y := band[0]; y < band[1]; y++ {
				for x := 0; x < 400; x++ {
					if c := img.RGBAAt(x, y); c.R != 255 || c.G != 255 || c.B != 255 {
						t.Fatalf("styled %v: painted at %d,%d outside the editor's box: %v", styled, x, y, c)
					}
				}
			}
		}
	}
}
