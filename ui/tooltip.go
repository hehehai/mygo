package ui

import (
	"slices"
	"time"
)

const (
	// tooltipDelay is how long the pointer rests on an element before its
	// tip shows, and tooltipWarm how soon after a tip goes another shows at
	// once, as the pointer moves along a toolbar: Base UI's, which a
	// theme's TooltipDelay changes.
	tooltipDelay = 600 * time.Millisecond
	tooltipWarm  = 400 * time.Millisecond
)

// tooltips is which element's tip shows, which the frame decides once
// (show), from what the last frame left.
type tooltips struct {
	frame uint64
	pass  int
	// shown is the element whose tip shows in the frame, byFocus telling
	// that the keyboard focus brought it rather than the pointer.
	shown   uint64
	byFocus bool
	// hover is the innermost element with a tip under the pointer since
	// hoverSince, focus the one with the keyboard focus since focusSince.
	hover, focus           uint64
	hoverSince, focusSince time.Time
	// dismissed is the element whose tip a press or Escape hid, until
	// neither the pointer nor the focus is on it.
	dismissed uint64
	// lastShown is when a tip last showed, or went.
	lastShown time.Time
	// For TooltipRest: moved is when the pointer last moved, cooled when
	// it last scrolled, pressed or dragged, shownSince when the tip of
	// shown began to show, and faded whether it came after its delay
	// (and fades in) rather than at once in the warm-up.
	moved, cooled, shownSince time.Time
	faded                     bool
}

// tipKey is the identity of the tip of an element, so that the tip of
// the next element, in the warm-up, is a tip of its own, not this one
// moving there.
type tipKey struct{ anchor uint64 }

// TooltipBase shows a tip without a look by anchor: fn styles the tip and
// builds its content. It shows once the pointer has rested on anchor for
// 0.6 seconds, at once as anchor gets the keyboard focus from the
// keyboard, and at once too within 0.4 seconds of another tip going, as
// the pointer moves along a toolbar; the theme's TooltipDelay changes
// both, its TooltipRest has them wait for the pointer to rest, and its
// TooltipFadeIn and TooltipFadeOut fade the tip. It goes as the pointer leaves anchor
// or presses it, as a menu or popover of anchor opens, and with Escape,
// until the pointer comes back or the focus does; it does not show while
// that menu or popover shows. Over elements inside one another, the
// innermost one's tip shows; one shows at a time.
//
// The tip goes above anchor, centered, or below it where there is no room
// above; fn may place it elsewhere with AttachTo, as after it:
//
//	ui.TooltipBase(c, b, func(tip ui.Element) {
//		tip.AttachTo(b, ui.AnchorEnd, ui.AnchorStart).MarginStart(6)
//		tip.Padding(4, 8).Radius(6).Background(dark).TextColor(light)
//		tip.Children(func() { ui.Text(c, "Share") })
//	})
//
// The pointer goes through the tip, and assistive technology sees it as a
// tooltip, which it does not read: give anchor a Description to tell what
// the tip does. It returns the tip, or nil while it does not show;
// Element.Tooltip is TooltipBase with the theme's look.
func coreTooltipBase(c *context, anchor *node, fn func(tip *node)) *node {
	tip, _ := tooltipBase(c, anchor, fn)
	return tip
}

// tooltipBase is TooltipBase, which also reports whether the keyboard
// focus brought the tip rather than the pointer.
func tooltipBase(c *context, anchor *node, fn func(tip *node)) (*node, bool) {
	anchor.flags |= flagHover
	anchor.tip = true
	// The theme the anchor builds with sets how long the pointer rests on
	// it, which showTooltip reads from its state.
	anchor.st.tipDelay, anchor.st.tipWarm = c.theme.tooltipDelay()
	anchor.st.tipRest = c.theme.TooltipRest
	rt := c.rt
	// A press or a click on the anchor, by the pointer, the keys or
	// assistive technology, hides its tip, as Base UI's closeOnClick: a
	// menu button's menu opens as the pointer goes down, without a click.
	if anchor.st.clicks > 0 || slices.ContainsFunc(rt.downs, func(id uint64) bool { return rt.pressedWithin(id, anchor) }) {
		rt.tips.dismissed = anchor.id
	}
	if c.inert || rt.showTooltip(c) != anchor.id || rt.tips.dismissed == anchor.id {
		return nil, false
	}
	var tip *node
	coreOverlay(c, func() {
		tip = coreBox(c).Absolute().IgnoreInput(true).Role(RoleTooltip)
		if in, out := c.theme.TooltipFadeIn, c.theme.TooltipFadeOut; (in > 0 || out > 0) && !rt.preferences().ReduceMotion {
			tip.Key(tipKey{anchor.id})
			tip.Transition(rt.tips.fade(in, out))
		}
		fn(tip)
		if tip.attach == 0 {
			tip.AttachTo(anchor, AnchorTop, AnchorBottom)
		}
		if tip.OverlayShortcut(0, KeyEscape) {
			rt.tips.dismissed = anchor.id
		}
	})
	return tip, rt.tips.byFocus
}

