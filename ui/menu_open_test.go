package ui

import (
	"reflect"
	"testing"
)

func TestOpenContextMenu(t *testing.T) {
	var (
		anchor, plain Handle
		chosen        string
		hide, inBuild bool
	)
	tt := NewTester(func(c *Context) {
		Column(c).Fill().Children(func() {
			if !hide {
				// No room of its own: only code opens its menu.
				Box(c).Size(0, 0).Bind(&anchor).ContextMenu(func(m *Menu) {
					if m.Item("Copy image").Chosen() {
						chosen = "copy"
					}
				})
			}
			Box(c).Size(50, 50).Bind(&plain)
			if inBuild {
				// Asked for as the view builds, after the control.
				inBuild = false
				if !anchor.OpenContextMenu(c, 10, 20) {
					t.Error("did not open the menu as the view builds")
				}
			}
		})
	}, 200, 200)
	defer tt.rt.close()
	tt.Frame()
	c := tt.rt.public
	if plain.OpenContextMenu(c, 10, 10) {
		t.Fatal("opened the menu of a control without one")
	}
	if !anchor.OpenContextMenu(c, 120, 80) {
		t.Fatal("did not open the menu")
	}
	tt.Frame()
	if got, want := tt.MenuTree(), []MenuEntry{{Label: "Copy image", Enabled: true}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("menu %+v, want %+v", got, want)
	}
	if at := tt.h.menuAt; at != [2]float32{120, 80} {
		t.Errorf("the menu opened at %v, want 120,80", at)
	}
	if err := tt.ChooseMenuItem("Copy image"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if chosen != "copy" {
		t.Fatalf("chose %q", chosen)
	}
	// Asked for while the view builds, after the control built.
	tt.CloseMenu()
	inBuild = true
	tt.Frame()
	tt.Frame()
	if at := tt.h.menuAt; tt.Menu() == nil || at != [2]float32{10, 20} {
		t.Errorf("asked for as the view builds: menu %q at %v", tt.Menu(), at)
	}
	tt.CloseMenu()
	hide = true
	tt.Frame()
	tt.Frame()
	if anchor.OpenContextMenu(c, 1, 1) {
		t.Error("opened the menu of a control gone")
	}
}
