package richtext

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// TestEditorMenuCommands: a right-click on a rich text editor shows the
// editing items of a text field; an app's EditorOptions.Menu labels and
// orders them its own way with MenuItem.Command, which runs the editor's
// commands (Paste and Match Style, Undo too) on the text, and keeps the
// selection the right-click found.
func TestEditorMenuCommands(t *testing.T) {
	doc := NewDocument(NewText("hello world"))
	app := false
	aiChosen := 0
	tt := ui.NewTester(func(c *ui.Context) {
		var opts EditorOptions
		if app {
			opts.Menu = func(m *ui.Menu) {
				m.Item("Alles auswählen").Command("selectAll")
				m.Item("Als reiner Text einfügen").Command("pasteAndMatchStyle")
				m.Item("Widerrufen").Disabled(!doc.CanUndo()).Command("undo")
				m.Separator()
				if m.Item("Make it shorter").Chosen() {
					aiChosen++
				}
			}
		}
		Editor(c, doc, opts).Fill()
	}, 500, 400)
	tt.RightClickAt(250, 200)
	if !slices.Contains(tt.Menu(), "Copy") || !slices.Contains(tt.Menu(), "Select All") {
		t.Fatalf("items %q", tt.Menu())
	}
	tt.CloseMenu()
	app = true
	tt.Frame()
	tt.RightClickAt(250, 200)
	if err := tt.ChooseMenuItem("Alles auswählen"); err != nil {
		t.Fatalf("%v; menu %q", err, tt.Menu())
	}
	tt.Frame()
	if a, z := doc.Selection(); a != 0 || z != 11 {
		t.Fatalf("select all: %d..%d", a, z)
	}
	// Pasted as plain text over the selection the right-click kept.
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.HTML, []byte("<b>bold</b>")), transfer.Bytes(transfer.Text, []byte("plain")))))
	tt.RightClickAt(100, 100)
	if err := tt.ChooseMenuItem("Als reiner Text einfügen"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := runsOf(doc.Text()); got != "plain" {
		t.Fatalf("pasted as plain text: %q", got)
	}
	tt.RightClickAt(100, 100)
	if err := tt.ChooseMenuItem("Widerrufen"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := doc.Text().String(); got != "hello world" {
		t.Fatalf("undone: %q", got)
	}
	// An item of the app's own reports its choice.
	tt.RightClickAt(100, 100)
	if err := tt.ChooseMenuItem("Make it shorter"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if aiChosen != 1 {
		t.Fatalf("the app's item chosen %d times", aiChosen)
	}
}
