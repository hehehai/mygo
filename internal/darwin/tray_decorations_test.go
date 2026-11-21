//go:build darwin && !ios

package darwin

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// nopTrayHandler is a tray's handler that hears nothing.
type nopTrayHandler struct{}

func (nopTrayHandler) Clicked()           {}
func (nopTrayHandler) RightClicked()      {}
func (nopTrayHandler) AppearanceChanged() {}

// TestTrayDecorations: the overlay covers the button's image exactly,
// pixel for pixel as the button draws it, with no title (the image
// centred) and with titles of every width (the image on the left of an
// unread count), as the title comes, grows and goes; the dot keeps to the
// image's top right corner the same way.
func TestTrayDecorations(t *testing.T) {
	load()
	onMainThread(func() { withPool(func() { trayDecorations(t) }) })
}

func trayDecorations(t *testing.T) {
	if objc.GetClass("MyGoTrayTarget") == 0 {
		registerTrayTarget()
	}
	send(class("NSApplication"), "sharedApplication")
	saved := theBackend
	b := &Backend{trays: map[id]*tray{}}
	theBackend = b
	defer func() { theBackend = saved }()
	pt, err := b.NewTray("", nopTrayHandler{})
	if err != nil {
		t.Error(err)
		return
	}
	tr := pt.(*tray)
	defer tr.Destroy()
	if tr.Handle() != uintptr(tr.button) || tr.Handle() == 0 {
		t.Errorf("Handle %#x, the button %#x", tr.Handle(), tr.button)
	}
	blue, red := color.NRGBA{0, 0, 255, 255}, color.NRGBA{255, 0, 0, 255}
	if err := tr.SetImage(solidPNG(18, 18, blue), false); err != nil {
		t.Error(err)
		return
	}
	if err := tr.SetOverlay(solidPNG(18, 18, red)); err != nil {
		t.Error(err)
		return
	}
	if err := tr.SetOverlay([]byte("not a png")); err == nil {
		t.Error("an overlay of no image: no error")
	}
	tr.SetDot(platform.Color{R: 0, G: 255, B: 0, A: 255}, true)
	for _, title := range []string{"", "3", "21", "99+", "9", "", "1000"} {
		tr.SetTitle(title)
		send(tr.overlay, "setHidden:", 1)
		send(tr.dot, "setHidden:", 1)
		hidden, err := renderView(tr.button)
		send(tr.overlay, "setHidden:", 0)
		shown, err2 := renderView(tr.button)
		send(tr.dot, "setHidden:", 0)
		if err := errors.Join(err, err2); err != nil {
			t.Error(err)
			return
		}
		icon, over := bbox(hidden, blue), bbox(shown, red)
		if icon.Empty() || over != icon {
			t.Errorf("title %q: overlay drawn at %v, the icon at %v (button %v, overlay frame %v)",
				title, over, icon, msgRect(tr.button, sel("bounds")), msgRect(tr.overlay, sel("frame")))
		}
		if left := bbox(shown, blue); !left.Empty() {
			t.Errorf("title %q: icon left uncovered at %v", title, left)
		}
		_, r, _ := trayImageRect(tr.dot)
		f := msgRect(tr.dot, sel("frame"))
		cx, cy := f.Origin.X+f.Size.Width/2, f.Origin.Y+f.Size.Height/2
		top := r.Origin.Y + r.Size.Height - trayDotInset
		if sendBool(tr.button, "isFlipped") {
			top = r.Origin.Y + trayDotInset
		}
		if f.Size.Width != trayDotSize || math.Abs(cx-(r.Origin.X+r.Size.Width-trayDotInset)) > 0.01 || math.Abs(cy-top) > 0.01 {
			t.Errorf("title %q: dot at %v, the image at %v", title, f, r)
		}
	}
	tr.SetDot(platform.Color{}, false)
	if err := tr.SetOverlay(nil); err != nil || tr.overlay != 0 || tr.dot != 0 {
		t.Errorf("removed: overlay %#x, dot %#x, %v", tr.overlay, tr.dot, err)
	}
}

// solidPNG is a w×h PNG of colour c.
func solidPNG(w, h int, c color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

// renderView draws view and its subviews offscreen.
func renderView(view id) (image.Image, error) {
	bounds := msgRect(view, sel("bounds"))
	rep := msgInitRect(view, sel("bitmapImageRepForCachingDisplayInRect:"), bounds)
	msgInitRectID(view, sel("cacheDisplayInRect:toBitmapImageRep:"), bounds, rep)
	data := goBytes(send(rep, "representationUsingType:properties:", 4, uintptr(send(class("NSDictionary"), "dictionary"))))
	return png.Decode(bytes.NewReader(data))
}

// bbox is the box of img's opaque pixels of c's dominant channel (the
// display's colour space shifts the colours a little: sRGB red is drawn
// as 234, 51, 35).
func bbox(img image.Image, c color.NRGBA) image.Rectangle {
	var r image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if p.A > 200 && near(p.R, c.R) && near(p.G, c.G) && near(p.B, c.B) {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

// near: both strong (over 150) or both weak (under 100).
func near(a, b uint8) bool { return a > 150 && b > 150 || a < 100 && b < 100 }
