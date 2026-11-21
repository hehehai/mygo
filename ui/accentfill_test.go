package ui

import "testing"

// TestAccentFill: SetAccentFill fills what carries AccentText — a
// Select's highlighted option, a list's chosen row, a primary button, a
// checked box — while Accent stays Accent; a copy given another Accent
// fills with that.
func TestAccentFill(t *testing.T) {
	th := LightTheme()
	th.Accent = Hex("#0d9384")
	fill := Hex("#0c8477")
	th.SetAccentFill(fill)
	if got := th.AccentFill(); got != fill {
		t.Fatalf("AccentFill %v, want %v", got, fill)
	}
	other := *th
	other.Accent = Hex("#ff0000")
	if got := other.AccentFill(); got != other.Accent {
		t.Fatalf("a copy with another Accent fills with %v, want its Accent", got)
	}
	if got := LightTheme().AccentFill(); got != LightTheme().Accent {
		t.Fatalf("without SetAccentFill: %v", got)
	}

	sel, on, row := "One", true, 0
	rows := ListState{Selected: &row}
	tt := NewTester(func(c *Context) {
		c.SetTheme(th)
		Column(c).Padding(10).Gap(10).AlignItems(Start).Children(func() {
			Select(c.Key("sel"), &sel, []string{"One", "Two"}).Width(120)
			Checkbox(c.Key("check"), &on, "Box")
			PrimaryButton(c.Key("go"), "Go")
			List(c.Key("list"), &rows, 2, func(i int) { Text(c, []string{"Row A", "Row B"}[i]) }).Height(60).Width(120)
		})
	}, 400, 400)
	tt.Frame()
	px := func(r Rect, dx float32) Color {
		p := tt.Image().RGBAAt(int(r.X+dx), int(r.Y+r.H/2))
		return Color{R: p.R, G: p.G, B: p.B, A: 255}
	}
	for _, s := range []struct {
		text string
		dx   float32
	}{{"Go", -4}, {"Row A", 60}} { // the row's room past its text
		r, ok := tt.Find(s.text)
		if !ok {
			t.Fatalf("no %q", s.text)
		}
		if got := px(r, s.dx); got != fill {
			t.Errorf("%q on %v, want the fill %v", s.text, got, fill)
		}
	}
	if err := tt.Click("One"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, KeyDown)
	tt.Frame()
	var two Rect
	for _, n := range tt.AccessAll() {
		if n.Name == "Two" {
			two = n.Bounds
		}
	}
	if two.W == 0 {
		t.Fatal("no option Two")
	}
	if got := px(two, 2); got != fill {
		t.Errorf("the highlighted option on %v, want the fill %v", got, fill)
	}
}
