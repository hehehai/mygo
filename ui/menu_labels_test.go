package ui

import (
	"reflect"
	"strings"
	"testing"
)

// TestSetMenuLabels: every label of a view's menu, its submenus' too,
// shows as the function gives it, in the system's menus and drawn ones,
// and the item chosen is still told.
func TestSetMenuLabels(t *testing.T) {
	SetMenuLabels(func(s string) string { return strings.TrimSuffix(s, "…") })
	defer SetMenuLabels(nil)
	for _, drawn := range []bool{false, true} {
		chosen := ""
		tt := NewTester(func(c *Context) {
			build := func(m *Menu) {
				if m.Item("Rename…").Chosen() {
					chosen = "rename"
				}
				m.Separator()
				m.Submenu("Move to…", func(sub *Menu) {
					if sub.Item("Trash").Chosen() {
						chosen = "trash"
					}
				})
			}
			if drawn {
				Box(c).Size(100, 100).PopoverMenu(build)
			} else {
				Box(c).Size(100, 100).ContextMenu(build)
			}
		}, 100, 100)
		tt.Frame()
		tt.RightClickAt(50, 50)
		tt.Frame()
		want := []MenuEntry{
			{Label: "Rename", Enabled: true},
			{Separator: true, Enabled: true},
			{Label: "Move to", Enabled: true, Submenu: []MenuEntry{{Label: "Trash", Enabled: true}}},
		}
		if got := tt.MenuTree(); !reflect.DeepEqual(got, want) {
			t.Fatalf("drawn %v: menu %+v\nwant %+v", drawn, got, want)
		}
		if err := tt.ChooseMenuItem("Move to", "Trash"); err != nil {
			t.Fatalf("drawn %v: %v", drawn, err)
		}
		tt.Frame()
		if chosen != "trash" {
			t.Fatalf("drawn %v: chose %q", drawn, chosen)
		}
		tt.rt.close()
	}
}
