//go:build darwin && !ios

package darwin

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestRejoinSpaces: a hidden panel whose collection behavior changed is
// given the one it was made with as it shows, counted once; one that has
// it is given it again uncounted; a window that shows, or no panel, is
// left alone. SpaceInfo reads it all back.
func TestRejoinSpaces(t *testing.T) {
	load()
	onMainThread(func() { withPool(func() { rejoinSpaces(t) }) })
}

func rejoinSpaces(t *testing.T) {
	// An accessory, never activated: the panel is nonactivating and
	// ordered in off every screen.
	send(send(class("NSApplication"), "sharedApplication"), "setActivationPolicy:", 1)
	const want = collectionCanJoinAllSpaces | collectionStationary | collectionIgnoresCycle | collectionFullScreenAuxiliary
	p := msgInitWindow(send(class("NSPanel"), "alloc"), sel("initWithContentRect:styleMask:backing:defer:"),
		NSRect{Size: NSSize{Width: 120, Height: 40}}, styleNonactivatingPanel, nsBackingStoreBuffered, false)
	defer send(p, "close")
	send(p, "setReleasedWhenClosed:", 0)
	send(p, "setLevel:", nsOverMenuBarWindowLevel)
	// Off every screen: shown, it is never seen.
	msgSetRectBool(p, sel("setFrame:display:"), NSRect{Origin: NSPoint{X: -4000, Y: -4000}, Size: NSSize{Width: 120, Height: 40}}, false)
	w := &window{win: p, opts: &platform.WindowOptions{Panel: true, OverMenuBar: true}, behavior: want}
	behavior := func() uint { return uint(sendInt(p, "collectionBehavior")) }

	send(p, "setCollectionBehavior:", collectionIgnoresCycle) // changed behind the panel's back
	w.rejoinSpaces()
	if behavior() != want || w.rejoins != 1 {
		t.Errorf("after a change: behavior %#x, want %#x; rejoins %d, want 1", behavior(), want, w.rejoins)
	}
	w.rejoinSpaces()
	if behavior() != want || w.rejoins != 1 {
		t.Errorf("unchanged: behavior %#x; rejoins %d, want 1", behavior(), w.rejoins)
	}
	s := w.SpaceInfo()
	if !s.Known || s.Behavior != want || s.Level != nsOverMenuBarWindowLevel || s.Visible || !s.OnActiveSpace || s.Rejoined != 1 || s.Number == 0 || s.Style != styleNonactivatingPanel {
		t.Errorf("SpaceInfo = %+v", s)
	}

	send(p, "orderFrontRegardless")
	send(p, "setCollectionBehavior:", collectionIgnoresCycle)
	w.rejoinSpaces() // showing: left where it is
	if behavior() != collectionIgnoresCycle || w.rejoins != 1 {
		t.Errorf("shown: behavior %#x, rejoins %d", behavior(), w.rejoins)
	}
	send(p, "orderOut:", 0)
	w.opts.Panel = false
	w.rejoinSpaces() // no panel: none of its business
	if behavior() != collectionIgnoresCycle {
		t.Errorf("no panel: behavior %#x", behavior())
	}
	w.closed = true
	if s := w.SpaceInfo(); s.Known {
		t.Errorf("SpaceInfo of a closed window = %+v", s)
	}
}
