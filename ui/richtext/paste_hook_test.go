package richtext

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// TestPasteHook: EditorOptions.OnPaste is asked about files and pictures
// pasted, with every format offered; taken, nothing else is pasted; not
// taken, the paste goes on as without it. Text, HTML and Paste and Match
// Style never ask it.
func TestPasteHook(t *testing.T) {
	doc := NewDocument(Text{})
	var asked []transfer.Data
	var offered [][]transfer.Format
	take := true
	tt := ui.NewTester(func(c *ui.Context) {
		Editor(c, doc, EditorOptions{OnPaste: func(clip transfer.Data, f []transfer.Format) bool {
			asked, offered = append(asked, clip), append(offered, f)
			return take
		}}).Fill()
	}, 500, 400)
	tt.Press(250, 200)
	tt.Release(250, 200)
	paste := func(d transfer.Data) {
		tt.SetClipboardData(d)
		tt.Frame()
		tt.Key(ui.Cmd, ui.KeyV)
		tt.Frame()
	}
	// Files: taken, the text stays empty.
	files, err := transfer.FileData("/tmp/report.pdf", "/tmp/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	paste(files)
	if len(asked) != 1 || doc.Text().Len() != 0 {
		t.Fatalf("files: asked %d, text %q", len(asked), doc.Text().String())
	}
	if got, _ := asked[0].Files(); !slices.Equal(got, []string{"/tmp/report.pdf", "/tmp/chart.png"}) {
		t.Errorf("files given %v", got)
	}
	// A picture (with its HTML, as a browser copies one): asked, with the
	// HTML among what is offered; not taken, the HTML pastes.
	take = false
	paste(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte("\\x89PNG")), transfer.Bytes(transfer.HTML, []byte("<b>hi</b>")))))
	if len(asked) != 2 || !slices.Contains(asked[1].Formats(), transfer.PNG) || !slices.Contains(offered[1], transfer.HTML) {
		t.Fatalf("picture: asked %d, formats %v, offered %v", len(asked), asked[len(asked)-1].Formats(), offered[len(offered)-1])
	}
	if got := runsOf(doc.Text()); got != "*hi*" {
		t.Errorf("not taken: pasted %q", got)
	}
	// Text and HTML alone: not asked.
	paste(transfer.New(transfer.NewItem(transfer.Bytes(transfer.HTML, []byte("<i>x</i>")), transfer.Bytes(transfer.Text, []byte("x")))))
	paste(transfer.TextData("plain"))
	if len(asked) != 2 {
		t.Errorf("asked about text: %d", len(asked))
	}
	// Paste and Match Style: never asked.
	take = true
	tt.SetClipboardData(files)
	tt.Frame()
	tt.Command("pasteAndMatchStyle")
	if len(asked) != 2 {
		t.Errorf("asked on Paste and Match Style: %d", len(asked))
	}
}
