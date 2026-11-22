package ui

import (
	"runtime"
	"time"
)

// Theme holds the colors and metrics widgets use. Change a copy of
// LightTheme or DarkTheme and set it with Context.SetTheme.
type Theme struct {
	Dark bool
	// DisableAutoDimming keeps disabled elements at full opacity. Applications
	// opting in must provide their own disabled colors; input, focus and
	// accessibility still use the element's disabled state.
	DisableAutoDimming bool
	// Background fills the window.
	Background Color
	// Surface is the face of buttons, inputs and other controls;
	// SurfaceHover and SurfacePressed while hovered or pressed.
	Surface        Color
	SurfaceHover   Color
	SurfacePressed Color
	// Border outlines controls.
	Border Color
	// Text and TextMuted color text; TextMuted is for secondary text and
	// placeholders.
	Text      Color
	TextMuted Color
	// Accent colors primary buttons, checked controls and focus rings;
	// AccentText is text on it. Where Accent is too light for AccentText
	// to read on, SetAccentFill gives what carries it a darker fill.
	Accent        Color
	AccentHover   Color
	AccentPressed Color
	AccentText    Color
	// InactiveAccent is what the accent fades to in a window that is not
	// drawn as active (Context.WindowActive), as AppKit fades an inactive
	// window's: the fill of selected rows, of checked check boxes, radio
	// buttons and switches, of sliders and progress bars, the line of the
	// selected tab and the icons of a sidebar, with AccentText on it as on
	// the accent; primary buttons show as the others there. Left zero, it
	// is a gray AccentText stands out on; set it to Accent to keep the
	// accent in every window.
	InactiveAccent Color
	Danger         Color
	// Warning and Success color what calls for attention and what went
	// well, as a Meter's levels.
	Warning Color
	Success Color
	// Selection highlights selected text.
	Selection Color
	// Focus is the ring around the control with the keyboard focus,
	// FocusRingWidth DIPs wide (2 by default; 0 is 2) and FocusRingOffset
	// DIPs outside its edge (1 in LightTheme and DarkTheme): 0 puts the
	// ring on the edge, and a negative offset inside it.
	// Element.FocusRingOffset and Element.FocusRingColor change them for
	// one element.
	Focus           Color
	FocusRingWidth  float32
	FocusRingOffset float32
	// Inverse fills tooltips and toasts, the theme turned over so that
	// they stand out from what they are over: dark in a light theme, light
	// in a dark one. InverseText is text on it. Left zero, they are Text
	// and Background.
	Inverse     Color
	InverseText Color
	// Scrollbar colors scroll bar thumbs, ScrollbarWidth DIPs wide (6 by
	// default; 0 is 6): the defaults of ScrollbarStyle, which overrides
	// them where it sets a field. It styles their tracks, hover and drag
	// colors, the width of their bars and thumbs, their corners and
	// shortest thumb; a theme that sets neither draws as it always has.
	Scrollbar      Color
	ScrollbarWidth float32
	ScrollbarStyle ScrollbarStyle
	// Radius rounds the corners of controls.
	Radius float32
	// DialogRadius rounds the corners of dialogs: Modal's and
	// AlertDialog's panels (0 is 2.5 units of Spacing).
	DialogRadius float32
	// PopupRadius rounds the panels that drop from a control: Popover's,
	// PopoverMenu's, the lists of Select, Combobox and Autocomplete,
	// DatePicker's and ColorPicker's panels (0 is Radius+2). Set, the
	// options and items in those lists are rounded with it less the
	// panels' padding, concentric with the panel (at least 4); 0 leaves
	// them at Radius.
	PopupRadius float32
	// ToastRadius rounds the toasts the view does not build itself (0 is
	// 2 units of Spacing); TooltipRadius tooltips (0 is 1.25 units).
	ToastRadius   float32
	TooltipRadius float32
	// Spacing is the unit of the room widgets leave inside and between
	// their parts: their paddings and gaps, and the sizes of check boxes,
	// switches, sliders and the rows of tables and trees, are multiples
	// of it. 4 by default: 3 makes widgets compact, 5 roomy; 0 is 4.
	Spacing float32
	// FontSize is the size of text, Font its family ("" is the system's).
	FontSize float32
	Font     string
	// TitleWeight is the weight of the titles widgets draw: a
	// SidebarSection's, a Table's column headers and header rows, an
	// AlertDialog's, a DatePicker's month, a toast's title (over its
	// description) and action, a chosen tab's, a fieldset's legend. 0
	// keeps theirs (600; an alert's and a month's 700); 400 draws them in
	// the regular weight, as for an app whose type sets titles apart by
	// size and colour alone.
	TitleWeight int
	// TooltipDelay is how long the pointer rests on an element before its
	// tooltip shows (Tooltip, TooltipBase): 0 is 0.6 seconds, and a
	// negative delay shows it at once. Within two thirds of it after
	// another tip goes, as the pointer moves along a toolbar, the next one
	// shows at once: 0.4 seconds by default.
	TooltipDelay time.Duration
	// TooltipRest, if set, waits for the pointer to rest: a tip's delay
	// counts from when the pointer last moved over the element, not from
	// when it came onto it; the keyboard focus's tip waits the delay too,
	// from when the focus moved, and does not show once the pointer has
	// gone onto another element since; within the warm-up after another
	// tip went, the next one shows once the pointer rests a sixth of the
	// delay rather than at once; a scroll, a press or a drag (of a value
	// or of what another app drags over the window) hides the tip and
	// ends the warm-up. Off by default: tips as TooltipDelay says.
	TooltipRest bool
	// TooltipFadeIn and TooltipFadeOut, if set, fade a tip in as it shows
	// after its delay (one shown at once, in the warm-up, comes at once)
	// and out as it goes, as macOS fades its tooltips (about 130 and 100
	// ms). 0, as by default, shows and hides it at once; so does a
	// desktop asking for less motion (Preferences.ReduceMotion).
	TooltipFadeIn, TooltipFadeOut time.Duration

	// followed is what WithPreferences changed, nil where it has not.
	followed *followedTheme

	// accentFill fills what carries AccentText while Accent is
	// accentFillOf (SetAccentFill).
	accentFill, accentFillOf Color
}

