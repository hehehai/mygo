package ui

import "testing"

// TestPressedWithin: a bar holding a slider is pressed within while the
// slider is held, the pointer dragged off the bar too, and no more once
// it is let go of; a press elsewhere is not within it.
func TestPressedWithin(t *testing.T) {
	v := 0.5
	var within, other bool
	tt := NewTester(func(c *Context) {
		Column(c.Key("page")).Grow(1).Children(func() {
			o := Box(c.Key("other")).Height(100).OnClick(func() {})
			bar := Row(c.Key("bar")).Height(40)
			bar.Children(func() { Slider(c.Key("s"), &v, 0, 1).Grow(1).Label("Position") })
			within, other = bar.PressedWithin(), o.PressedWithin()
		})
	}, 400, 300)
	tt.Frame()
	if within {
		t.Fatal("pressed within before a press")
	}
	tt.Press(200, 120)
	tt.Frame()
	if !within || other {
		t.Fatalf("the slider pressed: within %v, other %v", within, other)
	}
	tt.Move(150, 20) // off the bar, over the other box
	tt.Frame()
	if !within || other {
		t.Fatalf("dragged off the bar: within %v, other %v", within, other)
	}
	tt.Release(150, 20)
	tt.Frame()
	if within {
		t.Fatal("still pressed within once let go of")
	}
	tt.Press(200, 50)
	tt.Frame()
	if within || !other {
		t.Fatalf("a press on the other box: within %v, other %v", within, other)
	}
	tt.Release(200, 50)
}
