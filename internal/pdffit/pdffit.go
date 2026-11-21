// Package pdffit says whether a PDF a web view shows is zoomed in past
// the size its viewer gave it (its fit), from the web view's
// magnification, and holds a pinch zooming it out at that size.
//
// WebKit's PDF viewer (a full-frame PDF's plugin) sizes the PDF itself
// as it loads, and the web view's magnification (WKWebView's, the same as
// its pageZoom) is that scale: 1 is the PDF at its actual size, not at its
// fit, so a PDF fitting its view can show at 1.1 or 1.3 (a slide in a wide
// view), and a window resized changes it (its actual size is measured
// against the view's width). Counting the pinches the page took does not
// tell either: WebKit zooms out 1.6 times faster than the fingers close.
// So the fit is the magnification before anything zoomed the PDF (a
// pinch, a smart zoom, the app's zoom: Touch), and a PDF nothing zoomed
// is at its fit.
package pdffit

// Tolerance is how far past where a pinch left it a PDF counts as zoomed in.
const Tolerance = 0.01

// ZoomOutBoost is how much faster than the fingers WebKit zooms a page
// out (ViewGestureController, zoomOutBoost: its pinch's step is
// magnification × (1 + boost × the event's magnification)).
const ZoomOutBoost = 1.6

// Fit is a PDF's fit, unknown (zero) until something zooms it.
type Fit struct {
	// fit is the magnification before the first zoom; rest is where
	// zooming out last stopped at the fit (Hold), fit or a step above it.
	fit, rest float64
}

// Reset forgets the fit (a page loaded anew).
func (f *Fit) Reset() { *f = Fit{} }

// Touch says the PDF zooms now, at magnification mag before it does: the
// first one since Reset is its fit.
func (f *Fit) Touch(mag float64) {
	if f.fit == 0 && mag > 0 {
		f.fit, f.rest = mag, mag
	}
}

// Known reports whether something zoomed the PDF since Reset.
func (f *Fit) Known() bool { return f.fit > 0 }

// At is the fit (0 unknown).
func (f *Fit) At() float64 { return f.fit }

// ZoomedIn reports whether the PDF at magnification mag is zoomed in:
// past where zooming out stopped at its fit. A PDF nothing zoomed is not.
func (f *Fit) ZoomedIn(mag float64) bool {
	return f.fit > 0 && mag > f.rest*(1+Tolerance)
}

// Hold reports whether a pinch's event zooming out by m (its
// magnification, under 0) would take the PDF, at mag now, under its fit:
// then the event is dropped (the PDF stays where it is, at most a step
// above its fit, and counts as at its fit after: rest).
func (f *Fit) Hold(mag, m float64) bool {
	if f.fit == 0 || m >= 0 || mag <= 0 {
		return false
	}
	if mag*(1+ZoomOutBoost*m) >= f.fit {
		return false
	}
	f.rest = max(mag, f.fit)
	return true
}
