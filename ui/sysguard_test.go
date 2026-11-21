package ui

import (
	"errors"
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/surface"
	"github.com/egoist/mygo/internal/sysguard"
	"github.com/egoist/mygo/transfer"
)

// recordingClipboard records what a window writes to the clipboard.
type recordingClipboard struct {
	platform.Clipboard
	wrote []string
}

func (c *recordingClipboard) WriteText(s string) { c.wrote = append(c.wrote, s) }
func (c *recordingClipboard) WriteData(d transfer.Data, _ bool, _ func()) error {
	c.wrote = append(c.wrote, "data")
	return nil
}

// A text input's Copy and Cut ask the system guard and write nothing when
// it refuses; allowed, they write.
func TestSystemGuardClipboard(t *testing.T) {
	var asked []string
	allow := false
	sysguard.Set(func(action, target string) bool {
		asked = append(asked, action)
		return allow
	})
	defer sysguard.Set(nil)
	cb := &recordingClipboard{}
	h := &windowHost{conn: &surface.Conn{Clipboard: cb, Post: func(f func()) { f() }}}
	h.writeClipboard("secret")
	h.writeClipboardData(transfer.TextData("secret"))
	var err error
	h.writeClipboardThen("secret", func(e error) { err = e })
	if len(cb.wrote) != 0 || !errors.Is(err, sysguard.ErrGuarded) {
		t.Errorf("refused: wrote %q, then %v", cb.wrote, err)
	}
	if !slices.Equal(asked, []string{"clipboard", "clipboard", "clipboard"}) {
		t.Errorf("asked %q", asked)
	}
	allow = true
	h.writeClipboard("ok")
	if !slices.Equal(cb.wrote, []string{"ok"}) {
		t.Errorf("allowed: wrote %q", cb.wrote)
	}
}
