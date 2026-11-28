package windows

import "github.com/egoist/mygo/internal/platform"

// DWM window attributes and values used for transparent frame policy and
// Windows 11 corner and border control (dwmapi.h).
const (
	dwmwaNCRenderingPolicy      = 2
	dwmwaWindowCornerPreference = 33
	dwmwaBorderColor            = 34

	dwmncrpUseWindowStyle = 0 // DWMNCRP_USEWINDOWSTYLE
	dwmncrpDisabled       = 1 // DWMNCRP_DISABLED

	dwmwcpDefault    = 0 // DWMWCP_DEFAULT
	dwmwcpDoNotRound = 1 // DWMWCP_DONOTROUND
	dwmwcpRound      = 2 // DWMWCP_ROUND

	dwmColorNone    = 0xFFFFFFFE // DWMWA_COLOR_NONE: no color, not the accent
	dwmColorDefault = 0xFFFFFFFF // DWMWA_COLOR_DEFAULT: system default color
)

// transparentFrameSpec describes the DWM attributes to apply to a frameless
// transparent non-flyout window.
type transparentFrameSpec struct {
	applyNCRendering bool
	ncRendering      int32 // dwmncrpUseWindowStyle or dwmncrpDisabled

	applyWin11Chrome bool
	cornerPreference int32  // dwmwcpDefault, dwmwcpDoNotRound, or dwmwcpRound
	borderColor      uint32 // dwmColorDefault or dwmColorNone
}

// plainTransparentDWM reports a frameless transparent non-flyout window with
// no shadow, no system material, and no explicit CornerRadius: DWM must not
// draw non-client borders, rounded corners, or a drop shadow around its outer
// rectangle.
func plainTransparentDWM(opts *platform.WindowOptions, frameless, shadow bool, vibrancy string) bool {
	return frameless && opts.Transparent && opts.Flyout == nil && vibrancy == "" && !shadow && opts.CornerRadius <= 0
}

// wantsRoundedCorners reports whether a non-flyout window asks Windows 11 DWM
// for rounded corners (DWMWCP_ROUND): frameless windows with CornerRadius > 0,
// and panels that are not plain transparent shadowless overlays.
func wantsRoundedCorners(opts *platform.WindowOptions, frameless, shadow bool, vibrancy string) bool {
	if opts.Flyout != nil {
		return false
	}
	if frameless && opts.CornerRadius > 0 {
		return true
	}
	return opts.Panel && !plainTransparentDWM(opts, frameless, shadow, vibrancy)
}

// transparentFrameDWM returns the DWM attributes to apply to a frameless
// transparent non-flyout window. For non-transparent, titled, or flyout
// windows, ok is false and nothing is changed.
func transparentFrameDWM(opts *platform.WindowOptions, frameless, shadow, noRedirect, win11 bool, vibrancy string) (spec transparentFrameSpec, ok bool) {
	if !frameless || !opts.Transparent || opts.Flyout != nil {
		return spec, false
	}
	plain := plainTransparentDWM(opts, frameless, shadow, vibrancy)
	// WebView windows always manage DWM non-client rendering (with WM_NCPAINT
	// and WM_NCACTIVATE suppressed when plain); DirectComposition native UI
	// surfaces have no GDI redirection bitmap (WS_EX_NOREDIRECTIONBITMAP), so
	// disabling DWM NC rendering removes the outer shadow, border, and frame
	// without affecting DirectComposition or painting classic GDI borders.
	if !opts.Surface || noRedirect {
		spec.applyNCRendering = true
		spec.ncRendering = dwmncrpUseWindowStyle
		if plain {
			spec.ncRendering = dwmncrpDisabled
		}
	}
	if win11 {
		spec.applyWin11Chrome = true
		if plain {
			spec.cornerPreference = dwmwcpDoNotRound
			spec.borderColor = dwmColorNone
		} else {
			spec.cornerPreference = dwmwcpDefault
			if wantsRoundedCorners(opts, frameless, shadow, vibrancy) {
				spec.cornerPreference = dwmwcpRound
			}
			spec.borderColor = dwmColorDefault
		}
	}
	return spec, true
}
