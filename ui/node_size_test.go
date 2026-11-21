package ui

import (
	"testing"
	"unsafe"
)

// TestNodeSize: an element is a whole number of cache lines (element.go,
// the end of node).
func TestNodeSize(t *testing.T) {
	if n := unsafe.Sizeof(node{}); n%64 != 0 {
		t.Errorf("node is %d bytes, %d over a whole number of cache lines", n, n%64)
	}
}
