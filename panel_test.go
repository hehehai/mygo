//go:build !js

package mygo

import (
	"strings"
	"testing"
)

// TestPanelOptions gives a panel's backend a window above others, out of
// the taskbar, that cannot minimize, maximize or go full screen.
func TestPanelOptions(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Panel: true})
	o := fw.Opts
	if !w.IsPanel() || !o.Panel || !o.AlwaysOnTop || !o.SkipTaskbar || o.Minimizable || o.Maximizable || o.Fullscreenable {
		t.Errorf("panel: IsPanel %v, options %+v", w.IsPanel(), o)
	}
	if !o.Focusable || !o.Resizable || !o.Closable {
		t.Errorf("a panel lost abilities it keeps: %+v", o)
	}
	w, fw = testWindow(t, WindowOptions{})
	if w.IsPanel() || fw.Opts.Panel {
		t.Error("a window without Panel is a panel")
	}
}

// TestPanelEscape closes a panel on an Escape its page leaves, as the
// user closing it, which OnClose listeners may prevent.
func TestPanelEscape(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Panel: true})
	keep, closes := true, 0
	w.OnClose(func(e *CloseEvent) {
		closes++
		if keep {
			e.PreventDefault()
		}
	})
	escape := func() {
		page(fw, `{"t":"escape"}`)
		onMain(func() {}) // the close posted after the message
	}
	escape()
	if closes != 1 || w.IsDestroyed() {
		t.Fatalf("OnClose heard %d closes, destroyed %v", closes, w.IsDestroyed())
	}
	keep = false
	escape()
	if !w.IsDestroyed() {
		t.Error("Escape did not close the panel")
	}
}

// TestPanelKeyboard gives a panel's backend when it takes the keyboard:
// on a text input alone (KeyboardOnlyForText), or never (not Focusable);
// and a window that is no panel neither.
func TestPanelKeyboard(t *testing.T) {
	_, fw := testWindow(t, WindowOptions{Panel: true, PanelKeyboard: PanelKeyboardForText})
	if o := fw.Opts; !o.KeyboardOnlyForText || !o.Focusable {
		t.Errorf("a panel for text: KeyboardOnlyForText %v, Focusable %v", o.KeyboardOnlyForText, o.Focusable)
	}
	_, fw = testWindow(t, WindowOptions{Panel: true, PanelKeyboard: PanelKeyboardNever})
	if o := fw.Opts; o.KeyboardOnlyForText || o.Focusable || !o.Panel {
		t.Errorf("a panel never focused: KeyboardOnlyForText %v, Focusable %v, Panel %v", o.KeyboardOnlyForText, o.Focusable, o.Panel)
	}
	for _, k := range []PanelKeyboard{PanelKeyboardForText, PanelKeyboardNever} {
		_, fw = testWindow(t, WindowOptions{PanelKeyboard: k})
		if o := fw.Opts; o.KeyboardOnlyForText || !o.Focusable {
			t.Errorf("a window that is no panel took PanelKeyboard %d: %+v", k, o)
		}
	}
}

// TestPanelOverMenuBar passes OverMenuBar to the backend of a panel, and
// of no other window.
func TestPanelOverMenuBar(t *testing.T) {
	_, fw := testWindow(t, WindowOptions{Panel: true, OverMenuBar: true})
	if !fw.Opts.OverMenuBar {
		t.Error("a panel lost OverMenuBar")
	}
	_, fw = testWindow(t, WindowOptions{OverMenuBar: true})
	if fw.Opts.OverMenuBar {
		t.Error("a window that is no panel took OverMenuBar")
	}
}

// TestPanelGivesUpKey: a panel that took the keyboard gives it up — it
// blurs, and the app's window takes it back — where Blur leaves it key
// on macOS. A window without the keyboard gives nothing up.
func TestPanelGivesUpKey(t *testing.T) {
	main, fmain := testWindow(t, WindowOptions{Hidden: true})
	panel, fpanel := testWindow(t, WindowOptions{Panel: true, Hidden: true})
	onMain(fmain.ShowInactive)
	fpanel.KeyHeir = fmain
	var events []string
	main.OnFocus(func() { events = append(events, "main focus") })
	main.OnBlur(func() { events = append(events, "main blur") })
	panel.OnFocus(func() { events = append(events, "panel focus") })
	panel.OnBlur(func() { events = append(events, "panel blur") })

	panel.Show()
	onMain(fpanel.Focus)
	if !panel.IsFocused() {
		t.Fatal("the panel shown: not focused")
	}
	events = nil
	panel.GiveUpKey()
	onMain(func() {})
	if panel.IsFocused() || !main.IsFocused() || FocusedWindow() != main {
		t.Errorf("given up: panel focused %v, main %v, FocusedWindow %v", panel.IsFocused(), main.IsFocused(), FocusedWindow())
	}
	if got := strings.Join(events, ","); got != "panel blur,main focus" {
		t.Errorf("events %q", got)
	}
	events = nil
	panel.GiveUpKey() // not key: nothing
	onMain(func() {})
	if len(events) != 0 || !main.IsFocused() || fpanel.GaveUpKey != 2 {
		t.Errorf("given up again: events %q, main focused %v, calls %d", events, main.IsFocused(), fpanel.GaveUpKey)
	}
}
