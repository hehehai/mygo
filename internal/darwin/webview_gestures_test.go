//go:build darwin && !ios

package darwin

import "testing"

// TestPageTookPinch: the pinches a page takes count its zoom, never under
// its own size; a smart zoom counts as twice it.
func TestPageTookPinch(t *testing.T) {
	w := &window{zoomed: zoomCount{scale: 1}}
	w.pageTookPinch(-0.3)
	if w.zoomed.scale != 1 {
		t.Errorf("pinched in at its own size: %v", w.zoomed.scale)
	}
	w.pageTookPinch(0.5)
	if w.zoomed.scale != 1.5 {
		t.Errorf("pinched out: %v", w.zoomed.scale)
	}
	w.zoomed = zoomCount{scale: 1, smart: true}
	w.pageTookPinch(-0.25)
	if w.zoomed.smart || w.zoomed.scale != 1.5 {
		t.Errorf("after a smart zoom: %+v", w.zoomed)
	}
}
