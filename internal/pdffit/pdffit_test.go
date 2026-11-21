package pdffit

import "testing"

// webkit is WebKit's pinch on a PDF (ViewGestureController's
// handleMagnificationGesture, its resistance past the limits left out):
// each event takes the magnification by its boost.
type webkit struct{ mag float64 }

func (w *webkit) pinch(m float64) {
	b := 1.0
	if m < 0 {
		b = ZoomOutBoost
	}
	w.mag += w.mag * b * m
}

// pinchPage is a pinch the page has, of events of magnification m: those
// Hold drops do not reach WebKit.
func pinchPage(f *Fit, w *webkit, m float64, n int) (dropped int) {
	f.Touch(w.mag)
	for range n {
		if f.Hold(w.mag, m) {
			dropped++
			continue
		}
		w.pinch(m)
	}
	return dropped
}

func TestAtItsFit(t *testing.T) {
	for _, fit := range []float64{1, 1.08, 1.37, 0.8} { // a slide in a wide view is over 1; one shown under its actual size under it
		var f Fit
		if f.ZoomedIn(fit) || f.Known() {
			t.Errorf("fit %v: a PDF nothing zoomed is at its fit", fit)
		}
		if f.Hold(fit, -0.05) {
			t.Errorf("fit %v: nothing to hold before anything zoomed it", fit)
		}
	}
}

// A PDF zoomed in, then pinched in: WebKit zooms it out to its fit, not
// under it, and the next pinch in is the app's (the page closes).
func TestPinchedBackToItsFit(t *testing.T) {
	for _, fit := range []float64{1, 1.08, 1.37} {
		var f Fit
		w := &webkit{fit}
		pinchPage(&f, w, 0.02, 40) // fingers open: zoomed in
		if !f.ZoomedIn(w.mag) || f.At() != fit {
			t.Fatalf("fit %v: at %.3f zoomed in = %v, fit %v", fit, w.mag, f.ZoomedIn(w.mag), f.At())
		}
		if d := pinchPage(&f, w, -0.02, 200); d == 0 {
			t.Fatalf("fit %v: pinched in far past its fit, nothing held", fit)
		}
		if w.mag < fit || w.mag > fit*1.05 {
			t.Errorf("fit %v: pinched back to %.3f, want its fit", fit, w.mag)
		}
		if f.ZoomedIn(w.mag) {
			t.Errorf("fit %v: at %.3f after the pinch back, still zoomed in: the next pinch in would not close", fit, w.mag)
		}
		// Zoomed in again and back: the same.
		pinchPage(&f, w, 0.03, 10)
		if !f.ZoomedIn(w.mag) {
			t.Errorf("fit %v: zoomed in again to %.3f, not zoomed", fit, w.mag)
		}
		pinchPage(&f, w, -0.03, 100)
		if w.mag < fit || f.ZoomedIn(w.mag) {
			t.Errorf("fit %v: back again at %.3f, zoomed %v", fit, w.mag, f.ZoomedIn(w.mag))
		}
	}
}

// A pinch in that stops before the fit leaves the PDF zoomed in: the
// next pinch in is WebKit's again.
func TestPinchedPartWay(t *testing.T) {
	var f Fit
	w := &webkit{1.2}
	pinchPage(&f, w, 0.05, 20)
	pinchPage(&f, w, -0.01, 5)
	if !f.ZoomedIn(w.mag) {
		t.Errorf("at %.3f (fit 1.2): not zoomed in", w.mag)
	}
}

// The app's zoom (its own buttons: the page zoom) counts as a zoom: the
// fit is where it was before.
func TestAppZoom(t *testing.T) {
	var f Fit
	f.Touch(1.08)
	if !f.ZoomedIn(1.08 * 1.25) {
		t.Error("zoomed in a step by the app: not zoomed")
	}
	if f.ZoomedIn(1.08 / 1.25) {
		t.Error("zoomed out a step by the app: zoomed")
	}
	f.Touch(2) // the fit stays the first
	if f.At() != 1.08 {
		t.Errorf("fit %v, want 1.08", f.At())
	}
	f.Reset()
	if f.Known() || f.ZoomedIn(2) {
		t.Error("a page loaded anew is at its fit")
	}
}

// Counting the pinches the page took (MyGo 0054's zoomCount), the fit at
// 1, said a PDF pinched back to its fit was still zoomed in: WebKit zooms
// out faster than the fingers close.
func TestCountingWasWrong(t *testing.T) {
	w := &webkit{1}
	count := 1.0
	for range 35 {
		w.pinch(0.02)
		count *= 1.02
	}
	for w.mag > 1 {
		w.pinch(-0.02)
		count *= 0.98
	}
	if count <= 1.01 {
		t.Fatalf("counted %.3f at %.3f", count, w.mag)
	}
	t.Logf("at %.3f WebKit's, counted %.3f: zoomed in by the count", w.mag, count)
}
