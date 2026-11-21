//go:build darwin && !ios

package darwin

// A touchpad's gestures over a web view (WebView.SetGestures): AppKit
// sends them to the web view under the pointer, and WebKit keeps what its
// page does not use (nothing goes up the responder chain: checked on
// macOS 26), so the content around a page never saw them. Some go to the
// window's view instead, as if the pointer were on it:
//
//   - sideways: a gesture going sideways first (its first 3 points), the
//     page not zoomed in;
//   - edges: a gesture going first toward an edge, along one axis, that
//     nothing under the pointer scrolls toward, with its glide: WebKit
//     says it did not handle the gesture's first moving event
//     (_webView:didNotHandleWheelEvent:, wheelroute); the main frame does
//     not bounce along that axis, else it would take that event;
//   - pinch in: a pinch whose fingers close first (its first
//     magnification, summed past 0.005) while the page is at its own size
//     (not zoomed in; a PDF at its fit, pdffit);
//
// and zoom says whether the page zooms on a pinch and a smart zoom at all
// (WKWebView's allowsMagnification: off, WebKit drops them). A glide of
// a gesture before SetGestures (a page loaded) is dropped (wheelroute):
// the gesture that paged to it glides on, and the new page took it. The first
// events of a gesture wait until it is decided (a few). A PDF zoomed in
// and pinched in zooms out to its fit, not under it, when pinches in go
// to the content: the next one does.

import (
	"log"
	"math"
	"os"
	"sync"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/internal/wheelroute"
)

// gestureRouting is what SetGestures asked.
type gestureRouting struct {
	sideways, pinchIn, zoom bool
	edges                   int  // wheelroute.Edges…
	edgesSideways           bool // the edges are left and right (else top and bottom)
}

// routesScroll reports whether some scrolling goes to the content.
func (g gestureRouting) routesScroll() bool { return g.sideways || g.edges != wheelroute.EdgesPage }

// Where a pinch goes.
const (
	routeNone = iota
	routeUndecided
	routePage
	routeContent
)

// pinchRoute is the pinch under way: where it goes (routeNone between
// pinches, routeUndecided until it is decided), its events kept until
// then, and its magnification so far.
type pinchRoute struct {
	route   int
	pending []id
	sum     float64
	// at is when the content last had one of its events (holdsShown).
	at time.Time
}

// pinchHold is how long after its last event a pinch the content has
// keeps its web view shown (holdsShown): its end lost.
const pinchHold = 600 * time.Millisecond

// holdsShown reports whether the web view stays shown, though the
// content no longer places it, for a pinch the content has: the content
// shows something else in its place as it shrinks with the fingers (an
// app's page closing), and AppKit sends the rest of the gesture to the
// view it began on — hidden, the rest never came to the content, which
// saw the pinch stop where it was a frame after it began (it stays under
// the content, which paints over it; hitTest: does not give it the
// pointer: it is not placed). Once the pinch ends it is hidden.
func (w *window) holdsShown() bool {
	return w.pinch.route == routeContent && time.Since(w.pinch.at) < pinchHold
}

// pinchEnded hides the web view kept shown for the pinch that ended
// when the content no longer places it.
func (w *window) pinchEnded() {
	if w.host == nil || w.host.surface == nil || w.closed || w.clip == 0 {
		return
	}
	s := w.host.surface
	for _, p := range s.placed {
		if v, ok := p.WebView.(*window); ok && v == w {
			return
		}
	}
	if !sendBool(w.clip, "isHidden") {
		gestureLog("pinch ended: the web view kept shown for it hides")
		send(w.clip, "setHidden:", 1)
	}
}

// zoomCount is how much the page was pinched since SetGestures (scale,
// never under its own size) and whether a smart zoom (two fingers tapped
// twice) zoomed it: a PDF's zoom where WebKit does not say the page is a
// PDF (no _isDisplayingPDF; else its magnification says it: pdffit).
type zoomCount struct {
	scale float64
	smart bool
}

// SetGestures says which of a touchpad's gestures over the web view go
// to the window's view (sideways, edges, pinchIn) and whether its page
// zooms (zoom); edgesSideways makes the edges left and right, not top
// and bottom. Each call starts afresh: the page counts as at its own
// size (call it for each page loaded); without zoom it is set back to it.
func (w *window) SetGestures(sideways, pinchIn, zoom bool, edges int, edgesSideways bool) {
	w.dropPinch()
	w.gest = gestureRouting{sideways: sideways, pinchIn: pinchIn, zoom: zoom, edges: edges, edgesSideways: edgesSideways}
	w.pinch, w.zoomed = pinchRoute{}, zoomCount{scale: 1}
	w.scroll.Env = scrollEnv{w}
	g := wheelroute.Routing{Sideways: sideways, Edges: edges, EdgesSideways: edgesSideways, Zoomed: w.zoomedIn}
	w.scroll.Set(g)
	if w.web != 0 && respondsTo(w.web, "_setRubberBandingEnabled:") {
		send(w.web, "_setRubberBandingEnabled:", uintptr(wheelroute.Bounce(g)))
	}
	if w.web != 0 && respondsTo(w.web, "setAllowsMagnification:") {
		send(w.web, "setAllowsMagnification:", boolArg(zoom))
		if !zoom && msgFloat(w.web, sel("magnification")) != 1 {
			msgSetFloat(w.web, sel("setMagnification:"), 1)
		}
	}
	gestureLog("set sideways=%v pinchIn=%v zoom=%v edges=%d edgesSideways=%v", sideways, pinchIn, zoom, edges, edgesSideways)
}

