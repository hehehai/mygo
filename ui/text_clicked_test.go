package ui

import "testing"

// TextClicked reports a click on selectable text, with where it fell;
// a drag that selects is not one.
func TestTextClicked(t *testing.T) {
	var got SelectedText
	var clicked int
	var selecting bool
	tt := NewTester(func(c *Context) {
		col := Column(c).Selectable().Children(func() {
			Text(c, "first paragraph")
			Text(c, "second paragraph")
		})
		selecting = col.TextSelecting()
		if p, ok := col.TextClicked(); ok {
			got, clicked = p, clicked+1
		}
	}, 400, 200)
	r, _ := tt.Find("second paragraph")
	y := r.Y + r.H/2
	tt.ClickAt(r.X+r.W-1, y)
	if clicked != 1 || got.Text != "second paragraph" || got.Start != len("second paragraph") || got.End != got.Start {
		t.Fatalf("clicked %d: %+v", clicked, got)
	}
	tt.Frame()
	if clicked != 1 {
		t.Fatalf("the click was told again (%d)", clicked)
	}
	tt.Press(r.X+1, y)
	tt.Move(r.X+r.W-1, y)
	tt.Frame()
	if !selecting {
		t.Error("not selecting while dragging")
	}
	tt.Release(r.X+r.W-1, y)
	tt.Frame()
	if clicked != 1 {
		t.Fatalf("a drag was a click: %+v", got)
	}
	if selecting {
		t.Error("still selecting after the release")
	}
}
