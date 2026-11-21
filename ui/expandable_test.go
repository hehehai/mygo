package ui

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestExpandable: an Expandable element reads as expandable, expanded
// or not, and assistive technology asking to open or close it is
// reported once.
func TestExpandable(t *testing.T) {
	open := false
	asks := 0
	tt := coreNewTester(func(c *context) {
		e := coreRow(c).Key("folder").Role(RoleTreeItem).Label("Folder").Expandable(open)
		if o, ok := e.ExpandAsked(); ok {
			asks++
			open = o
		}
	}, 200, 100)
	tt.Frame()
	if n, ok := tt.Access("Folder"); !ok || !n.Expandable || n.Expanded {
		t.Fatalf("closed: %+v", n)
	}
	open = true
	tt.Frame()
	if n, _ := tt.Access("Folder"); !n.Expandable || !n.Expanded {
		t.Fatalf("open: %+v", n)
	}
	if asks != 0 {
		t.Fatalf("asked %d times without asking", asks)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	n := accessNode(t, tt.h.access, platform.RoleTreeItem, "Folder")
	if n.Actions&platform.ActionExpand == 0 {
		t.Fatalf("cannot expand: %+v", n)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessAction, ID: n.ID, Action: platform.AccessCollapse})
	tt.Frame()
	if asks != 1 || open {
		t.Fatalf("asked to close: %d asks, open %v", asks, open)
	}
	if n, _ := tt.Access("Folder"); n.Expanded {
		t.Fatalf("closed again: %+v", n)
	}
}
