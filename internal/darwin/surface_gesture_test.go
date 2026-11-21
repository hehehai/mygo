//go:build darwin && !ios

package darwin

import (
	"testing"

	"github.com/ebitengine/purego"

	"github.com/egoist/mygo/internal/platform"
)

// gestureHandler records a surface's events and takes them as take says.
type gestureHandler struct {
	platform.WindowHandler
	events []platform.SurfaceEvent
	take   bool
}

func (h *gestureHandler) SurfaceEvent(ev platform.SurfaceEvent) bool {
	h.events = append(h.events, ev)
	return h.take
}

// cgEvents makes CoreGraphics events, which AppKit makes NSEvents of, as
// the window server's are: no window needed.
type cgEvents struct {
	create    func(uintptr) uintptr
	setInt    func(uintptr, uint32, int64)
	setDouble func(uintptr, uint32, float64)
}

func newCGEvents() *cgEvents {
	load()
	c := &cgEvents{}
	purego.RegisterLibFunc(&c.create, libCG, "CGEventCreate")
	purego.RegisterLibFunc(&cgEventSetType, libCG, "CGEventSetType")
	purego.RegisterLibFunc(&c.setInt, libCG, "CGEventSetIntegerValueField")
	purego.RegisterLibFunc(&c.setDouble, libCG, "CGEventSetDoubleValueField")
	return c
}

// scroll is a touchpad's scroll event of the scroll phase and momentum
// phase (CGScrollPhase, CGMomentumScrollPhase).
func (c *cgEvents) scroll(t *testing.T, phase, momentum int64) id {
	cg := c.create(0)
	cgEventSetType(cg, 22)  // kCGEventScrollWheel
	c.setInt(cg, 88, 1)     // kCGScrollWheelEventIsContinuous
	c.setDouble(cg, 96, 3)  // kCGScrollWheelEventPointDeltaAxis1
	c.setDouble(cg, 97, 5)  // kCGScrollWheelEventPointDeltaAxis2
	c.setInt(cg, 99, phase) // kCGScrollWheelEventScrollPhase
	c.setInt(cg, 123, momentum)
	ev := send(class("NSEvent"), "eventWithCGEvent:", cg)
	if ev == 0 || sendInt(ev, "type") != 22 {
		t.Fatal("could not make a scroll event")
	}
	return ev
}

// gesture is a gesture event of the HID system's type hid (8 zoom, 16
// swipe), which AppKit makes a magnify or a swipe event of.
func (c *cgEvents) gesture(hid int64, set func(cg uintptr)) id {
	cg := c.create(0)
	cgEventSetType(cg, 29) // kCGSEventGesture
	c.setInt(cg, 110, hid) // gesture type
	set(cg)
	return send(class("NSEvent"), "eventWithCGEvent:", cg)
}

// A touchpad's scroll carries its gesture's phase, and its glide its own;
// a mouse wheel's has none.
func TestScrollGesturePhase(t *testing.T) {
	c := newCGEvents()
	withPool(func() {
		view := msgInitRect(send(class("NSView"), "alloc"), sel("initWithFrame:"), NSRect{Size: NSSize{Width: 400, Height: 300}})
		defer release(view)
		h := &gestureHandler{take: true}
		s := &surface{w: &window{h: h}, view: view}
		for _, tc := range []struct {
			phase, momentum int64
			want            platform.GesturePhase
			glide           bool
		}{
			{1, 0, platform.GestureBegan, false},   // kCGScrollPhaseBegan
			{2, 0, platform.GestureChanged, false}, // kCGScrollPhaseChanged
			{4, 0, platform.GestureEnded, false},   // kCGScrollPhaseEnded
			{8, 0, platform.GestureEnded, false},   // kCGScrollPhaseCancelled
			{128, 0, platform.GestureChanged, false},
			{0, 1, platform.GestureBegan, true}, // kCGMomentumScrollPhaseBegin
			{0, 2, platform.GestureChanged, true},
			{0, 3, platform.GestureEnded, true},
		} {
			h.events = nil
			scrollWheel(s, c.scroll(t, tc.phase, tc.momentum))
			if len(h.events) != 1 {
				t.Fatalf("phase %d momentum %d: %d events", tc.phase, tc.momentum, len(h.events))
			}
			if e := h.events[0]; e.Kind != platform.PointerScroll || !e.Precise || e.Phase != tc.want || e.Momentum != tc.glide {
				t.Errorf("phase %d momentum %d: %+v, want phase %d momentum %v", tc.phase, tc.momentum, e, tc.want, tc.glide)
			}
		}
	})
}

// A pinch carries its phase; a swipe goes to the content, and back or
// forward only when the content leaves it.
func TestMagnifyAndSwipe(t *testing.T) {
	c := newCGEvents()
	withPool(func() {
		view := msgInitRect(send(class("NSView"), "alloc"), sel("initWithFrame:"), NSRect{Size: NSSize{Width: 400, Height: 300}})
		defer release(view)
		h := &gestureHandler{take: true}
		s := &surface{w: &window{h: h}, view: view}
		for _, tc := range []struct {
			phase int64
			want  platform.GesturePhase
		}{{1, platform.GestureBegan}, {2, platform.GestureChanged}, {4, platform.GestureEnded}, {8, platform.GestureEnded}} {
			ev := c.gesture(8, func(cg uintptr) {
				c.setDouble(cg, 113, 0.25)  // zoom value
				c.setInt(cg, 132, tc.phase) // gesture phase (IOHIDEventPhase)
			})
			if ev == 0 || sendInt(ev, "type") != 30 { // NSEventTypeMagnify
				t.Skip("this macOS makes no magnify event of a zoom gesture")
			}
			h.events = nil
			magnify(s, ev)
			if len(h.events) != 1 || h.events[0].Kind != platform.PointerZoom || h.events[0].Scale != 1.25 || h.events[0].Phase != tc.want {
				t.Errorf("pinch of phase %d: %+v, want scale 1.25, phase %d", tc.phase, h.events, tc.want)
			}
		}

		left := c.gesture(16, func(cg uintptr) { c.setInt(cg, 115, 1<<2) }) // kIOHIDSwipeLeft
		if left == 0 || sendInt(left, "type") != 31 {                       // NSEventTypeSwipe
			t.Skip("this macOS makes no swipe event of a swipe gesture")
		}
		back, _ := swipeBack(msgFloat(left, sel("deltaX")))
		h.events, h.take = nil, true
		swipe(s, left)
		if len(h.events) != 1 || h.events[0].Kind != platform.PointerSwipe || (h.events[0].DX < 0) != back {
			t.Fatalf("a swipe the content took: %+v (back %v)", h.events, back)
		}
		h.events, h.take = nil, false
		swipe(s, left)
		want := platform.KeyForward
		if back {
			want = platform.KeyBack
		}
		if len(h.events) != 3 || h.events[1].Kind != platform.KeyPressed || h.events[1].Key != want || h.events[2].Kind != platform.KeyReleased {
			t.Fatalf("a swipe the content left: %+v, want it then %v pressed and released", h.events, want)
		}
	})
}
