package ui

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestSidebar(t *testing.T) {
	place, changes := "recents", 0
	locations := true
	tt := coreNewTester(func(c *context) {
		coreRow(c).Fill().AlignItems(Stretch).Children(func() {
			if coreSidebar(c, &place, func() {
				coreSidebarSection(c, "Favorites", nil, func() {
					coreSidebarItem(c, "recents", nil, "Recents")
					coreSidebarItem(c, "desktop", nil, "Desktop").Children(func() { coreBadge(c, "3") })
					coreSidebarItem(c, "documents", nil, "Documents")
				})
				coreSidebarSection(c, "Locations", &locations, func() {
					coreSidebarItem(c, "mac", nil, "Macintosh HD")
				})
			}).Width(200).Changed() {
				changes++
			}
			coreButton(c, "Other")
		})
	}, 500, 400)
	tt.SetPreferences(Preferences{ReduceMotion: true, TextScale: 1})
	tt.Click("Desktop")
	if place != "desktop" || changes != 1 {
		t.Fatalf("a click: %q (%d changes)", place, changes)
	}
	// The keys choose among the items, across sections.
	for _, step := range []struct {
		key  Key
		want string
	}{{KeyDown, "documents"}, {KeyDown, "mac"}, {KeyDown, "mac"}, {KeyUp, "documents"}, {KeyHome, "recents"}, {KeyEnd, "mac"}} {
		tt.Key(0, step.key)
		if place != step.want {
			t.Fatalf("%v chose %q", step.key, place)
		}
	}
	// Typing the first letters.
	tt.Key(0, KeyD)
	if place != "desktop" {
		t.Fatalf("d chose %q", place)
	}
	tt.Key(0, KeyO)
	if place != "documents" {
		t.Fatalf("do chose %q", place)
	}
	// One stop of Tab.
	tt.Key(0, KeyTab)
	if !tt.Focused("Other") {
		t.Fatal("Tab did not leave the sidebar")
	}
	tt.Key(Shift, KeyTab)
	// A click on a section's title hides its items, which the keys skip.
	tt.Click("Locations")
	if locations || tt.HasText("Macintosh HD") {
		t.Fatal("a click on the title left the section open")
	}
	tt.Key(0, KeyEnd)
	if place != "documents" {
		t.Errorf("End with Locations hidden chose %q", place)
	}
	// Assistive technology: a tree whose item chosen has the focus.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tree := tt.h.access
	side := accessNode(t, tree, platform.RoleTree, "")
	docs := accessNode(t, tree, platform.RoleTreeItem, "Documents")
	if side.States&platform.AccessSelectable == 0 || docs.Level != 2 || docs.States&platform.AccessChecked == 0 || tree.Focus != docs.ID {
		t.Errorf("the sidebar %+v, Documents %+v, the focus on %d", side, docs, tree.Focus)
	}
	if n := accessNode(t, tree, platform.RoleTreeItem, "Desktop"); n.Label != "Desktop 3" || n.States&platform.AccessChecked != 0 {
		t.Errorf("Desktop: %+v", n)
	}
	head := accessNode(t, tree, platform.RoleTreeItem, "Locations")
	if head.States&(platform.AccessExpandable|platform.AccessExpanded) != platform.AccessExpandable || head.Level != 1 {
		t.Errorf("Locations: %+v", head)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessAction, ID: head.ID, Action: platform.AccessExpand})
	if !locations || !tt.HasText("Macintosh HD") {
		t.Error("expanding Locations")
	}
	// Focusing an item chooses it.
	mac := accessNode(t, tt.h.access, platform.RoleTreeItem, "Macintosh")
	tt.send(platform.SurfaceEvent{Kind: platform.AccessAction, ID: mac.ID, Action: platform.AccessFocus})
	if place != "mac" {
		t.Errorf("focusing Macintosh HD chose %q", place)
	}
}

// Items out of view, added without being built (SidebarEntry), are
// chosen by the keys and typing as the items built are, in their order.
func TestSidebarEntry(t *testing.T) {
	place := "a"
	names := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}
	ids := []string{"a", "b", "g", "d", "e"}
	built := map[string]bool{}
	tt := coreNewTester(func(c *context) {
		clear(built)
		coreSidebar(c, &place, func() {
			coreSidebarSection(c, "Places", nil, func() {
				for i, id := range ids {
					// Only the first item and the one chosen are built.
					if i == 0 || id == place {
						coreSidebarItem(c, id, nil, names[i])
						built[id] = true
					} else {
						coreSidebarEntry(c, id, names[i])
					}
				}
			})
		}).Width(200)
		coreSidebarEntry(c, "outside", "Outside") // not in a sidebar: nothing
	}, 400, 300)
	tt.Click("Alpha")
	for _, step := range []struct {
		key  Key
		want string
	}{{KeyDown, "b"}, {KeyDown, "g"}, {KeyUp, "b"}, {KeyEnd, "e"}, {KeyHome, "a"}} {
		tt.Key(0, step.key)
		if place != step.want || !built[place] {
			t.Fatalf("%v chose %q (built %v)", step.key, place, built)
		}
	}
	tt.Key(0, KeyD)
	if place != "d" || !built["d"] || built["g"] {
		t.Fatalf("d chose %q (built %v)", place, built)
	}
	if tt.HasText("Gamma") || !tt.HasText("Delta") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

// A click with Command or Shift on an item that ExtendsChoice leaves the
// choice to the app, which reads the click; on another item, or without
// them, it chooses as before.
func TestSidebarExtendsChoice(t *testing.T) {
	place, changes := "a", 0
	var extended []string
	tt := coreNewTester(func(c *context) {
		if coreSidebar(c, &place, func() {
			coreSidebarSection(c, "Places", nil, func() {
				for _, id := range []string{"a", "b", "c"} {
					it := coreSidebarItem(c, id, nil, "Item "+id).ExtendsChoice()
					if it.Clicked() && it.ClickModifiers()&(Cmd|Shift) != 0 {
						extended = append(extended, id)
					}
				}
				coreSidebarItem(c, "d", nil, "Item d")
			})
		}).Width(200).Changed() {
			changes++
		}
	}, 400, 300)
	tt.ClickWith(Cmd, "Item b")
	tt.ClickWith(Shift, "Item c")
	if place != "a" || changes != 0 || len(extended) != 2 || extended[0] != "b" || extended[1] != "c" {
		t.Fatalf("⌘ and ⇧ clicks: %q, %d changes, extended %q", place, changes, extended)
	}
	tt.Click("Item b")
	if place != "b" || changes != 1 {
		t.Fatalf("a click: %q, %d changes", place, changes)
	}
	tt.ClickWith(Cmd, "Item d")
	if place != "d" || changes != 2 {
		t.Fatalf("⌘-click on an item that does not extend: %q, %d changes", place, changes)
	}
}
