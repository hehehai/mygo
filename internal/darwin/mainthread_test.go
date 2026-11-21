//go:build darwin && !ios

package darwin

import (
	"os"
	"runtime"
	"testing"
)

// The tests run on goroutines of their own, as always; the main thread
// waits for functions onMainThread hands it, for the tests that make
// windows: AppKit throws on an NSWindow made on another thread.
func init() { runtime.LockOSThread() } // the main goroutine stays on the main thread

var mainCalls = make(chan func())

func TestMain(m *testing.M) {
	go func() { os.Exit(m.Run()) }()
	for fn := range mainCalls {
		fn()
	}
}

// onMainThread runs fn on the process's main thread and waits for it.
func onMainThread(fn func()) {
	done := make(chan struct{})
	mainCalls <- func() {
		defer close(done)
		fn()
	}
	<-done
}
