package ui

import (
	"image"
	"image/draw"
	"slices"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/raster"
	"github.com/egoist/mygo/transfer"
)

type dataSource struct {
	data    func() transfer.Data
	options transfer.DragOptions
}

// DragData makes e a native drag source after a primary press moves a few
// DIPs. Data can go to other windows and applications. A source also given
// Drag(value) offers that value to typed targets in this process only.
// The default operation is copy and the default preview is the element.
func (e *node) DragData(data transfer.Data, options ...transfer.DragOptions) *node {
	return e.DragDataFrom(func() transfer.Data { return data }, options...)
}

// DragDataFrom obtains a transfer when the gesture starts, useful for a
// changing selection. The function and options.Done run on the main thread.
// See transfer.Lazy for deferring individual serialized representations.
func (e *node) DragDataFrom(data func() transfer.Data, options ...transfer.DragOptions) *node {
	if data == nil {
		panic("ui: nil drag data source")
	}
	if len(options) > 1 {
		panic("ui: DragData expects at most one options value")
	}
	var o transfer.DragOptions
	if len(options) == 1 {
		o = options[0]
	}
	e.flags |= flagDraggable | flagHover
	e.st.dataSource = &dataSource{data, o}
	return e
}

// DropData makes e take native transfers matching options, and reports a
// completed drop once, in the next frame. Decode custom formats explicitly
// from the returned Data. Empty Formats accepts nothing.
func coreDropData(e *node, options transfer.DropOptions) (transfer.Drop, bool) {
	e.st.dataTarget = &options
	e.c.rt.noteTarget()
	if d := e.st.dataDropped; d != nil {
		e.c.rt.consumed = true
		return *d, true
	}
	return transfer.Drop{}, false
}

// DataDragOver makes e a native destination, as DropData does, and returns
// the advertised formats and operations while a matching drag is over it.
// Hover never invokes a data provider.
func coreDataDragOver(e *node, options transfer.DropOptions) (transfer.Offer, bool) {
	e.st.dataTarget = &options
	e.c.rt.noteTarget()
	rt := e.c.rt
	if rt.dataOver == e.id && rt.incoming != nil {
		offer := rt.incoming.Offer
		offer.Formats = slices.Clone(offer.Formats)
		return offer, true
	}
	return transfer.Offer{}, false
}

func (rt *engine) dataTargetAt(x, y float32, d *platform.DataDragEvent) *state {
	d.Operation, d.Formats = transfer.None, nil
	for _, id := range rt.hitChain(x, y) {
		s := rt.states[id]
		if s == nil || s.flags&flagDisabled != 0 {
			continue
		}
		if s.accepts != nil && d.Local != nil && s.accepts(d.Local) {
			d.Operation = transfer.Negotiate(d.Offer.Operations, transfer.Copy, d.Offer.Suggested)
			if d.Operation != transfer.None {
				return s
			}
		}
		if o := s.dataTarget; o != nil {
			for _, f := range o.Formats {
				if f != platform.DragSessionFormat && slices.Contains(d.Offer.Formats, f) && !slices.Contains(d.Formats, f) {
					d.Formats = append(d.Formats, f)
				}
			}
			d.Operation = transfer.Negotiate(d.Offer.Operations, o.Operations, d.Offer.Suggested)
			if len(d.Formats) > 0 && d.Operation != transfer.None {
				return s
			}
			d.Formats = nil
		}
	}
	d.Operation = transfer.None
	return nil
}

func (rt *engine) dataDragOver(x, y float32, d *platform.DataDragEvent) bool {
	if d == nil {
		return false
	}
	over := uint64(0)
	if s := rt.dataTargetAt(x, y, d); s != nil {
		over = s.id
	}
	copy := *d
	copy.Offer.Formats = slices.Clone(d.Offer.Formats)
	rt.incoming, rt.dataOver = &copy, over
	rt.pointerX, rt.pointerY = x, y
	rt.requestFrame()
	return over != 0
}

func (rt *engine) clearDataOver() {
	if rt.incoming != nil {
		rt.incoming, rt.dataOver = nil, 0
		if !rt.closed {
			rt.requestFrame()
		}
	}
}

func (rt *engine) dataDrop(x, y float32, d *platform.DataDragEvent) bool {
	rt.clearDataOver()
	if d == nil {
		return false
	}
	s := rt.dataTargetAt(x, y, d)
	if s == nil {
		return false
	}
	if s.accepts != nil && d.Local != nil && s.accepts(d.Local) {
		s.droppedValue, s.hasDropped = d.Local, true
		s.dropX, s.dropY = x, y
	} else {
		data, err := d.Data.Materialize(d.Formats)
		if err != nil {
			d.Operation = transfer.None
			return false
		}
		s.dataDropped = &transfer.Drop{Data: data, Operation: d.Operation}
	}
	rt.requestFrame()
	return true
}

// noteTarget notes that the pass made an element a native drop target.
func (rt *engine) noteTarget() {
	rt.targetsBuilt.frame, rt.targetsBuilt.pass = rt.frame, rt.pass
}

