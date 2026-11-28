package windows

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

func TestWheelStateStickyPrecise(t *testing.T) {
	var ws wheelState
	t0 := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)

	// Pure notched mouse wheel without prior fine deltas stays Precise=false.
	r := ws.handleWheel(t0, false, -120, 0, false, 50, 60, 3, 0)
	if r.n != 1 || r.timer != wheelTimerNone {
		t.Fatalf("mouse wheel: n=%d timer=%v, want 1, None", r.n, r.timer)
	}
	if ev := r.events[0]; ev.Kind != platform.PointerScroll || ev.Precise || ev.DY != 99 || ev.DX != 0 {
		t.Fatalf("mouse wheel event = %+v, want Precise=false DY=99", ev)
	}

	// Precision Touchpad fine delta enters sticky precise mode.
	t1 := t0.Add(20 * time.Millisecond)
	r = ws.handleWheel(t1, false, -24, 0, false, 50, 60, 3, 0)
	if r.n != 1 || !r.events[0].Precise {
		t.Fatalf("fine vertical delta: got %+v, want Precise=true", r.events[0])
	}
	if wantDY := 24.0 / 120 * 3 * 33; math.Abs(r.events[0].DY-wantDY) > 1e-9 {
		t.Fatalf("fine vertical DY = %v, want %v", r.events[0].DY, wantDY)
	}

	// An occasional ±120 frame during touchpad scrolling stays Precise=true.
	t2 := t1.Add(16 * time.Millisecond)
	r = ws.handleWheel(t2, false, -120, 0, false, 50, 60, 3, 0)
	if r.n != 1 || !r.events[0].Precise || r.events[0].DY != 99 {
		t.Fatalf("±120 inside sticky window: got %+v, want Precise=true DY=99", r.events[0])
	}

	// A fine horizontal WM_MOUSEHWHEEL refreshes the sticky window.
	t3 := t1.Add(250 * time.Millisecond)
	r = ws.handleWheel(t3, true, 18, 0, false, 50, 60, 3, 0)
	if r.n != 1 || !r.events[0].Precise || r.events[0].DY != 0 {
		t.Fatalf("fine horizontal delta: got %+v, want Precise=true", r.events[0])
	}
	if wantDX := 18.0 / 120 * 3 * 33; math.Abs(r.events[0].DX-wantDX) > 1e-9 {
		t.Fatalf("fine horizontal DX = %v, want %v", r.events[0].DX, wantDX)
	}

	// 200ms after the horizontal fine delta (450ms after t1), ±120 on either
	// axis is still inside the refreshed 300ms window.
	t4 := t3.Add(200 * time.Millisecond)
	r = ws.handleWheel(t4, true, 120, 0, false, 50, 60, 3, 0)
	if r.n != 1 || !r.events[0].Precise || r.events[0].DX != 99 {
		t.Fatalf("horizontal ±120 in refreshed sticky window: got %+v, want Precise=true DX=99", r.events[0])
	}
	r = ws.handleWheel(t4.Add(10*time.Millisecond), false, 120, 0, false, 50, 60, 3, 0)
	if r.n != 1 || !r.events[0].Precise || r.events[0].DY != -99 {
		t.Fatalf("vertical ±120 in refreshed sticky window: got %+v, want Precise=true DY=-99", r.events[0])
	}

	// Once 300ms pass since the last fine delta (t3), ±120 returns to mouse-wheel mode.
	t5 := t3.Add(preciseStickyWindow + time.Millisecond)
	r = ws.handleWheel(t5, false, -120, 0, false, 50, 60, 3, 0)
	if r.n != 1 || r.events[0].Precise {
		t.Fatalf("±120 after sticky window expired: got %+v, want Precise=false", r.events[0])
	}
}

func TestWheelStateSmallDeltaSmoothness(t *testing.T) {
	var ws wheelState
	now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	for _, d := range []int16{1, 6, 12, 40, -18} {
		r := ws.handleWheel(now, false, d, 0, false, 10, 20, 3, 0)
		want := -float64(d) / 120 * 3 * 33
		if r.n != 1 || !r.events[0].Precise || math.Abs(r.events[0].DY-want) > 1e-9 {
			t.Fatalf("delta=%d: got %+v, want Precise=true DY=%v", d, r.events[0], want)
		}
	}
}

