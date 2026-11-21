package richtext

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestImageResize: a picture selected alone shows a handle at each
// corner; a drag of the bottom right one keeps its aspect ratio, and the
// size it ends at is one step of undo.
func TestImageResize(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for x := range 200 {
		img.Set(x, 0, color.RGBA{255, 0, 0, 255})
	}
	bm := ui.NewBitmap(img)
	doc := NewDocument(TextOf(Paragraph{Runs: []Run{
		{Text: "a"},
		{Text: string(ObjectReplacement), Attributes: Attributes{Attachment: &Attachment{Image: bm, Width: 100, Height: 50, URL: "cid:x"}}},
		{Text: " b"},
	}}))
	tt, _ := richTester(t, doc)
	doc.SetSelection(1, 2)
	tt.Frame()
	box, ok := doc.AttachmentRect(1)
	if !ok || box.W != 100 || box.H != 50 {
		t.Fatalf("box %+v %v", box, ok)
	}
	// A press on the bottom right corner's handle resizes; one beside it
	// selects text as before.
	x, y := box.X+box.W, box.Y+box.H
	tt.Press(x, y)
	tt.Move(x+40, y+3)
	tt.Frame()
	if box, _ := doc.AttachmentRect(1); box.W < 139 || box.W > 141 {
		t.Fatalf("dragging: box %+v", box)
	}
	tt.Release(x+40, y+3)
	tt.Frame()
	at := doc.Text().AttributesAt(1).Attachment
	if at == nil || at.Width < 139 || at.Width > 141 || abs32(at.Height-at.Width/2) > 0.01 {
		t.Fatalf("resized to %+v", at)
	}
	if a, z := doc.Selection(); a != 1 || z != 2 {
		t.Fatalf("selection %d–%d", a, z)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if at := doc.Text().AttributesAt(1).Attachment; at.Width != 100 || at.Height != 50 {
		t.Fatalf("undo: %+v", at)
	}
}

// TestImageSelectedByPress: a press on a picture selects it alone, so its
// handles show and it can be resized.
func TestImageSelectedByPress(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	doc := NewDocument(TextOf(Paragraph{Runs: []Run{
		{Text: "a"},
		{Text: string(ObjectReplacement), Attributes: Attributes{Attachment: &Attachment{Image: ui.NewBitmap(img), Width: 100, Height: 50}}},
		{Text: " b"},
	}}))
	tt, _ := richTester(t, doc)
	tt.Frame()
	box, ok := doc.AttachmentRect(1)
	if !ok {
		t.Fatal("picture not painted")
	}
	tt.Press(box.X+box.W/2, box.Y+box.H/2)
	tt.Release(box.X+box.W/2, box.Y+box.H/2)
	tt.Frame()
	if a, z := doc.Selection(); a != 1 || z != 2 {
		t.Fatalf("pressed picture selected %d–%d", a, z)
	}
}
