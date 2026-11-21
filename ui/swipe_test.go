package ui

import (
	"slices"
	"testing"
)

// TestSwipe: a touchpad's swipe goes to the innermost element under the
// pointer that takes it, else to the focus; a two-finger scroll keeps its
// gesture's phase and glide (a viewer pages by them).
func TestSwipe(t *testing.T) {
	type got struct {
		who      string
		kind     InputKind
		dx       float32
		phase    GesturePhase
		momentum bool
	}
	var seen []got
	record := func(who string, take bool) func(InputEvent) bool {
		return func(ev InputEvent) bool {
			if ev.Kind == InputSwipe || ev.Kind == InputScroll {
				seen = append(seen, got{who, ev.Kind, ev.DX, ev.Phase, ev.Momentum})
			}
			return take
		}
	}
	focusTakes := false
	tt := NewTester(func(c *Context) {
		Column(c).Size(200, 200).Children(func() {
			Row(c).Size(200, 100).HandleInput(record("outer", true)).Children(func() {
				Box(c).Size(100, 100).HandleInput(record("left", true))
				Box(c).Size(100, 100).HandleInput(record("right", false))
			})
			Row(c).Size(200, 50).Children(func() {
				Box(c).Size(100, 50).HandleInput(record("away", false))
			})
			Box(c).Size(50, 50).Focusable().Focus().HandleInput(func(ev InputEvent) bool {
				if ev.Kind == InputSwipe {
					seen = append(seen, got{"focus", ev.Kind, ev.DX, ev.Phase, ev.Momentum})
				}
				return focusTakes
			})
		})
	}, 200, 200)
	defer tt.rt.close()
	tt.Frame()

	tt.Swipe(30, 40, 1, 0)
	tt.Swipe(150, 40, -1, 0) // right does not take it: the row does
	want := []got{{"left", InputSwipe, 1, 0, false}, {"right", InputSwipe, -1, 0, false}, {"outer", InputSwipe, -1, 0, false}}
	if !slices.Equal(seen, want) {
		t.Fatalf("swipes: %v, want %v", seen, want)
	}

	// Over an element that leaves it, the focus has it; taken by none, the
	// platform goes back or forward.
	seen = nil
	if tt.Swipe(30, 120, 1, 0) {
		t.Error("a swipe nothing took was taken")
	}
	focusTakes = true
	if !tt.Swipe(30, 120, -1, 0) {
		t.Error("a swipe the focus took was not taken")
	}
	want = []got{{"away", InputSwipe, 1, 0, false}, {"focus", InputSwipe, 1, 0, false}, {"away", InputSwipe, -1, 0, false}, {"focus", InputSwipe, -1, 0, false}}
	if !slices.Equal(seen, want) {
		t.Fatalf("swipes to the focus: %v, want %v", seen, want)
	}

	seen = nil
	tt.ScrollGesture(30, 40, -20, 0, GestureBegan, false)
	tt.ScrollGesture(30, 40, -30, 0, GestureChanged, false)
	tt.ScrollGesture(30, 40, 0, 0, GestureEnded, false)
	tt.ScrollGesture(30, 40, -10, 0, GestureBegan, true)
	tt.Wheel(30, 40, 0, 40) // a wheel: no phase
	want = []got{{"left", InputScroll, -20, GestureBegan, false}, {"left", InputScroll, -30, GestureChanged, false},
		{"left", InputScroll, 0, GestureEnded, false}, {"left", InputScroll, -10, GestureBegan, true}, {"left", InputScroll, 0, GestureNone, false}}
	if !slices.Equal(seen, want) {
		t.Fatalf("scrolls: %v, want %v", seen, want)
	}
}