// popupShows notes that a popup attached to e shows in the frame: its
// menu, its popover or its options, while which its tip does not show.
func (e *node) popupShows() {
	if e.hasState() {
		e.st.popped = e.c.rt.frame
	}
}

// hideTip hides the tip of the element of s as its menu opens, by the
// pointer, the keys or assistive technology, until neither the pointer
// nor the focus is on it, as a press on it does.
func (rt *engine) hideTip(s *state) {
	if s.tip {
		rt.tips.dismissed = s.id
	}
}

// Tooltip shows s by the element when the pointer rests on it, or the
// keyboard focus comes to it, as TooltipBase does: above the element,
// centered, or below it where there is no room above. It describes the
// element to assistive technology where Description does not.
func (e *node) Tooltip(s string) *node {
	if e.description == "" {
		e.description = s
	}
	if s == "" {
		return e
	}
	c := e.c
	t := c.theme
	fill, text := t.inverse()
	tooltipBase(c, e, func(tip *node) {
		tip.MaxWidth(t.Space(80)).Padding(t.Space(1.25), t.Space(2)).Radius(t.tooltipRadius()).
			Background(fill).TextColor(text).FontSize(t.FontSize - 1)
		tip.Shadow(0, 2, 8, 0, RGBA(0, 0, 0, 0.2))
		tip.Children(func() { coreText(c, s) })
		tip.AttachTo(e, AnchorTop, AnchorBottom).Margin(0, 0, t.Space(1.5), 0)
	})
	return e
}

