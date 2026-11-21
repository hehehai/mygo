//go:build !js

package mygo

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestVibrancyStyle passes a material's shape to the backend whole: a HUD
// inset from the window's edges, and a notch's shelf at a frame of its
// content with its bottom corners rounded.
func TestVibrancyStyle(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Transparent: true, Vibrancy: VibrancyHUD})
	got := func() platform.VibrancyStyle {
		return onMainValue(func() platform.VibrancyStyle { return fw.VibrancyStyle })
	}
	w.SetVibrancyStyle(VibrancyStyle{Top: 18, Right: 18, Bottom: 18, Left: 18, Radius: 24, Dark: true})
	if s := got(); s != (platform.VibrancyStyle{Top: 18, Right: 18, Bottom: 18, Left: 18, Radius: 24, Dark: true}) {
		t.Errorf("HUD: %+v", s)
	}
	w.SetVibrancyStyle(VibrancyStyle{Frame: Rectangle{X: 10, Y: 0, Width: 200, Height: 40}, Radius: 12, Corners: VibrancyBottomLeft | VibrancyBottomRight, Fade: 0.25})
	want := platform.VibrancyStyle{Frame: platform.Rect{X: 10, Width: 200, Height: 40}, Radius: 12, Corners: 4 | 8, Fade: 0.25}
	if s := got(); s != want {
		t.Errorf("shelf: %+v, want %+v", s, want)
	}
	w.SetVibrancyStyle(VibrancyStyle{Hidden: true})
	if s := got(); !s.Hidden {
		t.Errorf("hidden: %+v", s)
	}
}