func TestWheelStatePinchDetectionAndPhases(t *testing.T) {
	var ws wheelState
	t0 := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)

	// Idle endPinch is a zero-cost no-op.
	if _, kill, ok := ws.endPinch(); kill || ok {
		t.Fatalf("idle endPinch: kill=%v ok=%v, want false, false", kill, ok)
	}

	// 1. Physical Ctrl + notched mouse wheel (±120) must remain PointerScroll + ModCtrl.
	r := ws.handleWheel(t0, false, 120, wheelMkControl, true, 100, 80, 3, platform.ModCtrl)
	if r.n != 1 || r.timer != wheelTimerNone {
		t.Fatalf("Ctrl+wheel: n=%d timer=%v, want 1, None", r.n, r.timer)
	}
	if ev := r.events[0]; ev.Kind != platform.PointerScroll || ev.Mods != platform.ModCtrl || ev.Precise {
		t.Fatalf("Ctrl+wheel event = %+v, want PointerScroll ModCtrl Precise=false", ev)
	}

	// 2. VMware / Precision Touchpad ±120 + MK_CONTROL with physical Ctrl NOT down
	// starts a pinch with GestureBegan and arms the one-shot timer.
	r = ws.handleWheel(t0.Add(10*time.Millisecond), false, 120, wheelMkControl, false, 100, 80, 3, 0)
	if r.n != 1 || r.timer != wheelTimerReset {
		t.Fatalf("pinch ±120 start: n=%d timer=%v, want 1, Reset", r.n, r.timer)
	}
	wantScale120 := math.Exp(120.0 / 120 * math.Ln2 / 3)
	if ev := r.events[0]; ev.Kind != platform.PointerZoom || ev.Phase != platform.GestureBegan ||
		math.Abs(ev.Scale-wantScale120) > 1e-9 || ev.X != 100 || ev.Y != 80 {
		t.Fatalf("pinch start event = %+v, want GestureBegan scale=%v at (100,80)", ev, wantScale120)
	}

	// 3. Subsequent fine frame -> GestureChanged, resets timer, updates last coords.
	r = ws.handleWheel(t0.Add(26*time.Millisecond), false, -30, wheelMkControl, false, 112, 94, 3, 0)
	if r.n != 1 || r.timer != wheelTimerReset {
		t.Fatalf("pinch changed: n=%d timer=%v, want 1, Reset", r.n, r.timer)
	}
	wantScaleNeg30 := math.Exp(-30.0 / 120 * math.Ln2 / 3)
	if ev := r.events[0]; ev.Kind != platform.PointerZoom || ev.Phase != platform.GestureChanged ||
		math.Abs(ev.Scale-wantScaleNeg30) > 1e-9 || ev.X != 112 || ev.Y != 94 {
		t.Fatalf("pinch changed event = %+v, want GestureChanged scale=%v at (112,94)", ev, wantScaleNeg30)
	}

	// 4. Mid-session ±120 frame stays pinch even if physicalCtrl is true.
	r = ws.handleWheel(t0.Add(42*time.Millisecond), false, 120, wheelMkControl, true, 115, 96, 3, 0)
	if r.n != 1 || r.timer != wheelTimerReset || r.events[0].Kind != platform.PointerZoom || r.events[0].Phase != platform.GestureChanged {
		t.Fatalf("mid-session ±120: got %+v timer=%v, want GestureChanged Reset", r.events[0], r.timer)
	}

	// 5. Timer fires after inactivity -> GestureEnded at last coords with Scale=1 and KillTimer.
	ev, kill, ok := ws.pinchTimeout()
	if !kill || !ok {
		t.Fatalf("pinchTimeout: kill=%v ok=%v, want true, true", kill, ok)
	}
	if ev.Kind != platform.PointerZoom || ev.Phase != platform.GestureEnded || ev.Scale != 1 || ev.X != 115 || ev.Y != 96 {
		t.Fatalf("pinchTimeout event = %+v, want GestureEnded Scale=1 at (115,96)", ev)
	}

	// 6. Duplicate/stale timer firing emits no second GestureEnded, still kills timer.
	if _, kill, ok := ws.pinchTimeout(); !kill || ok {
		t.Fatalf("stale pinchTimeout: kill=%v ok=%v, want true, false", kill, ok)
	}

	// 7. After pinch ended, physical Ctrl + ±120 mouse wheel goes back to PointerScroll.
	r = ws.handleWheel(t0.Add(300*time.Millisecond), false, -120, wheelMkControl, true, 115, 96, 3, platform.ModCtrl)
	if r.n != 1 || r.timer != wheelTimerNone || r.events[0].Kind != platform.PointerScroll || r.events[0].Mods != platform.ModCtrl {
		t.Fatalf("Ctrl+wheel after pinch ended: got %+v timer=%v, want PointerScroll ModCtrl", r.events[0], r.timer)
	}
}