// SetAccentFill makes fill the face of what carries AccentText on the
// accent — the highlighted option of a Select, Combobox or PopoverMenu,
// a chosen row of a list, table or sidebar, a primary button, a checked
// box or radio, a chosen day or time field, the count of rows dragged —
// while Accent stays the colour of rings, progress, sliders, switches,
// tabs and links: a mid-tone accent such as teal, on which white text
// falls short of 4.5:1, gets a slightly darker fill white reads on. Set
// Accent first: a copy of the theme given another Accent (a section of
// the view recoloured, or the desktop's accent by WithPreferences) fills
// with that Accent again. AccentHover and AccentPressed are a primary
// button's under the pointer and pressed: derive them from fill. In a
// window not drawn as active the fill fades as the accent does
// (InactiveAccent).
func (t *Theme) SetAccentFill(fill Color) { t.accentFill, t.accentFillOf = fill, t.Accent }

// AccentFill is the face of what carries AccentText: SetAccentFill's
// fill while Accent is the one it was set for, else Accent.
func (t *Theme) AccentFill() Color {
	if t.accentFill.A > 0 && t.accentFillOf == t.Accent {
		return t.accentFill
	}
	return t.Accent
}

// followedTheme is a theme before and after following the desktop's
// preferences.
type followedTheme struct {
	_        [0]func() // used by pointer, so there is no value equality to link
	from, to Theme
}

// Space returns n units of the theme's Spacing, to size elements of your
// own in step with the widgets: ui.Column(c).Gap(t.Space(2)).
func (t *Theme) Space(n float32) float32 {
	s := t.Spacing
	if s <= 0 {
		s = 4
	}
	return n * s
}

// dialogRadius, popupRadius, toastRadius and tooltipRadius are the
// corners of dialogs, popups, toasts and tooltips: the theme's, or the
// defaults.
func (t *Theme) dialogRadius() float32  { return orRadius(t.DialogRadius, t.Space(2.5)) }
func (t *Theme) popupRadius() float32   { return orRadius(t.PopupRadius, t.Radius+2) }
func (t *Theme) toastRadius() float32   { return orRadius(t.ToastRadius, t.Space(2)) }
func (t *Theme) tooltipRadius() float32 { return orRadius(t.TooltipRadius, t.Space(1.25)) }

// popupOptionRadius rounds an option in a popup's list (padded by one
// unit): concentric with the panel when PopupRadius is set, else Radius.
func (t *Theme) popupOptionRadius() float32 {
	if t.PopupRadius <= 0 {
		return t.Radius
	}
	return max(t.PopupRadius-t.Space(1), 4)
}

// titleWeight is the weight of a widget's title: the theme's
// TitleWeight, or def.
func (t *Theme) titleWeight(def int) int {
	if t.TitleWeight > 0 {
		return t.TitleWeight
	}
	return def
}