// zoomedIn reports whether the page is zoomed in: its magnification
// (WebKit's pinches and smart zooms of a document); a PDF's past its fit
// (pdffit: a PDF's magnification and page zoom are its plugin's scale, 1
// at its actual size); without _isDisplayingPDF, the pinches and smart
// zoom its plugin took, or its page zoom. A page that does not zoom is not.
func (w *window) zoomedIn() bool {
	if !w.gest.zoom || w.web == 0 {
		return false
	}
	if pdf, known := w.showsPDF(); known {
		if pdf {
			return w.pdf.ZoomedIn(w.pdfScale())
		}
		return msgFloat(w.web, sel("magnification")) > 1.01 // a document: its magnification says it all
	}
	if msgFloat(w.web, sel("magnification")) > 1.01 || msgFloat(w.web, sel("pageZoom")) > 1.01 {
		return true
	}
	return w.zoomed.smart || w.zoomed.scale > 1.01
}

// showsPDF reports whether the page is a PDF (WebKit's
// _isDisplayingPDF), and whether WebKit says.
func (w *window) showsPDF() (pdf, known bool) {
	if w.web == 0 || !respondsTo(w.web, "_isDisplayingPDF") {
		return false, false
	}
	return sendBool(w.web, "_isDisplayingPDF"), true
}

// pdfScale is a PDF's scale: its magnification, or its page zoom set by
// the app and not yet the plugin's (the larger).
func (w *window) pdfScale() float64 {
	return max(msgFloat(w.web, sel("magnification")), msgFloat(w.web, sel("pageZoom")))
}

// pdfZooms says a PDF shown zooms now (a pinch, a smart zoom, the app's
// page zoom): its scale before is its fit, the first time (pdffit).
func (w *window) pdfZooms() {
	if pdf, _ := w.showsPDF(); pdf && !w.pdf.Known() {
		w.pdf.Touch(w.pdfScale())
		gestureLog("pdf zooms: its fit is %.3f", w.pdf.At())
	}
}

// pageLoaded forgets a PDF's fit: a page was committed.
func (w *window) pageLoaded() { w.pdf.Reset() }

// toPage reports whether ev, a pinch's event the page has, goes on to
// it, and counts it (pageTookPinch): a PDF's fit is known first; when
// pinches in go to the content, one zooming a PDF out under its fit is
// dropped (it stops at its fit, pdffit.Hold).
func (w *window) toPage(ev id) bool {
	m := msgFloat(ev, sel("magnification"))
	if w.gest.zoom {
		w.pdfZooms()
	}
	if w.gest.pinchIn && w.pdf.Known() && uint(send(ev, "phase"))&0x4 != 0 { // NSEventPhaseChanged
		if mag := msgFloat(w.web, sel("magnification")); w.pdf.Hold(mag, m) {
			gestureLog("pinch held at the PDF's fit: magnification=%.3f fit=%.3f event=%.4f", mag, w.pdf.At(), m)
			return false
		}
	}
	w.pageTookPinch(m)
	return true
}

// pageTookPinch counts a pinch's event the page took (m its
// magnification): a smart zoom counts as twice the size.
func (w *window) pageTookPinch(m float64) {
	z := &w.zoomed
	if z.smart {
		z.scale, z.smart = max(z.scale, 2), false
	}
	z.scale = min(max(z.scale*(1+m), 1), 8)
}

// pinnedState is WebKit's _pinnedState (the edges the main frame is
// scrolled to; one that does not scroll is at all four), false when the
// web view does not say.
func (w *window) pinnedState() (uint, bool) {
	if w.web == 0 || !respondsTo(w.web, "_pinnedState") {
		return 0, false
	}
	return uint(send(w.web, "_pinnedState")), true
}

// routeScroll sends ev, a scroll event over the web view, to the page or
// to the content (wheelroute).
func (w *window) routeScroll(ev id) {
	momentum := uint(send(ev, "momentumPhase"))
	w.scroll.Scroll(wheelroute.Event{
		Ref:           uintptr(ev),
		Phase:         uint(send(ev, "phase")),
		Momentum:      momentum != 0,
		MomentumPhase: momentum,
		DX:            msgFloat(ev, sel("scrollingDeltaX")),
		DY:            msgFloat(ev, sel("scrollingDeltaY")),
		At:            msgFloat(ev, sel("timestamp")),
		Precise:       sendBool(ev, "hasPreciseScrollingDeltas"),
	})
}

