package windows

// This file has no build constraint: the surface's wheel and touchpad
// state machine (sticky precise scroll classification, touchpad pinch
// detection, and synthetic GestureBegan/GestureChanged/GestureEnded
// lifecycle) is plain Go, tested on every platform.

import (
	"math"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

const (
	// wheelDelta is Windows' WHEEL_DELTA: one detent of a notched mouse wheel.
	wheelDelta = 120
	// wheelMkControl is MK_CONTROL in WM_MOUSEWHEEL / WM_MOUSEHWHEEL wParam.
	wheelMkControl = 0x0008

	// preciseStickyWindow keeps wheel messages classified as precise
	// (touchpad) across occasional ±120 frames from Precision Touchpads or
	// VM trackpad forwarding, so two-finger scrolling does not flip into
	// mouse-wheel easing and drop travel on the next sub-detent frame.
	preciseStickyWindow = 300 * time.Millisecond

	// pinchEndTimeout is how long after the last touchpad pinch wheel
	// message the surface waits before synthesizing GestureEnded and
	// stopping its one-shot timer.
	pinchEndTimeout = 180 * time.Millisecond

	// wheelLineDIPs is the DIP distance scrolled per system wheel line:
	// 33 DIPs × default 3 lines ≈ 100 DIPs per detent, matching browsers.
	wheelLineDIPs = 33
)

// wheelTimerAction tells the surface how to update its one-shot pinch-end
// Win32 timer after processing a wheel message.
type wheelTimerAction uint8

const (
	wheelTimerNone  wheelTimerAction = iota
	wheelTimerReset                  // SetTimer(hwnd, timerPinchEnd, pinchEndTimeout)
	wheelTimerKill                   // KillTimer(hwnd, timerPinchEnd)
)

// wheelResult holds the surface events and timer update produced by a
// single WM_MOUSEWHEEL or WM_MOUSEHWHEEL message.
type wheelResult struct {
	events [2]platform.SurfaceEvent
	n      int
	timer  wheelTimerAction
}

// wheelState tracks per-surface touchpad scroll stickiness and synthetic
// pinch gesture phases. Its zero value is ready to use.
type wheelState struct {
	preciseUntil time.Time

	pinching   bool
	pinchX     float64
	pinchY     float64
	pinchMods  platform.Modifiers
	timerArmed bool
}

// isPinch reports whether a wheel message should be treated as a touchpad
// pinch rather than scrolling.
//
// Windows Precision Touchpads (and VM trackpad forwarders such as VMware
// Fusion) report two-finger pinch as WM_MOUSEWHEEL with MK_CONTROL set in
// wParam while no physical keyboard Ctrl key is held. Checking:
//   - fine sub-detent delta (delta%120 != 0), or
//   - an already active pinch session, or
//   - no physical keyboard Ctrl key held
// catches both fine and ±120 pinch frames without hijacking genuine
// physical Ctrl + notched mouse wheel scrolling.
func (w *wheelState) isPinch(horizontal bool, delta int16, wParamLow uint16, physicalCtrl bool) bool {
	if horizontal || delta == 0 || wParamLow&wheelMkControl == 0 {
		return false
	}
	return int(delta)%wheelDelta != 0 || w.pinching || !physicalCtrl
}

// handleWheel processes a WM_MOUSEWHEEL (horizontal=false) or
// WM_MOUSEHWHEEL (horizontal=true) message at DIP position (x, y).
func (w *wheelState) handleWheel(now time.Time, horizontal bool, delta int16, wParamLow uint16, physicalCtrl bool, x, y float64, lines int, mods platform.Modifiers) wheelResult {
	var res wheelResult
	if delta == 0 {
		if ev, kill, ok := w.endPinch(); ok || kill {
			if ok {
				res.events[res.n] = ev
				res.n++
			}
			if kill {
				res.timer = wheelTimerKill
			}
		}
		return res
	}
	if w.isPinch(horizontal, delta, wParamLow, physicalCtrl) {
		phase := platform.GestureChanged
		if !w.pinching {
			phase = platform.GestureBegan
			w.pinching = true
		}
		w.pinchX, w.pinchY, w.pinchMods = x, y, mods
		w.timerArmed = true
		res.events[0] = platform.SurfaceEvent{
			Kind:  platform.PointerZoom,
			X:     x,
			Y:     y,
			Scale: math.Exp(float64(delta) / wheelDelta * math.Ln2 / 3),
			Phase: phase,
			Mods:  mods,
		}
		res.n = 1
		res.timer = wheelTimerReset
		return res
	}

	// Non-pinch scroll ends any in-progress pinch first.
	if ev, kill, ok := w.endPinch(); ok || kill {
		if ok {
			res.events[res.n] = ev
			res.n++
		}
		if kill {
			res.timer = wheelTimerKill
		}
	}

	fine := int(delta)%wheelDelta != 0
	if fine {
		w.preciseUntil = now.Add(preciseStickyWindow)
	}
	precise := fine || (!w.preciseUntil.IsZero() && !now.After(w.preciseUntil))

	if lines <= 0 {
		lines = 3
	}
	dist := float64(delta) / wheelDelta * float64(lines) * wheelLineDIPs
	ev := platform.SurfaceEvent{
		Kind:    platform.PointerScroll,
		X:       x,
		Y:       y,
		Mods:    mods,
		Precise: precise,
	}
	if horizontal {
		ev.DX = dist
	} else {
		ev.DY = -dist
	}
	res.events[res.n] = ev
	res.n++
	return res
}

// endPinch ends an active synthetic pinch gesture (e.g. on focus loss,
// non-pinch input, or surface destruction). When idle it is a zero-cost
// no-op returning ok=false, killTimer=false.
func (w *wheelState) endPinch() (ev platform.SurfaceEvent, killTimer, ok bool) {
	killTimer = w.timerArmed
	w.timerArmed = false
	if !w.pinching {
		return platform.SurfaceEvent{}, killTimer, false
	}
	w.pinching = false
	return platform.SurfaceEvent{
		Kind:  platform.PointerZoom,
		X:     w.pinchX,
		Y:     w.pinchY,
		Scale: 1,
		Phase: platform.GestureEnded,
		Mods:  w.pinchMods,
	}, killTimer, true
}

// pinchTimeout handles timerPinchEnd firing: it always asks the caller to
// KillTimer (Win32 timers repeat until killed) and emits GestureEnded if a
// pinch gesture was active.
func (w *wheelState) pinchTimeout() (ev platform.SurfaceEvent, killTimer, ok bool) {
	w.timerArmed = false
	if !w.pinching {
		return platform.SurfaceEvent{}, true, false
	}
	w.pinching = false
	return platform.SurfaceEvent{
		Kind:  platform.PointerZoom,
		X:     w.pinchX,
		Y:     w.pinchY,
		Scale: 1,
		Phase: platform.GestureEnded,
		Mods:  w.pinchMods,
	}, true, true
}