func orRadius(r, def float32) float32 {
	if r > 0 {
		return r
	}
	return def
}

// inactiveAccent returns what the accent fades to in a window that is not
// drawn as active (InactiveAccent).
func (t *Theme) inactiveAccent() Color {
	if t.InactiveAccent != Transparent {
		return t.InactiveAccent
	}
	// As far from AccentText as text on it needs, 4.5 times its contrast.
	far := Color{A: 255}
	if t.AccentText.gray().R <= 127 {
		far = Color{R: 255, G: 255, B: 255, A: 255}
	}
	return t.AccentText.Mix(far, 0.55)
}

// inverse returns the fill of tooltips and toasts, and the color of text
// on it.
func (t *Theme) inverse() (fill, text Color) {
	fill, text = t.Inverse, t.InverseText
	if fill == Transparent {
		fill = t.Text
	}
	if text == Transparent {
		text = t.Background
	}
	return fill, text
}

// focusRingWidth returns how wide the focus ring is (FocusRingWidth).
func (t *Theme) focusRingWidth() float32 {
	if t.FocusRingWidth <= 0 {
		return 2
	}
	return t.FocusRingWidth
}

func (t *Theme) scrollbarWidth() float32 {
	if t.ScrollbarWidth <= 0 {
		return 6
	}
	return t.ScrollbarWidth
}

// tooltipDelay returns how long the pointer rests on an element before its
// tip shows (TooltipDelay), and how soon after a tip goes the next one
// shows at once.
func (t *Theme) tooltipDelay() (delay, warm time.Duration) {
	switch d := t.TooltipDelay; {
	case d == 0:
		return tooltipDelay, tooltipWarm
	case d < 0:
		return 0, 0
	default:
		return d, d * 2 / 3
	}
}

// Rem returns n times the theme's FontSize, as CSS's rem, to size elements
// with the text.
func (t *Theme) Rem(n float32) float32 {
	if t.FontSize <= 0 {
		return n * defaultFontSize()
	}
	return n * t.FontSize
}

func defaultFontSize() float32 {
	switch runtime.GOOS {
	case "darwin":
		return 13
	case "ios":
		return 17 // UIFont's body
	case "android":
		return 16 // Material's body large
	}
	return 14
}

// LightTheme returns the theme for a light appearance.
func LightTheme() *Theme {
	return &Theme{
		Background:      Hex("#ffffff"),
		Surface:         Hex("#f4f4f5"),
		SurfaceHover:    Hex("#e9e9ec"),
		SurfacePressed:  Hex("#dddde1"),
		Border:          Hex("#d9d9de"),
		Text:            Hex("#18181b"),
		TextMuted:       Hex("#71717a"),
		Accent:          Hex("#2563eb"),
		AccentHover:     Hex("#1d4ed8"),
		AccentPressed:   Hex("#1e40af"),
		AccentText:      Hex("#ffffff"),
		Danger:          Hex("#dc2626"),
		Warning:         Hex("#d97706"),
		Success:         Hex("#16a34a"),
		Selection:       RGBA(37, 99, 235, 0.25),
		Focus:           RGBA(37, 99, 235, 0.55),
		FocusRingWidth:  2,
		FocusRingOffset: 1,
		Scrollbar:       RGBA(0, 0, 0, 0.32),
		Radius:          6,
		Spacing:         4,
		FontSize:        defaultFontSize(),
	}
}

// DarkTheme returns the theme for a dark appearance.
func DarkTheme() *Theme {
	return &Theme{
		Dark:            true,
		Background:      Hex("#18181b"),
		Surface:         Hex("#27272a"),
		SurfaceHover:    Hex("#323236"),
		SurfacePressed:  Hex("#3c3c41"),
		Border:          Hex("#3f3f46"),
		Text:            Hex("#f4f4f5"),
		TextMuted:       Hex("#a1a1aa"),
		Accent:          Hex("#3b82f6"),
		AccentHover:     Hex("#60a5fa"),
		AccentPressed:   Hex("#2563eb"),
		AccentText:      Hex("#ffffff"),
		Danger:          Hex("#ef4444"),
		Warning:         Hex("#f59e0b"),
		Success:         Hex("#22c55e"),
		Selection:       RGBA(59, 130, 246, 0.4),
		Focus:           RGBA(96, 165, 250, 0.6),
		FocusRingWidth:  2,
		FocusRingOffset: 1,
		Scrollbar:       RGBA(255, 255, 255, 0.35),
		Radius:          6,
		Spacing:         4,
		FontSize:        defaultFontSize(),
	}
}
