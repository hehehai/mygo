package windows

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

type pointerReport struct {
	p        platform.Point
	dragging bool
}

type testPointerEnv struct {
	sample     pointerSample
	buttons    int
	active     bool
	toDIP      func(platform.Point) platform.Point
	toDIPCalls int
	startCalls []uint32
	stopCalls  int
	startOK    bool
	core       *pointerWatchCore
}

func newTestPointerEnv(initial platform.Point, space *dipSpace) *testPointerEnv {
	e := &testPointerEnv{
		sample:  pointerSample{phys: initial, ok: true},
		startOK: true,
	}
	if space != nil {
		e.toDIP = func(p platform.Point) platform.Point {
			e.toDIPCalls++
			return space.pointToDIP(p.X, p.Y)
		}
	} else {
		e.toDIP = func(p platform.Point) platform.Point {
			e.toDIPCalls++
			return p
		}
	}
	e.core = newPointerWatchCore(pointerWatchHooks{
		sample:   func() pointerSample { return e.sample },
		toDIP:    func(p platform.Point) platform.Point { return e.toDIP(p) },
		buttons:  func() int { return e.buttons },
		isActive: func() bool { return e.active },
		startTimer: func(intervalMs uint32) bool {
			if !e.startOK {
				return false
			}
			e.startCalls = append(e.startCalls, intervalMs)
			return true
		},
		stopTimer: func() {
			e.stopCalls++
		},
	})
	return e
}

func TestPointerWatchDeduplicationAndIdleCadence(t *testing.T) {
	// 200% scaling: two physical pixels map to the same DIP.
	space := newDIPSpace([]screenDisplay{{
		bounds: platform.Rect{X: 0, Y: 0, Width: 3840, Height: 2160},
		work:   platform.Rect{X: 0, Y: 0, Width: 3840, Height: 2080},
		dpi:    192,
	}}, 96)
	env := newTestPointerEnv(platform.Point{X: 200, Y: 100}, &space)

	var got []pointerReport
	stop := env.core.watch(func(p platform.Point, dragging bool) {
		got = append(got, pointerReport{p: p, dragging: dragging})
	})
	defer stop()

	// Starting the watcher seeds the initial position without a callback and
	// begins polling at fast cadence.
	if len(got) != 0 {
		t.Fatalf("initial reports = %+v, want none", got)
	}
	if !reflect.DeepEqual(env.startCalls, []uint32{pointerPollFastMs}) {
		t.Fatalf("startCalls = %v, want [%d]", env.startCalls, pointerPollFastMs)
	}
	seedDIPCalls := env.toDIPCalls

	// Stationary ticks emit zero callbacks and skip display/DIP conversion.
	for i := 0; i < pointerPollRest-1; i++ {
		env.core.tick()
	}
	if len(got) != 0 {
		t.Fatalf("stationary reports = %+v, want none", got)
	}
	if env.toDIPCalls != seedDIPCalls {
		t.Fatalf("toDIPCalls = %d while stationary, want %d (no display enumeration)", env.toDIPCalls, seedDIPCalls)
	}
	if env.core.intervalMs != pointerPollFastMs {
		t.Fatalf("intervalMs before rest threshold = %d, want %d", env.core.intervalMs, pointerPollFastMs)
	}

	// Reaching pointerPollRest slows the timer to pointerPollSlowMs.
	env.core.tick()
	if env.core.intervalMs != pointerPollSlowMs {
		t.Fatalf("intervalMs after rest = %d, want %d", env.core.intervalMs, pointerPollSlowMs)
	}
	// Further stationary ticks do not re-arm the timer or emit callbacks.
	startCount := len(env.startCalls)
	for i := 0; i < 10; i++ {
		env.core.tick()
	}
	if len(env.startCalls) != startCount {
		t.Fatalf("extra startTimer calls while resting: %v", env.startCalls)
	}
	if len(got) != 0 {
		t.Fatalf("resting reports = %+v, want none", got)
	}

	// Sub-DIP physical move (200 -> 201 at 200% DPI still maps to DIP 100):
	// wakes fast cadence without emitting a duplicate DIP callback.
	env.sample.phys = platform.Point{X: 201, Y: 100}
	env.core.tick()
	if env.core.intervalMs != pointerPollFastMs {
		t.Fatalf("intervalMs after sub-DIP move = %d, want %d", env.core.intervalMs, pointerPollFastMs)
	}
	if len(got) != 0 {
		t.Fatalf("sub-DIP move emitted duplicate report: %+v", got)
	}

	// Moving to a new DIP coordinate emits once.
	env.sample.phys = platform.Point{X: 202, Y: 10}
	env.core.tick()
	env.core.tick() // same pixel again: deduplicated
	want := []pointerReport{{p: platform.Point{X: 101, Y: 5}, dragging: false}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reports = %+v, want %+v", got, want)
	}
}

