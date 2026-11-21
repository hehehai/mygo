package promised

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

// TestForgotten: a promise no one waits for is forgotten after keep, and
// its folder removed with what was written in it; one waited for keeps
// its folder (the caller removes it).
func TestForgotten(t *testing.T) {
	keep = 30 * time.Millisecond
	t.Cleanup(func() { keep = Keep })
	dir := t.TempDir()
	forgotten, taken := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	for _, d := range []string{forgotten, taken} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "pic.png"), []byte("x"), 0o644)
	}
	t1, p1 := New(1, forgotten)
	p1.Add(filepath.Join(forgotten, "pic.png"), nil)
	t2, p2 := New(1, taken)
	p2.Add(filepath.Join(taken, "pic.png"), nil)
	if _, _, err := Wait(context.Background(), t2); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(forgotten); errors.Is(err, os.ErrNotExist) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the forgotten promise's folder stays")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, _, err := Wait(context.Background(), t1); !errors.Is(err, ErrUnknown) {
		t.Errorf("forgotten: %v", err)
	}
	time.Sleep(3 * keep)
	if _, err := os.Stat(filepath.Join(taken, "pic.png")); err != nil {
		t.Errorf("the taken promise's folder went: %v", err)
	}
}
