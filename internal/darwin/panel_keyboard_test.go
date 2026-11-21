//go:build darwin && !ios

package darwin

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestPanelKeyboard checks the classes that keep a panel from taking the
// keyboard: a panel that is not Focusable (mygo.PanelKeyboardNever) can
// never be the key window, one that is can; and a click on the surface
// does not need the panel to become key (needsPanelToBecomeKey), so that
// a panel that becomes key only if needed (KeyboardOnlyForText) takes
// the keyboard for a text input alone (TakeKeyboard). The panels are made,
// never shown.
func TestPanelKeyboard(t *testing.T) {
	load()
	registerClasses()
	withPool(func() {
		previous := theBackend
		b := &Backend{byNSWindow: map[id]*window{}}
		theBackend = b
		defer func() { theBackend = previous }()
		for _, focusable := range []bool{true, false} {
			win := msgInitWindow(send(class("MyGoPanel"), "alloc"), sel("initWithContentRect:styleMask:backing:defer:"),
				NSRect{Size: NSSize{Width: 200, Height: 100}}, styleNonactivatingPanel, nsBackingStoreBuffered, true)
			send(win, "setReleasedWhenClosed:", 0)
			b.byNSWindow[win] = &window{b: b, win: win, opts: &platform.WindowOptions{Panel: true, Focusable: focusable}}
			if got := sendBool(win, "canBecomeKeyWindow"); got != focusable {
				t.Errorf("a panel Focusable %v: canBecomeKeyWindow %v", focusable, got)
			}
			if sendBool(win, "canBecomeMainWindow") {
				t.Errorf("a panel Focusable %v can become the main window", focusable)
			}
			delete(b.byNSWindow, win)
			release(win)
		}
		view := msgInitRect(send(class("MyGoSurfaceView"), "alloc"), sel("initWithFrame:"), NSRect{Size: NSSize{Width: 10, Height: 10}})
		if sendBool(view, "needsPanelToBecomeKey") {
			t.Error("a click on the surface needs its panel to become key")
		}
		release(view)
	})
}
