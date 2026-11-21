package ui

import "testing"

// TestAccessibilityActive: a view learns whether assistive technology
// reads the window, and builds for it from the frame after it asked.
func TestAccessibilityActive(t *testing.T) {
	var seen []bool
	tt := NewTester(func(c *Context) {
		on := c.AccessibilityActive()
		seen = append(seen, on)
		Column(c).Children(func() {
			Text(c, "row")
			if on {
				Button(c, "Hidden action").Opacity(0)
			}
		})
	}, 200, 100)
	tt.Frame()
	if seen[len(seen)-1] {
		t.Fatal("active before anything asked")
	}
	if _, ok := tt.Access("Hidden action"); ok {
		t.Fatal("built for assistive technology before it asked")
	}
	tt.AccessibilityOn()
	if !seen[len(seen)-1] {
		t.Fatalf("not active once assistive technology reads the window: %v", seen)
	}
	if _, ok := tt.Access("Hidden action"); !ok {
		t.Fatal("what the view built for assistive technology is not in the tree")
	}
	var none *Context
	if none.AccessibilityActive() {
		t.Error("a nil context")
	}
}
