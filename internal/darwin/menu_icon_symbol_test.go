//go:build darwin && !ios

package darwin

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestMenuIconSymbolAndScale: an icon of an SF Symbol is the system's
// symbol image, a template; an unknown name gives none. An icon with a
// Scale is its pixels at that scale, not fit to the 16 point box.
func TestMenuIconSymbolAndScale(t *testing.T) {
	load()
	LinkMenuIcons()
	defer func() { menuImage = nil }()
	var b bytes.Buffer
	png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 160, 28)))
	withPool(func() {
		for _, c := range []struct {
			name string
			want bool
		}{{"doc.on.doc", true}, {"trash", true}, {"no.such.symbol.mygo", false}} {
			img := iconImage(&platform.MenuIcon{Symbol: c.name, Template: true})
			if got := img != 0; got != c.want {
				t.Errorf("%q: image %v, want %v", c.name, got, c.want)
			}
			if img != 0 && !sendBool(img, "isTemplate") {
				t.Errorf("%q: not a template", c.name)
			}
		}
		img := iconImage(&platform.MenuIcon{PNG: b.Bytes(), Scale: 2, Template: true})
		if img == 0 {
			t.Fatal("no image of the scaled icon")
		}
		if sz := msgSize(img, sel("size")); sz.Width != 80 || sz.Height != 14 {
			t.Errorf("scaled icon %v, want 80×14 points", sz)
		}
		fit := iconImage(&platform.MenuIcon{PNG: b.Bytes()})
		if sz := msgSize(fit, sel("size")); sz.Width != 16 {
			t.Errorf("fitted icon %v, want 16 points wide", sz)
		}
		item := autorelease(send(send(class("NSMenuItem"), "alloc"), "initWithTitle:action:keyEquivalent:",
			uintptr(nsString("Copy")), 0, uintptr(nsString(""))))
		applyItemState(item, &platform.MenuItem{Label: "Copy", Enabled: true, Visible: true, Icon: &platform.MenuIcon{Symbol: "doc.on.doc", Template: true}})
		if send(item, "image") == 0 {
			t.Error("the item shows no symbol")
		}
	})
}