// showTooltip returns the element whose tip shows in the frame, deciding it
// once: the innermost element with a tip under the pointer once it has
// rested there long enough, or the one with the keyboard focus from the
// keyboard, whichever came last.
func (rt *engine) showTooltip(c *context) uint64 {
	t := &rt.tips
	if t.frame == rt.frame && t.pass == rt.pass {
		return t.shown
	}
	t.frame, t.pass = rt.frame, rt.pass
	now := c.now
	prev := t.shown
	if prev != 0 {
		// The tip of the last frame showed until now, or, waiting for the
		// pointer to rest, until a scroll, press or drag since hid it.
		t.lastShown = now
		if s := rt.states[prev]; s != nil && s.tipRest && !t.cooled.Before(t.shownSince) {
			t.lastShown, prev = time.Time{}, 0
		}
	}
	tipped := func(id uint64) bool {
		s := rt.states[id]
		return s != nil && s.tip && s.flags&flagDisabled == 0
	}
	var hover uint64
	for _, id := range rt.hover {
		if tipped(id) {
			hover = id
			break
		}
	}
	if hover != t.hover {
		t.hover, t.hoverSince = hover, now
	}
	// The element with the keyboard focus, or the one it gives it to (as
	// a menu's highlighted item, an ActiveDescendant), whose tip shows as a
	// focused element's does.
	var focus uint64
	if rt.focusVisible && rt.windowFocus() {
		if f := rt.states[rt.focused]; f != nil {
			switch {
			case tipped(f.activeDescendant):
				focus = f.activeDescendant
			case tipped(rt.focused):
				focus = rt.focused
			}
		}
	}
	if focus != t.focus {
		t.focus, t.focusSince = focus, now
	}
	if t.dismissed != hover && t.dismissed != focus {
		t.dismissed = 0
	}
	if rt.pressed != nil && hover != 0 {
		t.dismissed = hover
	}
	if s := rt.states[hover]; hover != 0 && s.tipRest && (rt.pressed != nil || rt.drag != nil || rt.incoming != nil || rt.dropOver != 0) {
		// Pressed or dragging: no tip until the pointer rests again.
		t.cooled = now
	}
	// An element whose menu or popover shows, in this frame or the last
	// one, shows no tip: the pointer coming back to a menu button while
	// its menu shows, or the focus coming back to it as the menu closes,
	// leaves it hidden until they leave it.
	for _, id := range [2]uint64{hover, focus} {
		if s := rt.states[id]; id != 0 && s != nil && s.popped != 0 && s.popped+1 >= rt.frame {
			t.dismissed = id
		}
	}
	if hover == t.dismissed {
		hover = 0
	}
	if focus == t.dismissed {
		focus = 0
	}
	t.shown, t.byFocus = 0, false
	fs := rt.states[focus]
	focusRest := focus != 0 && fs != nil && fs.tipRest
	switch {
	case focusRest && !t.focusSince.Before(t.hoverSince):
		// Waiting for rest, the focus's tip shows after the delay, and
		// only while the pointer has not gone onto another element since.
		if t.rested(rt, now, prev, focus, fs, t.focusSince) {
			t.shown, t.byFocus = focus, true
		}
	case focus != 0 && !focusRest && (hover == 0 || !t.focusSince.Before(t.hoverSince)):
		t.shown, t.byFocus, t.faded = focus, true, true
	case hover != 0:
		s := rt.states[hover]
		if s.tipRest {
			if t.rested(rt, now, prev, hover, s, latest(t.hoverSince, t.moved, t.cooled)) {
				t.shown = hover
			}
			break
		}
		warm := !t.lastShown.IsZero() && t.hoverSince.Sub(t.lastShown) <= s.tipWarm
		wait := s.tipDelay - now.Sub(t.hoverSince)
		if wait > 0 && !warm {
			rt.scheduleAt(now.Add(wait))
			break
		}
		t.shown, t.faded = hover, wait <= 0
	}
	if t.shown != 0 && t.shown != prev {
		t.shownSince = now
	}
	return t.shown
}

// rested reports whether the tip of id, whose state is s, shows in a frame
// at now, waiting for the pointer or the focus to rest (TooltipRest) since
// since: at once if it showed in the last frame (prev), after s's delay,
// or a sixth of it within the warm-up after another tip went. It asks for
// the frame it is due in.
func (t *tooltips) rested(rt *engine, now time.Time, prev, id uint64, s *state, since time.Time) bool {
	if prev == id {
		return true
	}
	delay := s.tipDelay
	warm := !t.lastShown.IsZero() && t.lastShown.After(t.cooled) && since.Sub(t.lastShown) <= s.tipWarm
	if warm {
		delay /= 6
	}
	if wait := delay - now.Sub(since); wait > 0 {
		rt.scheduleAt(now.Add(wait))
		return false
	}
	t.faded = !warm
	return true
}

// fade returns the transition of a tip fading in over in, as it shows
// after its delay, and out over out: one shown at once in the warm-up
// comes without fading in.
func (t *tooltips) fade(in, out time.Duration) ElementTransition {
	f := ElementTransition{Duration: max(in, time.Millisecond), Ease: EaseOut, ExitDuration: out, Colors: true}
	if out <= 0 {
		f.ExitDuration = -1
	}
	if in > 0 && t.faded {
		f.Enter = &Motion{Opacity: 0}
	}
	if out > 0 {
		f.Exit = &Motion{Opacity: 0}
	}
	return f
}

// coolTips notes a scroll or a drag over the window, which hides the tip
// of an element waiting for the pointer to rest (TooltipRest) and ends
// the warm-up.
func (rt *engine) coolTips() {
	t := &rt.tips
	t.cooled = rt.now()
	if s := rt.states[t.shown]; t.shown != 0 && s != nil && s.tipRest {
		rt.requestFrame()
	}
}

// latest returns the latest of times.
func latest(times ...time.Time) time.Time {
	var l time.Time
	for _, at := range times {
		if at.After(l) {
			l = at
		}
	}
	return l
}
