//go:build darwin && !ios

package darwin

import "testing"

// TestIsAppLaunchEvent tells the launch of the app alone (the Dock,
// Finder, open -a) from one to open URLs or files.
func TestIsAppLaunchEvent(t *testing.T) {
	const gurl = 0x4755524C // 'GURL'
	for _, c := range []struct {
		class, id uint32
		want      bool
	}{
		{kCoreEventClass, 0x6F617070, true},  // 'oapp': the Dock, Finder, open -a
		{kCoreEventClass, 0x72617070, true},  // 'rapp'
		{kCoreEventClass, 0x6F646F63, false}, // 'odoc'
		{gurl, gurl, false},
		{kCoreEventClass, kAEQuitApplication, false},
	} {
		if got := isAppLaunchEvent(c.class, c.id); got != c.want {
			t.Errorf("%08x/%08x: %v, want %v", c.class, c.id, got, c.want)
		}
	}
}
