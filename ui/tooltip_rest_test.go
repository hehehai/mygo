package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// restTester shows two buttons, Save and Open, with red tips over white,
// under a theme set by th (nil keeps the default one). It returns where
// the tip of Save shows, once it has.
type restTester struct {
	tt      *Tester
	tick    func(d time.Duration)
	theme   *Theme
	tipRect map[string]Rect
	scrollY float32
}

func newRestTester(t *testing.T, th func(*Theme)) *restTester {
	t.Helper()
	r := &restTester{tipRect: map[string]Rect{}}
	var now *time.Time
	r.tt, now = clockTester(func(c *context) {
		theme := *c.Theme()
		if th != nil {
			th(&theme)
		}
		r.theme = &theme
		coreThemed(c, &theme).Fill().Background(RGB(255, 255, 255)).Children(func() {
			coreColumn(c).Padding(80, 40).Gap(20).Children(func() {
				for _, name := range []string{"Save", "Open"} {
					b := coreButton(c, name).Label(name)
					tip := coreTooltipBase(c, b, func(tip *node) {
						tip.Width(60).Height(20).Background(RGB(255, 0, 0)).Children(func() { coreText(c, name+" tip") })
					})
					if tip != nil {
						r.tipRect[name] = tip.Bounds()
					}
				}
			})
		})
	}, 400, 300)
	r.tick = ticker(r.tt, now)
	return r
}

// restTheme is Refto's: tips after the pointer rests 0.7 s, fading in
// over 130 ms and out over 100.
func restTheme(t *Theme) {
	t.TooltipDelay = 700 * time.Millisecond
	t.TooltipRest = true
	t.TooltipFadeIn = 130 * time.Millisecond
	t.TooltipFadeOut = 100 * time.Millisecond
}

func (r *restTester) on(name string) {
	b, _ := r.tt.Find(name)
	r.tt.Move(b.X+b.W/2, b.Y+b.H/2)
}

func (r *restTester) shows(name string) bool { return r.tt.HasText(name + " tip") }

// opacity returns how opaque the tip of name is where it showed last, by
// its red over the white (green 255 none, 0 all).
func (r *restTester) opacity(t *testing.T, name string) float32 {
	t.Helper()
	rect, ok := r.tipRect[name]
	if !ok {
		t.Fatalf("the tip of %s never showed", name)
	}
	// A corner of it, clear of its text.
	g := r.tt.Image().RGBAAt(int(rect.X+rect.W-3), int(rect.Y+rect.H-3)).G
	return 1 - float32(g)/255
}

// TestTooltipDefaultUnchanged checks that a theme without TooltipRest
// and fades keeps the tips as they were: the delay counts from coming
// onto the element even as the pointer moves over it, and the tip shows
// whole in its first frame.
func TestTooltipDefaultUnchanged(t *testing.T) {
	r := newRestTester(t, nil)
	b, _ := r.tt.Find("Save")
	r.tt.Move(b.X+4, b.Y+b.H/2)
	r.tick(400 * time.Millisecond)
	r.tt.Move(b.X+b.W-4, b.Y+b.H/2) // moving over it still counts
	r.tick(250 * time.Millisecond)
	if !r.shows("Save") {
		t.Fatal("the default tip waits for the pointer to rest")
	}
	r.tt.Frame()
	if o := r.opacity(t, "Save"); o < 0.99 {
		t.Errorf("the default tip shows at opacity %.2f in its first frame", o)
	}
	// The next one at once, as before.
	r.on("Open")
	r.tick(16 * time.Millisecond)
	if !r.shows("Open") {
		t.Error("no warm-up by default")
	}
	// Scrolling over it keeps it, as before.
	o, _ := r.tt.Find("Open")
	r.tt.Scroll(o.X+o.W/2, o.Y+o.H/2, 0, 10)
	r.tick(16 * time.Millisecond)
	if !r.shows("Open") {
		t.Error("a scroll hides the default tip")
	}
	// Gone at once as the pointer leaves.
	r.tt.Move(390, 290)
	r.tick(16 * time.Millisecond)
	if r.shows("Open") || r.tt.FramePending() {
		t.Errorf("the default tip lingers: shows %v, frame pending %v", r.shows("Open"), r.tt.FramePending())
	}
}

