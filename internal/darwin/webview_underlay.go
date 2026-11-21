//go:build darwin && !ios

package darwin

import (
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// A web view's underlay (WebView.SetUnderlay): a picture under its page,
// a layer-backed view in its pane under the web view (over the pane's
// background), the picture its layer's contents stretched across a
// rectangle of the window's content (the surface's DIPs). Where the page
// paints nothing (its background transparent) it shows, continuing what
// the content paints around the web view; the pane and the clip view cut
// it to the web view's place and corners.

// SetUnderlay shows the picture of pix (RGBA, premultiplied) under the
// page across r, fading in over the one before for fade seconds; none
// (pix nil) takes it away.
func (w *window) SetUnderlay(pix []byte, stride, width, height int, r platform.RectF, fade float64) {
	if w.closed || w.pane == 0 || w.foreign {
		return
	}
	withPool(func() {
		if pix == nil || width <= 0 || height <= 0 || len(pix) < stride*height {
			w.dropUnderlay()
			return
		}
		loadSurface()
		fresh := w.underlay == 0
		if fresh {
			// Flipped as the surface's view, whose layer shows its frames
			// as CGImages the same way — and, as it, it updates its layer
			// itself (wantsUpdateLayer: nothing) and is never redrawn by
			// AppKit: a layer-backed view drawn by AppKit (a plain view)
			// had its contents replaced by its own empty drawing as its
			// frame changed from nothing to the window's size, right after
			// the picture was set (Refto #401: the page over the window's
			// background, white, on the Mac).
			w.underlay = msgInitRect(send(class("MyGoUnderlayView"), "alloc"), sel("initWithFrame:"), NSRect{})
			send(w.underlay, "setWantsLayer:", 1)
			send(w.underlay, "setLayerContentsRedrawPolicy:", 0) // NSViewLayerContentsRedrawNever
			send(w.underlay, "setAutoresizingMask:", 0)
			send(w.pane, "addSubview:positioned:relativeTo:", uintptr(w.underlay), nsWindowBelow, uintptr(w.web))
		}
		layer := send(w.underlay, "layer")
		send(layer, "setContentsGravity:", uintptr(nsString("resize")))
		data, _, _ := purego.SyscallN(cfDataCreate, 0, uintptr(unsafe.Pointer(&pix[0])), uintptr(stride*height))
		provider, _, _ := purego.SyscallN(cgProviderCreate, data)
		const bitmapInfo = 1 // premultiplied alpha last: RGBA
		img, _, _ := purego.SyscallN(cgImageCreate, uintptr(width), uintptr(height), 8, 32, uintptr(stride), cgColorSpaceSRGB, bitmapInfo, provider, 0, 0, 0)
		if fade > 0 && !fresh {
			t := send(class("CATransition"), "animation")
			send(t, "setType:", uintptr(nsString("fade")))
			msgSetFloat(t, sel("setDuration:"), fade)
			send(layer, "addAnimation:forKey:", uintptr(t), uintptr(nsString("underlay")))
		}
		send(layer, "setContents:", img)
		purego.SyscallN(cgImageRelease, img)
		purego.SyscallN(cgProviderRelease, provider)
		cfRelease(data)
		w.underRect = r
		w.placeUnderlay()
	})
}

// SetUnderlayRect moves the underlay to r (the window's content changed
// size), its picture kept.
func (w *window) SetUnderlayRect(r platform.RectF) {
	w.underRect = r
	if w.underlay != 0 {
		withPool(w.placeUnderlay)
	}
}

// placeUnderlay puts the underlay at its rectangle in the pane, whose
// bounds (flipped) are the page's size scaled to the pane's frame, as
// last placed (PlaceWebViews).
func (w *window) placeUnderlay() {
	if w.underlay == 0 || w.paneAt.W <= 0 || w.paneAt.H <= 0 {
		return
	}
	r, f := w.underRect, w.paneAt
	kx, ky := w.paneSize.Width/f.W, w.paneSize.Height/f.H
	msgSetRect(w.underlay, sel("setFrame:"), NSRect{Origin: NSPoint{(r.X - f.X) * kx, (r.Y - f.Y) * ky}, Size: NSSize{r.W * kx, r.H * ky}})
}

// dropUnderlay takes the underlay away.
func (w *window) dropUnderlay() {
	if w.underlay == 0 {
		return
	}
	send(w.underlay, "removeFromSuperview")
	release(w.underlay)
	w.underlay = 0
}

// registerUnderlayViewClass registers the underlay's view: flipped, its
// layer's contents its own (set by SetUnderlay, never drawn by AppKit),
// nothing for assistive technology, the pointer not its.
func registerUnderlayViewClass() {
	classDef("MyGoUnderlayView", "NSView", nil, []objc.MethodDef{
		method("isFlipped", func(self id, _ objc.SEL) bool { return true }),
		method("wantsUpdateLayer", func(self id, _ objc.SEL) bool { return true }),
		method("updateLayer", func(self id, _ objc.SEL) {}),
		method("hitTest:", func(self id, _ objc.SEL, p NSPoint) id { return 0 }),
		method("accessibilityChildren", func(self id, _ objc.SEL) id { return nsArray() }),
		method("isAccessibilityElement", func(self id, _ objc.SEL) bool { return false }),
	})
}
