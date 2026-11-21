package ui

import (
	"math"
	"testing"
)

// TestAlignFirstLines: an icon button after a label sits on the middle of
// the label's first line, whether the line is lower or higher than the
// button, and stays there when the label wraps.
func TestAlignFirstLines(t *testing.T) {
	for _, tc := range []struct {
		name   string
		text   string
		line   float32 // LineHeight, 0 the font's
		width  float32
		button float32
	}{
		{"short line", "Name", 0, 300, 30},
		{"tall line", "Name", 3, 300, 18},
		{"wrapped", "A name long enough to wrap over two lines or more", 0, 120, 18},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var label, button *node
			tt := coreNewTester(func(c *context) {
				coreColumn(c).Padding(10).Children(func() {
					coreRow(c).Width(tc.width).AlignItems(Start).AlignFirstLines().Children(func() {
						label = coreText(c, tc.text).Shrink(1).MinWidth(0)
						if tc.line > 0 {
							label.LineHeight(tc.line)
						}
						button = coreButtonBase(c).Size(tc.button, tc.button).Label("i").Shrink(0)
					})
				})
			}, 400, 300)
			tt.Frame()
			tt.Frame()
			l := firstLine(label)
			if l == nil {
				t.Fatal("no line")
			}
			if tc.name == "wrapped" && len(label.tl.Lines) < 2 {
				t.Fatalf("the label did not wrap: %d lines", len(label.tl.Lines))
			}
			lb, bb := label.Bounds(), button.Bounds()
			lineMid := lb.Y + label.contentY() + l.Baseline - (l.Ascent-l.Descent)/2
			if mid := bb.Y + bb.H/2; math.Abs(float64(mid-lineMid)) > 0.01 {
				t.Errorf("button's middle %v, the first line's %v (label %v, button %v)", mid, lineMid, lb, bb)
			}
			if lb.Y < 10 || bb.Y < 10 {
				t.Errorf("above the row: label %v, button %v", lb, bb)
			}
		})
	}
}

// TestAlignFirstLinesColumns: a row lining up first lines puts a label on
// the first line of the column beside it — the text of a check box; a
// switch's middle, not the description below the switch — and a row that
// does not leaves its children at the top.
func TestAlignFirstLinesColumns(t *testing.T) {
	on := false
	var sw *node
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(20).Children(func() {
			coreRow(c).Gap(10).AlignItems(Start).AlignFirstLines().Children(func() {
				coreText(c, "Notify:")
				coreColumn(c).Gap(6).Children(func() {
					coreCheckbox(c, &on, "Show alerts")
					coreText(c, "Below the check box.")
				})
			})
			coreRow(c).Gap(10).AlignItems(Start).AlignFirstLines().Children(func() {
				coreText(c, "Offline:")
				coreColumn(c).Gap(4).Children(func() {
					sw = coreSwitch(c, &on)
					coreText(c, "Your groups see you as offline.").FontSize(11)
				})
			})
			coreRow(c).Gap(10).AlignItems(Start).Children(func() {
				coreText(c, "Plain:")
				coreColumn(c).Children(func() {
					coreBox(c).Size(40, 40)
					coreText(c, "Under a box.")
				})
			})
		})
	}, 400, 400)
	find := func(s string) Rect {
		t.Helper()
		r, ok := tt.Find(s)
		if !ok {
			t.Fatalf("no %q: %q", s, tt.Texts())
		}
		return r
	}
	if l, c := find("Notify:"), find("Show alerts"); abs32(l.Y-c.Y) > 1 || abs32(l.H-c.H) > 1 {
		t.Errorf("label %v, check box text %v: not on one baseline", l, c)
	}
	l, s := find("Offline:"), sw.Bounds()
	if d := (l.Y + l.H/2) - (s.Y + s.H/2); abs32(d) > 2 {
		t.Errorf("label %v centred %.1f off the switch %v", l, d, s)
	}
	if note := find("Your groups see you as offline."); note.Y < s.Y+s.H {
		t.Errorf("the description %v is not below the switch %v", note, s)
	}
	if p, u := find("Plain:"), find("Under a box."); p.Y > 1 && p.Y >= u.Y {
		t.Errorf("a row without AlignFirstLines moved its label: %v (the text below the box %v)", p, u)
	}
}
