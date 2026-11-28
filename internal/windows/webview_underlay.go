//go:build windows && (amd64 || arm64)

package windows

import (
	"errors"
	"log"
	"slices"
	"unsafe"

	"github.com/egoist/mygo/internal/platform"
)

// A web view's underlay (WebView.SetUnderlay): a DirectComposition surface of
// premultiplied BGRA pixels on underVis, inserted in clipVis directly below
// webVis so transparent parts of the page show it. clipVis cuts it to the
// web view's clip rectangle and applies any placement opacity to both layers
// together. Windows applies a new underlay immediately (fade is ignored).

// Vtable index and enum value from dcomp.h and dcomptypes.h.
const (
	dcompVisualSetBitmapInterpolationMode = 11 // IDCompositionVisual (between SetEffect=10 and SetBorderMode=12)
	dcompBitmapInterpolationLinear        = 1  // DCOMPOSITION_BITMAP_INTERPOLATION_MODE_LINEAR
)

// Compile-time check matching the interface asserted in package mygo's WebView.SetUnderlay.
var _ interface {
	SetUnderlay(pix []byte, stride, width, height int, r platform.RectF, fade float64)
	SetUnderlayRect(r platform.RectF)
} = (*window)(nil)

// SetUnderlay shows the picture of pix (RGBA, premultiplied) under the
// embedded web view's page across r; nil or empty pix clears the underlay and
// restores the web view's configured background color.
func (w *window) SetUnderlay(pix []byte, stride, width, height int, r platform.RectF, fade float64) {
	if w.closed || w.host == nil {
		return
	}
	s := w.host.surface
	if s == nil || s.layer == nil {
		return
	}
	if pix == nil || width <= 0 || height <= 0 || stride < width*4 || len(pix)/stride < height {
		w.dropUnderlay()
		return
	}
	px, ok := rgbaToPremulBGRA(nil, pix, stride, width, height)
	if !ok {
		w.dropUnderlay()
		return
	}
	samePixels := w.hasUnderlay && w.underW == width && w.underH == height && slices.Equal(w.underPx, px)
	if !samePixels {
		w.underPx = px
		w.underW, w.underH = width, height
	}
	w.underRect = r
	was := w.hasUnderlay
	if s.layer.ownDev {
		// A device created via DCompositionCreateDevice2(0, ...) cannot create
		// DirectComposition surfaces; keep the web view's normal background
		// rather than exposing an empty transparent hole.
		w.hasUnderlay = false
		w.underPx = nil
		w.underW, w.underH = 0, 0
		w.underRect = platform.RectF{}
		if was && w.controller != 0 {
			w.applyWebViewBackground()
		}
		return
	}
	w.hasUnderlay = true
	if s.layer.dcomp == 0 {
		// Waiting for recompose after GPU device loss; rebuildWebLayer will
		// recreate underVis/underSurf and upload underPx.
		if !was && w.controller != 0 {
			w.applyWebViewBackground()
		}
		return
	}
	if err := w.uploadUnderlay(!samePixels); err != nil {
		if dev := s.layer.dev; dev != nil && dev.lost() {
			w.b.loseComposition(err)
			if !was && w.controller != 0 {
				w.applyWebViewBackground()
			}
			return
		}
		log.Printf("mygo: cannot draw web view underlay through DirectComposition: %v", err)
		w.dropUnderlay()
		return
	}
	w.placeUnderlay()
	s.layer.commit()
	if !was && w.controller != 0 {
		w.applyWebViewBackground()
	}
}

// SetUnderlayRect moves the underlay to r (for example when the window
// resizes), keeping its uploaded picture.
func (w *window) SetUnderlayRect(r platform.RectF) {
	if w.closed || w.host == nil {
		return
	}
	w.underRect = r
	if w.underVis == 0 {
		return
	}
	s := w.host.surface
	if s == nil || s.layer == nil || s.layer.dcomp == 0 {
		return
	}
	w.placeUnderlay()
	s.layer.commit()
}

