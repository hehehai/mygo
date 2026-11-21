package ui

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/transfer"
)

type localValue struct{ name string }

// TestDraggedDataBack: a drag an element of the window started (DragData
// with Drag) and dropped back on the window comes with its value: an
// element taking the value gets it, not the data, and an element taking
// only the data gets the data; DataDrag follows it wherever it is.
func TestDraggedDataBack(t *testing.T) {
	var value *localValue
	var data []transfer.Drop
	var info DataDragInfo
	var dragging bool
	var results []transfer.Result
	v := &localValue{"a"}
	tt := coreNewTester(func(c *context) {
		coreBox(c).Size(80, 40).Drag(v).DragData(transfer.TextData("out"), transfer.DragOptions{Done: func(r transfer.Result) { results = append(results, r) }})
		opts := transfer.DropOptions{Formats: []transfer.Format{transfer.Text}, Operations: transfer.Copy | transfer.Move}
		typed := coreBox(c).Size(100, 50)
		if d, ok := coreDrop[*localValue](typed); ok {
			value = d
		}
		files := coreBox(c).Size(100, 50)
		if d, ok := coreDropData(files, opts); ok {
			data = append(data, d)
		}
		info, dragging = c.DataDrag()
	}, 200, 200)
	tt.Press(10, 10)
	tt.Move(20, 20)
	if _, ok := tt.DraggedData(); !ok {
		t.Fatal("no drag")
	}
	if tt.DragDraggedDataOver(150, 150, transfer.None) { // over nothing that takes it, but followed
		t.Fatal("taken over nothing")
	}
	if !dragging || info.X != 150 || info.Y != 150 || info.Local != v {
		t.Fatalf("DataDrag %+v %v", info, dragging)
	}
	if !tt.DropDraggedDataAt(20, 60, transfer.Copy) {
		t.Fatal("the value's target refused it")
	}
	tt.Frame()
	if value != v || len(data) != 0 {
		t.Fatalf("value %v data %v", value, data)
	}
	if len(results) != 1 || results[0].Operation != transfer.Copy {
		t.Fatalf("results %+v", results)
	}
	tt.Frame()
	if dragging {
		t.Fatal("DataDrag after the drop")
	}
	tt.Press(10, 10)
	tt.Move(20, 20)
	value = nil
	if !tt.DropDraggedDataAt(20, 110, transfer.None) {
		t.Fatal("the data's target refused it")
	}
	tt.Frame()
	if value != nil || len(data) != 1 {
		t.Fatalf("value %v data %v", value, data)
	}
	if b, _ := data[0].Data.Read(transfer.Text); string(b) != "out" {
		t.Fatalf("dropped %q", b)
	}
}

// TestScaleDataDrag: the preview of a drag the window started is shown
// smaller and back, the point pressed kept under the pointer.
func TestScaleDataDrag(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 60))
	for i := range src.Pix {
		src.Pix[i] = 255
	}
	scale := float32(1)
	tt := coreNewTester(func(c *context) {
		coreBox(c).Size(100, 60).DragData(transfer.TextData("out"), transfer.DragOptions{Preview: src, Hotspot: image.Pt(50, 30)})
		c.ScaleDataDrag(scale)
	}, 200, 200)
	tt.Press(50, 30)
	tt.Move(60, 40)
	img, hot := tt.DraggedPreview()
	if img != image.Image(src) || hot != image.Pt(50, 30) {
		t.Fatalf("preview %v %v", img.Bounds(), hot)
	}
	scale = 0.5
	tt.Frame()
	img, hot = tt.DraggedPreview()
	if img.Bounds() != image.Rect(0, 0, 50, 30) || hot != image.Pt(25, 15) {
		t.Fatalf("half: %v %v", img.Bounds(), hot)
	}
	if c := color.RGBAModel.Convert(img.At(10, 10)).(color.RGBA); c.R != 255 || c.A != 255 {
		t.Fatalf("pixel %v", c)
	}
	scale = 1
	tt.Frame()
	if img, hot = tt.DraggedPreview(); img.Bounds() != src.Bounds() || hot != image.Pt(50, 30) {
		t.Fatalf("back: %v %v", img.Bounds(), hot)
	}
	tt.FinishDataDrag(transfer.Result{Canceled: true})
	if img, _ = tt.DraggedPreview(); img != nil {
		t.Fatal("a preview after the drag")
	}
	scale = 0.5
	tt.Frame() // no drag: nothing
}

