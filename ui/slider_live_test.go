package ui

import (
	"testing"
	"time"
)

// TestSliderLiveValue: a slider given LiveValue paints the value as it
// paints, frame by frame without building the view, while fn asks for
// frames; pressed, it paints its bound value; without, as before.
func TestSliderLiveValue(t *testing.T) {
	v := 10.0
	live := true
	var start time.Time
	asks := 0
	tt := newRepaintTester(func(c *context) {
		s := coreSlider(c, &v, 0, 100).Width(180).Label("Position")
		if live {
			s.LiveValue(func(p *Painter, r Rect) float64 {
				asks++
				p.AnimationFrame()
				return float64(p.Now().Sub(start).Milliseconds()) / 10 // 100 a second
			})
		}
	})
	start = tt.clock
	// fillEnd is where the accent's fill of the track ends.
	fillEnd := func() float32 {
		fills := fillsOf(tt.Tester, tt.rt.c.theme.Accent)
		if len(fills) == 0 {
			return -1
		}
		return fills[0].X + fills[0].W
	}
	track := at(t, tt.Tester, "Position")
	ends := []float32{}
	for range 4 {
		n := asks
		built, more := tt.frame(100 * time.Millisecond) // 10 more each
		if built || !more || asks != n+1 {
			t.Fatalf("a frame: built %v, asked for another %v, value asked %d times", built, more, asks-n)
		}
		ends = append(ends, fillEnd())
	}
	scale := tt.rt.painter.scale
	kw := tt.rt.c.theme.Space(5) * scale
	step := (float32(track.W)*scale - kw) / 10 // 10 of 100 a frame
	for i := 1; i < len(ends); i++ {
		if d := ends[i] - ends[i-1]; d < step-1 || d > step+1 {
			t.Errorf("frame %d: the fill moved %v pixels, want %v (%v)", i, d, step, ends)
		}
	}
	// Pressed, the bound value shows.
	live = true
	tt.Press(track.X+track.W/2, track.Y+track.H/2)
	tt.Frame()
	if want := float32(track.X)*scale + kw/2 + (float32(track.W)*scale-kw)*float32(v/100); v < 40 || v > 60 || fillEnd() < want-2 || fillEnd() > want+2 {
		t.Errorf("pressed at the middle: value %v, the fill ends at %v, want %v", v, fillEnd(), want)
	}
	tt.Release(track.X+track.W/2, track.Y+track.H/2)
	// Without LiveValue, a frame asks for nothing.
	live = false
	tt.Frame()
	for range 5 {
		tt.frame(100 * time.Millisecond) // the knob let go of
	}
	n := asks
	if _, more := tt.frame(100 * time.Millisecond); more || asks != n {
		t.Errorf("without LiveValue: asked for another frame %v, value asked %d times", more, asks-n)
	}
}
