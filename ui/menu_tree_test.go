package ui

import (
	"reflect"
	"testing"
)

// TestMenuTreeAndTidySeparators: a menu shows its items' shortcuts and
// states (MenuTree); separators first, last, after another and empty
// submenus are left out, in the system's menus and in drawn ones, and a
// choice still finds its item.
func TestMenuTreeAndTidySeparators(t *testing.T) {
	for _, drawn := range []bool{false, true} {
		chosen := ""
		tt := NewTester(func(c *Context) {
			build := func(m *Menu) {
				m.Separator()
				if m.Item("Copy").Shortcut(Cmd, KeyC).Chosen() {
					chosen = "copy"
				}
				m.Separator()
				m.Separator()
				m.Submenu("Empty", func(sub *Menu) { sub.Separator() })
				m.Submenu("More", func(sub *Menu) {
					if sub.Item("Rename").Chosen() {
						chosen = "rename"
					}
					sub.Separator()
				})
				m.Item("Pinned").Checked(true)
				m.Separator()
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
			{Label: "Copy", Shortcut: accelerator(Cmd, KeyC), Enabled: true},
			{Separator: true, Enabled: true},
			{Label: "More", Enabled: true, Submenu: []MenuEntry{{Label: "Rename", Enabled: true}}},
			{Label: "Pinned", Enabled: true, Checked: true},
		}
		if got := tt.MenuTree(); !reflect.DeepEqual(got, want) {
			t.Fatalf("drawn %v: menu %+v\nwant %+v", drawn, got, want)
		}
		if err := tt.ChooseMenuItem("More", "Rename"); err != nil {
			t.Fatalf("drawn %v: %v", drawn, err)
		}
		tt.Frame()
		if chosen != "rename" {
			t.Fatalf("drawn %v: chose %q", drawn, chosen)
		}
		tt.rt.close()
	}
}
