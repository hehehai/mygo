package windows

import (
	"math"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestRGBAToPremulBGRA(t *testing.T) {
	t.Run("premultiplied and padded stride", func(t *testing.T) {
		// 2x2 image with stride 12 (4 bytes of row padding).
		pix := []byte{
			// row 0: opaque red, 50% premultiplied green, 4 bytes padding
			0xFF, 0x00, 0x00, 0xFF, 0x00, 0x80, 0x00, 0x80, 0xDE, 0xAD, 0xBE, 0xEF,
			// row 1: opaque blue, fully transparent, 4 bytes padding
			0x11, 0x22, 0xCC, 0xFF, 0x00, 0x00, 0x00, 0x00, 0xCA, 0xFE, 0xBA, 0xBE,
		}
		got, ok := rgbaToPremulBGRA(nil, pix, 12, 2, 2)
		if !ok {
			t.Fatal("rgbaToPremulBGRA returned ok=false")
		}
		want := []uint32{
			0xFFFF0000, // A=FF R=FF G=00 B=00
			0x80008000, // A=80 R=00 G=80 B=00 (already premultiplied, not darkened)
			0xFF1122CC, // A=FF R=11 G=22 B=CC
			0x00000000, // A=00 R=00 G=00 B=00
		}
		if len(got) != len(want) {
			t.Fatalf("len(got) = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("pixel %d = %#08x, want %#08x", i, got[i], want[i])
			}
		}
	})

	t.Run("straight alpha channels exceeding alpha are premultiplied safely", func(t *testing.T) {
		pix := []byte{
			// straight-alpha orange at 50% alpha, and RGB>0 with alpha=0
			255, 128, 64, 128,
			200, 100, 50, 0,
		}
		got, ok := rgbaToPremulBGRA(nil, pix, 8, 2, 1)
		if !ok {
			t.Fatal("rgbaToPremulBGRA returned ok=false")
		}
		// (255*128+127)/255 = 128, (128*128+127)/255 = 64, (64*128+127)/255 = 32
		want0 := uint32(128)<<24 | uint32(128)<<16 | uint32(64)<<8 | uint32(32)
		if got[0] != want0 {
			t.Errorf("pixel 0 = %#08x, want %#08x", got[0], want0)
		}
		if got[1] != 0 {
			t.Errorf("pixel 1 = %#08x, want 0x00000000", got[1])
		}
		for i, p := range got {
			a := (p >> 24) & 0xFF
			r := (p >> 16) & 0xFF
			g := (p >> 8) & 0xFF
			b := p & 0xFF
			if r > a || g > a || b > a {
				t.Errorf("pixel %d violates premultiplied invariant: a=%d r=%d g=%d b=%d", i, a, r, g, b)
			}
		}
	})

	t.Run("reuses destination buffer", func(t *testing.T) {
		buf := make([]uint32, 0, 8)
		pix := []byte{10, 20, 30, 255, 40, 50, 60, 255}
		got, ok := rgbaToPremulBGRA(buf, pix, 8, 2, 1)
		if !ok {
			t.Fatal("rgbaToPremulBGRA returned ok=false")
		}
		if cap(got) != 8 || len(got) != 2 {
			t.Fatalf("got len=%d cap=%d, want len=2 cap=8", len(got), cap(got))
		}
		if got[0] != 0xFF0A141E || got[1] != 0xFF28323C {
			t.Errorf("got = [%#08x, %#08x]", got[0], got[1])
		}
	})

	t.Run("invalid inputs rejected", func(t *testing.T) {
		valid := make([]byte, 16)
		cases := []struct {
			name   string
			pix    []byte
			stride int
			w, h   int
		}{
			{"nil pix", nil, 8, 2, 2},
			{"zero width", valid, 8, 0, 2},
			{"zero height", valid, 8, 2, 0},
			{"negative size", valid, 8, -1, 2},
			{"stride too small", valid, 4, 2, 2},
			{"short buffer", valid[:12], 8, 2, 2},
		}
		for _, tc := range cases {
			if _, ok := rgbaToPremulBGRA(nil, tc.pix, tc.stride, tc.w, tc.h); ok {
				t.Errorf("%s: expected ok=false", tc.name)
			}
		}
	})
}

func TestUnderlayTransform(t *testing.T) {
	approx := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

	cases := []struct {
		name      string
		underRect platform.RectF
		clip      platform.RectF
		underW    int
		underH    int
		dpiScale  float64
		want      [6]float32
	}{
		{
			name:      "100% DPI centered card over full window",
			underRect: platform.RectF{X: 0, Y: 0, W: 1200, H: 800},
			clip:      platform.RectF{X: 200, Y: 100, W: 500, H: 400},
			underW:    48,
			underH:    48,
			dpiScale:  1.0,
			want:      [6]float32{25, 0, 0, 800.0 / 48.0, -200, -100},
		},
		{
			name:      "150% DPI fractional clip snaps identically to clipVis",
			underRect: platform.RectF{X: 0, Y: 0, W: 1200, H: 800},
			clip:      platform.RectF{X: 201, Y: 101, W: 500, H: 400},
			underW:    48,
			underH:    48,
			dpiScale:  1.5,
			// cx = round(201*1.5) = 302, cy = round(101*1.5) = 152
			// cx + tx == 0, cy + ty == 0 so screen alignment has zero subpixel seam.
			want: [6]float32{37.5, 0, 0, 25, -302, -152},
		},
		{
			name:      "200% DPI offset underlay rect",
			underRect: platform.RectF{X: 50, Y: 20, W: 800, H: 600},
			clip:      platform.RectF{X: 100, Y: 80, W: 400, H: 300},
			underW:    64,
			underH:    32,
			dpiScale:  2.0,
			want:      [6]float32{25, 0, 0, 37.5, -100, -120},
		},
		{
			name:      "partial overlap and negative underlay origin",
			underRect: platform.RectF{X: -100, Y: -50, W: 600, H: 400},
			clip:      platform.RectF{X: 50, Y: 50, W: 300, H: 200},
			underW:    48,
			underH:    48,
			dpiScale:  1.0,
			want:      [6]float32{12.5, 0, 0, 400.0 / 48.0, -150, -100},
		},
		{
			name:      "underlay starts inside clip",
			underRect: platform.RectF{X: 120, Y: 90, W: 240, H: 180},
			clip:      platform.RectF{X: 100, Y: 50, W: 300, H: 300},
			underW:    24,
			underH:    18,
			dpiScale:  1.5,
			// cx = round(150) = 150, cy = round(75) = 75
			// tx = 120*1.5 - 150 = 30, ty = 90*1.5 - 75 = 60
			want: [6]float32{15, 0, 0, 15, 30, 60},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := underlayTransform(tc.underRect, tc.clip, tc.underW, tc.underH, tc.dpiScale)
			if !ok {
				t.Fatal("underlayTransform returned ok=false")
			}
			for i := range tc.want {
				if !approx(got[i], tc.want[i]) {
					t.Errorf("m[%d] = %v, want %v (full=%v)", i, got[i], tc.want[i], got)
				}
			}
			// Verify surface-pixel endpoints: clipVis origin + transform(0,0) and transform(underW,underH).
			cx := float32(math.Round(tc.clip.X * tc.dpiScale))
			cy := float32(math.Round(tc.clip.Y * tc.dpiScale))
			x0 := cx + got[4]
			y0 := cy + got[5]
			x1 := cx + got[0]*float32(tc.underW) + got[4]
			y1 := cy + got[3]*float32(tc.underH) + got[5]
			if !approx(x0, float32(tc.underRect.X*tc.dpiScale)) || !approx(y0, float32(tc.underRect.Y*tc.dpiScale)) {
				t.Errorf("top-left surface px = (%v, %v), want (%v, %v)", x0, y0, tc.underRect.X*tc.dpiScale, tc.underRect.Y*tc.dpiScale)
			}
			if !approx(x1, float32((tc.underRect.X+tc.underRect.W)*tc.dpiScale)) || !approx(y1, float32((tc.underRect.Y+tc.underRect.H)*tc.dpiScale)) {
				t.Errorf("bottom-right surface px = (%v, %v), want (%v, %v)", x1, y1, (tc.underRect.X+tc.underRect.W)*tc.dpiScale, (tc.underRect.Y+tc.underRect.H)*tc.dpiScale)
			}
		})
	}

	t.Run("invalid bounds return ok=false", func(t *testing.T) {
		r := platform.RectF{X: 0, Y: 0, W: 100, H: 100}
		if _, ok := underlayTransform(platform.RectF{}, r, 48, 48, 1); ok {
			t.Error("empty underRect should fail")
		}
		if _, ok := underlayTransform(r, platform.RectF{}, 48, 48, 1); ok {
			t.Error("empty clip should fail")
		}
		if _, ok := underlayTransform(r, r, 0, 48, 1); ok {
			t.Error("zero underW should fail")
		}
		if _, ok := underlayTransform(r, r, 48, 0, 1); ok {
			t.Error("zero underH should fail")
		}
		if _, ok := underlayTransform(r, r, 48, 48, 0); ok {
			t.Error("zero dpiScale should fail")
		}
		if _, ok := underlayTransform(platform.RectF{X: math.NaN(), W: 10, H: 10}, r, 48, 48, 1); ok {
			t.Error("NaN underRect should fail")
		}
	})
}