func TestPointerWatchMixedDPICoordinates(t *testing.T) {
	// Primary 4K 200% at (0,0), secondary 1080p 100% to the right at (3840,0),
	// third 150% to the left at (-2880,0).
	displays := []screenDisplay{
		{
			bounds: platform.Rect{X: 0, Y: 0, Width: 3840, Height: 2160},
			work:   platform.Rect{X: 0, Y: 0, Width: 3840, Height: 2080},
			dpi:    192,
		},
		{
			bounds: platform.Rect{X: 3840, Y: 0, Width: 1920, Height: 1080},
			work:   platform.Rect{X: 3840, Y: 0, Width: 1920, Height: 1040},
			dpi:    96,
		},
		{
			bounds: platform.Rect{X: -2880, Y: 0, Width: 2880, Height: 1800},
			work:   platform.Rect{X: -2880, Y: 0, Width: 2880, Height: 1740},
			dpi:    144,
		},
	}
	space := newDIPSpace(displays, 96)
	env := newTestPointerEnv(platform.Point{X: 1000, Y: 1000}, &space)

	var got []pointerReport
	stop := env.core.watch(func(p platform.Point, dragging bool) {
		got = append(got, pointerReport{p: p, dragging: dragging})
	})
	defer stop()

	// Move near the top-center of every display (shelf hot zone) and confirm
	// every reported Point matches space.pointToDIP and lies inside Display.Bounds.
	for i, d := range displays {
		bounds, _ := space.displayDIPs(i)
		topCenterPx := platform.Point{
			X: d.bounds.X + d.bounds.Width/2,
			Y: d.bounds.Y + 4,
		}
		env.sample.phys = topCenterPx
		env.core.tick()

		if len(got) != i+1 {
			t.Fatalf("display %d: got %d reports, want %d", i, len(got), i+1)
		}
		p := got[i].p
		wantP := space.pointToDIP(topCenterPx.X, topCenterPx.Y)
		if p != wantP {
			t.Errorf("display %d: point = %+v, want %+v", i, p, wantP)
		}
		if p.X < bounds.X || p.X >= bounds.X+bounds.Width || p.Y < bounds.Y || p.Y >= bounds.Y+bounds.Height {
			t.Errorf("display %d: point %+v outside Display.Bounds %+v", i, p, bounds)
		}
	}
}

