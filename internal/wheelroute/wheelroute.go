// Package wheelroute decides where a touchpad's scrolling gestures over a
// web view go: to its page or to the window's view around it, as if the
// pointer were on that (WebView.SetGestures; the darwin backend feeds it
// AppKit's events and WebKit's answers). It holds no platform state: the
// events are opaque references the Env keeps, sends on and lets go of.
//
// A gesture is decided from its first points (3 of travel):
//
//   - sideways (Routing.Sideways): going sideways first, the page not
//     zoomed in — the view's;
//   - edges (Routing.Edges): going first toward an edge along the edges'
//     axis. When the main frame says it can scroll that way (its pinned
//     state), the page's. Else the page is asked: it gets the gesture's
//     first moving event, the rest wait, and if WebKit says it did not
//     handle that event (NotHandled) — nothing under the pointer scrolls
//     that way: not the frame, a box of the page or a PDF's plugin — the
//     gesture is the view's with that event; no answer in AskWait (or
//     AskEvents events) and it is the page's. WebKit tells only a
//     gesture's start apart from its edges (later events at an edge
//     bounce), so a gesture that scrolls the page to its end stays the
//     page's, with its glide: the next one, toward the edge, is the
//     view's. The main frame's pinned state alone is wrong after a load
//     (all four edges until it scrolls) and for a PDF (its plugin
//     scrolls, the frame does not);
//   - else the page's.
//
// A glide (momentum) goes where its gesture went; a glide of a gesture
// the router did not see begin since Set (a page loaded) is dropped: the
// gesture that turned to the page — the view's — or scrolled the page
// before goes on gliding after the new page loaded, and the new page
// would take it (it scrolled half way down as it showed). A mouse
// wheel's events (not Precise) are the page's.
package wheelroute

import (
	"math"
	"time"
)

// Phase bits, as AppKit's NSEventPhase.
const (
	PhaseBegan     = 0x1
	PhaseChanged   = 0x4
	PhaseEnded     = 0x8
	PhaseCancelled = 0x10
	PhaseMayBegin  = 0x20
)

// The bits of WebKit's _WKRectEdge (1 << CGRectEdge): the edges the main
// frame is scrolled to.
const (
	PinnedLeft   = 1
	PinnedTop    = 2
	PinnedRight  = 4
	PinnedBottom = 8
)

// Edges: what a gesture toward an edge does.
const (
	EdgesPage     = iota // stays the page's
	EdgesScrolled        // the view's when nothing scrolls that way (as EdgesAll since WebKit is asked)
	EdgesAll             // the view's when nothing scrolls that way
)

// How long a gesture asked about waits for WebKit's answer, at most
// (on a Mac it came within a millisecond: the UI process's scrolling
// tree answers).
const (
	AskWait   = 40 * time.Millisecond
	AskEvents = 5
)

// Event is a scroll event over the web view.
type Event struct {
	Ref      uintptr // the platform's event
	Phase    uint    // Phase… bits (0 in a glide)
	Momentum bool    // part of a glide
	// MomentumPhase is a glide's phase (Phase… bits, as AppKit's
	// momentumPhase: Began, Changed, Ended); 0 when not known.
	MomentumPhase uint
	DX, DY        float64 // its travel (+ toward the page's left, its top)
	At            float64 // its time
	Precise       bool    // a touchpad's (else a mouse wheel's)
}

// Routing is what SetGestures asked for scrolling.
type Routing struct {
	Sideways      bool
	Edges         int  // Edges…
	EdgesSideways bool // the edges are left and right (else top and bottom)
	Zoomed        func() bool
}

// Active reports whether some scrolling may go to the view.
func (g Routing) Active() bool { return g.Sideways || g.Edges != EdgesPage }

// Env is what the router acts on, on the main thread.
type Env interface {
	Page(Event)                       // send it to the page
	View(Event)                       // send it to the window's view
	Keep(Event)                       // retain it (held)
	Drop(Event)                       // release it
	Pinned() (edges uint, known bool) // the main frame's pinned state
	After(d time.Duration, fn func()) // run fn on the main thread after d
	Log(format string, args ...any)   // nil-safe logging
}

// Where a gesture goes.
const (
	routeNone = iota
	routeUndecided
	routePage
	routeView
	routeAsking
)

// Router routes the gestures over one web view.
type Router struct {
	Env     Env
	routing Routing
	route   int
	pending []Event
	dx, dy  float64
	// Asking: the event the page got, how many gestures were asked about
	// (a late timer's is not this one's), when.
	probe   Event
	asked   int
	askedAt time.Time
	// dropped counts the events of the glide being dropped (logged as it
	// ends), glides those of all the glides dropped (Dropped).
	dropped int
	glides  int
}

// Set starts afresh with g (a page loaded); held events are let go of.
func (r *Router) Set(g Routing) {
	r.drop()
	r.glideDone()
	r.routing, r.route, r.dx, r.dy = g, routeNone, 0, 0
}

// Dropped is how many events of glides the router dropped (a glide of
// a gesture before Set), all told.
func (r *Router) Dropped() int { return r.glides }

// Routing is what Set was given.
func (r *Router) Routing() Routing { return r.routing }

