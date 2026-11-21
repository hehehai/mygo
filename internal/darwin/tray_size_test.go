//go:build darwin && !ios

package darwin

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"testing"
)

// pngAt encodes a w×h image with a pHYs chunk of dpi (none when 0).
func pngAt(t *testing.T, w, h int, dpi float64) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	if dpi == 0 {
		return data
	}
	// pHYs goes after IHDR (8 bytes of signature, then 25 of IHDR).
	ppm := uint32(dpi/0.0254 + 0.5)
	body := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32([]byte("pHYs"), ppm), ppm)
	body = append(body, 1) // unit: metre
	chunk := binary.BigEndian.AppendUint32(nil, 9)
	chunk = append(chunk, body...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(body))
	return append(append(append([]byte{}, data[:33]...), chunk...), data[33:]...)
}

// TestDeclaredTraySize: a tray image whose PNG declares 2× or 3× and fits
// the menu bar keeps its size in points; others are scaled to 18 points.
func TestDeclaredTraySize(t *testing.T) {
	load()
	for _, c := range []struct {
		w, h int
		dpi  float64
		ok   bool
		pw   float64
		ph   float64
	}{
		{38, 38, 144, true, 19, 19},
		{60, 57, 216, true, 20, 19},
		{44, 44, 144, true, 22, 22},
		{48, 48, 144, false, 0, 0}, // 24 pt: taller than the menu bar
		{36, 36, 0, false, 0, 0},   // no resolution
		{36, 36, 72, false, 0, 0},
	} {
		withPool(func() {
			img := autorelease(send(send(class("NSImage"), "alloc"), "initWithData:", uintptr(nsData(pngAt(t, c.w, c.h, c.dpi)))))
			if img == 0 {
				t.Fatalf("%dx%d at %v dpi: no image", c.w, c.h, c.dpi)
			}
			w, h, ok := declaredTraySize(img, msgSize(img, sel("size")))
			if ok != c.ok || w != c.pw || h != c.ph {
				t.Errorf("%dx%d at %v dpi: %v×%v %v, want %v×%v %v", c.w, c.h, c.dpi, w, h, ok, c.pw, c.ph, c.ok)
			}
		})
	}
}
