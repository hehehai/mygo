package windows

import (
	"math"

	"github.com/egoist/mygo/internal/platform"
)

// effectiveWebViewBackground returns the packed COREWEBVIEW2_COLOR
// (A, R, G, B bytes from low to high) for WebView2's DefaultBackgroundColor.
// Embedded web views with an active underlay compose transparently (0) so
// transparent parts of the page reveal the underlay visual beneath it; all
// other web views keep the existing vibrancy, transparent, and background
// color behavior.
func effectiveWebViewBackground(bg *platform.Color, embedded, hasUnderlay, transparent, noRedirect bool, vibrancy string) uint32 {
	if embedded && hasUnderlay {
		return 0
	}
	switch {
	case vibrancy != "":
		return 0
	case transparent && (bg == nil || !noRedirect):
		return 0
	case bg != nil && bg.A == 0:
		return 0
	case bg != nil:
		return 0xFF | uint32(bg.R)<<8 | uint32(bg.G)<<16 | uint32(bg.B)<<24
	default:
		return 0xFFFFFFFF // opaque white: A, R, G, B bytes
	}
}

// rgbaToPremulBGRA converts top-down RGBA pixels (stride bytes per row) into
// tightly packed premultiplied BGRA uint32 pixels (0xAARRGGBB on little-endian
// Windows, matching DXGI_FORMAT_B8G8R8A8_UNORM + DXGI_ALPHA_MODE_PREMULTIPLIED).
//
// Go's image.RGBA already stores premultiplied channels (R, G, B <= A), which
// are converted with an exact R/B swap. If any pixel has a channel exceeding
// its alpha (as straight-alpha RGBA input can), its channels are premultiplied
// by alpha so DirectComposition's premultiplied-alpha invariant always holds.
func rgbaToPremulBGRA(dst []uint32, pix []byte, stride, width, height int) ([]uint32, bool) {
	if pix == nil || width <= 0 || height <= 0 || stride < width*4 || len(pix)/stride < height {
		return nil, false
	}
	n := width * height
	if cap(dst) < n {
		dst = make([]uint32, n)
	} else {
		dst = dst[:n]
	}
	for y := range height {
		row := pix[y*stride : y*stride+width*4]
		out := dst[y*width : (y+1)*width]
		for x := range width {
			i := x * 4
			r, g, b, a := row[i], row[i+1], row[i+2], row[i+3]
			if r > a || g > a || b > a {
				aa := uint32(a)
				r = uint8((uint32(r)*aa + 127) / 255)
				g = uint8((uint32(g)*aa + 127) / 255)
				b = uint8((uint32(b)*aa + 127) / 255)
			}
			out[x] = uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
		}
	}
	return dst, true
}

// underlayTransform returns the D2D_MATRIX_3X2_F ([sx, 0, 0, sy, tx, ty]) that
// stretches an underW×underH DirectComposition surface across underRect inside
// clipVis, whose origin is placed at (round(clip.X*dpiScale), round(clip.Y*dpiScale))
// in surface pixels. ok is false when any dimension or scale is non-positive
// or non-finite.
func underlayTransform(underRect, clip platform.RectF, underW, underH int, dpiScale float64) (m [6]float32, ok bool) {
	if underW <= 0 || underH <= 0 || underRect.W <= 0 || underRect.H <= 0 || clip.W <= 0 || clip.H <= 0 || dpiScale <= 0 {
		return m, false
	}
	for _, v := range [...]float64{underRect.X, underRect.Y, underRect.W, underRect.H, clip.X, clip.Y, clip.W, clip.H, dpiScale} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return m, false
		}
	}
	cx := float32(math.Round(clip.X * dpiScale))
	cy := float32(math.Round(clip.Y * dpiScale))
	tx := float32(underRect.X*dpiScale) - cx
	ty := float32(underRect.Y*dpiScale) - cy
	sx := float32((underRect.W * dpiScale) / float64(underW))
	sy := float32((underRect.H * dpiScale) / float64(underH))
	return [6]float32{sx, 0, 0, sy, tx, ty}, true
}
