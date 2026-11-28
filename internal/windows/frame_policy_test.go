package windows

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestDWMFrameConstants(t *testing.T) {
	if dwmwaNCRenderingPolicy != 2 {
		t.Errorf("dwmwaNCRenderingPolicy = %d, want 2", dwmwaNCRenderingPolicy)
	}
	if dwmwaWindowCornerPreference != 33 {
		t.Errorf("dwmwaWindowCornerPreference = %d, want 33", dwmwaWindowCornerPreference)
	}
	if dwmwaBorderColor != 34 {
		t.Errorf("dwmwaBorderColor = %d, want 34", dwmwaBorderColor)
	}
	if dwmncrpUseWindowStyle != 0 || dwmncrpDisabled != 1 {
		t.Errorf("DWMNCRP constants = (%d, %d), want (0, 1)", dwmncrpUseWindowStyle, dwmncrpDisabled)
	}
	if dwmwcpDefault != 0 || dwmwcpDoNotRound != 1 || dwmwcpRound != 2 {
		t.Errorf("DWMWCP constants = (%d, %d, %d), want (0, 1, 2)", dwmwcpDefault, dwmwcpDoNotRound, dwmwcpRound)
	}
	if dwmColorNone != 0xFFFFFFFE || dwmColorDefault != 0xFFFFFFFF {
		t.Errorf("DWM border colors = (%#x, %#x), want (0xfffffffe, 0xffffffff)", dwmColorNone, dwmColorDefault)
	}
}

