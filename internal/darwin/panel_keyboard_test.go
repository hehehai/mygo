//go:build darwin && !ios

package darwin

import (
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/internal/platform"
)

// TestPanelKeyboard checks MyGoPanel's canBecomeKeyWindow: a panel that
// is not Focusable (mygo.PanelKeyboardNever) can never be the key window,
// one that is can. The method is put on a panel class of the test's own:
// registering MyGo's window and surface classes here would clash with
// the stand-ins other tests of the package define (MyGoWebView,
// MyGoDragSource). The panels are made, never shown.
func TestPanelKeyboard(t *testing.T) {
	load()
	onMainThread(func() {
		withPool(func() {
			if !hasClass("MyGoTestPanel") {
				classDef("MyGoTestPanel", "NSPanel", nil, []objc.MethodDef{
					method("canBecomeKeyWindow", panelCanBecomeKey),
				})
			}
			previous := theBackend
			b := &Backend{byNSWindow: map[id]*window{}}
			theBackend = b
			defer func() { theBackend = previous }()
			for _, focusable := range []bool{true, false} {
				win := msgInitWindow(send(class("MyGoTestPanel"), "alloc"), sel("initWithContentRect:styleMask:backing:defer:"),
					NSRect{Size: NSSize{Width: 200, Height: 100}}, styleNonactivatingPanel, nsBackingStoreBuffered, true)
				send(win, "setReleasedWhenClosed:", 0)
				b.byNSWindow[win] = &window{b: b, win: win, opts: &platform.WindowOptions{Panel: true, Focusable: focusable}}
				if got := sendBool(win, "canBecomeKeyWindow"); got != focusable {
					t.Errorf("a panel Focusable %v: canBecomeKeyWindow %v", focusable, got)
				}
				delete(b.byNSWindow, win)
				release(win)
			}
		})
	})
}
