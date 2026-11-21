package ui

import (
	"slices"
	"testing"
)

// TestDropGlide: a view that drops the glide under way (a viewer turned
// to another page as the fingers lifted) has none of its events, to the
// glide's end; the next gesture and its glide come as ever, and a glide
// cut short by a new gesture ends the dropping too.
func TestDropGlide(t *testing.T) {
	type got struct {
		dy       float32
		phase    GesturePhase
		momentum bool
	}
	var seen []got
	drop := false
	tt := NewTester(func(c *Context) {
		if drop {
			c.DropGlide()
			drop = false
		}
		Box(c).Size(200, 100).HandleInput(func(ev InputEvent) bool {
			if ev.Kind == InputScroll {
				seen = append(seen, got{ev.DY, ev.Phase, ev.Momentum})
			}
			return true
		})
	}, 200, 100)
	defer tt.rt.close()
	tt.Frame()

	tt.ScrollGesture(30, 40, 0, 10, GestureBegan, false)
	tt.ScrollGesture(30, 40, 0, 0, GestureEnded, false)
	drop = true // the page turned
	tt.Frame()
	tt.ScrollGesture(30, 40, 0, 8, GestureBegan, true)
	tt.ScrollGesture(30, 40, 0, 6, GestureChanged, true)
	tt.ScrollGesture(30, 40, 0, 0, GestureEnded, true)
	tt.ScrollGesture(30, 40, 0, 5, GestureBegan, false) // the next gesture
	tt.ScrollGesture(30, 40, 0, 0, GestureEnded, false)
	tt.ScrollGesture(30, 40, 0, 4, GestureBegan, true)
	tt.ScrollGesture(30, 40, 0, 0, GestureEnded, true)
	want := []got{{10, GestureBegan, false}, {0, GestureEnded, false},
		{5, GestureBegan, false}, {0, GestureEnded, false}, {4, GestureBegan, true}, {0, GestureEnded, true}}
	if !slices.Equal(seen, want) {
		t.Fatalf("after DropGlide: %v, want %v", seen, want)
	}

	seen = nil
	drop = true
	tt.Frame()
	tt.ScrollGesture(30, 40, 0, 8, GestureBegan, true)
	tt.ScrollGesture(30, 40, 0, 3, GestureBegan, false) // a touch cuts it short: no end
	tt.ScrollGesture(30, 40, 0, 2, GestureBegan, true)
	tt.Wheel(30, 40, 0, 7) // a wheel
	want = []got{{3, GestureBegan, false}, {2, GestureBegan, true}, {7, GestureNone, false}}
	if !slices.Equal(seen, want) {
		t.Fatalf("a glide cut short: %v, want %v", seen, want)
	}
}
