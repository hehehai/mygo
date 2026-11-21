//go:build darwin && !ios

package darwin

// WebKit's own controls over a PDF (WebView.SetPDFControls): a full-frame
// PDF's plugin asks WebKit for its HUD, a WKPDFHUDView added to the
// WKWebView (zoom out, zoom in, open in Preview, save), which fades in
// as the pointer moves over the page. An app that draws its own controls
// hides it: it is hidden as it is added (didAddSubview:), and those there
// are hidden or shown again as asked. A hidden view takes no clicks.

import "github.com/ebitengine/purego/objc"

// pdfHUDClass is WebKit's class of a PDF's controls.
const pdfHUDClass = "WKPDFHUDView"

// isPDFHUD reports whether v is a PDF's controls.
func isPDFHUD(v id) bool {
	return hasClass(pdfHUDClass) && sendBool(v, "isKindOfClass:", uintptr(class(pdfHUDClass)))
}

// SetPDFHUD shows WebKit's controls over a PDF (on, the default) or hides
// them, those there now and those added later.
func (w *window) SetPDFHUD(on bool) {
	w.noPDFHUD = !on
	if w.web == 0 {
		return
	}
	withPool(func() {
		for _, v := range arrayItems(send(w.web, "subviews")) {
			if isPDFHUD(v) {
				send(v, "setHidden:", boolArg(!on))
			}
		}
	})
}

// pdfHUDMethods are MyGoWebView's overrides that hide a PDF's controls
// as WebKit adds them.
func pdfHUDMethods() []objc.MethodDef {
	return []objc.MethodDef{
		method("didAddSubview:", func(self id, cmd objc.SEL, sub id) {
			sendSuper(self, "MyGoWebView", cmd, uintptr(sub))
			if w := theBackend.byWebView[self]; w != nil && w.noPDFHUD && isPDFHUD(sub) {
				send(sub, "setHidden:", 1)
			}
		}),
	}
}
