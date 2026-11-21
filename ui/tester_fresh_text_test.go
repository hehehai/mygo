package ui

import (
	"bytes"
	"fmt"
	"testing"
)

// TestTesterDrawsAsFresh: what a Tester draws does not depend on what
// Testers before it drew, as each has a text system of its own (#1730):
// glyphs drawn scaled, which sample their atlas between pixels, draw the
// same after another Tester filled its atlases many times over. AudioSmith
// emptied the shared atlases between snapshot tests for this
// (ui.ForgetGlyphs), which Testers no longer need.
func TestTesterDrawsAsFresh(t *testing.T) {
	view := func(c *context) {
		c.SetZoom(0.73)
		coreColumn(c).Padding(7).Children(func() {
			coreText(c, "Waveform 波形 · 0:42 / 3:17").FontSize(13)
			coreText(c, "Studio script — 第二段").FontSize(17)
		})
	}
	draw := func() []byte {
		tt := coreNewTester(view, 240, 80)
		tt.Frame()
		tt.Frame()
		return bytes.Clone(tt.Image().Pix)
	}
	want := draw()
	busy := coreNewTester(func(c *context) {
		coreColumn(c).Children(func() {
			for i := range 60 {
				coreText(c, fmt.Sprintf("%d 字形 glyphs %c%c%c", i, 'A'+rune(i%26), 0x4e00+rune(i*37), 0x3040+rune(i%80))).FontSize(float32(9 + i%23))
			}
		})
	}, 800, 1400)
	for range 3 {
		busy.Frame()
	}
	if got := draw(); !bytes.Equal(got, want) {
		t.Fatal("a Tester drew otherwise after another filled its atlases")
	}
}
