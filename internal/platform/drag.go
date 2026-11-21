package platform

import "github.com/egoist/mygo/transfer"

// DragSessionFormat carries an unguessable, process-local registry key.
// It never contains a pointer. Only the core resolves it while a drag lives.
const DragSessionFormat transfer.Format = "application/x-mygo-drag-session"

// DragRequest starts a native drag on the main thread during a pointer
// gesture. Done must be called exactly once on the main thread. Windows
// may run a native nested event loop; other backends return immediately.
type DragRequest struct {
	Data       transfer.Data
	Operations transfer.Operation
	// LocalOperations is what the drag offers this process's windows
	// (DragOptions.Local; none: Operations), where the platform tells
	// them from other apps' (macOS: the dragging context).
	LocalOperations           transfer.Operation
	Session                   string
	X, Y                      float64
	Preview                   []byte // premultiplied BGRA, one pixel per DIP
	Width, Height, HotX, HotY int
	Done                      func(transfer.Result)
}

// OperationsFor is what the drag offers a destination of this process
// (local) or of another app: another app never gets more than
// Operations, so a file is not moved away from a source that offers its
// own windows a move.
func (r DragRequest) OperationsFor(local bool) transfer.Operation {
	if local && r.LocalOperations != transfer.None {
		return r.LocalOperations
	}
	return r.Operations
}

// DataDragEvent is a synchronous destination query. The core resolves
// Session to Local and the original Data only for a live local source.
// Content sets Operation and Formats to accept. Backends read those
// formats on drop, never while hovering, and copy the bytes into Data.
type DataDragEvent struct {
	Offer     transfer.Offer
	Session   string
	Local     any
	Data      transfer.Data
	Operation transfer.Operation
	Formats   []transfer.Format
	HasFiles  bool
	Resolved  bool // the core substituted its process-local data
}

// DragPreviewSurface is a surface that can change the preview of the
// native drag it started while the drag lasts (macOS): preview as in
// DragRequest, its point (hotX, hotY) kept under the pointer. Main thread
// only; nothing without such a drag.
type DragPreviewSurface interface {
	SetDataDragPreview(preview []byte, width, height, hotX, hotY int)
}
