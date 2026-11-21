package ui

import (
	"testing"
	"time"
)

// TestAfterPausedWhileWindowHidden: a view keeping a clock with After
// asks no frame while nothing of the window shows when it asks for one
// only while WindowHidden is false, and builds again as the window shows.
// AudioSmith patched After to wait while the window was occluded; its
// views do this instead.
func TestAfterPausedWhileWindowHidden(t *testing.T) {
	builds := 0
	tt := coreNewTester(func(c *context) {
		builds++
		coreText(c, "clock")
		if !c.WindowHidden() {
			c.After(100 * time.Millisecond)
		}
	}, 100, 100)
	builds = 0
	tt.Advance(time.Second)
	if builds < 9 || builds > 11 {
		t.Fatalf("shown: %d builds in a second, want 10", builds)
	}
	tt.SetWindowHidden(true)
	builds = 0
	tt.Advance(5 * time.Second)
	// The build as it hides, and the timer armed while it showed.
	if builds > 2 {
		t.Fatalf("hidden: %d builds in 5 s", builds)
	}
	builds = 0
	tt.SetWindowHidden(false)
	tt.Advance(time.Second)
	if builds < 9 || builds > 12 {
		t.Fatalf("shown again: %d builds in a second, want 10", builds)
	}
}
