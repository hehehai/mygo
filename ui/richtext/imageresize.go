package richtext

// A picture in the body is resized by its corners, as in a word
// processor: selected alone, it shows a handle at each corner; a drag of
// one keeps the picture's aspect ratio, and the size it ends at is one
// undoable step, written to the picture's Width and Height. Its box is
// where the editor lays it out (Document.AttachmentRect).

import (
	"github.com/egoist/mygo/ui"
)

const (
	// handleSize is a corner handle's square, in DIPs; handleReach is how
	// far from a corner's centre a press still takes the handle.
	handleSize   = 8
	handleReach  = 7
	minImageSide = 24
)

// The corners of a picture's box, by index: bit 0 right, bit 1 bottom.
const (
	topLeft = iota
	topRight
	bottomLeft
	bottomRight
)

// imageResize is a drag of a picture's corner.
type imageResize struct {
	i      int     // the picture's rune
	corner int     // topLeft … bottomRight
	x0, y0 float64 // where the press was, in the content
	box    ui.Rect // the picture's box when the press was (content coordinates)
	w, h   float32 // the size it shows now
}

// AttachmentRect returns the box of the picture at rune i in the editor
// showing the document, in DIPs from the top left of the editor's element,
// scrolled, as CaretRect gives the caret: while a drag of one of its
// corners resizes it, the size it shows. ok is false when rune i is not a
// picture laid out, until an editor showed the document, and while the
// picture is scrolled out of view.
//
// Call it on the main thread, as the view builds.
func (d *Document) AttachmentRect(i int) (r ui.Rect, ok bool) {
	v := d.laidOut()
	if v == nil {
		return ui.Rect{}, false
	}
	box, ok := v.imageBox(i)
	if !ok {
		return ui.Rect{}, false
	}
	if v.resize != nil && v.resize.i == i {
		box.W, box.H = v.resize.w, v.resize.h
	}
	r = ui.Rect{X: v.padX + box.X, Y: v.padY + float32(float64(box.Y)-v.scroll), W: box.W, H: box.H}
	return r, r.Y+r.H > 0 && r.Y < v.rect.H
}

// imageBox returns the box of the picture at rune i in content
// coordinates (before scrolling), false when rune i is not a picture laid
// out.
func (v *view) imageBox(i int) (ui.Rect, bool) {
	if i < 0 || i >= v.shown.Len() {
		return ui.Rect{}, false
	}
	if at := v.shown.AttributesAt(i).Attachment; at == nil || at.Image == nil {
		return ui.Rect{}, false
	}
	p := v.shown.paraAt(i)
	if p < 0 || p >= len(v.paras) {
		return ui.Rect{}, false
	}
	pv := v.paraLayout(p)
	px, py := v.origin(p)
	l := v.local(p, i)
	for _, o := range pv.layout.Objects() {
		if pv.runeOfUnit(o.Index) == l {
			return ui.Rect{X: float32(px) + o.Rect.X, Y: float32(py) + o.Rect.Y, W: o.Rect.W, H: o.Rect.H}, true
		}
	}
	return ui.Rect{}, false
}

// selectedImage returns the picture selected alone, with its box.
func (v *view) selectedImage() (int, ui.Rect, bool) {
	a, z := v.selection()
	if z != a+1 {
		return 0, ui.Rect{}, false
	}
	box, ok := v.imageBox(a)
	return a, box, ok
}

// cornerAt reports which corner's handle the content point (x, y) is on,
// for the picture selected.
func (v *view) cornerAt(x float32, y float64) (int, bool) {
	_, box, ok := v.selectedImage()
	if !ok || v.opts.ReadOnly {
		return 0, false
	}
	if v.resize != nil {
		box.W, box.H = v.resize.w, v.resize.h
	}
	for k := range 4 {
		cx, cy := float64(box.X), float64(box.Y)
		if k&1 != 0 {
			cx += float64(box.W)
		}
		if k&2 != 0 {
			cy += float64(box.H)
		}
		if nearHandle(float64(x), cx) && nearHandle(y, cy) {
			return k, true
		}
	}
	return 0, false
}

// nearHandle reports whether a and b are within a handle's reach.
func nearHandle(a, b float64) bool {
	d := a - b
	return d <= handleReach && d >= -handleReach
}

// cornerCursor is the pointer over a corner: diagonal arrows.
func cornerCursor(corner int) ui.Cursor {
	if corner == topLeft || corner == bottomRight {
		return ui.CursorResizeNWSE
	}
	return ui.CursorResizeNESW
}

// startResize begins a drag of a picture's corner, if the press is on one.
func (v *view) startResize(ev ui.InputEvent) bool {
	x, y := v.contentPoint(ev.X, ev.Y)
	k, ok := v.cornerAt(x, y)
	if !ok {
		return false
	}
	i, box, _ := v.selectedImage()
	v.resize = &imageResize{i: i, corner: k, x0: float64(x), y0: y, box: box, w: box.W, h: box.H}
	return true
}

// resizeTo sizes the picture of the drag to the pointer: the larger of
// the two projections of the drag (width, or height in the picture's
// aspect ratio), so the corner follows the pointer either way.
func (v *view) resizeTo(ev ui.InputEvent) {
	r := v.resize
	x, y := v.contentPoint(ev.X, ev.Y)
	dx, dy := x-float32(r.x0), float32(y-r.y0)
	if r.corner&1 == 0 {
		dx = -dx // the left corners: dragged left, the picture grows
	}
	if r.corner&2 == 0 {
		dy = -dy // the top corners: dragged up, it grows
	}
	if r.box.W <= 0 || r.box.H <= 0 {
		return
	}
	w := r.box.W + dx
	if wh := r.box.W + dy*r.box.W/r.box.H; abs32(wh-r.box.W) > abs32(w-r.box.W) {
		w = wh
	}
	w = max(w, minImageSide)
	if v.look.width > 0 {
		w = min(w, v.look.width)
	}
	r.w, r.h = w, w*r.box.H/r.box.W
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

// finishResize ends the drag: a size that changed becomes one step of
// undo, which the editor reports as a change.
func (v *view) finishResize() {
	r := v.resize
	v.resize = nil
	if abs32(r.w-r.box.W) < 1 && abs32(r.h-r.box.H) < 1 {
		return
	}
	v.command(func(d *Document) {
		d.Edit(func(ed *Edit) {
			attrs := ed.Text().AttributesAt(r.i)
			if attrs.Attachment == nil {
				return
			}
			next := *attrs.Attachment
			next.Width, next.Height = r.w, r.h
			attrs.Attachment = &next
			ed.ReplaceString(r.i, r.i+1, string(ObjectReplacement), attrs)
			ed.Select(r.i, r.i+1)
		})
	})
}

// paintHandles draws the corner handles of the picture selected alone, at
// the box it shows (the drag's size while dragging).
func (v *view) paintHandles(p *ui.Painter, ox, oy float32, scroll float64) {
	_, box, ok := v.selectedImage()
	if !ok {
		return
	}
	if v.resize != nil {
		box.W, box.H = v.resize.w, v.resize.h
	}
	look := &v.look
	white := ui.Hex("#ffffff")
	for k := range 4 {
		cx := ox + box.X
		cy := oy + float32(float64(box.Y)-scroll)
		if k&1 != 0 {
			cx += box.W
		}
		if k&2 != 0 {
			cy += box.H
		}
		r := ui.Rect{X: cx - handleSize/2, Y: cy - handleSize/2, W: handleSize, H: handleSize}
		p.Fill(r, white, 2)
		p.Stroke(r, look.Caret, 2, 1)
	}
}
