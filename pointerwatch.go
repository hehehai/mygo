package mygo

import (
	"errors"

	"github.com/egoist/mygo/internal/platform"
)

// ErrPointerWatch is WatchPointer's error where the platform cannot tell
// where the pointer is in other apps.
var ErrPointerWatch = errors.New("mygo: this platform cannot watch the pointer")

// PointerWatch is what a PointerWatcher reports.
type PointerWatch struct {
	// Region is the index of the first of the watcher's regions the
	// pointer is in, -1 none.
	Region int
	// Point is the pointer, in screen coordinates.
	Point Point
	// Dragging tells that the user drags content (files, links, text) with
	// the primary button, from any app, as the drag pasteboard shows.
	Dragging bool
}

// PointerWatcher tells an app when the pointer goes into and out of
// regions of the screen, and when the user starts and stops dragging
// content, in any app, while the app's windows hear nothing of it: a
// shelf that unfolds as the pointer, or a file dragged, comes to the
// notch at the top of the display (Display.Notch) does. Create it with
// Screen.WatchPointer.
type PointerWatcher struct {
	fn      func(PointerWatch)
	regions []Rectangle
	last    PointerWatch
	seen    Point // where the backend last said the pointer was
	stop    func()
}

// WatchPointer calls fn on the main thread as the pointer goes into a
// region of the watcher (SetRegions), into another or out of them, and
// as content starts and stops being dragged, with what it is then. A
// watcher has no regions until SetRegions; Stop ends it. It works on
// macOS (event monitors: no permission is asked) and Windows; elsewhere
// it returns ErrPointerWatch.
func (s *ScreenModule) WatchPointer(fn func(PointerWatch)) (*PointerWatcher, error) {
	needsApp("Screen.WatchPointer")
	w := onMainValue(func() *PointerWatcher {
		pw, ok := backend().Screen().(platform.PointerWatcher)
		if !ok {
			return nil
		}
		w := &PointerWatcher{fn: fn, last: PointerWatch{Region: -1}}
		w.stop = pw.WatchPointer(func(p platform.Point, dragging bool) { w.moved(p, dragging) })
		return w
	})
	if w == nil {
		return nil, ErrPointerWatch
	}
	return w, nil
}

// SetRegions replaces the watcher's regions (screen coordinates). The
// pointer is not looked at again until it moves, or until Refresh.
func (w *PointerWatcher) SetRegions(regions ...Rectangle) {
	if w == nil {
		return
	}
	onMain(func() { w.regions = append(w.regions[:0], regions...) })
}

// Stop ends the watcher.
func (w *PointerWatcher) Stop() {
	if w == nil {
		return
	}
	onMain(func() {
		if w.stop != nil {
			w.stop()
			w.stop = nil
		}
	})
}

// PointerCheck is what PointerWatcher.Refresh found.
type PointerCheck struct {
	// Read: the platform read the pointer (false: it cannot, or the
	// watcher is stopped; nothing else is set but Before).
	Read bool
	// Before is what the watcher had reported last; Now is where the
	// pointer is, in the regions as they are now. Reported: they differ,
	// and the watcher's function was called with Now.
	Before, Now PointerWatch
	Reported    bool
	// Seen is where the backend last said the pointer was (reported or
	// not); Missed: the pointer is elsewhere now, so moves came to no
	// event monitor or timer since (or came just now). A report without
	// Missed comes from regions changed under a pointer at rest.
	Seen   Point
	Missed bool
	// Buttons are the mouse buttons down (bit 0 the primary one).
	Buttons int
	// Unstuck: content was believed dragged with the primary button up
	// (its release came to no event monitor); the drag ended.
	Unstuck bool
	// Active: the app is active. Polling: the pointer is read on a timer,
	// as it is while the app is active. Repolled: that timer was missing,
	// and was started again.
	Active, Polling, Repolled bool
	// Monitors: the event monitors are there; Watchers: how many watchers
	// the app has.
	Monitors bool
	Watchers int
	// DragTimer: the timer that waits for a drag's end runs.
	DragTimer bool
}

// Refresh reads the pointer and the mouse buttons now, as if it moved
// there: what it finds in the regions as they are now (SetRegions does not
// look) is reported if it differs from the last report, and a drag still
// believed in with the primary button up ends. It is cheap (a few reads
// of the window server's state), for an app that checks now and then
// that the watcher is right, or wants to know where the pointer is after
// it changed the regions. On a platform that cannot, it does nothing.
func (w *PointerWatcher) Refresh() PointerCheck {
	if w == nil {
		return PointerCheck{}
	}
	return onMainValue(func() PointerCheck {
		c := PointerCheck{Before: w.last}
		pr, ok := backend().Screen().(platform.PointerRefresher)
		if !ok || w.stop == nil {
			return c
		}
		n := pr.RefreshPointer()
		c.Read, c.Buttons, c.Unstuck = true, n.Buttons, n.Unstuck
		c.Active, c.Polling, c.Repolled = n.Active, n.Polling, n.Repolled
		c.Monitors, c.Watchers, c.DragTimer = n.Monitors, n.Watchers, n.DragTimer
		c.Now = w.at(n.Point, n.Dragging)
		c.Seen, c.Missed = w.seen, Point(n.Point) != w.seen
		c.Reported = w.moved(n.Point, n.Dragging)
		return c
	})
}

// at is what the watcher reports for the pointer at p.
func (w *PointerWatcher) at(p platform.Point, dragging bool) PointerWatch {
	now := PointerWatch{Region: -1, Point: Point(p), Dragging: dragging}
	for i, r := range w.regions {
		if p.X >= r.X && p.Y >= r.Y && p.X < r.X+r.Width && p.Y < r.Y+r.Height {
			now.Region = i
			break
		}
	}
	return now
}

// moved is the backend's report (main thread); it reports whether it
// called the watcher's function.
func (w *PointerWatcher) moved(p platform.Point, dragging bool) bool {
	if w.stop == nil {
		return false
	}
	w.seen = Point(p)
	now := w.at(p, dragging)
	if now.Region == w.last.Region && now.Dragging == w.last.Dragging {
		return false
	}
	w.last = now
	w.fn(now)
	return true
}
