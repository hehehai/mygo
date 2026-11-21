package promised

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWait(t *testing.T) {
	token, p := New(2, "/tmp/x")
	go func() {
		p.Add("/tmp/x/a.png", nil)
		p.Add("", errors.New("not written"))
	}()
	files, dir, err := Wait(context.Background(), token)
	if err != nil || dir != "/tmp/x" || len(files) != 1 || files[0] != "/tmp/x/a.png" {
		t.Fatalf("Wait = %v %q %v", files, dir, err)
	}
	if _, _, err := Wait(context.Background(), token); !errors.Is(err, ErrUnknown) {
		t.Fatalf("second Wait: %v", err)
	}
	// Nothing written before ctx ends: its error.
	token, _ = New(1, "/tmp/y")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if files, _, err := Wait(ctx, token); len(files) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed out: %v %v", files, err)
	}
	// All failed: the first error.
	token, p = New(1, "/tmp/z")
	p.Add("", errors.New("denied"))
	if _, _, err := Wait(context.Background(), token); err == nil || err.Error() != "denied" {
		t.Fatalf("failed: %v", err)
	}
}
