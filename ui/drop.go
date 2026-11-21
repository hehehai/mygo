package ui

// Files dragged from other apps, such as Finder or Explorer, drop on the
// elements that ask for them with DroppedFiles. The window takes them only
// over such an element, or anywhere when the window has OnFileDrop
// listeners, which get what no element takes.

// DroppedFiles returns the paths of the files dropped on the element since
// the last frame, nil when none, and makes the element take files dragged
// over it.
func (e *node) DroppedFiles() []string {
	e.flags |= flagDropTarget
	if len(e.st.dropped) > 0 {
		e.c.rt.consumed = true
	}
	return e.st.dropped
}

// DroppedFilesAt is DroppedFiles with where they were dropped, in the
// element's box: a timeline places a file at the time under the drop.
func (e *node) DroppedFilesAt() (files []string, x, y float32) {
	files = e.DroppedFiles()
	if len(files) == 0 {
		return nil, 0, 0
	}
	return files, e.st.fileDropX, e.st.fileDropY
}

// FileDragOver reports whether files dragged from another app are over
// the element, which takes them if they are dropped, for showing it would.
func (e *node) FileDragOver() bool {
	e.flags |= flagDropTarget
	return e.c.rt.dropOver == e.id
}

// FileDragAt is FileDragOver with where the files are, in the element's
// box, for showing where they would go (a frame follows each move over
// the element that takes them).
func (e *node) FileDragAt() (x, y float32, over bool) {
	if !e.FileDragOver() || !e.hasState() {
		return 0, 0, false
	}
	x, y = e.st.local(e.c.rt.dropOverX, e.c.rt.dropOverY)
	return x, y, true
}

// dropTarget returns the innermost element at (x, y) that takes files.
func (rt *engine) dropTarget(x, y float32) *state {
	for _, id := range rt.hitChain(x, y) {
		if s := rt.states[id]; s != nil && s.flags&flagDropTarget != 0 && s.flags&flagDisabled == 0 {
			return s
		}
	}
	return nil
}

// fileDrag follows files dragged over (x, y), or out of the window for
// negative coordinates, and reports whether an element takes them there.
func (rt *engine) fileDrag(x, y float32) bool {
	var over uint64
	if x >= 0 && y >= 0 {
		if s := rt.dropTarget(x, y); s != nil {
			over = s.id
		}
	}
	if over != rt.dropOver || over != 0 && (x != rt.dropOverX || y != rt.dropOverY) {
		rt.dropOver = over
		rt.requestFrame()
	}
	rt.dropOverX, rt.dropOverY = x, y
	return over != 0
}

// fileDrop drops files at (x, y), and reports whether an element took them.
func (rt *engine) fileDrop(x, y float32, files []string) bool {
	rt.fileDrag(-1, -1)
	s := rt.dropTarget(x, y)
	if s == nil || len(files) == 0 {
		return false
	}
	s.dropped = append(s.dropped, files...)
	s.fileDropX, s.fileDropY = s.local(x, y)
	rt.requestFrame()
	return true
}
