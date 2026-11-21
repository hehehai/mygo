package ui

import (
	"testing"
	"time"
)

// TestThemeTitleWeight: the widgets' titles take the theme's TitleWeight,
// their own weights when it is 0.
func TestThemeTitleWeight(t *testing.T) {
	weights := func(th *Theme) map[string]int {
		open, tab, side := true, 0, "a"
		day := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
		tt := NewTester(func(c *Context) {
			c.SetTheme(th)
			Column(c).Children(func() {
				Sidebar(c, &side, func() {
					SidebarSection(c, "Section", nil, func() { SidebarItem(c, "a", nil, "Item") })
				})
				Tabs(c, &tab, "Tab", "Other")
				Fieldset(c, "Legend", func() { Text(c, "Field") })
				Calendar(c, &day)
			})
			AlertDialog(c, &open, "Alert", "Message", "OK")
		}, 400, 600)
		tt.Frame()
		got := map[string]int{}
		var walk func(n *node)
		walk = func(n *node) {
			for ; n != nil; n = n.next {
				if n.text != "" && n.ts.set&setWeight != 0 {
					got[n.text] = n.ts.weight
				}
				walk(n.first)
			}
		}
		walk(tt.rt.c.root)
		return got
	}
	for _, s := range []struct {
		weight int
		want   map[string]int
	}{
		{0, map[string]int{"Section": 600, "Legend": 600, "Alert": 700, "November 2026": 700}},
		{400, map[string]int{"Section": 400, "Legend": 400, "Alert": 400, "November 2026": 400}},
	} {
		th := LightTheme()
		th.TitleWeight = s.weight
		got := weights(th)
		for text, w := range s.want {
			if got[text] != w {
				t.Errorf("TitleWeight %d: %q weighs %d, want %d (%v)", s.weight, text, got[text], w, got)
			}
		}
		// The chosen tab: its row carries the weight.
		if tab := got["Tab"]; tab != 0 {
			t.Errorf("TitleWeight %d: the tab's text has a weight of its own: %d", s.weight, tab)
		}
	}
}