func TestWheelStatePinchInterruptedByScrollOrInput(t *testing.T) {
	var ws wheelState
	t0 := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)

	// Start a fine touchpad pinch.
	r := ws.handleWheel(t0, false, 40, wheelMkControl, false, 60, 70, 3, 0)
	if r.n != 1 || r.events[0].Phase != platform.GestureBegan || r.timer != wheelTimerReset {
		t.Fatalf("start pinch: got n=%d ev=%+v timer=%v", r.n, r.events[0], r.timer)
	}

	// Horizontal wheel (never a pinch) interrupts it: GestureEnded first, then PointerScroll, KillTimer.
	r = ws.handleWheel(t0.Add(20*time.Millisecond), true, 30, wheelMkControl, false, 65, 75, 3, 0)
	if r.n != 2 || r.timer != wheelTimerKill {
		t.Fatalf("scroll interrupting pinch: n=%d timer=%v, want 2, Kill", r.n, r.timer)
	}
	if r.events[0].Kind != platform.PointerZoom || r.events[0].Phase != platform.GestureEnded || r.events[0].Scale != 1 || r.events[0].X != 60 || r.events[0].Y != 70 {
		t.Fatalf("first event = %+v, want GestureEnded at (60,70)", r.events[0])
	}
	if r.events[1].Kind != platform.PointerScroll || !r.events[1].Precise || r.events[1].DX <= 0 {
		t.Fatalf("second event = %+v, want precise horizontal PointerScroll", r.events[1])
	}

	// Next pinch starts fresh with GestureBegan.
	r = ws.handleWheel(t0.Add(40*time.Millisecond), false, -20, wheelMkControl, true, 80, 90, 3, platform.ModCtrl)
	if r.n != 1 || r.events[0].Kind != platform.PointerZoom || r.events[0].Phase != platform.GestureBegan || r.events[0].Mods != platform.ModCtrl {
		t.Fatalf("new pinch with fine delta + Ctrl: got %+v", r.events[0])
	}

	// External interruption (focus loss / key down / button press / destroy) ends cleanly.
	ev, kill, ok := ws.endPinch()
	if !kill || !ok || ev.Kind != platform.PointerZoom || ev.Phase != platform.GestureEnded || ev.Scale != 1 || ev.Mods != platform.ModCtrl {
		t.Fatalf("endPinch: kill=%v ok=%v ev=%+v", kill, ok, ev)
	}
	if _, kill, ok := ws.endPinch(); kill || ok {
		t.Fatalf("second endPinch should be no-op: kill=%v ok=%v", kill, ok)
	}

	// Zero-delta wheel during active pinch ends the pinch cleanly and emits no scroll.
	ws.handleWheel(t0.Add(60*time.Millisecond), false, 30, wheelMkControl, false, 10, 20, 3, 0)
	r = ws.handleWheel(t0.Add(70*time.Millisecond), false, 0, wheelMkControl, false, 10, 20, 3, 0)
	if r.n != 1 || r.timer != wheelTimerKill || r.events[0].Phase != platform.GestureEnded {
		t.Fatalf("zero delta during pinch: n=%d timer=%v ev=%+v", r.n, r.timer, r.events[0])
	}
}

func TestWheelStateIsPinchPredicate(t *testing.T) {
	var ws wheelState
	cases := []struct {
		name         string
		pinching     bool
		horizontal   bool
		delta        int16
		wParamLow    uint16
		physicalCtrl bool
		want         bool
	}{
		{"plain vertical scroll fine", false, false, 24, 0, false, false},
		{"plain vertical scroll notch", false, false, 120, 0, false, false},
		{"horizontal with MK_CONTROL fine", false, true, 24, wheelMkControl, false, false},
		{"zero delta with MK_CONTROL", false, false, 0, wheelMkControl, false, false},
		{"precision touchpad fine pinch", false, false, 36, wheelMkControl, false, true},
		{"precision/VM ±120 pinch (no physical ctrl)", false, false, -120, wheelMkControl, false, true},
		{"physical ctrl + mouse wheel ±120", false, false, 120, wheelMkControl, true, false},
		{"physical ctrl + fine touchpad", false, false, 36, wheelMkControl, true, true},
		{"active pinch + ±120 + ctrl", true, false, 120, wheelMkControl, true, true},
	}
	for _, tc := range cases {
		ws.pinching = tc.pinching
		if got := ws.isPinch(tc.horizontal, tc.delta, tc.wParamLow, tc.physicalCtrl); got != tc.want {
			t.Errorf("%s: isPinch=%v, want %v", tc.name, got, tc.want)
		}
	}
}
