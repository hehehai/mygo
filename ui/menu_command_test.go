package ui

import "testing"

// TestMenuItemCommand: an item given Command does the editing command on
// the text input the menu opened on, as EditItems' items do, and Chosen
// does not report it.
func TestMenuItemCommand(t *testing.T) {
	text := "Untitled"
	chosen := 0
	tt := NewTester(func(c *Context) {
		TextInput(c, &text).Label("Name").ContextMenu(func(m *Menu) {
			if m.Item("Choose all of it").Command("selectAll").Chosen() {
				chosen++
			}
		})
	}, 300, 80)
	r, _ := tt.Find("Untitled")
	tt.ClickAt(r.X+2, r.Y+r.H/2)
	tt.RightClickAt(r.X+2, r.Y+r.H/2)
	if err := tt.ChooseMenuItem("Choose all of it"); err != nil {
		t.Fatalf("%v; menu %q", err, tt.Menu())
	}
	tt.Frame()
	tt.Type("Brand")
	tt.Frame()
	if text != "Brand" || chosen != 0 {
		t.Fatalf("text %q, chosen %d", text, chosen)
	}
}
