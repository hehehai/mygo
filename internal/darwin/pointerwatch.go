package darwin

import (
	"math"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/internal/platform"
)

// The pointer as it moves in any app (platform.PointerWatcher): AppKit
// sends an app's windows no mouse moves while another app is active (but
// to the tracking areas of panels and flyouts, which track always), but
// its event monitors see them. One global monitor (the events of other
// apps) and one local monitor (the app's own) serve all watchers; they
// are made with the first watcher, and kept. While the app is active the window server sends the moves to its
// key window alone, and to nobody without one (the app active with its
// windows closed): then the pointer is read 30 times a second while it
// moves, 4 times once it has rested half a second (a move seen there, or
// by a monitor, reads it 30 times again), and not at all without a
// watcher.

// pointerWatchers are the watchers' callbacks by id. Main thread only.
var (
	pointerWatchers = map[int]func(platform.Point, bool){}
	pointerWatchID  int
	pointerMonitors [2]id
	// pointerPoll reads the pointer while the app is active; polled is
	// where it was last read.
	pointerPoll id
	polled      NSPoint
	// pollSlow: the pointer rested (still polls in a row), it is read
	// pollSlowEvery.
	pollSlow bool
	still    int
	// dragCount is the drag pasteboard's change count as the left button
	// went down; dragging tells that it changed since (content is being
	// dragged), until the button goes up.
	dragCount int
	dragging  bool
	// dragEnd checks the left button while content is dragged: the
	// release that ends a drag may come to no monitor (the source's drag
	// session takes it; the app is not active, so nothing polls).
	dragEnd id
)

// NSEventMask: left mouse down, up, dragged; mouse moved.
const pointerMask = 1<<1 | 1<<2 | 1<<6 | 1<<5

// How often the pointer is read while the app is active: every
// pollEvery while it moves, every pollSlowEvery after pollRest reads
// that found it where it was.
const (
	pollEvery     = 1.0 / 30
	pollSlowEvery = 1.0 / 4
	pollRest      = 15
)

// WatchPointer calls fn with the pointer (screen coordinates) and whether
// content is being dragged, on the main thread, at every move, drag,
// press and release of the left button, in any app.
func (screen) WatchPointer(fn func(p platform.Point, dragging bool)) (stop func()) {
	pointerWatchID++
	n := pointerWatchID
	pointerWatchers[n] = fn
	watchPointer()
	if theBackend != nil && sendBool(theBackend.app, "isActive") {
		pollPointer(true)
	}
	return func() {
		delete(pointerWatchers, n)
		if len(pointerWatchers) == 0 {
			pollPointer(false) // nobody to tell
		}
	}
}

// watchPointer makes the monitors once.
func watchPointer() {
	if pointerMonitors[0] != 0 {
		return
	}
	global := newBlock(func(_ objc.Block, ev id) { pointerEvent(ev) })
	local := newBlock(func(_ objc.Block, ev id) id {
		pointerEvent(ev)
		return ev
	})
	ev := class("NSEvent")
	pointerMonitors[0] = retain(send(ev, "addGlobalMonitorForEventsMatchingMask:handler:", pointerMask, uintptr(global)))
	pointerMonitors[1] = retain(send(ev, "addLocalMonitorForEventsMatchingMask:handler:", pointerMask, uintptr(local)))
	center := send(class("NSNotificationCenter"), "defaultCenter")
	queue := send(class("NSOperationQueue"), "mainQueue")
	active := newBlock(func(_ objc.Block, n id) { pollPointer(len(pointerWatchers) > 0) })
	inactive := newBlock(func(_ objc.Block, n id) { pollPointer(false) })
	send(center, "addObserverForName:object:queue:usingBlock:", uintptr(nsString("NSApplicationDidBecomeActiveNotification")), 0, uintptr(queue), uintptr(active))
	send(center, "addObserverForName:object:queue:usingBlock:", uintptr(nsString("NSApplicationDidResignActiveNotification")), 0, uintptr(queue), uintptr(inactive))
}

// pollPointer starts or stops reading the pointer (the app active, a
// watcher).
func pollPointer(on bool) {
	if !on {
		if pointerPoll != 0 {
			send(pointerPoll, "invalidate")
			release(pointerPoll)
			pointerPoll = 0
		}
		return
	}
	if pointerPoll != 0 {
		return
	}
	pollStart(false)
}

// pollStart reads the pointer every pollEvery, or pollSlowEvery (slow).
func pollStart(slow bool) {
	loadSurface() // msgTimerBlock
	pollSlow, still = slow, 0
	blk := newBlock(func(_ objc.Block, t id) {
		mac := msgPoint(class("NSEvent"), sel("mouseLocation"))
		if mac == polled {
			if still++; still == pollRest && !pollSlow {
				pollPace(true) // rested
			}
			return
		}
		polled = mac
		pollPace(false)
		pointerAt(mac)
	})
	every := pollEvery
	if slow {
		every = pollSlowEvery
	}
	pointerPoll = retain(msgTimerBlock(class("NSTimer"), sel("timerWithTimeInterval:repeats:block:"), every, true, uintptr(blk)))
	msgSetFloat(pointerPoll, sel("setTolerance:"), every/5) // fires with the system's other timers
	send(send(class("NSRunLoop"), "mainRunLoop"), "addTimer:forMode:", uintptr(pointerPoll), kCFRunLoopCommonModes)
}

