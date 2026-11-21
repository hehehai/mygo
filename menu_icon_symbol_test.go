//go:build !js

package mygo

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/ui"
)

// TestMenuIconSymbolAndScaled: a symbol's icon and a scaled image's carry
// what the backends show, are shared by the items that show the same, and
// reach a view's menu (Tester.MenuTree) and a content menu's items.
func TestMenuIconSymbolAndScaled(t *testing.T) {
	copyIcon := NewMenuIconSymbol("doc.on.doc")
	if p := copyIcon.PlatformMenuIcon(); p.Symbol != "doc.on.doc" || len(p.PNG) != 0 || !p.Template || p.Scale != 0 {
		t.Fatalf("symbol icon %+v", p)
	}
	if NewMenuIconSymbol("doc.on.doc").PlatformMenuIcon() != copyIcon.PlatformMenuIcon() {
		t.Error("two icons of one symbol do not share it")
	}
	if NewMenuIconSymbol("trash").PlatformMenuIcon() == copyIcon.PlatformMenuIcon() {
		t.Error("icons of two symbols share one")
	}
	stars := flag(t, 160, 28, color.NRGBA{0, 0, 0, 255}) // five stars, 80 by 14 points at 2x
	starsIcon, err := NewMenuIconScaled(stars, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if p := starsIcon.PlatformMenuIcon(); p.Scale != 2 || !p.Template || p.Symbol != "" {
		t.Fatalf("scaled icon %+v", p)
	}
	fitted, err := NewMenuIcon(stars, true)
	if err != nil {
		t.Fatal(err)
	}
	if fitted.PlatformMenuIcon() == starsIcon.PlatformMenuIcon() || fitted.PlatformMenuIcon().Scale != 0 {
		t.Error("a scaled icon and a fitted one of the same image share one")
	}
	for _, sc := range []float64{0, -1} {
		if _, err := NewMenuIconScaled(stars, sc, true); err == nil {
			t.Errorf("scale %v: no error", sc)
		}
	}
	if _, err := NewMenuIconScaled([]byte("not a png"), 2, true); err == nil {
		t.Error("not a PNG: no error")
	}

	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(100, 100).ContextMenu(func(m *ui.Menu) {
			m.Item("Copy").Icon(copyIcon)
			m.Submenu("Rating", func(sub *ui.Menu) {
				sub.Item("3 stars").Icon(starsIcon)
			}).Icon(NewMenuIconSymbol("star"))
		})
	}, 100, 100)
	tt.Frame()
	tt.RightClickAt(50, 50)
	tt.Frame()
	want := []ui.MenuEntry{
		{Label: "Copy", Symbol: "doc.on.doc", Template: true, Enabled: true},
		{Label: "Rating", Symbol: "star", Template: true, Enabled: true, Submenu: []ui.MenuEntry{
			{Label: "3 stars", Image: starsIcon.PlatformMenuIcon().PNG, Template: true, Enabled: true},
		}},
	}
	if got := tt.MenuTree(); !reflect.DeepEqual(got, want) {
		t.Errorf("menu %+v\nwant %+v", got, want)
	}

	// A view's menu goes through Menu on its way to the backend.
	pm := &platform.Menu{Items: []*platform.MenuItem{
		{ID: 1, Label: "Copy", Icon: copyIcon.PlatformMenuIcon(), Enabled: true, Visible: true},
		{ID: 2, Label: "3 stars", Icon: starsIcon.PlatformMenuIcon(), Enabled: true, Visible: true},
	}}
	snap := NewMenu(contentMenu(pm, func(int) {})).snapshot()
	if snap.Items[0].Icon != copyIcon.PlatformMenuIcon() || snap.Items[1].Icon != starsIcon.PlatformMenuIcon() {
		t.Errorf("content menu icons %+v %+v", snap.Items[0].Icon, snap.Items[1].Icon)
	}
}
