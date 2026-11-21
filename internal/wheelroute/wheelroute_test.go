package wheelroute

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeEnv records where events went; its timers run when the test says.
type fakeEnv struct {
	pinned uint
	known  bool
	sent   []string // "page 3", "view 4"
	kept   map[uintptr]int
	timers []func()
}

func newEnv(pinned uint) *fakeEnv {
	return &fakeEnv{pinned: pinned, known: true, kept: map[uintptr]int{}}
}

func (f *fakeEnv) Page(e Event)                     { f.sent = append(f.sent, fmt.Sprint("page ", e.Ref)) }
func (f *fakeEnv) View(e Event)                     { f.sent = append(f.sent, fmt.Sprint("view ", e.Ref)) }
func (f *fakeEnv) Keep(e Event)                     { f.kept[e.Ref]++ }
func (f *fakeEnv) Drop(e Event)                     { f.kept[e.Ref]-- }
func (f *fakeEnv) Pinned() (uint, bool)             { return f.pinned, f.known }
func (f *fakeEnv) After(_ time.Duration, fn func()) { f.timers = append(f.timers, fn) }
func (f *fakeEnv) Log(string, ...any)               {}

// fire runs the timers due.
func (f *fakeEnv) fire() {
	t := f.timers
	f.timers = nil
	for _, fn := range t {
		fn()
	}
}

// took is where the events went, "page 1, page 2, view 3".
func (f *fakeEnv) took() string { s := strings.Join(f.sent, ", "); f.sent = nil; return s }

// leaks reports events kept and not let go of.
func (f *fakeEnv) leaks() []uintptr {
	var l []uintptr
	for ref, n := range f.kept {
		if n != 0 {
			l = append(l, ref)
		}
	}
	return l
}

// gesture makes a touchpad gesture's events from ref on: began, n-2
// changed, ended, each dy (− down: toward the bottom), then glide
// momentum events; times from at, 1/60 s apart.
func gesture(ref uintptr, at float64, n int, dy float64, glide int) []Event {
	var evs []Event
	for k := range n {
		ph := uint(PhaseChanged)
		switch k {
		case 0:
			ph = PhaseBegan
		case n - 1:
			ph = PhaseEnded
		}
		evs = append(evs, Event{Ref: ref, Phase: ph, DY: dy, At: at, Precise: true})
		ref++
		at += 1.0 / 60
	}
	for range glide {
		evs = append(evs, Event{Ref: ref, Momentum: true, DY: dy / 2, At: at, Precise: true})
		ref++
		at += 1.0 / 60
	}
	return evs
}

var vertical = Routing{Edges: EdgesAll}

// TestPageScrolledToItsEnd: a gesture that starts where the page scrolls
// is the page's to its end with its glide, though the page reaches its
// bottom on the way (its pinned state changes, WebKit stops handling the
// events); the next gesture down, from the bottom, is the view's (#137:
// on the Mac a card turned in the gesture that scrolled it down).
func TestPageScrolledToItsEnd(t *testing.T) {
	env := newEnv(PinnedTop | PinnedLeft | PinnedRight) // at the top
	r := &Router{Env: env}
	r.Set(vertical)
	evs := gesture(1, 10, 8, -40, 6)
	for i, e := range evs {
		if i == 3 {
			env.pinned = PinnedBottom | PinnedLeft | PinnedRight // reached the bottom
		}
		r.Scroll(e)
		if i >= 3 {
			r.NotHandled(e.Ref, e.At) // at the end: WebKit does not scroll it
		}
	}
	env.fire()
	if got := env.took(); strings.Contains(got, "view") || strings.Count(got, "page") != len(evs) {
		t.Fatalf("the gesture that scrolled to the end went %s", got)
	}
	evs = gesture(100, 20, 8, -40, 0) // from the bottom, down
	for i, e := range evs {
		r.Scroll(e)
		if i == 0 {
			r.NotHandled(e.Ref, e.At)
		}
	}
	if got, want := env.took(), "page 100, view 100, view 101, view 102, view 103, view 104, view 105, view 106, view 107"; got != want {
		t.Errorf("the next gesture down went\n%s, want\n%s", got, want)
	}
	if l := env.leaks(); len(l) > 0 {
		t.Errorf("events not let go of: %v", l)
	}
}