func (rt *engine) syncDropFormats() {
	formats := append(rt.dropScratch[:0], transfer.FileList, transfer.URIList, platform.DragSessionFormat)
	// Only a pass that made an element a native drop target (noteTarget)
	// has formats to add; its states are those it built.
	if t := rt.targetsBuilt; t.frame == rt.frame && t.pass == rt.pass {
		for _, s := range rt.built {
			if s.dataTarget == nil || s.flags&flagDisabled != 0 || s.seen != rt.frame || s.pass != rt.pass {
				continue
			}
			for _, f := range s.dataTarget.Formats {
				if !slices.Contains(formats, f) {
					formats = append(formats, f)
				}
			}
		}
	}
	// Stable ordering prevents native registration on unrelated frames.
	slices.Sort(formats)
	if !slices.Equal(formats, rt.dropFormats) {
		rt.dropScratch, rt.dropFormats = rt.dropFormats, formats
		rt.host.setDropFormats(formats)
	} else {
		rt.dropScratch = formats
	}
}

// dataPreview snapshots the last scene, resampling the source's visible
// bounds to the window's DIPs. No GPU readback or native API is needed.
func (rt *engine) dataPreview(s *state) (image.Image, image.Point) {
	_, _, scale := rt.size()
	z := rt.zoomFactor()
	w, h := min(int(s.vw*z), 512), min(int(s.vh*z), 512)
	if w <= 0 || h <= 0 {
		return nil, image.Point{}
	}
	var renderer raster.Renderer
	renderer.Render(&rt.scene)
	m := &renderer.Image
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := int((s.vx+float32(x)/z)*scale), int((s.vy+float32(y)/z)*scale)
			if px < 0 || py < 0 || px >= m.W || py >= m.H {
				continue
			}
			i, j := (py*m.W+px)*4, (y*w+x)*4
			img.Pix[j], img.Pix[j+1], img.Pix[j+2], img.Pix[j+3] = m.Pix[i+2], m.Pix[i+1], m.Pix[i], m.Pix[i+3]
		}
	}
	x, y := s.world.Point(s.x+s.pressX, s.y+s.pressY)
	return img, image.Pt(min(max(int((x-s.vx)*z), 0), w-1), min(max(int((y-s.vy)*z), 0), h-1))
}

// DataDragInfo is a native drag over the window: where the pointer is, in
// the window's DIPs, what the drag offers (its formats, operations and
// the one the keys held suggest), and Local, the value an element of this
// process drags with it (Drag), nil from another app.
type DataDragInfo struct {
	X, Y  float32
	Offer transfer.Offer
	Local any
}

// DataDrag reports the native drag over the window, wherever it is (over
// an element that takes it or not), false when none is: a view follows a
// drag of its own across regions, as a sidebar it comes over.
func (c *context) DataDrag() (DataDragInfo, bool) {
	rt := c.rt
	if rt == nil || rt.incoming == nil {
		return DataDragInfo{}, false
	}
	in := rt.incoming
	offer := in.Offer
	offer.Formats = slices.Clone(offer.Formats)
	return DataDragInfo{X: rt.pointerX, Y: rt.pointerY, Offer: offer, Local: in.Local}, true
}

// ScaleDataDrag shows the preview of the native drag an element of this
// window started (DragData) at scale of its size, 1 its own, keeping the
// point pressed under the pointer: a drag made small over a list it
// drops into. It does nothing without such a drag or a preview, and on
// platforms that cannot change a drag's preview (all but macOS); scale
// is kept between 0.05 and 1.
func (c *context) ScaleDataDrag(scale float32) {
	rt := c.rt
	if rt == nil {
		return
	}
	d := rt.drag
	if d == nil || !d.native || d.canceled || d.preview == nil {
		return
	}
	scale = min(max(scale, 0.05), 1)
	was := d.scale
	if was == 0 {
		was = 1
	}
	if scale == was {
		return
	}
	d.scale = scale
	img, hot := scalePreview(d.preview, d.hot, scale)
	if img != nil {
		rt.host.setDataDragPreview(img, hot)
	}
}

// scalePreview is img at scale of its size, each pixel the mean of those
// under it, with hot moved with it.
func scalePreview(img image.Image, hot image.Point, scale float32) (image.Image, image.Point) {
	b := img.Bounds()
	if scale == 1 {
		return img, hot
	}
	sw, sh := b.Dx(), b.Dy()
	w, h := max(int(float32(sw)*scale+0.5), 1), max(int(float32(sh)*scale+0.5), 1)
	if sw <= 0 || sh <= 0 {
		return nil, image.Point{}
	}
	src, ok := img.(*image.RGBA)
	if !ok || src.Rect.Min != (image.Point{}) {
		src = image.NewRGBA(image.Rect(0, 0, sw, sh))
		draw.Draw(src, src.Rect, img, b.Min, draw.Src)
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0, y1 := y*sh/h, max((y+1)*sh/h, y*sh/h+1)
		for x := 0; x < w; x++ {
			x0, x1 := x*sw/w, max((x+1)*sw/w, x*sw/w+1)
			var r, g, bl, a, n uint32
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					p := row[sx*4 : sx*4+4]
					r, g, bl, a = r+uint32(p[0]), g+uint32(p[1]), bl+uint32(p[2]), a+uint32(p[3])
					n++
				}
			}
			o := out.Pix[y*out.Stride+x*4:]
			o[0], o[1], o[2], o[3] = uint8(r/n), uint8(g/n), uint8(bl/n), uint8(a/n)
		}
	}
	hx := min(max(int(float32(hot.X)*float32(w)/float32(sw)), 0), w-1)
	hy := min(max(int(float32(hot.Y)*float32(h)/float32(sh)), 0), h-1)
	return out, image.Pt(hx, hy)
}