func TestPointerWatchDragStartAndRelease(t *testing.T) {
	env := newTestPointerEnv(platform.Point{X: 500, Y: 400}, nil)
	var got []pointerReport
	stop := env.core.watch(func(p platform.Point, dragging bool) {
		got = append(got, pointerReport{p: p, dragging: dragging})
	})
	defer stop()

	// Press primary button without moving: no drag yet, stays on fast cadence
	// even if held still longer than pointerPollRest.
	env.sample.primaryDown = true
	for i := 0; i < pointerPollRest+5; i++ {
		env.core.tick()
	}
	if len(got) != 0 {
		t.Fatalf("press without move reported %+v, want none", got)
	}
	if env.core.intervalMs != pointerPollFastMs {
		t.Fatalf("intervalMs while button held = %d, want %d", env.core.intervalMs, pointerPollFastMs)
	}

	// Small click jitter below pointerDragThresholdDIP stays Dragging: false.
	env.sample.phys = platform.Point{X: 502, Y: 401}
	env.core.tick()
	env.sample.primaryDown = false
	env.core.tick()

	// Press and drag past threshold toward the top shelf.
	env.sample.phys = platform.Point{X: 502, Y: 401}
	env.sample.primaryDown = true
	env.core.tick()
	env.sample.phys = platform.Point{X: 502, Y: 397} // |dy| == 4 >= threshold
	env.core.tick()
	env.sample.phys = platform.Point{X: 700, Y: 6} // over top shelf
	env.core.tick()

	// Pause mid-drag over the shelf: zero duplicate callbacks, stays fast.
	for i := 0; i < pointerPollRest+5; i++ {
		env.core.tick()
	}
	if env.core.intervalMs != pointerPollFastMs {
		t.Fatalf("intervalMs while dragging = %d, want %d", env.core.intervalMs, pointerPollFastMs)
	}

	// Moving back to press origin while still held keeps Dragging: true.
	env.sample.phys = platform.Point{X: 502, Y: 401}
	env.core.tick()

	// Release primary button while stationary: must immediately emit Dragging: false.
	env.sample.primaryDown = false
	env.core.tick()
	env.core.tick() // stationary after release: no duplicate

	want := []pointerReport{
		{p: platform.Point{X: 502, Y: 401}, dragging: false},
		{p: platform.Point{X: 502, Y: 397}, dragging: true},
		{p: platform.Point{X: 700, Y: 6}, dragging: true},
		{p: platform.Point{X: 502, Y: 401}, dragging: true},
		{p: platform.Point{X: 502, Y: 401}, dragging: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reports = %+v, want %+v", got, want)
	}
}

func TestPointerWatchButtonAlreadyDownAtStart(t *testing.T) {
	env := newTestPointerEnv(platform.Point{X: 300, Y: 300}, nil)
	env.sample.primaryDown = true

	var got []pointerReport
	stop := env.core.watch(func(p platform.Point, dragging bool) {
		got = append(got, pointerReport{p: p, dragging: dragging})
	})
	defer stop()

	env.sample.phys = platform.Point{X: 310, Y: 280}
	env.core.tick()
	env.sample.primaryDown = false
	env.core.tick()

	want := []pointerReport{
		{p: platform.Point{X: 310, Y: 280}, dragging: true},
		{p: platform.Point{X: 310, Y: 280}, dragging: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reports = %+v, want %+v", got, want)
	}
}

func TestPointerWatchCursorReadFailure(t *testing.T) {
	env := newTestPointerEnv(platform.Point{X: 640, Y: 10}, nil)
	var got []pointerReport
	stop := env.core.watch(func(p platform.Point, dragging bool) {
		got = append(got, pointerReport{p: p, dragging: dragging})
	})
	defer stop()

	// Start a drag first.
	env.sample.primaryDown = true
	env.core.tick()
	env.sample.phys = platform.Point{X: 640, Y: 2}
	env.core.tick()

	// Simulate UAC / lock screen: GetCursorPos fails (returns 0,0, ok=false)
	// and primary button is released. Must NOT jump to (0,0), must end drag
	// at last known coordinate.
	env.sample = pointerSample{phys: platform.Point{}, ok: false, primaryDown: false}
	env.core.tick()
	// Repeated failures while idle emit nothing.
	env.core.tick()

	want := []pointerReport{
		{p: platform.Point{X: 640, Y: 2}, dragging: true},
		{p: platform.Point{X: 640, Y: 2}, dragging: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reports = %+v, want %+v", got, want)
	}
}

func TestPointerWatchStopIdempotentAndNoLeak(t *testing.T) {
	beforeG := runtime.NumGoroutine()
	env := newTestPointerEnv(platform.Point{X: 100, Y: 100}, nil)

	var got1, got2 []pointerReport
	stop1 := env.core.watch(func(p platform.Point, dragging bool) {
		got1 = append(got1, pointerReport{p: p, dragging: dragging})
	})
	stop2 := env.core.watch(func(p platform.Point, dragging bool) {
		got2 = append(got2, pointerReport{p: p, dragging: dragging})
	})

	// Second watcher shares the single active timer.
	if len(env.startCalls) != 1 {
		t.Fatalf("startCalls = %v, want 1 timer shared by both watchers", env.startCalls)
	}

	env.sample.phys = platform.Point{X: 120, Y: 100}
	env.core.tick()
	if len(got1) != 1 || len(got2) != 1 {
		t.Fatalf("both watchers should receive move: got1=%+v got2=%+v", got1, got2)
	}

	// Stopping watcher 1 twice is idempotent and leaves watcher 2 active.
	stop1()
	stop1()
	if env.stopCalls != 0 || !env.core.polling {
		t.Fatalf("timer stopped while watcher 2 still active: stopCalls=%d polling=%v", env.stopCalls, env.core.polling)
	}

	env.sample.phys = platform.Point{X: 140, Y: 100}
	env.core.tick()
	if len(got1) != 1 || len(got2) != 2 {
		t.Fatalf("after stop1: got1=%+v got2=%+v", got1, got2)
	}

	// Stopping watcher 2 stops the timer cleanly and is idempotent.
	stop2()
	stop2()
	if env.stopCalls != 1 || env.core.polling || len(env.core.watchers) != 0 {
		t.Fatalf("after all stopped: stopCalls=%d polling=%v watchers=%d", env.stopCalls, env.core.polling, len(env.core.watchers))
	}

	// Stray ticks after stop do nothing.
	env.sample.phys = platform.Point{X: 200, Y: 200}
	env.core.tick()
	if len(got1) != 1 || len(got2) != 2 {
		t.Fatalf("reports after stop: got1=%+v got2=%+v", got1, got2)
	}

	// Re-subscribing later starts fresh.
	var got3 []pointerReport
	stop3 := env.core.watch(func(p platform.Point, dragging bool) {
		got3 = append(got3, pointerReport{p: p, dragging: dragging})
	})
	if len(env.startCalls) != 2 || !env.core.polling {
		t.Fatalf("re-subscribe did not restart timer: startCalls=%v polling=%v", env.startCalls, env.core.polling)
	}
	env.sample.phys = platform.Point{X: 220, Y: 200}
	env.core.tick()
	if len(got3) != 1 || got3[0].p != (platform.Point{X: 220, Y: 200}) {
		t.Fatalf("re-subscribe reports = %+v", got3)
	}
	stop3()
	if env.stopCalls != 2 || env.core.polling {
		t.Fatalf("final stop: stopCalls=%d polling=%v", env.stopCalls, env.core.polling)
	}

	if afterG := runtime.NumGoroutine(); afterG > beforeG {
		t.Fatalf("goroutines grew from %d to %d: pointer watch must spawn zero goroutines", beforeG, afterG)
	}
}

func TestPointerWatchReentrantStop(t *testing.T) {
	env := newTestPointerEnv(platform.Point{X: 10, Y: 10}, nil)
	var stopA, stopB func()
	var calledA, calledB int

	stopA = env.core.watch(func(platform.Point, bool) {
		calledA++
		stopA()
		stopB()
	})
	stopB = env.core.watch(func(platform.Point, bool) {
		calledB++
		stopA()
		stopB()
	})

	env.sample.phys = platform.Point{X: 50, Y: 10}
	env.core.tick()

	// Exactly one watcher runs and cancels both; neither runs twice and the
	// timer stays stopped.
	if calledA+calledB != 1 {
		t.Fatalf("calledA=%d calledB=%d, want total 1", calledA, calledB)
	}
	if env.core.polling || env.stopCalls != 1 {
		t.Fatalf("after reentrant stop: polling=%v stopCalls=%d", env.core.polling, env.stopCalls)
	}
}

func TestPointerWatchRefresh(t *testing.T) {
	env := newTestPointerEnv(platform.Point{X: 700, Y: 10}, nil)
	env.active = true
	env.startOK = false // simulate timer unavailable at initial watch

	stop := env.core.watch(func(platform.Point, bool) {})
	defer stop()

	if env.core.polling {
		t.Fatal("polling should be false when startTimer failed")
	}

	// Refresh repairs missing timer (Repolled) and reports live state.
	env.startOK = true
	env.sample.phys = platform.Point{X: 720, Y: 8}
	env.buttons = 1
	n := env.core.refresh()
	if !n.Repolled || !n.Polling || !n.Active || n.Watchers != 1 || n.Buttons != 1 || n.Point != (platform.Point{X: 720, Y: 8}) || n.Unstuck {
		t.Fatalf("refresh after missing timer = %+v", n)
	}

	// Second refresh does not repoll.
	n = env.core.refresh()
	if n.Repolled || !n.Polling {
		t.Fatalf("second refresh = %+v", n)
	}

	// Real drag with primary button still down stays dragging.
	env.core.dragging = true
	env.buttons = 1
	n = env.core.refresh()
	if n.Unstuck || !n.Dragging {
		t.Fatalf("real drag refresh = %+v", n)
	}

	// Stuck drag with primary button up (even if secondary button is down) ends.
	env.buttons = 2
	n = env.core.refresh()
	if !n.Unstuck || n.Dragging || env.core.dragging {
		t.Fatalf("stuck drag refresh = %+v, dragging=%v", n, env.core.dragging)
	}
	// Repeating refresh reports Unstuck false.
	n = env.core.refresh()
	if n.Unstuck || n.Dragging {
		t.Fatalf("second unstuck refresh = %+v", n)
	}

	// Once all watchers stop, refresh does not restart the timer.
	stop()
	n = env.core.refresh()
	if n.Polling || n.Repolled || n.Watchers != 0 {
		t.Fatalf("refresh with no watchers = %+v", n)
	}
}
