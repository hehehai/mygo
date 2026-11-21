//go:build darwin && !ios

package darwin

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestVibrancyStyle places a material view as styles ask, in a content
// view 300 by 200 (not flipped; no window): inset from the edges and
// following them, at a frame from the top left kept there, hidden, faded,
// and back to the whole content for the zero style.
func TestVibrancyStyle(t *testing.T) {
	load()
	withPool(func() {
		view := msgInitRect(send(class("NSView"), "alloc"), sel("initWithFrame:"), NSRect{Size: NSSize{Width: 300, Height: 200}})
		defer release(view)
		effect := msgInitRect(send(class("NSVisualEffectView"), "alloc"), sel("initWithFrame:"), NSRect{Size: NSSize{Width: 300, Height: 200}})
		defer release(effect)
		send(view, "addSubview:", uintptr(effect))
		w := &window{view: view, effect: effect}
		frame := func() NSRect { return msgRect(effect, sel("frame")) }
		alpha := func() float64 { return msgFloat(effect, sel("alphaValue")) }

		w.vstyle = platform.VibrancyStyle{Top: 10, Right: 20, Bottom: 30, Left: 40, Radius: 8, Dark: true}
		w.styleEffect()
		if f := frame(); f != (NSRect{Origin: NSPoint{X: 40, Y: 30}, Size: NSSize{Width: 240, Height: 160}}) {
			t.Errorf("inset: frame %+v", f)
		}
		if send(effect, "maskImage") == 0 || send(effect, "appearance") == 0 || w.framed {
			t.Errorf("inset: mask %v, appearance %v, framed %v", send(effect, "maskImage"), send(effect, "appearance"), w.framed)
		}
		if m := sendInt(effect, "autoresizingMask"); m != nsViewWidthHeightSizable {
			t.Errorf("inset: autoresizing %d", m)
		}

		w.vstyle = platform.VibrancyStyle{Frame: platform.Rect{X: 50, Y: 0, Width: 100, Height: 40}, Radius: 12, Corners: cornerBottomLeft | cornerBottomRight, Fade: 0.5}
		w.styleEffect()
		if f := frame(); f != (NSRect{Origin: NSPoint{X: 50, Y: 160}, Size: NSSize{Width: 100, Height: 40}}) {
			t.Errorf("frame: frame %+v", f)
		}
		if !w.framed || alpha() != 0.5 || send(effect, "appearance") != 0 || sendBool(effect, "isHidden") {
			t.Errorf("frame: framed %v, alpha %v, appearance %v", w.framed, alpha(), send(effect, "appearance"))
		}
		if m := sendInt(effect, "autoresizingMask"); m != nsViewMinYMargin {
			t.Errorf("frame: autoresizing %d", m)
		}

		w.vstyle = platform.VibrancyStyle{Frame: platform.Rect{X: 50, Width: 100, Height: 40}, Hidden: true}
		w.styleEffect()
		if !sendBool(effect, "isHidden") || w.framed {
			t.Errorf("hidden: hidden %v, framed %v", sendBool(effect, "isHidden"), w.framed)
		}

		w.vstyle = platform.VibrancyStyle{}
		w.styleEffect()
		if f := frame(); f != (NSRect{Size: NSSize{Width: 300, Height: 200}}) || sendBool(effect, "isHidden") || alpha() != 1 || send(effect, "maskImage") != 0 {
			t.Errorf("zero: frame %+v, hidden %v, alpha %v, mask %v", f, sendBool(effect, "isHidden"), alpha(), send(effect, "maskImage"))
		}
	})
}

// TestVibrancyMask makes a stretchable mask of the corners' size, and
// one for some corners only.
func TestVibrancyMask(t *testing.T) {
	load()
	withPool(func() {
		for _, corners := range []uint8{0, cornerBottomLeft | cornerBottomRight} {
			img := vibrancyMask(12, corners)
			if s := msgSize(img, sel("size")); s != (NSSize{Width: 25, Height: 25}) {
				t.Errorf("corners %b: size %+v", corners, s)
			}
			if m := sendInt(img, "resizingMode"); m != 1 {
				t.Errorf("corners %b: resizing mode %d", corners, m)
			}
		}
	})
}
