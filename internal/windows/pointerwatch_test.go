//go:build windows && (amd64 || arm64)

package windows

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestScreenImplementsPointerWatcher(t *testing.T) {
	var s platform.Screen = screen{}
	if _, ok := s.(platform.PointerWatcher); !ok {
		t.Fatal("windows screen does not implement platform.PointerWatcher")
	}
	if _, ok := s.(platform.PointerRefresher); !ok {
		t.Fatal("windows screen does not implement platform.PointerRefresher")
	}
}

func TestRefreshPointerUnstuck(t *testing.T) {
	saved := pointerWatch
	pointerWatch = nil
	defer func() { pointerWatch = saved }()

	pw := getPointerWatch()
	pw.dragging = true

	first := screen{}.RefreshPointer()
	if first.Buttons&1 != 0 {
		t.Skip("the primary mouse button is down")
	}
	if !first.Unstuck || first.Dragging || pw.dragging {
		t.Fatalf("first RefreshPointer = %+v, dragging=%v: want stuck drag ended", first, pw.dragging)
	}

	second := screen{}.RefreshPointer()
	if second.Unstuck {
		t.Fatalf("second RefreshPointer = %+v: nothing left to unstuck", second)
	}
}