func TestWantsRoundedCorners(t *testing.T) {
	for _, tc := range []struct {
		name      string
		opts      platform.WindowOptions
		frameless bool
		shadow    bool
		vibrancy  string
		want      bool
	}{
		{
			name:      "transparent shadowless surface panel",
			opts:      platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true},
			frameless: true,
			want:      false,
		},
		{
			name:      "transparent shadowless webview panel",
			opts:      platform.WindowOptions{Panel: true, Frameless: true, Transparent: true},
			frameless: true,
			want:      false,
		},
		{
			name:      "transparent surface panel with shadow",
			opts:      platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true, HasShadow: true},
			frameless: true,
			shadow:    true,
			want:      true,
		},
		{
			name:      "transparent shadowless surface panel with vibrancy",
			opts:      platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true, Vibrancy: "acrylic"},
			frameless: true,
			vibrancy:  "acrylic",
			want:      true,
		},
		{
			name:      "transparent shadowless panel with explicit corner radius",
			opts:      platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true, CornerRadius: 12},
			frameless: true,
			want:      true,
		},
		{
			name:      "opaque frameless panel without shadow",
			opts:      platform.WindowOptions{Panel: true, Frameless: true, Surface: true},
			frameless: true,
			want:      true,
		},
		{
			name:      "titled panel",
			opts:      platform.WindowOptions{Panel: true, Surface: true, HasShadow: true},
			frameless: false,
			shadow:    true,
			want:      true,
		},
		{
			name:      "opaque frameless window with corner radius",
			opts:      platform.WindowOptions{Frameless: true, CornerRadius: 12},
			frameless: true,
			want:      true,
		},
		{
			name:      "opaque frameless window without corner radius",
			opts:      platform.WindowOptions{Frameless: true},
			frameless: true,
			want:      false,
		},
		{
			name:      "shadowed flyout handled separately",
			opts:      platform.WindowOptions{Frameless: true, Transparent: true, Surface: true, HasShadow: true, Flyout: &platform.Flyout{}},
			frameless: true,
			shadow:    true,
			want:      false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := wantsRoundedCorners(&tc.opts, tc.frameless, tc.shadow, tc.vibrancy); got != tc.want {
				t.Errorf("wantsRoundedCorners = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTransparentFrameDWM(t *testing.T) {
	for _, tc := range []struct {
		name       string
		opts       platform.WindowOptions
		frameless  bool
		shadow     bool
		noRedirect bool
		win11      bool
		vibrancy   string
		wantOK     bool
		want       transparentFrameSpec
	}{
		{
			name:       "Win11 transparent shadowless surface panel disables DWM chrome",
			opts:       platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true},
			frameless:  true,
			noRedirect: true,
			win11:      true,
			wantOK:     true,
			want: transparentFrameSpec{
				applyNCRendering: true,
				ncRendering:      dwmncrpDisabled,
				applyWin11Chrome: true,
				cornerPreference: dwmwcpDoNotRound,
				borderColor:      dwmColorNone,
			},
		},
		{
			name:       "Win10 transparent shadowless surface panel disables NC rendering only",
			opts:       platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true},
			frameless:  true,
			noRedirect: true,
			win11:      false,
			wantOK:     true,
			want: transparentFrameSpec{
				applyNCRendering: true,
				ncRendering:      dwmncrpDisabled,
			},
		},
		{
			name:       "Win11 transparent shadowless surface without DirectComposition skips DWMNCRP_DISABLED",
			opts:       platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true},
			frameless:  true,
			noRedirect: false,
			win11:      true,
			wantOK:     true,
			want: transparentFrameSpec{
				applyNCRendering: false,
				applyWin11Chrome: true,
				cornerPreference: dwmwcpDoNotRound,
				borderColor:      dwmColorNone,
			},
		},
		{
			name:       "Win11 transparent shadowless non-panel surface disables DWM chrome",
			opts:       platform.WindowOptions{Frameless: true, Transparent: true, Surface: true},
			frameless:  true,
			noRedirect: true,
			win11:      true,
			wantOK:     true,
			want: transparentFrameSpec{
				applyNCRendering: true,
				ncRendering:      dwmncrpDisabled,
				applyWin11Chrome: true,
				cornerPreference: dwmwcpDoNotRound,
				borderColor:      dwmColorNone,
			},
		},
		{
			name:       "Win11 transparent shadowless webview panel disables DWM chrome",
			opts:       platform.WindowOptions{Panel: true, Frameless: true, Transparent: true},
			frameless:  true,
			noRedirect: true,
			win11:      true,
			wantOK:     true,
			want: transparentFrameSpec{
				applyNCRendering: true,
				ncRendering:      dwmncrpDisabled,
				applyWin11Chrome: true,
				cornerPreference: dwmwcpDoNotRound,
				borderColor:      dwmColorNone,
			},
		},
		{
			name:       "Win11 transparent shadowed surface panel keeps rounded chrome",
			opts:       platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true, HasShadow: true},
			frameless:  true,
			shadow:     true,
			noRedirect: true,
			win11:      true,
			wantOK:     true,
			want: transparentFrameSpec{
				applyNCRendering: true,
				ncRendering:      dwmncrpUseWindowStyle,
				applyWin11Chrome: true,
				cornerPreference: dwmwcpRound,
				borderColor:      dwmColorDefault,
			},
		},
		{
			name:       "Win11 transparent shadowless surface panel with vibrancy restores rounded chrome",
			opts:       platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true},
			frameless:  true,
			noRedirect: true,
			win11:      true,
			vibrancy:   "acrylic",
			wantOK:     true,
			want: transparentFrameSpec{
				applyNCRendering: true,
				ncRendering:      dwmncrpUseWindowStyle,
				applyWin11Chrome: true,
				cornerPreference: dwmwcpRound,
				borderColor:      dwmColorDefault,
			},
		},
		{
			name:       "Win11 transparent shadowless panel with explicit CornerRadius keeps rounded chrome",
			opts:       platform.WindowOptions{Panel: true, Frameless: true, Transparent: true, Surface: true, CornerRadius: 12},
			frameless:  true,
			noRedirect: true,
			win11:      true,
			wantOK:     true,
			want: transparentFrameSpec{
				applyNCRendering: true,
				ncRendering:      dwmncrpUseWindowStyle,
				applyWin11Chrome: true,
				cornerPreference: dwmwcpRound,
				borderColor:      dwmColorDefault,
			},
		},
		{
			name:       "Win11 transparent shadowed non-panel surface uses system defaults",
			opts:       platform.WindowOptions{Frameless: true, Transparent: true, Surface: true, HasShadow: true},
			frameless:  true,
			shadow:     true,
			noRedirect: true,
			win11:      true,
			wantOK:     true,
			want: transparentFrameSpec{
				applyNCRendering: true,
				ncRendering:      dwmncrpUseWindowStyle,
				applyWin11Chrome: true,
				cornerPreference: dwmwcpDefault,
				borderColor:      dwmColorDefault,
			},
		},
		{
			name:       "opaque frameless panel untouched",
			opts:       platform.WindowOptions{Panel: true, Frameless: true, Surface: true},
			frameless:  true,
			noRedirect: false,
			win11:      true,
			wantOK:     false,
		},
		{
			name:       "transparent framed window untouched",
			opts:       platform.WindowOptions{Transparent: true, Surface: true},
			frameless:  false,
			noRedirect: true,
			win11:      true,
			wantOK:     false,
		},
		{
			name:       "shadowless flyout untouched",
			opts:       platform.WindowOptions{Frameless: true, Transparent: true, Surface: true, Flyout: &platform.Flyout{}},
			frameless:  true,
			noRedirect: true,
			win11:      true,
			wantOK:     false,
		},
		{
			name:       "shadowed flyout untouched",
			opts:       platform.WindowOptions{Frameless: true, Transparent: true, Surface: true, HasShadow: true, Flyout: &platform.Flyout{}},
			frameless:  true,
			shadow:     true,
			noRedirect: true,
			win11:      true,
			wantOK:     false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := transparentFrameDWM(&tc.opts, tc.frameless, tc.shadow, tc.noRedirect, tc.win11, tc.vibrancy)
			if ok != tc.wantOK {
				t.Fatalf("transparentFrameDWM ok = %v, want %v (spec=%+v)", ok, tc.wantOK, got)
			}
			if got != tc.want {
				t.Errorf("transparentFrameDWM = %+v, want %+v", got, tc.want)
			}
		})
	}
}
