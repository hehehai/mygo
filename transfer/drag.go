package transfer

import "image"

// Operation is a native drag effect, or a bitmask of allowed effects.
type Operation uint8

const (
	None Operation = 0
	Copy Operation = 1
	Move Operation = 2
)

// Allowed supplies the safe default (copy) and removes unsupported effects.
func (o Operation) Allowed() Operation {
	if o == None {
		return Copy
	}
	return o & (Copy | Move)
}

// Negotiate chooses one effect shared by source and destination. A source's
// modifier-key suggestion wins when allowed; otherwise copy precedes move.
func Negotiate(source, destination, suggested Operation) Operation {
	allowed := source & destination.Allowed()
	if suggested == Copy || suggested == Move {
		if allowed&suggested != 0 {
			return suggested
		}
	}
	if allowed&Copy != 0 {
		return Copy
	}
	if allowed&Move != 0 {
		return Move
	}
	return None
}

// DragOptions configures a native source. The default operation is copy.
type DragOptions struct {
	Operations Operation
	// Local is what the drag offers the windows of this process (none:
	// Operations); other apps get Operations only. A source whose files
	// must not leave where they are (a file manager moves a file it is
	// offered a move for) offers them Copy, and its own windows Copy|Move
	// too, which tell by the effect they answer (Element.DropEffect) what
	// a drop does — the pointer shows it. Where the platform cannot tell
	// the process's own windows from other apps' the drag offers
	// Operations everywhere (only macOS tells them).
	Local Operation
	// Preview is copied at drag start. Hotspot is in image pixels; the
	// preview is displayed in DIPs (one pixel per DIP).
	Preview image.Image
	Hotspot image.Point
	// Done runs on the main thread exactly once, including cancellation,
	// startup failure and source-window destruction. Delete or remove the
	// original only after a successful Move; MyGo never deletes files.
	Done func(Result)
}

// LocalOperations is what the drag offers the windows of this process
// (Local, else Operations), the safe default supplied.
func (o DragOptions) LocalOperations() Operation {
	if o.Local != None {
		return o.Local.Allowed()
	}
	return o.Operations.Allowed()
}

// Result reports the destination's final effect. None means no drop.
type Result struct {
	Operation Operation
	// Canceled reports that the drag ended without a drop: let go over
	// no target, or canceled.
	Canceled bool
	// Escaped reports that a canceled drag was given up (Escape,
	// Window.CancelDataDrag, or the source window closing), rather than
	// let go over no target.
	Escaped bool
	// Point is where the drag ended, in screen coordinates, as
	// Screen.CursorScreenPoint reports them.
	Point image.Point
	Err   error
}

// Offer describes a drag during hover, without reading its bytes.
type Offer struct {
	Formats    []Format
	Operations Operation
	Suggested  Operation
}

// Preferred selects a format without requesting any representation bytes.
func (o Offer) Preferred(formats ...Format) (Format, bool) {
	return PreferredFormat(o.Formats, formats...)
}

// DropOptions describes a destination. At least one offered format must
// match Formats. Empty Formats accepts none; zero Operations permits copy.
type DropOptions struct {
	Formats    []Format
	Operations Operation
}

// Drop is a completed transfer. Its data is independent of the native
// session and can be read later, from any goroutine.
type Drop struct {
	Data      Data
	Operation Operation
}
