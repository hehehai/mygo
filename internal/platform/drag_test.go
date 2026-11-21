package platform

import (
	"testing"

	"github.com/egoist/mygo/transfer"
)

// TestOperationsFor: other apps are never offered more than Operations
// (Finder would move a file offered a move), this process's windows
// LocalOperations when set.
func TestOperationsFor(t *testing.T) {
	r := DragRequest{Operations: transfer.Copy, LocalOperations: transfer.Copy | transfer.Move}
	if got := r.OperationsFor(false); got != transfer.Copy {
		t.Fatalf("other apps: %v", got)
	}
	if got := r.OperationsFor(true); got != transfer.Copy|transfer.Move {
		t.Fatalf("this process: %v", got)
	}
	r.LocalOperations = transfer.None
	if got := r.OperationsFor(true); got != transfer.Copy {
		t.Fatalf("no local: %v", got)
	}
}