// TestStalePinnedState: after a load WebKit says the main frame is at all
// its edges until it scrolls; a gesture down from the top is asked about,
// handled (no answer), and stays the page's with the events held.
func TestStalePinnedState(t *testing.T) {
	env := newEnv(0xf)
	r := &Router{Env: env}
	r.Set(vertical)
	evs := gesture(1, 10, 5, -40, 3)
	r.Scroll(evs[0])
	if got := env.took(); got != "page 1" {
		t.Fatalf("asked with %q, want the first event", got)
	}
	for _, e := range evs[1:4] {
		r.Scroll(e)
	}
	if got := env.took(); got != "" {
		t.Fatalf("held events went %s", got)
	}
	env.fire() // no answer in AskWait
	if got := env.took(); got != "page 2, page 3, page 4" {
		t.Fatalf("after the wait: %s", got)
	}
	for _, e := range evs[4:] {
		r.Scroll(e)
	}
	r.NotHandled(evs[0].Ref, evs[0].At) // too late
	if got := env.took(); got != "page 5, page 6, page 7, page 8" {
		t.Fatalf("the rest: %s", got)
	}
	if l := env.leaks(); len(l) > 0 {
		t.Errorf("events not let go of: %v", l)
	}
}

// TestPDFAtItsEnd: a PDF's frame is pinned at all edges (its plugin
// scrolls); at the PDF's end WebKit does not handle the gesture's first
// event: the gesture is the view's with it and its glide.
func TestPDFAtItsEnd(t *testing.T) {
	env := newEnv(0xf)
	r := &Router{Env: env}
	r.Set(Routing{Edges: EdgesScrolled})
	evs := gesture(1, 10, 4, -40, 2)
	r.Scroll(evs[0])
	r.Scroll(evs[1])
	r.NotHandled(evs[0].Ref, evs[0].At)
	for _, e := range evs[2:] {
		r.Scroll(e)
	}
	env.fire() // the timer finds it decided
	if got, want := env.took(), "page 1, view 1, view 2, view 3, view 4, view 5, view 6"; got != want {
		t.Errorf("went %s, want %s", got, want)
	}
	if l := env.leaks(); len(l) > 0 {
		t.Errorf("events not let go of: %v", l)
	}
}

// TestAskOnlyAboutTheStart: an answer for another event than the one
// asked about (one before it: the touch's zero event, an old gesture's)
// decides nothing; the first moving event is the one asked about; the
// events held are at most AskEvents.
func TestAskOnlyAboutTheStart(t *testing.T) {
	env := newEnv(0xf)
	r := &Router{Env: env}
	r.Set(vertical)
	r.Scroll(Event{Ref: 1, Phase: PhaseMayBegin, At: 9.9, Precise: true})
	evs := gesture(2, 10, 12, -2, 0)
	evs[0].DY = 0 // began, not moving
	for _, e := range evs[:3] {
		r.Scroll(e)
	}
	if got := env.took(); got != "page 1, page 2, page 3" {
		t.Fatalf("asked with %s, want the touch, the began and the first moving event", got)
	}
	r.NotHandled(1, 9.9)
	r.NotHandled(2, 10)
	for _, e := range evs[3 : 3+AskEvents] {
		r.Scroll(e)
	}
	var want []string // the one held when it was decided, and those after it
	for ref := 4; ref <= 5+AskEvents-1; ref++ {
		want = append(want, fmt.Sprint("page ", ref))
	}
	if got := env.took(); got != strings.Join(want, ", ") {
		t.Fatalf("an answer about other events decided: %s", got)
	}
	if r.route != routePage {
		t.Errorf("route %v after %d events held, want the page's", r.route, AskEvents)
	}
}