// TestTooltipRest checks TooltipRest's delay: from when the pointer last
// moved over the element, the tip staying as it moves once shown.
func TestTooltipRest(t *testing.T) {
	r := newRestTester(t, restTheme)
	b, _ := r.tt.Find("Save")
	r.tt.Move(b.X+4, b.Y+b.H/2)
	r.tick(500 * time.Millisecond)
	r.tt.Move(b.X+b.W-4, b.Y+b.H/2)
	r.tick(500 * time.Millisecond)
	if r.shows("Save") {
		t.Fatal("the tip shows 0.5 s after the pointer last moved")
	}
	r.tick(250 * time.Millisecond)
	if !r.shows("Save") {
		t.Fatal("no tip 0.75 s after the pointer rested")
	}
	r.tt.Move(b.X+b.W/2, b.Y+b.H/2)
	r.tick(16 * time.Millisecond)
	if !r.shows("Save") {
		t.Error("moving over the element hides its shown tip")
	}
}

// TestTooltipFadeIn checks that a tip shown after its delay fades in,
// steadily, over TooltipFadeIn, and out over TooltipFadeOut, and that no
// frame is asked for once it is in or gone.
func TestTooltipFadeIn(t *testing.T) {
	r := newRestTester(t, restTheme)
	r.on("Save")
	r.tick(690 * time.Millisecond)
	if r.shows("Save") {
		t.Fatal("the tip shows within the delay")
	}
	r.tick(20 * time.Millisecond)
	if !r.shows("Save") {
		t.Fatal("no tip after the delay")
	}
	last := float32(-1)
	var seq []float32
	for range 10 {
		o := r.opacity(t, "Save")
		seq = append(seq, o)
		if o+0.01 < last {
			t.Fatalf("the fade goes back: %v", seq)
		}
		last = o
		r.tick(16 * time.Millisecond)
	}
	if seq[0] > 0.3 || seq[len(seq)-1] < 0.99 {
		t.Errorf("fade in %v: not from about 0 to 1", seq)
	}
	if seq[3] < 0.2 || seq[3] > 0.95 {
		t.Errorf("fade in %v: not midway at 48 ms", seq)
	}
	r.tick(time.Second)
	if r.tt.FramePending() {
		t.Error("a frame is asked for once the tip is in")
	}
	// Out.
	r.tt.Move(390, 290)
	r.tick(0)
	seq = seq[:0]
	for range 8 {
		seq = append(seq, r.opacity(t, "Save"))
		r.tick(16 * time.Millisecond)
	}
	if seq[0] < 0.8 || seq[2] > 0.95 || seq[len(seq)-1] > 0.01 {
		t.Errorf("fade out %v", seq)
	}
	r.tick(time.Second)
	if r.tt.FramePending() || r.shows("Save") {
		t.Error("the tip lingers once gone")
	}
}

// TestTooltipRestWarm checks the warm-up: after a tip, the next element's
// shows once the pointer rests a sixth of the delay, whole, without fading
// in, and the pointer sweeping over elements shows none.
func TestTooltipRestWarm(t *testing.T) {
	r := newRestTester(t, restTheme)
	r.on("Save")
	r.tick(time.Second)
	if !r.shows("Save") {
		t.Fatal("no tip")
	}
	r.on("Open")
	r.tick(50 * time.Millisecond)
	if r.shows("Open") {
		t.Error("in the warm-up, the next tip shows before the pointer rests")
	}
	r.tick(80 * time.Millisecond)
	if !r.shows("Open") {
		t.Fatal("in the warm-up, no tip after the pointer rested a sixth of the delay")
	}
	r.tt.Frame()
	if o := r.opacity(t, "Open"); o < 0.99 {
		t.Errorf("a tip in the warm-up fades in: opacity %.2f", o)
	}
	// Long after, the delay again.
	r.tt.Move(390, 290)
	r.tick(time.Second)
	r.on("Save")
	r.tick(300 * time.Millisecond)
	if r.shows("Save") {
		t.Error("the warm-up lasts after the tips went a second ago")
	}
}