// uploadUnderlay ensures underVis and underSurf exist on s.layer and writes
// w.underPx into underSurf when pixelsChanged or a new visual/surface was
// created.
func (w *window) uploadUnderlay(pixelsChanged bool) error {
	s := w.host.surface
	if s == nil || s.layer == nil || s.layer.dev == nil || s.layer.dcomp == 0 || w.clipVis == 0 || w.webVis == 0 {
		return nil
	}
	if len(w.underPx) < w.underW*w.underH || w.underW <= 0 || w.underH <= 0 {
		return nil
	}
	l := s.layer
	freshVisual := w.underVis == 0
	if freshVisual {
		v := l.visual()
		if v == 0 {
			return errors.New("mygo: IDCompositionDevice::CreateVisual returned nil")
		}
		comCall(v, dcompVisualSetBitmapInterpolationMode, dcompBitmapInterpolationLinear)
		// Insert directly below w.webVis inside w.clipVis (insertAbove = FALSE).
		if hr := comCall(w.clipVis, dcompVisualAddVisual, v, 0, w.webVis); failed(hr) {
			release(v)
			return hresultError("IDCompositionVisual::AddVisual", hr)
		}
		w.underVis = v
	}
	needSurface := w.underSurf == 0 || w.underSurfW != w.underW || w.underSurfH != w.underH
	if needSurface {
		var surf uintptr
		if hr := comCall(l.dcomp, dcompDevCreateSurface, uintptr(w.underW), uintptr(w.underH), dxgiFormatB8G8R8A8, dxgiAlphaPremultiplied,
			uintptr(unsafe.Pointer(&surf))); failed(hr) {
			return hresultError("IDCompositionDevice::CreateSurface", hr)
		}
		if hr := comCall(w.underVis, dcompVisualSetContent, surf); failed(hr) {
			release(surf)
			return hresultError("IDCompositionVisual::SetContent", hr)
		}
		release(w.underSurf)
		w.underSurf = surf
		w.underSurfW, w.underSurfH = w.underW, w.underH
	}
	if !pixelsChanged && !needSurface && !freshVisual {
		return nil
	}
	var tex uintptr
	var offset point
	if hr := comCall(w.underSurf, dcompSurfaceBeginDraw, 0, uintptr(unsafe.Pointer(&iidID3D11Texture2D)),
		uintptr(unsafe.Pointer(&tex)), uintptr(unsafe.Pointer(&offset))); failed(hr) {
		return hresultError("IDCompositionSurface::BeginDraw", hr)
	}
	// The surface may live in a DirectComposition atlas: upload at offset.
	box := [6]uint32{uint32(offset.X), uint32(offset.Y), 0, uint32(offset.X + int32(w.underW)), uint32(offset.Y + int32(w.underH)), 1}
	comCall(l.dev.ctx, d3dCtxUpdateSubresource, tex, 0, uintptr(unsafe.Pointer(&box)), uintptr(unsafe.Pointer(&w.underPx[0])), uintptr(w.underW*4), 0)
	release(tex)
	if hr := comCall(w.underSurf, dcompSurfaceEndDraw); failed(hr) {
		return hresultError("IDCompositionSurface::EndDraw", hr)
	}
	return nil
}

// placeUnderlay sets underVis's 2D transform inside clipVis for the current
// placement, underRect, and surface DPI.
func (w *window) placeUnderlay() {
	if w.underVis == 0 || w.host == nil || w.host.surface == nil {
		return
	}
	k := float64(w.host.surface.dpi()) / 96
	m, ok := underlayTransform(w.underRect, w.placement.Clip, w.underW, w.underH, k)
	if !ok {
		place(w.underVis, 0, 0, 0, 0)
		return
	}
	comCall(w.underVis, dcompVisualSetTransform, uintptr(unsafe.Pointer(&m)))
}

// releaseUnderlayVisuals detaches and releases the DirectComposition visual
// and surface of the underlay without clearing the cached pixel state.
func (w *window) releaseUnderlayVisuals() bool {
	removed := false
	if w.underVis != 0 {
		if w.clipVis != 0 {
			comCall(w.clipVis, dcompVisualRemoveVisual, w.underVis)
			removed = true
		}
		comCall(w.underVis, dcompVisualSetContent, 0)
		release(w.underVis)
		w.underVis = 0
	}
	if w.underSurf != 0 {
		release(w.underSurf)
		w.underSurf = 0
	}
	w.underSurfW, w.underSurfH = 0, 0
	return removed
}

// dropUnderlay clears the underlay, releases its DirectComposition resources,
// commits the layer, and restores the web view's background color.
func (w *window) dropUnderlay() {
	was := w.hasUnderlay
	w.hasUnderlay = false
	w.underPx = nil
	w.underW, w.underH = 0, 0
	w.underRect = platform.RectF{}
	removed := w.releaseUnderlayVisuals()
	if removed && w.host != nil && w.host.surface != nil && w.host.surface.layer != nil {
		w.host.surface.layer.commit()
	}
	if was && w.controller != 0 {
		w.applyWebViewBackground()
	}
}