// TestDropEffect: an element taking this process's value tells the effect
// of a drop there (DropEffect: a move, a copy with Option), which the
// platform shows and the source is told; None does not take it there,
// the element around taking the data does; without DropEffect it is a
// copy. Other apps are offered the source's Operations only, its own
// windows Local too.
func TestDropEffect(t *testing.T) {
	v := &localValue{"a"}
	var results []transfer.Result
	var dropped *localValue
	var outer []transfer.Drop
	tt := coreNewTester(func(c *context) {
		coreBox(c).Size(80, 40).Drag(v).DragData(transfer.TextData("out"), transfer.DragOptions{Local: transfer.Copy | transfer.Move, Done: func(r transfer.Result) { results = append(results, r) }})
		around := coreColumn(c).Size(200, 150).Children(func() {
			moves := coreBox(c).Size(100, 50)
			moves.DropEffect(func(_ any, o transfer.Offer) transfer.Operation {
				if o.Suggested == transfer.Copy {
					return transfer.Copy
				}
				return transfer.Move
			})
			if d, ok := coreDrop[*localValue](moves); ok {
				dropped = d
			}
			refuses := coreBox(c).Size(100, 50)
			refuses.DropEffect(func(any, transfer.Offer) transfer.Operation { return transfer.None })
			coreDrop[*localValue](refuses)
		})
		if d, ok := coreDropData(around, transfer.DropOptions{Formats: []transfer.Format{transfer.Text}}); ok {
			outer = append(outer, d)
		}
		plain := coreBox(c).Size(100, 50)
		coreDrop[*localValue](plain)
	}, 300, 300)
	tt.Press(10, 10)
	tt.Move(20, 20)
	if other, local := tt.DraggedOperations(); other != transfer.Copy || local != transfer.Copy|transfer.Move {
		t.Fatalf("offered other apps %v, this process %v", other, local)
	}
	// Inside around (y 40..190, taking the data): moves 40..90, refuses
	// 90..140; plain 190..240.
	if !tt.DragDraggedDataOver(20, 60, transfer.None) || tt.DragOperation() != transfer.Move {
		t.Fatalf("over the mover: %v", tt.DragOperation())
	}
	if !tt.DragDraggedDataOver(20, 60, transfer.Copy) || tt.DragOperation() != transfer.Copy {
		t.Fatalf("over the mover with Option: %v", tt.DragOperation())
	}
	if !tt.DragDraggedDataOver(20, 110, transfer.None) || tt.DragOperation() != transfer.Copy { // around's data
		t.Fatalf("over the refuser: %v", tt.DragOperation())
	}
	if !tt.DragDraggedDataOver(20, 160, transfer.None) || tt.DragOperation() != transfer.Copy { // around: its data, a copy
		t.Fatalf("over the data's target: %v", tt.DragOperation())
	}
	if !tt.DragDraggedDataOver(20, 210, transfer.None) || tt.DragOperation() != transfer.Copy {
		t.Fatalf("over the plain target: %v", tt.DragOperation())
	}
	if !tt.DropDraggedDataAt(20, 60, transfer.None) {
		t.Fatal("the mover refused it")
	}
	tt.Frame()
	if dropped != v || len(outer) != 0 || len(results) != 1 || results[0].Operation != transfer.Move {
		t.Fatalf("dropped %v outer %v results %+v", dropped, outer, results)
	}
	// Not taken by the refuser: the data's target around has it.
	tt.Press(10, 10)
	tt.Move(20, 20)
	if !tt.DropDraggedDataAt(20, 110, transfer.None) {
		t.Fatal("nothing took it")
	}
	tt.Frame()
	if len(outer) != 1 || len(results) != 2 || results[1].Operation != transfer.Copy {
		t.Fatalf("outer %v results %+v", outer, results)
	}
	// Over nothing that takes it: refused, the drag goes back.
	tt.Press(10, 10)
	tt.Move(20, 20)
	if tt.DropDraggedDataAt(250, 280, transfer.None) || tt.DragOperation() != transfer.None {
		t.Fatalf("taken over nothing: %v", tt.DragOperation())
	}
	if len(results) != 3 || !results[2].Canceled {
		t.Fatalf("results %+v", results)
	}
	// A source offering its windows only a copy: a wanted move is a copy.
	tt2 := coreNewTester(func(c *context) {
		coreBox(c).Size(80, 40).Drag(v).DragData(transfer.TextData("out"))
		moves := coreBox(c).Size(100, 50)
		moves.DropEffect(func(any, transfer.Offer) transfer.Operation { return transfer.Move })
		coreDrop[*localValue](moves)
	}, 300, 300)
	tt2.Press(10, 10)
	tt2.Move(20, 20)
	if other, local := tt2.DraggedOperations(); other != transfer.Copy || local != transfer.Copy {
		t.Fatalf("default offers %v %v", other, local)
	}
	if !tt2.DragDraggedDataOver(20, 60, transfer.None) || tt2.DragOperation() != transfer.Copy {
		t.Fatalf("copy-only source: %v", tt2.DragOperation())
	}
}
