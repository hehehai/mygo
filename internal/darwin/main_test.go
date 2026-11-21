//go:build darwin && !ios

package darwin

import (
	"os"
	"runtime"
	"testing"
)

// AppKit makes windows (a status item's) on the main thread only: the
// test binary's main goroutine stays on it and runs what tests give
// onMainThread while the tests run.
func init() { runtime.LockOSThread() }

var mainThreadQueue = make(chan func())

func TestMain(m *testing.M) {
	done := make(chan int)
	go func() { done <- m.Run() }()
	for {
		select {
		case fn := <-mainThreadQueue:
			fn()
		case code := <-done:
			os.Exit(code)
		}
	}
}

// onMainThread runs fn on the main thread (fn must not call t.Fatal).
func onMainThread(fn func()) {
	done := make(chan struct{})
	mainThreadQueue <- func() { defer close(done); fn() }
	<-done
}