// TestNoEdges: a mouse wheel and a page without edges keep their events.
func TestNoEdges(t *testing.T) {
	env := newEnv(0xf)
	r := &Router{Env: env}
	r.Set(Routing{})
	for _, e := range gesture(1, 10, 3, -40, 1) {
		r.Scroll(e)
	}
	r.Set(vertical)
	r.Scroll(Event{Ref: 9, DY: -10})
	if got := env.took(); got != "page 1, page 2, page 3, page 4, page 9" {
		t.Errorf("went %s", got)
	}
}

// TestSetLetsGo: a page loaded while a gesture is asked about lets go of
// its events; the old gesture's timer decides nothing.
func TestSetLetsGo(t *testing.T) {
	env := newEnv(0xf)
	r := &Router{Env: env}
	r.Set(vertical)
	for _, e := range gesture(1, 10, 3, -40, 0)[:2] {
		r.Scroll(e)
	}
	r.Set(vertical)
	env.took()
	env.fire()
	if got := env.took(); got != "" {
		t.Errorf("after Set the old gesture's events went %s", got)
	}
	if l := env.leaks(); len(l) > 0 {
		t.Errorf("events not let go of: %v", l)
	}
}

// TestDecide: which gestures go where from their first points and the
// main frame's pinned state.
func TestDecide(t *testing.T) {
	const top, bottom, left, right = PinnedTop, PinnedBottom, PinnedLeft, PinnedRight
	sideways := Routing{Sideways: true}
	allX := Routing{Edges: EdgesAll, EdgesSideways: true}
	for _, c := range []struct {
		name   string
		g      Routing
		dx, dy float64
		zoomed bool
		pinned uint
		known  bool
		want   int
	}{
		{"sideways: sideways first", sideways, -5, 1, false, 0, false, routeView},
		{"sideways: up and down stay", sideways, 1, -5, false, top | bottom, true, routePage},
		{"sideways: zoomed in, the page's", sideways, -5, 1, true, 0, false, routePage},
		{"page: nothing", Routing{}, 0, 5, false, 0xf, true, routePage},
		{"edges: up at the top", vertical, 0, 5, false, top | left | right, true, routeAsking},
		{"edges: down at the top, the frame scrolls", vertical, 0, -5, false, top | left | right, true, routePage},
		{"edges: down at the bottom", vertical, 0, -5, false, bottom | left | right, true, routeAsking},
		{"edges: in the middle", vertical, 0, -5, false, left | right, true, routePage},
		{"edges: unknown", vertical, 0, -5, false, 0, false, routeAsking},
		{"edges: all four (a load, a PDF, a page that fits)", vertical, 0, -5, false, 0xf, true, routeAsking},
		{"edges up and down: sideways stays the page's", vertical, -5, 0, false, 0xf, true, routePage},
		{"edges sideways: up and down stay the page's", allX, 0, -5, false, 0xf, true, routePage},
		{"edges: left at the left edge", allX, 5, 0, false, left | top, true, routeAsking},
		{"edges: right at the left edge", allX, -5, 0, false, left | top, true, routePage},
		{"edges: right at the right edge", allX, -5, 0, false, right, true, routeAsking},
		{"edges: zoomed, at the bottom", vertical, 0, -5, true, bottom, true, routeAsking},
	} {
		if got := decide(c.g, c.dx, c.dy, c.zoomed, c.pinned, c.known); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, routeName(got), routeName(c.want))
		}
	}
	if Bounce(vertical) != PinnedLeft|PinnedRight || Bounce(allX) != PinnedTop|PinnedBottom || Bounce(Routing{Sideways: true}) != 0xf {
		t.Error("Bounce")
	}
}

// glideOf makes a glide's events from ref on (momentum began, changed,
// ended), each dy, times from at.
func glideOf(ref uintptr, at float64, n int, dy float64) []Event {
	var evs []Event
	for k := range n {
		ph := uint(PhaseChanged)
		switch k {
		case 0:
			ph = PhaseBegan
		case n - 1:
			ph = PhaseEnded
		}
		evs = append(evs, Event{Ref: ref, Momentum: true, MomentumPhase: ph, DY: dy, At: at, Precise: true})
		ref++
		at += 1.0 / 60
	}
	return evs
}