// pollPace reads the pointer slowly (it rested) or fast (it moved), if
// it is read.
func pollPace(slow bool) {
	if !slow {
		still = 0
	}
	if pointerPoll == 0 || pollSlow == slow {
		return
	}
	send(pointerPoll, "invalidate")
	release(pointerPoll)
	pollStart(slow)
}

// pointerEvent tells the watchers of an event of a monitor.
func pointerEvent(ev id) {
	switch sendInt(ev, "type") {
	case 1: // NSEventTypeLeftMouseDown
		dragCount, dragging = dragPasteboardCount(), false
	case 2: // NSEventTypeLeftMouseUp
		dragging = false
	case 6: // NSEventTypeLeftMouseDragged
		if !dragging && dragPasteboardCount() != dragCount {
			dragging = true
			watchDragEnd()
		}
	}
	mac := msgPoint(class("NSEvent"), sel("mouseLocation"))
	polled = mac
	pollPace(false) // it moves: read it often again
	pointerAt(mac)
}

// pointerAt tells the watchers the pointer is at mac (AppKit's
// coordinates).
func pointerAt(mac NSPoint) {
	if len(pointerWatchers) == 0 {
		return
	}
	p := platform.Point{X: int(math.Floor(mac.X)), Y: int(math.Floor(primaryScreenHeight() - mac.Y))}
	for _, fn := range pointerWatchers {
		fn(p, dragging)
	}
}

// watchDragEnd reads the left button 20 times a second while content is
// dragged, and tells the watchers once it is up.
func watchDragEnd() {
	if dragEnd != 0 {
		return
	}
	loadSurface() // msgTimerBlock
	blk := newBlock(func(_ objc.Block, t id) {
		if dragging && !dragStuck(dragging, sendInt(class("NSEvent"), "pressedMouseButtons")) {
			return
		}
		send(dragEnd, "invalidate")
		release(dragEnd)
		dragEnd = 0
		if dragging {
			dragging = false
			mac := msgPoint(class("NSEvent"), sel("mouseLocation"))
			polled = mac
			pointerAt(mac)
		}
	})
	dragEnd = retain(msgTimerBlock(class("NSTimer"), sel("timerWithTimeInterval:repeats:block:"), 1.0/20, true, uintptr(blk)))
	send(send(class("NSRunLoop"), "mainRunLoop"), "addTimer:forMode:", uintptr(dragEnd), kCFRunLoopCommonModes)
}

// RefreshPointer reads the pointer and the buttons now
// (platform.PointerRefresher), and repairs the watching: a drag believed
// in with the primary button up ends (its release came to no monitor and
// the drag's timer did not see it), the pointer is read on a timer again
// if the app is active with watchers and none runs, and the monitors are
// made if they are missing. The watchers are not told: the caller is.
func (screen) RefreshPointer() platform.PointerNow {
	mac := msgPoint(class("NSEvent"), sel("mouseLocation"))
	n := platform.PointerNow{
		Point:   platform.Point{X: int(math.Floor(mac.X)), Y: int(math.Floor(primaryScreenHeight() - mac.Y))},
		Buttons: sendInt(class("NSEvent"), "pressedMouseButtons"),
		Active:  theBackend != nil && sendBool(theBackend.app, "isActive"),
	}
	if dragStuck(dragging, n.Buttons) {
		dragging, n.Unstuck = false, true
		if dragEnd != 0 {
			send(dragEnd, "invalidate")
			release(dragEnd)
			dragEnd = 0
		}
	}
	if len(pointerWatchers) > 0 {
		if pointerMonitors[0] == 0 {
			watchPointer()
		}
		if n.Active && pointerPoll == 0 {
			pollPointer(true)
			n.Repolled = true
		}
	}
	n.Dragging, n.Polling, n.DragTimer = dragging, pointerPoll != 0, dragEnd != 0
	n.Monitors, n.Watchers = pointerMonitors[0] != 0 && pointerMonitors[1] != 0, len(pointerWatchers)
	return n
}

// dragStuck reports whether a drag believed in (dragging) is over: the
// primary button is not down (buttons, NSEvent.pressedMouseButtons).
func dragStuck(dragging bool, buttons int) bool { return dragging && buttons&1 == 0 }

func dragPasteboardCount() int {
	pb := send(class("NSPasteboard"), "pasteboardWithName:", uintptr(nsString("Apple CFPasteboard drag")))
	if pb == 0 {
		return 0
	}
	return sendInt(pb, "changeCount")
}
