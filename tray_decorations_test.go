//go:build !js

package mygo

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/internal/fake"
	"github.com/egoist/mygo/internal/platform"
)

// TestTrayDecorations: the overlay and the dot reach the platform's tray
// and go again; a bad color changes nothing; the native button is the
// backend's until the icon is destroyed.
func TestTrayDecorations(t *testing.T) {
	tray, err := NewTray(TrayOptions{Icon: []byte("png"), IconIsTemplate: true})
	if err != nil {
		t.Fatal(err)
	}
	read := func() (overlay []byte, dot platform.Color, ok bool) {
		onMain(func() {})
		return fb.TrayDecorations()
	}
	if o, _, ok := read(); o != nil || ok {
		t.Fatalf("a new tray shows overlay %q, dot %v", o, ok)
	}
	if err := tray.SetOverlay([]byte("flag")); err != nil {
		t.Fatal(err)
	}
	if o, _, _ := read(); string(o) != "flag" {
		t.Errorf("overlay %q", o)
	}
	if err := tray.SetDot("#ff3b30"); err != nil {
		t.Fatal(err)
	}
	if _, c, ok := read(); !ok || c != (platform.Color{R: 0xff, G: 0x3b, B: 0x30, A: 0xff}) {
		t.Errorf("dot %v %v, want #ff3b30", c, ok)
	}
	if err := tray.SetDot("not a color"); err == nil || errors.Is(err, ErrTrayOverlay) {
		t.Errorf("a bad color: %v", err)
	}
	if _, c, ok := read(); !ok || c.R != 0xff {
		t.Errorf("a bad color changed the dot: %v %v", c, ok)
	}
	if err := tray.SetDot(""); err != nil {
		t.Fatal(err)
	}
	if err := tray.SetOverlay(nil); err != nil {
		t.Fatal(err)
	}
	if o, _, ok := read(); o != nil || ok {
		t.Errorf("removed: overlay %q, dot %v", o, ok)
	}
	if got := tray.NativeButton(); got != fake.TrayHandle {
		t.Errorf("NativeButton = %#x, want the backend's %#x", got, fake.TrayHandle)
	}
	tray.Destroy()
	if got := tray.NativeButton(); got != 0 {
		t.Errorf("NativeButton after Destroy = %#x, want 0", got)
	}
	if err := tray.SetOverlay([]byte("flag")); err != nil {
		t.Errorf("SetOverlay after Destroy: %v", err)
	}
}
