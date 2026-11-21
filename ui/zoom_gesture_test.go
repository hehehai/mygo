package ui

import (
	"slices"
	"testing"
)

// A touchpad's pinch whose phases the platform tells goes to the innermost
// element under the pointer that takes its first event, at the pointer's
// place in its box, and the rest of the gesture follows it wherever the
// pointer goes; a smart zoom is a gesture of one event.
func TestZoomGesture(t *testing.T) {
	type got struct {
		who   string
		scale float32
		phase GesturePhase
		x, y  float32
	}
	var seen []got
	record := func(who string, take bool) func(InputEvent) bool {
		return func(ev InputEvent) bool {
			if ev.Kind == InputZoom {
				seen = append(seen, got{who, ev.Scale, ev.Phase, ev.X, ev.Y})
			}
			return take
		}
	}
	tt := NewTester(func(c *Context) {
		Row(c).Size(200, 100).HandleInput(record("outer", true)).Children(func() {
			Box(c).Size(100, 100).HandleInput(record("left", true))
			Box(c).Size(100, 100).HandleInput(record("right", false))
		})
	}, 200, 100)
	defer tt.rt.close()
	tt.Frame()

	tt.Zoom(30, 40, GestureBegan, 1.1)
	tt.Zoom(150, 40, GestureChanged, 0.95) // moved over another element: still left's
	tt.Zoom(150, 40, GestureEnded, 1)
	want := []got{{"left", 1.1, GestureBegan, 30, 40}, {"left", 0.95, GestureChanged, 150, 40}, {"left", 1, GestureEnded, 150, 40}}
	if !slices.Equal(seen, want) {
		t.Fatalf("left's pinch: %v, want %v", seen, want)
	}

	// Right leaves it: it goes on out to the row, which takes the rest.
	seen = nil
	tt.PinchGesture(150, 50, 1.2, 1.3)
	want = []got{{"right", 1.2, GestureBegan, 50, 50}, {"outer", 1.2, GestureBegan, 150, 50},
		{"outer", 1.3, GestureChanged, 150, 50}, {"outer", 1, GestureEnded, 150, 50}}
	if !slices.Equal(seen, want) {
		t.Fatalf("right's pinch: %v, want %v", seen, want)
	}

	// A gesture whose first event came before (no element took it) is
	// the element's under the pointer from its next event, as a first.
	seen = nil
	tt.Zoom(30, 40, GestureChanged, 1.1)
	tt.Zoom(30, 40, GestureEnded, 1)
	want = []got{{"left", 1.1, GestureBegan, 30, 40}, {"left", 1, GestureEnded, 30, 40}}
	if !slices.Equal(seen, want) {
		t.Fatalf("a gesture joined late: %v, want %v", seen, want)
	}

	// A smart zoom is a gesture of one event: the next pinch goes to the
	// element under the pointer again.
	seen = nil
	tt.SmartZoom(150, 40)
	tt.Zoom(30, 40, GestureChanged, 1.1)
	tt.Zoom(30, 40, GestureEnded, 1)
	want = []got{{"right", 1, GestureSmart, 50, 40}, {"outer", 1, GestureSmart, 150, 40},
		{"left", 1.1, GestureBegan, 30, 40}, {"left", 1, GestureEnded, 30, 40}}
	if !slices.Equal(seen, want) {
		t.Fatalf("a smart zoom: %v, want %v", seen, want)
	}

	// Without phases (GestureNone), as before: the innermost element
	// handling input has each event, and a pinch of no zoom is none.
	seen = nil
	tt.Pinch(150, 40, 1.5)
	tt.Pinch(30, 40, 1)
	want = []got{{"right", 1.5, GestureNone, 50, 40}}
	if !slices.Equal(seen, want) {
		t.Fatalf("pinches without phases: %v, want %v", seen, want)
	}
}

// ScrollWith's and WheelWith's modifiers come with the scroll.
func TestScrollWith(t *testing.T) {
	type got struct {
		mods    Modifiers
		precise bool
	}
	var seen []got
	tt := NewTester(func(c *Context) {
		Box(c).Size(100, 100).HandleInput(func(ev InputEvent) bool {
			if ev.Kind == InputScroll {
				seen = append(seen, got{ev.Mods, ev.Precise})
			}
			return true
		})
	}, 100, 100)
	defer tt.rt.close()
	tt.Frame()
	tt.ScrollWith(Cmd, 50, 50, 0, -10)
	tt.WheelWith(Shift, 50, 50, 0, -10)
	tt.Scroll(50, 50, 0, -10)
	if want := []got{{Cmd, true}, {Shift, false}, {0, true}}; !slices.Equal(seen, want) {
		t.Fatalf("scrolls %v, want %v", seen, want)
	}
}
