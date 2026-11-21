//go:build darwin && !ios

package darwin

import "time"

// stopRetry is how often keepStopping tells the event loop to stop again
// while it has not.
const stopRetry = 250 * time.Millisecond

// keepStopping tells the event loop to stop again, every stopRetry, until
// Run returns. stop: alone does not always end it: AppKit stops -run after
// the event it is handling, and the event Quit posts can be taken by
// another loop first — a nested one waiting on the main thread (Step,
// under await: a listener or a window closing waits for a web view, a
// dialog, the clipboard) dequeues it, and -run goes back to waiting — and
// a modal session aborted that was not runPanel's never stops the loop
// after; an app in the background with no windows gets no other event.
// The core then finishes and exits the process after 3 s (stopGrace,
// upstream #805), skipping what main does after App.Run. Each retry runs
// on the main thread from the run loop's source, as App.Quit does: it
// aborts a modal session again, else stops and posts its event anew.
func (b *Backend) keepStopping() {
	if b.stopping {
		return
	}
	b.stopping = true
	go func() {
		for !b.ran.Load() {
			time.Sleep(stopRetry)
			if b.ran.Load() {
				return
			}
			b.post(func() {
				if b.ran.Load() {
					return
				}
				if send(b.app, "modalWindow") != 0 {
					b.quitAfterModal = true
					send(b.app, "abortModal")
					return
				}
				send(b.app, "stop:", 0)
				b.postEvent(0)
			})
		}
	}()
}