// pageDidNotScroll is WebKit saying nothing scrolled for ev (its UI
// delegate's _webView:didNotHandleWheelEvent:).
func (w *window) pageDidNotScroll(ev id) {
	if ev != 0 {
		w.scroll.NotHandled(uintptr(ev), msgFloat(ev, sel("timestamp")))
	}
}

// scrollEnv is what the web view's wheelroute.Router acts on.
type scrollEnv struct{ w *window }

func (e scrollEnv) Page(ev wheelroute.Event) {
	if e.w.closed {
		return
	}
	sendSuper(e.w.web, "MyGoWebView", sel("scrollWheel:"), ev.Ref)
}

func (e scrollEnv) View(ev wheelroute.Event) {
	if !e.w.closed && e.w.host != nil && e.w.host.surface != nil {
		send(e.w.host.surface.view, "scrollWheel:", ev.Ref)
	}
}

func (e scrollEnv) Keep(ev wheelroute.Event)  { send(id(ev.Ref), "retain") }
func (e scrollEnv) Drop(ev wheelroute.Event)  { release(id(ev.Ref)) }
func (e scrollEnv) Pinned() (uint, bool)      { return e.w.pinnedState() }
func (e scrollEnv) Log(f string, args ...any) { gestureLog(f, args...) }
func (e scrollEnv) After(d time.Duration, fn func()) {
	time.AfterFunc(d, func() { theBackend.post(fn) })
}

// routeMagnify sends ev, a pinch's event over the web view, to the
// content when the pinch is the content's, and reports whether it did;
// a pinch not decided yet keeps its events until it is (they go to
// whichever has it then).
func (w *window) routeMagnify(self id, cmd objc.SEL, ev id) bool {
	st := &w.pinch
	page := func(e id) {
		if w.toPage(e) {
			sendSuper(self, "MyGoWebView", cmd, uintptr(e))
		}
	}
	content := func(e id) {
		st.at = time.Now()
		gestureLog("pinch to content phase=%#x magnification=%.4f", uint(send(e, "phase")), msgFloat(e, sel("magnification")))
		if w.host.surface != nil {
			send(w.host.surface.view, "magnifyWithEvent:", uintptr(e))
		}
	}
	phase := uint(send(ev, "phase"))
	if phase&0x1 != 0 { // NSEventPhaseBegan
		w.dropPinch()
		st.route, st.sum = routePage, 0
		if !w.zoomedIn() {
			st.route = routeUndecided
		}
		w.logPinchBegan(st.route == routePage)
	}
	ended := phase&(0x8|0x10) != 0
	switch st.route {
	case routeContent:
		content(ev)
		if ended {
			st.route = routeNone
			w.pinchEnded()
		}
		return true
	case routeUndecided:
	default:
		return false // the page's (the caller sends it on)
	}
	st.pending = append(st.pending, send(ev, "retain"))
	st.sum += msgFloat(ev, sel("magnification"))
	if math.Abs(st.sum) < 0.005 && !ended {
		return true
	}
	st.route = routePage
	if st.sum < 0 {
		st.route = routeContent
	}
	gestureLog("pinch sum=%.3f magnification=%.2f counted=%.2f: content=%v", st.sum, msgFloat(w.web, sel("magnification")), w.zoomed.scale, st.route == routeContent)
	pending := st.pending
	st.pending = nil
	for _, e := range pending {
		if st.route == routeContent {
			content(e)
		} else {
			page(e)
		}
		release(e)
	}
	if ended {
		contentHad := st.route == routeContent
		st.route = routeNone
		if contentHad {
			w.pinchEnded()
		}
	}
	return true
}

// logPinchBegan logs what a pinch's start was decided on (page: zoomed
// in, it is the page's; else it waits for its first magnification).
func (w *window) logPinchBegan(page bool) {
	if !gestureLogOn() {
		return
	}
	pdf, known := w.showsPDF()
	to := "undecided (the content's if the fingers close)"
	if page {
		to = "the page's (zoomed in)"
	}
	gestureLog("pinch began: pinchIn=%v zoom=%v pdf=%v (known %v) magnification=%.3f pageZoom=%.3f fit=%.3f counted=%.2f smart=%v: %s",
		w.gest.pinchIn, w.gest.zoom, pdf, known, msgFloat(w.web, sel("magnification")), msgFloat(w.web, sel("pageZoom")),
		w.pdf.At(), w.zoomed.scale, w.zoomed.smart, to)
}

// dropPinch lets go of the events kept for a pinch not routed yet.
func (w *window) dropPinch() {
	for _, e := range w.pinch.pending {
		release(e)
	}
	w.pinch.pending = nil
}

// gestureLog writes where a web view's gestures went to the log package
// (stderr unless the app sends it elsewhere) when MYGO_WEBVIEW_GESTURES
// is set (checking them on a Mac).
func gestureLog(format string, args ...any) {
	if gestureLogOn() {
		log.Printf("mygo webview gestures: "+format, args...)
	}
}

var gestureLogOn = sync.OnceValue(func() bool { return os.Getenv("MYGO_WEBVIEW_GESTURES") != "" })
