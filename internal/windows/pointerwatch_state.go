package windows

// This file has no build constraint: the pointer watcher's state machine
// (deduplication, drag detection, adaptive polling cadence, and watcher
// lifecycle) is plain Go, tested on every platform.

import "github.com/egoist/mygo/internal/platform"

const (
	// pointerPollFastMs is the timer interval in milliseconds while the
	// pointer moves or the primary button is held (~32 Hz on Windows'
	// 15.6 ms system clock, matching macOS's 30 Hz active polling).
	pointerPollFastMs = 30
	// pointerPollSlowMs is the timer interval once the pointer has rested
	// with the primary button up for pointerPollRest consecutive ticks.
	pointerPollSlowMs = 250
	// pointerPollRest is the number of stationary ticks before slowing the
	// timer down from pointerPollFastMs to pointerPollSlowMs (~0.5 s).
	pointerPollRest = 15
	// pointerDragThresholdDIP is the minimum travel in DIPs along either
	// axis while the primary button is held before Dragging becomes true.
	// Matches Windows' default SM_CXDRAG/SM_CYDRAG (4 DIPs) so normal
	// clicks inside an open shelf do not trigger drop mode.
	pointerDragThresholdDIP = 4
)

// pointerSample is one raw reading of the pointer and primary mouse button.
type pointerSample struct {
	phys        platform.Point
	ok          bool // false if GetCursorPos failed (e.g. UAC / locked desktop)
	primaryDown bool
}

// pointerWatchHooks supplies the OS operations for pointerWatchCore.
type pointerWatchHooks struct {
	sample     func() pointerSample
	toDIP      func(phys platform.Point) platform.Point
	buttons    func() int
	isActive   func() bool
	startTimer func(intervalMs uint32) bool
	stopTimer  func()
}

// pointerWatchCore runs Screen.WatchPointer on Windows without global mouse
// hooks: a main-thread timer reads GetCursorPos and the primary button state,
// slows down while stationary, deduplicates DIP positions, and tracks drags
// across all applications. Main thread only.
type pointerWatchCore struct {
	hooks pointerWatchHooks

	nextID   int
	watchers map[int]func(platform.Point, bool)

	polling    bool
	intervalMs uint32
	still      int

	hasLast     bool
	lastPhys    platform.Point
	lastDIP     platform.Point
	lastDown    bool
	pressed     bool
	pressOrigin platform.Point
	dragging    bool
}

func newPointerWatchCore(hooks pointerWatchHooks) *pointerWatchCore {
	return &pointerWatchCore{
		hooks:    hooks,
		watchers: map[int]func(platform.Point, bool){},
	}
}

func (c *pointerWatchCore) resetState() {
	c.hasLast = false
	c.lastPhys = platform.Point{}
	c.lastDIP = platform.Point{}
	c.lastDown = false
	c.pressed = false
	c.pressOrigin = platform.Point{}
	c.dragging = false
	c.still = 0
}

// watch registers fn and starts polling when the first watcher is added.
// The returned stop function is idempotent and stops the timer once the
// last watcher leaves.
func (c *pointerWatchCore) watch(fn func(platform.Point, bool)) (stop func()) {
	if c.watchers == nil {
		c.watchers = map[int]func(platform.Point, bool){}
	}
	wasEmpty := len(c.watchers) == 0
	c.nextID++
	id := c.nextID
	c.watchers[id] = fn

	if wasEmpty {
		c.seed()
		c.still = 0
		c.setPolling(pointerPollFastMs)
	}

	stopped := false
	return func() {
		if stopped {
			return
		}
		stopped = true
		delete(c.watchers, id)
		if len(c.watchers) == 0 {
			c.stopPolling()
			c.resetState()
		}
	}
}

func (c *pointerWatchCore) seed() {
	if c.hooks.sample == nil {
		c.resetState()
		return
	}
	s := c.hooks.sample()
	if !s.ok {
		c.resetState()
		return
	}
	dip := c.toDIP(s.phys)
	c.hasLast = true
	c.lastPhys = s.phys
	c.lastDIP = dip
	c.lastDown = s.primaryDown
	c.pressed = s.primaryDown
	c.pressOrigin = dip
	c.dragging = false
}

func (c *pointerWatchCore) toDIP(phys platform.Point) platform.Point {
	if c.hooks.toDIP == nil {
		return phys
	}
	return c.hooks.toDIP(phys)
}

func (c *pointerWatchCore) setPolling(intervalMs uint32) bool {
	if c.polling && c.intervalMs == intervalMs {
		return true
	}
	if c.hooks.startTimer == nil || !c.hooks.startTimer(intervalMs) {
		return false
	}
	c.polling = true
	c.intervalMs = intervalMs
	return true
}

