package ui

import "testing"

// TestThemeRadii: DialogRadius, PopupRadius, ToastRadius and
// TooltipRadius round what floats over a window; 0 keeps the widgets'
// own corners.
func TestThemeRadii(t *testing.T) {
	def := LightTheme()
	if got := [5]float32{def.dialogRadius(), def.popupRadius(), def.toastRadius(), def.tooltipRadius(), def.popupOptionRadius()}; got != [5]float32{10, 8, 8, 5, 6} {
		t.Fatalf("the defaults: %v", got)
	}
	th := LightTheme()
	th.DialogRadius, th.PopupRadius, th.ToastRadius, th.TooltipRadius = 24, 16, 12, 8
	if got := [5]float32{th.dialogRadius(), th.popupRadius(), th.toastRadius(), th.tooltipRadius(), th.popupOptionRadius()}; got != [5]float32{24, 16, 12, 8, 12} {
		t.Fatalf("set: %v", got)
	}
	small := *th
	small.PopupRadius = 6
	if got := small.popupOptionRadius(); got != 4 {
		t.Fatalf("an option in a panel of 6: %v, want 4 at least", got)
	}

	// white says whether the pixel at (x, y) is the panels' white, not
	// the red page under them (dimmed, shaded).
	for _, s := range []struct {
		name  string
		theme *Theme
		round bool // the corner's pixel off the panel
	}{{"default", LightTheme(), false}, {"set", th, true}} {
		alert, pop := true, true
		var tt *Tester
		view := func(dialog bool) {
			tt = NewTester(func(c *Context) {
				c.SetTheme(s.theme)
				Column(c).Fill().Background(Hex("#ff0000")).Padding(40).AlignItems(Start).Children(func() {
					b := Button(c.Key("anchor"), "Anchor")
					if !dialog {
						Popover(c.Key("pop"), b, &pop, func() { Box(c).Size(120, 40).Label("inside") })
					}
				})
				if dialog {
					AlertDialog(c.Key("alert"), &alert, "Delete it?", "Gone for good.", "Cancel", "Delete")
				}
			}, 600, 400)
			tt.Frame()
		}
		white := func(x, y float32) bool {
			p := tt.Image().RGBAAt(int(x), int(y))
			return p.R == 255 && p.G == 255 && p.B == 255
		}
		bounds := func(name string) Rect {
			for _, n := range tt.AccessAll() {
				if n.Name == name {
					return n.Bounds
				}
			}
			t.Fatalf("%s: no %q", s.name, name)
			return Rect{}
		}
		// A corner's pixel 5 DIPs in from both edges: inside 10's arc,
		// outside 24's.
		view(true)
		a := bounds("Delete it?")
		if !white(a.X+a.W/2, a.Y+a.H/2) {
			t.Fatalf("%s: the alert's middle is not white", s.name)
		}
		if got := !white(a.X+5, a.Y+5); got != s.round {
			t.Errorf("%s: the alert's corner off it %v, want %v", s.name, got, s.round)
		}
		// The popover's panel: 4 DIPs of padding and its border around
		// what it holds; 3 in, inside 8's arc, outside 16's.
		view(false)
		in := bounds("inside")
		px, py := in.X-5, in.Y-5
		if !white(in.X+2, in.Y+2) {
			t.Fatalf("%s: the popover is not white", s.name)
		}
		if got := !white(px+3, py+3); got != s.round {
			t.Errorf("%s: the popover's corner off it %v, want %v", s.name, got, s.round)
		}
	}
}
