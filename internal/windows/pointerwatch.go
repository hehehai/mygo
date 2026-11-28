//go:build windows && (amd64 || arm64)

package windows

import (
	"unsafe"

	"github.com/egoist/mygo/internal/platform"
)

// The pointer as it moves in any app (platform.PointerWatcher,
// platform.PointerRefresher): a main-thread Win32 timer on the hidden
// application window polls GetCursorPos and GetAsyncKeyState without a
// low-level mouse hook (WH_MOUSE_LL can stall system mouse movement when
// the app's thread is busy). It runs at ~32 Hz while moving or dragging,
// slows to 4 Hz after resting ~0.5 s, stops completely when no watchers
// remain, and converts physical coordinates through screenSpace().pointToDIP
// so positions match Displays() and CursorPoint() across mixed-DPI monitors.

const (
	// timerPointerWatch is the application window's timer that polls the
	// pointer while at least one Screen.WatchPointer watcher is active.
	timerPointerWatch = 6

	smSwapButton = 23

	vkLButton  = 0x01
	vkRButton  = 0x02
	vkMButton  = 0x04
	vkXButton1 = 0x05
	vkXButton2 = 0x06
)

var (
	_ platform.PointerWatcher   = screen{}
	_ platform.PointerRefresher = screen{}

	// checkPointerWatch is set when the app calls Screen.WatchPointer so
	// apps that never watch the pointer do not link the watcher code.
	checkPointerWatch func(*Backend)

	pointerWatch *pointerWatchCore
)

func getPointerWatch() *pointerWatchCore {
	if pointerWatch == nil {
		pointerWatch = newPointerWatchCore(pointerWatchHooks{
			sample:   sampleWindowsPointer,
			toDIP:    windowsPointToDIP,
			buttons:  windowsMouseButtons,
			isActive: func() bool { return theBackend != nil && appController{theBackend}.IsActive() },
			startTimer: func(intervalMs uint32) bool {
				b := theBackend
				if b == nil || b.appHwnd == 0 || b.exiting {
					return false
				}
				r, _, _ := procSetTimer.Call(b.appHwnd, timerPointerWatch, uintptr(intervalMs), 0)
				return r != 0
			},
			stopTimer: func() {
				b := theBackend
				if b == nil || b.appHwnd == 0 {
					return
				}
				procKillTimer.Call(b.appHwnd, timerPointerWatch)
			},
		})
	}
	return pointerWatch
}

// WatchPointer calls fn on the main thread with the pointer in screen DIPs
// and whether a drag is in progress whenever the pointer moves or dragging
// starts or ends, in any app.
func (screen) WatchPointer(fn func(p platform.Point, dragging bool)) (stop func()) {
	checkPointerWatch = onPointerWatchTimer
	return getPointerWatch().watch(fn)
}

// RefreshPointer reads the pointer and mouse buttons now, ends any stuck
// drag when the primary button is up, and restarts polling if missing.
func (screen) RefreshPointer() platform.PointerNow {
	return getPointerWatch().refresh()
}

func onPointerWatchTimer(b *Backend) {
	if pointerWatch == nil || b == nil || b.exiting {
		return
	}
	pointerWatch.tick()
}

func primaryMouseVK() (primary, secondary uintptr) {
	if swapped, _, _ := procGetSystemMetrics.Call(smSwapButton); swapped != 0 {
		return vkRButton, vkLButton
	}
	return vkLButton, vkRButton
}

func asyncKeyDown(vk uintptr) bool {
	r, _, _ := procGetAsyncKeyState.Call(vk)
	return r&0x8000 != 0
}

func sampleWindowsPointer() pointerSample {
	var pt point
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	primary, _ := primaryMouseVK()
	return pointerSample{
		phys:        platform.Point{X: int(pt.X), Y: int(pt.Y)},
		ok:          r != 0,
		primaryDown: asyncKeyDown(primary),
	}
}

func windowsPointToDIP(phys platform.Point) platform.Point {
	return screenSpace().pointToDIP(phys.X, phys.Y)
}

func windowsMouseButtons() int {
	primary, secondary := primaryMouseVK()
	var mask int
	for i, vk := range [...]uintptr{primary, secondary, vkMButton, vkXButton1, vkXButton2} {
		if asyncKeyDown(vk) {
			mask |= 1 << i
		}
	}
	return mask
}