// Scroll routes e.
func (r *Router) Scroll(e Event) {
	if !e.Precise {
		r.Env.Page(e)
		return
	}
	if !e.Momentum && e.Phase&(PhaseBegan|PhaseMayBegin) != 0 { // a gesture starts
		r.glideDone()
		r.settle(routePage)
		for _, p := range r.pending { // a touch (MayBegin) not decided: the page's
			r.send(routePage, p)
		}
		r.pending = nil
		r.route, r.dx, r.dy = routeUndecided, 0, 0
	}
	switch r.route {
	case routeView:
		r.Env.View(e)
		return
	case routeAsking:
		r.keep(e)
		if len(r.pending) >= AskEvents {
			r.settle(routePage)
		}
		return
	case routeUndecided:
		if e.Momentum { // the glide of a gesture too short to decide
			r.Env.Page(e)
			return
		}
	case routeNone:
		if e.Momentum { // the glide of a gesture before Set: no one's
			r.dropGlide(e)
			return
		}
		r.Env.Page(e)
		return
	default:
		r.Env.Page(e)
		return
	}
	r.keep(e)
	r.dx += e.DX
	r.dy += e.DY
	ended := e.Phase&(PhaseEnded|PhaseCancelled) != 0
	if math.Abs(r.dx)+math.Abs(r.dy) < 3 && !ended {
		return
	}
	pinned, known := r.Env.Pinned()
	zoomed := r.routing.Zoomed != nil && r.routing.Zoomed()
	r.route = decide(r.routing, r.dx, r.dy, zoomed, pinned, known)
	r.log("scroll dx=%.1f dy=%.1f pinned=%#x known=%v zoomed=%v: %s", r.dx, r.dy, pinned, known, zoomed, routeName(r.route))
	pending := r.pending
	r.pending = nil
	if r.route != routeAsking {
		for _, p := range pending {
			r.send(r.route, p)
		}
		return
	}
	// The page gets the events up to the first that moves; the rest wait
	// for its answer.
	i := 0
	for i < len(pending)-1 && pending[i].DX == 0 && pending[i].DY == 0 {
		i++
	}
	for _, p := range pending[:i] {
		r.send(routePage, p)
	}
	r.probe, r.pending = pending[i], pending[i+1:]
	r.asked++
	asked := r.asked
	r.askedAt = time.Now()
	r.Env.Page(r.probe)
	r.Env.After(AskWait, func() {
		if r.route == routeAsking && r.asked == asked {
			r.settle(routePage)
		}
	})
}

// NotHandled is WebKit saying nothing scrolled for the event of time at
// (ref the event): the event a gesture asked about makes it the view's.
func (r *Router) NotHandled(ref uintptr, at float64) {
	if r.route != routeAsking || ref != r.probe.Ref && at < r.probe.At-1e-6 { // not asked, or an event before it
		return
	}
	r.log("not handled after %v: content", time.Since(r.askedAt).Round(time.Millisecond))
	r.settle(routeView)
}

// decide decides a gesture from its first points: dx, dy its travel,
// pinned the main frame's pinned state (known: WebKit said it).
func decide(g Routing, dx, dy float64, zoomed bool, pinned uint, known bool) int {
	sideways := math.Abs(dx) > math.Abs(dy)
	if sideways && g.Sideways && !zoomed {
		return routeView
	}
	if g.Edges == EdgesPage || sideways != g.EdgesSideways {
		return routePage
	}
	toward, d := uint(PinnedTop), dy
	if sideways {
		toward, d = PinnedLeft, dx
	}
	if d < 0 {
		toward <<= 2 // the bottom, the right
	}
	if known && pinned&toward == 0 {
		return routePage
	}
	return routeAsking
}

// Bounce is the edges the main frame may bounce at under g: not along
// the edges' axis, else WebKit takes a gesture's start there for it.
func Bounce(g Routing) uint {
	switch {
	case g.Edges == EdgesPage:
		return PinnedLeft | PinnedTop | PinnedRight | PinnedBottom
	case g.EdgesSideways:
		return PinnedTop | PinnedBottom
	}
	return PinnedLeft | PinnedRight
}

// settle ends asking about the gesture: it is route's, with the events
// held meanwhile (the view's: with the one asked about).
func (r *Router) settle(route int) {
	if r.route != routeAsking {
		return
	}
	if route == routePage {
		r.log("no answer in %v (%d events): page", time.Since(r.askedAt).Round(time.Millisecond), len(r.pending))
	}
	r.route = route
	probe, pending := r.probe, r.pending
	r.probe, r.pending = Event{}, nil
	if route == routeView {
		r.Env.View(probe)
	}
	r.Env.Drop(probe)
	for _, p := range pending {
		r.send(route, p)
	}
}

// send sends a held event on and lets go of it.
func (r *Router) send(route int, e Event) {
	if route == routeView {
		r.Env.View(e)
	} else {
		r.Env.Page(e)
	}
	r.Env.Drop(e)
}

func (r *Router) keep(e Event) {
	r.Env.Keep(e)
	r.pending = append(r.pending, e)
}

// drop lets go of the events held.
func (r *Router) drop() {
	for _, p := range r.pending {
		r.Env.Drop(p)
	}
	r.pending = nil
	if r.probe.Ref != 0 {
		r.Env.Drop(r.probe)
	}
	r.probe = Event{}
}

// dropGlide drops e, an event of a glide whose gesture the page did not
// have (it began before Set).
func (r *Router) dropGlide(e Event) {
	if r.dropped == 0 {
		r.log("glide of a gesture before the page loaded: dropped")
	}
	r.dropped++
	r.glides++
	if e.MomentumPhase&(PhaseEnded|PhaseCancelled) != 0 {
		r.glideDone()
	}
}

// glideDone logs the glide dropped, if any.
func (r *Router) glideDone() {
	if r.dropped > 0 {
		r.log("glide dropped: %d events", r.dropped)
		r.dropped = 0
	}
}

func (r *Router) log(format string, args ...any) { r.Env.Log(format, args...) }

func routeName(route int) string {
	switch route {
	case routeView:
		return "content"
	case routeAsking:
		return "asking"
	}
	return "page"
}
