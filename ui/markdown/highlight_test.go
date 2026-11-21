package markdown

import (
	"strings"

	"testing"

	"github.com/egoist/mygo/ui"
)

// Highlights color their passages; the selection and a click are told
// in the source's bytes.
func TestHighlights(t *testing.T) {
	src := "# Title\n\nSome **bold text** here.\n\nNext para."
	yellow := ui.RGB(255, 230, 0)
	o := &Options{}
	var start, end, clicked int
	var selected, hit bool
	tt := ui.NewTester(func(c *ui.Context) {
		el := View(c, src, o)
		if a, b, ok := SelectionRange(el); ok {
			start, end, selected = a, b, true
		} else {
			selected = false
		}
		if at, ok := ClickedAt(el); ok {
			clicked, hit = at, true
		}
	}, 600, 400)
	// The spans: "bold" alone colored.
	bold := strings.Index(src, "bold")
	o.Highlights = []Highlight{{Start: bold, End: bold + 4, Color: yellow}}
	tt.Frame()
	var st *viewState
	tt2 := ui.NewTester(func(c *ui.Context) { st = stateOf(View(c, src, o)) }, 600, 400)
	tt2.Frame()
	colored := ""
	for _, pc := range st.paras {
		for _, pi := range pc.p.pieces {
			for _, sp := range pi.spans {
				if sp.Background == yellow {
					colored += sp.Text
				}
			}
		}
	}
	if colored != "bold" {
		t.Fatalf("colored %q, want bold", colored)
	}
	// Changed highlights build the paragraphs again.
	o.Highlights = nil
	tt2.Frame()
	for _, pc := range st.paras {
		for _, pi := range pc.p.pieces {
			for _, sp := range pi.spans {
				if sp.Background == yellow {
					t.Fatalf("%q still colored", sp.Text)
				}
			}
		}
	}

	// A drag selects; its range is the source's.
	r, _ := tt.Find("Some bold text here.")
	y := r.Y + r.H/2
	tt.Press(r.X+1, y)
	tt.Move(r.X+r.W-1, y)
	tt.Release(r.X+r.W-1, y)
	tt.Frame()
	if !selected || src[start:end] != "Some **bold text** here." {
		t.Fatalf("selected %v %q", selected, src[start:max(start, end)])
	}
	// A click: where it fell, nothing selected.
	hit = false
	tt.ClickAt(r.X+r.W-3, y)
	if !hit || clicked < 30 || clicked > len("# Title\n\nSome **bold text** here.") {
		t.Fatalf("clicked %v at %d", hit, clicked)
	}
	tt.Frame()
	if selected {
		t.Error("a click selected")
	}

	// Scrolled to a block far below.
	long := strings.Repeat("Filler paragraph.\n\n", 80) + "The end line."
	scroll := false
	tt3 := ui.NewTester(func(c *ui.Context) {
		ui.Scroll(c).Height(300).Children(func() {
			root := View(c, long, o)
			if scroll {
				ScrollToSource(root, strings.Index(long, "end line"))
				scroll = false
			}
		})
	}, 600, 300)
	if r, ok := tt3.Find("The end line."); ok && r.Y < 300 {
		t.Fatalf("the end shows already at %v", r)
	}
	scroll = true
	tt3.Frame()
	tt3.Frame()
	if r, ok := tt3.Find("The end line."); !ok || r.Y > 300 || r.Y < 0 {
		t.Errorf("not scrolled to the end: %v %v", r, ok)
	}
}
