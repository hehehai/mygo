package markdown

import (
	"github.com/egoist/mygo/ui"
)

// Highlights: passages of the source marked in colors behind their
// text, as a reader's view marks what its reader highlighted; the app
// keeps them and asks where the selection is (SelectionRange) to add
// one, and where a click fell (ClickedAt) to open the one there.

// Highlight colors bytes Start to End of the source with Color, behind
// its text; a later one covers an earlier one where they overlap. Code
// blocks keep their own colors.
type Highlight struct {
	Start, End int
	Color      ui.Color
}

// hlCut is the first place after src, before end, where a highlight
// starts or ends; end when none does.
func (sb *spanBuilder) hlCut(src, end int) int {
	cut := end
	for _, h := range sb.hl {
		if h.Start > src && h.Start < cut {
			cut = h.Start
		}
		if h.End > src && h.End < cut {
			cut = h.End
		}
	}
	return cut
}

// hlAt is the color of the last highlight over the source's bytes src
// to end.
func (sb *spanBuilder) hlAt(src, end int) (ui.Color, bool) {
	var c ui.Color
	ok := false
	for _, h := range sb.hl {
		if h.Start <= src && src < h.End || src < h.Start && h.Start < end {
			c, ok = h.Color, true
		}
	}
	return c, ok && c.A > 0
}

// stateOf is the state of view el (what View or Stream returned).
func stateOf(el ui.Element) *viewState {
	if !el.Valid() {
		return nil
	}
	st := ui.Local(el, stateKey{}, func() *viewState { return nil })
	if st == nil {
		return nil
	}
	return *st
}

// SelectionRange returns the bytes of the source the selection of view
// el (what View returned, this frame) holds, as of the last frame; ok is
// false when nothing is selected, or the view shares its selection
// (Options.ShareSelection).
func SelectionRange(el ui.Element) (start, end int, ok bool) {
	st := stateOf(el)
	if st == nil || st.doc == nil {
		return 0, 0, false
	}
	parts := el.SelectedTexts()
	if len(parts) == 0 {
		return 0, 0, false
	}
	first, last := parts[0], parts[len(parts)-1]
	a, okA := st.elem(first.ID)
	b, okB := st.elem(last.ID)
	if !okA || !okB {
		return 0, 0, false
	}
	start, end = st.srcAt(a, first.Start, false), st.srcAt(b, last.End, true)
	if end <= start {
		return 0, 0, false
	}
	return start, end, true
}

// ClickedAt returns the byte of the source that a click on the text of
// view el (what View returned, this frame) fell on since the last frame
// (ui's TextClicked).
func ClickedAt(el ui.Element) (int, bool) {
	st := stateOf(el)
	if st == nil || st.doc == nil {
		return 0, false
	}
	p, ok := el.TextClicked()
	if !ok {
		return 0, false
	}
	inf, ok := st.elem(p.ID)
	if !ok {
		return 0, false
	}
	return st.srcAt(inf, p.Start, false), true
}

// ScrollToSource scrolls view el (what View returned, this frame) at its
// next build to the block holding byte at of the source, as to a
// highlight chosen in a list.
func ScrollToSource(el ui.Element, at int) {
	st := stateOf(el)
	if st == nil || st.doc == nil {
		return
	}
	d := st.doc
	for j := 1; j < d.Len(); j++ {
		if b := d.Block(j); b.Start <= at && at < b.End {
			st.scrollTo = j // the innermost: children come after their parent
		}
	}
}
