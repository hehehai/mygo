//go:build darwin && !ios

package darwin

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestDragStuck: a drag believed in ends once the primary button is up,
// whatever the other buttons; none believed in is never stuck.
func TestDragStuck(t *testing.T) {
	for _, tc := range []struct {
		dragging bool
		buttons  int
		want     bool
	}{
		{true, 0, true},
		{true, 1, false},
		{true, 2, true}, // the secondary button alone
		{true, 3, false},
		{false, 0, false},
		{false, 1, false},
	} {
		if got := dragStuck(tc.dragging, tc.buttons); got != tc.want {
			t.Errorf("dragStuck(%v, %d) = %v, want %v", tc.dragging, tc.buttons, got, tc.want)
		}
	}
}

// TestRefreshPointerUnstuck: RefreshPointer ends a drag believed in when
// no button is down (as on a test Mac nobody touches), stops the drag's
// timer, and reports the state it leaves.
func TestRefreshPointerUnstuck(t *testing.T) {
	load()
	var first, second platform.PointerNow
	var after bool
	withPool(func() {
		saved := dragging
		defer func() { dragging = saved }()
		dragging = true
		first = screen{}.RefreshPointer()
		after = dragging
		second = screen{}.RefreshPointer()
	})
	if first.Buttons&1 != 0 {
		t.Skip("the primary button is down")
	}
	if !first.Unstuck || first.Dragging || after {
		t.Errorf("RefreshPointer = %+v, dragging %v: want the drag ended", first, after)
	}
	if second.Unstuck {
		t.Errorf("second RefreshPointer = %+v: nothing to end", second)
	}
}
