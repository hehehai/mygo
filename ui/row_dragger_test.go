package ui

import (
	"fmt"
	"slices"
	"testing"
)

// A Table's cell that only drags a value (as a file's name dragged out to
// Finder) leaves its row the press: a click on it chooses the row, Cmd and
// Shift as on the rest of the row, a double click submits; a press moving
// drags the value, not the rows (DragSelect); elsewhere in the row a drag
// still chooses the rows it goes over.
func TestRowAroundDragger(t *testing.T) {
	s := &ListState{DragSelect: true}
	sel := &Selection[int]{}
	s.Selection = sel
	var dropped []string
	submitted, clicks := 0, 0
	tt := NewTester(func(c *Context) {
		Column(c).Fill().Children(func() {
			tb := Table(c, s, []TableColumn{{Title: "Name", ID: "name", Width: 120}, {Title: "Size", ID: "size", Width: 80}}, 10, func(row, col int) {
				switch col {
				case 0:
					Row(c.Key(row)).Grow(1).Drag(fmt.Sprintf("row %d", row)).Children(func() { Textf(c, "Name %d", row) })
				case 1:
					Textf(c, "Size %d", row)
				}
			}).Height(300)
			if tb.Submitted() {
				submitted++
			}
			if b := Button(c, "Button").Drag("button"); b.Clicked() {
				clicks++
			}
			target := Box(c).Size(200, 60).Label("Target")
			if v, ok := Drop[string](target); ok {
				dropped = append(dropped, v)
			}
		})
	}, 400, 500)
	tt.Frame()
	click := func(mods Modifiers, text string) {
		t.Helper()
		x, y := rowCenter(t, tt, text)
		tt.ClickAtWith(mods, x, y)
		tt.Frame()
	}
	click(0, "Name 1")
	if got := chosenRows(sel); !slices.Equal(got, []int{1}) {
		t.Fatalf("clicked Name 1: chose %v", got)
	}
	click(Cmd, "Name 3")
	click(Shift, "Name 5")
	if got := chosenRows(sel); !slices.Equal(got, []int{3, 4, 5}) {
		t.Fatalf("Cmd-clicked Name 3, Shift-clicked Name 5: chose %v", got)
	}
	x, y := rowCenter(t, tt, "Name 2")
	tt.ClickAt(x, y)
	tt.Advance(50e6)
	tt.ClickAt(x, y)
	tt.Frame()
	if submitted != 1 || !slices.Equal(chosenRows(sel), []int{2}) {
		t.Fatalf("double-clicked Name 2: submitted %d, chose %v", submitted, chosenRows(sel))
	}
	// Dragged: the value, and the rows stay as they were.
	target, _ := tt.Find("Target")
	drag(tt, x, y, target.X+20, target.Y+20)
	tt.Frame()
	if !slices.Equal(dropped, []string{"row 2"}) || !slices.Equal(chosenRows(sel), []int{2}) {
		t.Fatalf("dragged Name 2: dropped %q, chose %v", dropped, chosenRows(sel))
	}
	// The rest of the row chooses the rows a drag goes over.
	sx, sy := rowCenter(t, tt, "Size 4")
	_, sy6 := rowCenter(t, tt, "Size 6")
	tt.Press(sx, sy)
	tt.Move(sx, sy+3)
	tt.Move(sx, sy6)
	tt.Release(sx, sy6)
	tt.Frame()
	if got := chosenRows(sel); !slices.Equal(got, []int{4, 5, 6}) || len(dropped) != 1 {
		t.Fatalf("dragged over Size 4 to 6: chose %v, dropped %q", got, dropped)
	}
	// An element that drags and takes clicks keeps them.
	if err := tt.Click("Button"); err != nil || clicks != 1 {
		t.Fatalf("the button's clicks: %d (%v)", clicks, err)
	}
}