// TestTooltipRestCools checks that a scroll, a press and a drag hide the
// tip and end the warm-up: the tip comes back only after the full delay.
func TestTooltipRestCools(t *testing.T) {
	cases := map[string]func(r *restTester, x, y float32){
		"scroll": func(r *restTester, x, y float32) { r.tt.Scroll(x, y, 0, 10) },
		"press": func(r *restTester, x, y float32) {
			r.tt.Press(x, y)
			r.tick(100 * time.Millisecond)
			r.tt.Release(x, y)
		},
		"files": func(r *restTester, x, y float32) {
			r.tt.DragFiles(x, y)
			r.tick(100 * time.Millisecond)
			r.tt.DropFiles(x, y)
		},
	}
	for name, cool := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRestTester(t, restTheme)
			r.on("Save")
			r.tick(time.Second)
			if !r.shows("Save") {
				t.Fatal("no tip")
			}
			b, _ := r.tt.Find("Save")
			cool(r, b.X+b.W/2, b.Y+b.H/2)
			r.tick(16 * time.Millisecond)
			if r.shows("Save") {
				t.Errorf("the tip shows after a %s", name)
			}
			r.tick(300 * time.Millisecond)
			if r.shows("Save") {
				t.Errorf("the tip comes back in the warm-up after a %s", name)
			}
			r.tick(500 * time.Millisecond)
			if name != "press" && !r.shows("Save") {
				// A press hides it until the pointer leaves, as by default.
				t.Errorf("no tip once the pointer rested after a %s", name)
			}
		})
	}
}

// TestTooltipRestFocus checks the keyboard focus's tip waiting for the
// delay, and staying hidden as the pointer leaves a hovered element.
func TestTooltipRestFocus(t *testing.T) {
	r := newRestTester(t, restTheme)
	r.tt.Key(0, KeyTab)
	r.tick(300 * time.Millisecond)
	if r.shows("Save") {
		t.Error("the focus's tip shows before the delay")
	}
	r.tick(500 * time.Millisecond)
	if !r.shows("Save") {
		t.Fatal("no focus tip after the delay")
	}
	// The pointer on Open, then off it: Save's tip does not pop up.
	r.on("Open")
	r.tick(time.Second)
	if !r.shows("Open") || r.shows("Save") {
		t.Fatalf("hovering Open: texts %q", r.tt.Texts())
	}
	r.tt.Move(390, 290)
	r.tick(16 * time.Millisecond)
	r.tick(time.Second)
	if r.shows("Save") {
		t.Error("the focus's tip pops up as the pointer leaves another element")
	}
	// The focus moving brings it back, after the delay.
	r.tt.Key(0, KeyTab)
	r.tick(16 * time.Millisecond)
	if r.shows("Open") {
		t.Error("the moved focus's tip shows at once")
	}
	r.tick(time.Second)
	if !r.shows("Open") {
		t.Error("no tip for the moved focus")
	}
}

// TestTooltipFadeReduceMotion checks that asking for less motion keeps
// the delay but shows and hides the tip at once.
func TestTooltipFadeReduceMotion(t *testing.T) {
	r := newRestTester(t, restTheme)
	r.tt.SetPreferences(Preferences{ReduceMotion: true})
	r.on("Save")
	r.tick(600 * time.Millisecond)
	if r.shows("Save") {
		t.Fatal("less motion drops the delay")
	}
	r.tick(150 * time.Millisecond)
	if !r.shows("Save") {
		t.Fatal("no tip")
	}
	r.tt.Frame()
	if o := r.opacity(t, "Save"); o < 0.99 {
		t.Errorf("with less motion the tip fades in: %.2f", o)
	}
	r.tt.Move(390, 290)
	r.tick(16 * time.Millisecond)
	if o := r.opacity(t, "Save"); o > 0.01 || r.tt.FramePending() {
		t.Errorf("with less motion the tip fades out: %.2f, frame pending %v", o, r.tt.FramePending())
	}
}

// TestTooltipFrames writes the frames of a tip coming and going, with the
// default theme and with restTheme, to $MYGO_TOOLTIP_FRAMES, to compare.
func TestTooltipFrames(t *testing.T) {
	dir := os.Getenv("MYGO_TOOLTIP_FRAMES")
	if dir == "" {
		t.Skip("MYGO_TOOLTIP_FRAMES not set")
	}
	for name, th := range map[string]func(*Theme){"before": nil, "after": restTheme} {
		r := newRestTester(t, th)
		r.on("Save")
		ms := 0
		save := func(step int) {
			img := r.tt.Image().SubImage(image.Rect(0, 30, 400, 110))
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%04dms.png", name, ms)))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
			f.Close()
			r.tick(time.Duration(step) * time.Millisecond)
			ms += step
		}
		for ms < 560 {
			save(80)
		}
		for ms < 1000 {
			save(16)
		}
		r.tt.Move(390, 290)
		r.tick(0)
		for ms < 1200 {
			save(16)
		}
	}
}
