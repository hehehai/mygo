// Package promised keeps the files other apps promised in a drop (macOS
// file promises) until the app asks for them: a backend registers the
// promise as the drop ends and adds each file as its source writes it;
// transfer.Data.ReceivePromised waits for them by the token the drop's
// data carries.
package promised

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"time"
)

// Keep is how long a promise stays registered without being waited for;
// then its folder, and what the source wrote in it, is removed.
const Keep = 10 * time.Minute

// keep is Keep (tests shorten it).
var keep = Keep

// ErrUnknown: the token names no promise (taken already, or too old).
var ErrUnknown = errors.New("transfer: no such promised files")

// Promise is files promised in one drop.
type Promise struct {
	Dir  string // the folder they are written in
	want int

	mu    sync.Mutex
	files []string
	errs  []error
	done  chan struct{}
	ended bool
}

var reg sync.Map // token → *Promise

// New registers a promise of want files written in dir and returns its
// token.
func New(want int, dir string) (string, *Promise) {
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)
	p := &Promise{Dir: dir, want: want, done: make(chan struct{})}
	if want <= 0 {
		p.End()
	}
	reg.Store(token, p)
	time.AfterFunc(keep, func() { forget(token) })
	return token, p
}

// Add records a file written (path) or one that was not (err); the
// promise ends once all came.
func (p *Promise) Add(path string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err != nil {
		p.errs = append(p.errs, err)
	} else if path != "" {
		p.files = append(p.files, path)
	}
	if len(p.files)+len(p.errs) >= p.want {
		p.endLocked()
	}
}

// End ends the promise with what came.
func (p *Promise) End() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.endLocked()
}

func (p *Promise) endLocked() {
	if !p.ended {
		p.ended = true
		close(p.done)
	}
}

// Wait waits for the files of token's promise (until ctx ends: the files
// written by then) and forgets it. The error is the first file's that
// did not come when none did.
func Wait(ctx context.Context, token string) ([]string, string, error) {
	v, ok := reg.LoadAndDelete(token)
	if !ok {
		return nil, "", ErrUnknown
	}
	p := v.(*Promise)
	select {
	case <-p.done:
	case <-ctx.Done():
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	files := append([]string(nil), p.files...)
	if len(files) == 0 {
		if len(p.errs) > 0 {
			return nil, p.Dir, p.errs[0]
		}
		if err := ctx.Err(); err != nil {
			return nil, p.Dir, err
		}
	}
	return files, p.Dir, nil
}

// forget drops the promise of token no one waited for, and removes its
// folder: no one will take or remove the files written in it.
func forget(token string) {
	if v, ok := reg.LoadAndDelete(token); ok {
		if dir := v.(*Promise).Dir; dir != "" {
			os.RemoveAll(dir)
		}
	}
}