func TestEffectiveWebViewBackground(t *testing.T) {
	dark := &platform.Color{R: 0x18, G: 0x1A, B: 0x20, A: 0xFF}
	clear := &platform.Color{R: 0x18, G: 0x1A, B: 0x20, A: 0x00}
	wantDark := uint32(0xFF) | uint32(0x18)<<8 | uint32(0x1A)<<16 | uint32(0x20)<<24

	cases := []struct {
		name        string
		bg          *platform.Color
		embedded    bool
		hasUnderlay bool
		transparent bool
		noRedirect  bool
		vibrancy    string
		want        uint32
	}{
		{
			name:        "embedded with underlay forces transparent over opaque bg",
			bg:          dark,
			embedded:    true,
			hasUnderlay: true,
			want:        0,
		},
		{
			name:        "embedded with underlay forces transparent over default white",
			bg:          nil,
			embedded:    true,
			hasUnderlay: true,
			want:        0,
		},
		{
			name:        "embedded after clearing underlay restores opaque bg",
			bg:          dark,
			embedded:    true,
			hasUnderlay: false,
			want:        wantDark,
		},
		{
			name:        "embedded without underlay and nil bg defaults to opaque white",
			bg:          nil,
			embedded:    true,
			hasUnderlay: false,
			want:        0xFFFFFFFF,
		},
		{
			name:        "embedded transparent option without underlay stays transparent",
			bg:          nil,
			embedded:    true,
			hasUnderlay: false,
			transparent: true,
			want:        0,
		},
		{
			name:        "embedded alpha-zero bg without underlay stays transparent",
			bg:          clear,
			embedded:    true,
			hasUnderlay: false,
			want:        0,
		},
		{
			name:        "top-level opaque window unaffected",
			bg:          dark,
			embedded:    false,
			hasUnderlay: true, // ignored for non-embedded windows
			want:        wantDark,
		},
		{
			name:       "top-level vibrancy window stays transparent",
			bg:         dark,
			embedded:   false,
			vibrancy:   "mica",
			noRedirect: true,
			want:       0,
		},
		{
			name:        "top-level transparent window with noRedirect and opaque bg keeps bg",
			bg:          dark,
			embedded:    false,
			transparent: true,
			noRedirect:  true,
			want:        wantDark,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := effectiveWebViewBackground(tc.bg, tc.embedded, tc.hasUnderlay, tc.transparent, tc.noRedirect, tc.vibrancy)
			if got != tc.want {
				t.Errorf("effectiveWebViewBackground = %#08x, want %#08x", got, tc.want)
			}
		})
	}
}