// TestGlideAfterTurn (#174): a gesture from a page's end is the view's,
// which turns to the next item as the fingers lift; the next page loads
// (Set) while the gesture glides on. The rest of the glide is no one's —
// the new page took it and showed half way down — and the gesture after
// is routed as ever: down from its top, the page's.
func TestGlideAfterTurn(t *testing.T) {
	env := newEnv(0xf)
	r := &Router{Env: env}
	r.Set(vertical)
	evs := gesture(1, 10, 6, -40, 0) // from the end, down
	for i, e := range evs {
		r.Scroll(e)
		if i == 0 {
			r.NotHandled(e.Ref, e.At)
		}
	}
	env.took()
	glide := glideOf(100, 11, 12, -30)
	for _, e := range glide[:3] { // the item turns: the view's, as the gesture
		r.Scroll(e)
	}
	if got := env.took(); got != "view 100, view 101, view 102" {
		t.Fatalf("the glide before the next page loaded went %s", got)
	}
	r.Set(vertical) // the next item's page
	env.pinned = PinnedTop | PinnedLeft | PinnedRight
	for _, e := range glide[3:] {
		r.Scroll(e)
	}
	if got := env.took(); got != "" {
		t.Fatalf("the glide after the next page loaded went %s, want nowhere", got)
	}
	if r.Dropped() != len(glide)-3 {
		t.Errorf("dropped %d, want %d", r.Dropped(), len(glide)-3)
	}
	for _, e := range gesture(200, 13, 4, -40, 2) { // down from the new page's top
		r.Scroll(e)
	}
	if got, want := env.took(), "page 200, page 201, page 202, page 203, page 204, page 205"; got != want {
		t.Errorf("the next gesture went %s, want %s", got, want)
	}
	if l := env.leaks(); len(l) > 0 {
		t.Errorf("events not let go of: %v", l)
	}
}

// TestGlideOfThePageBefore: a page loaded while the page before glides
// (a key turned the item) does not take the rest of the glide; a touch
// (MayBegin) ends it, and the new gesture's events are the page's.
func TestGlideOfThePageBefore(t *testing.T) {
	env := newEnv(PinnedTop | PinnedLeft | PinnedRight)
	r := &Router{Env: env}
	r.Set(vertical)
	for _, e := range gesture(1, 10, 4, -40, 0) {
		r.Scroll(e)
	}
	glide := glideOf(10, 11, 8, -30)
	r.Scroll(glide[0])
	r.Set(vertical)
	for _, e := range glide[1:4] { // cut short by a touch: no momentum end
		r.Scroll(e)
	}
	r.Scroll(Event{Ref: 50, Phase: PhaseMayBegin, At: 12, Precise: true})
	for _, e := range gesture(51, 12.1, 3, -40, 0) {
		r.Scroll(e)
	}
	if got, want := env.took(), "page 1, page 2, page 3, page 4, page 10, page 50, page 51, page 52, page 53"; got != want {
		t.Errorf("went %s, want %s", got, want)
	}
	if r.Dropped() != 3 {
		t.Errorf("dropped %d, want 3", r.Dropped())
	}
}

// TestChangedAfterSet: a gesture's own events (not its glide) after Set
// stay the page's, as before: only a glide is dropped.
func TestChangedAfterSet(t *testing.T) {
	env := newEnv(PinnedTop | PinnedLeft | PinnedRight)
	r := &Router{Env: env}
	r.Set(vertical)
	evs := gesture(1, 10, 6, -40, 0)
	for _, e := range evs[:3] {
		r.Scroll(e)
	}
	r.Set(vertical)
	for _, e := range evs[3:] {
		r.Scroll(e)
	}
	if got := env.took(); got != "page 1, page 2, page 3, page 4, page 5, page 6" || r.Dropped() != 0 {
		t.Errorf("went %s (dropped %d)", got, r.Dropped())
	}
}
