//go:build darwin

package darwin

import "testing"

// TestKeyEquivalentShift: Shift and a character key give AppKit the
// character Shift types, without Shift in the mask, as AppKit's own
// menus do; other keys keep Shift in the mask.
func TestKeyEquivalentShift(t *testing.T) {
	const cmd, shift, alt = 1 << 20, 1 << 17, 1 << 19
	for _, c := range []struct {
		acc  string
		key  string
		mods uint
	}{
		{"CmdOrCtrl+J", "j", cmd},
		{"CmdOrCtrl+Shift+J", "J", cmd},
		{"Shift+CmdOrCtrl+F", "F", cmd},
		{"Alt+Shift+CmdOrCtrl+N", "N", cmd | alt},
		{"CmdOrCtrl+Shift+1", "!", cmd},
		{"CmdOrCtrl+Shift+/", "?", cmd},
		{"CmdOrCtrl+Shift+Enter", "\r", cmd | shift},
		{"Shift+F5", string(rune(0xF708)), shift},
	} {
		key, mods := keyEquivalent(c.acc)
		if key != c.key || mods != c.mods {
			t.Errorf("%s: %q %#x, want %q %#x", c.acc, key, mods, c.key, c.mods)
		}
	}
}