func (c *pointerWatchCore) stopPolling() {
	if !c.polling {
		return
	}
	c.polling = false
	c.intervalMs = 0
	c.still = 0
	if c.hooks.stopTimer != nil {
		c.hooks.stopTimer()
	}
}

func (c *pointerWatchCore) noteIdle() {
	c.still++
	if c.still >= pointerPollRest && c.intervalMs != pointerPollSlowMs {
		c.setPolling(pointerPollSlowMs)
	}
}

// tick runs on each timer wakeup. It emits no callback while stationary and
// avoids display enumeration when neither physical position nor button state
// changed.
func (c *pointerWatchCore) tick() {
	if len(c.watchers) == 0 {
		c.stopPolling()
		return
	}
	if c.hooks.sample == nil {
		return
	}
	s := c.hooks.sample()
	if !s.ok {
		if c.dragging && !s.primaryDown {
			c.dragging = false
			c.pressed = false
			c.lastDown = false
			c.still = 0
			c.dispatch(c.lastDIP, false)
			return
		}
		if !c.dragging && !s.primaryDown {
			c.noteIdle()
		}
		return
	}

	if c.hasLast && s.phys == c.lastPhys && s.primaryDown == c.lastDown {
		if !s.primaryDown && !c.dragging {
			c.noteIdle()
		} else {
			c.still = 0
			if c.intervalMs != pointerPollFastMs {
				c.setPolling(pointerPollFastMs)
			}
		}
		return
	}

	dip := c.toDIP(s.phys)
	c.still = 0
	if c.intervalMs != pointerPollFastMs {
		c.setPolling(pointerPollFastMs)
	}

	if !c.hasLast {
		c.hasLast = true
		c.lastPhys = s.phys
		c.lastDIP = dip
		c.lastDown = s.primaryDown
		c.pressed = s.primaryDown
		c.pressOrigin = dip
		c.dragging = false
		return
	}

	prevDIP := c.lastDIP
	prevDragging := c.dragging
	c.lastPhys = s.phys
	c.lastDIP = dip
	c.lastDown = s.primaryDown

	if s.primaryDown {
		if !c.pressed {
			c.pressed = true
			c.pressOrigin = dip
		} else if !c.dragging && dragExceeded(c.pressOrigin, dip) {
			c.dragging = true
		}
	} else {
		c.pressed = false
		c.dragging = false
	}

	if dip != prevDIP || c.dragging != prevDragging {
		c.dispatch(dip, c.dragging)
	}
}

func (c *pointerWatchCore) dispatch(p platform.Point, dragging bool) {
	if len(c.watchers) == 0 {
		return
	}
	type entry struct {
		id int
		fn func(platform.Point, bool)
	}
	var stack [4]entry
	snapshot := stack[:0]
	for id, fn := range c.watchers {
		if fn != nil {
			snapshot = append(snapshot, entry{id: id, fn: fn})
		}
	}
	for _, e := range snapshot {
		if _, ok := c.watchers[e.id]; !ok {
			continue
		}
		e.fn(p, dragging)
	}
}

// refresh reads the live pointer and button state for platform.PointerRefresher,
// ends a stuck drag if the primary button is up, and restarts the polling timer
// if watchers exist without one. Watchers are not notified directly.
func (c *pointerWatchCore) refresh() platform.PointerNow {
	var s pointerSample
	if c.hooks.sample != nil {
		s = c.hooks.sample()
	}
	pt := c.lastDIP
	if s.ok {
		pt = c.toDIP(s.phys)
	}
	buttons := 0
	if c.hooks.buttons != nil {
		buttons = c.hooks.buttons()
	} else if s.primaryDown {
		buttons = 1
	}
	n := platform.PointerNow{
		Point:   pt,
		Buttons: buttons,
		Active:  c.hooks.isActive != nil && c.hooks.isActive(),
	}
	if c.dragging && buttons&1 == 0 {
		c.dragging = false
		c.pressed = false
		c.lastDown = false
		n.Unstuck = true
	}
	if len(c.watchers) > 0 && !c.polling {
		if c.setPolling(pointerPollFastMs) {
			n.Repolled = true
		}
	}
	n.Dragging = c.dragging
	n.Polling = c.polling
	n.Watchers = len(c.watchers)
	return n
}

func dragExceeded(origin, cur platform.Point) bool {
	dx := cur.X - origin.X
	if dx < 0 {
		dx = -dx
	}
	dy := cur.Y - origin.Y
	if dy < 0 {
		dy = -dy
	}
	return dx >= pointerDragThresholdDIP || dy >= pointerDragThresholdDIP
}
