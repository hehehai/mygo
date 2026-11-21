//go:build !js

package mygo

import (
	"errors"
	"slices"
	"testing"

	"github.com/egoist/mygo/transfer"
)

func TestSystemGuard(t *testing.T) {
	var asked []string
	SetSystemGuard(func(action, target string) bool {
		asked = append(asked, action+" "+target)
		return false
	})
	defer SetSystemGuard(nil)
	if err := Shell.OpenExternal("https://example.com/"); !errors.Is(err, ErrGuarded) {
		t.Errorf("OpenExternal: %v", err)
	}
	if err := Shell.OpenPath("/tmp/x.ics"); !errors.Is(err, ErrGuarded) {
		t.Errorf("OpenPath: %v", err)
	}
	Shell.ShowItemInFolder("/tmp/x")
	if err := Shell.TrashItem("/tmp/x"); !errors.Is(err, ErrGuarded) {
		t.Errorf("TrashItem: %v", err)
	}
	if _, err := Shell.TrashItemResult("/tmp/y"); !errors.Is(err, ErrGuarded) {
		t.Errorf("TrashItemResult: %v", err)
	}
	if err := Shell.LaunchApp(InstalledApp{Path: "/Applications/Mail.app"}); !errors.Is(err, ErrGuarded) {
		t.Errorf("LaunchApp: %v", err)
	}
	n := NewNotification(NotificationOptions{ID: "guarded", Title: "x"})
	if err := n.Show(); !errors.Is(err, ErrGuarded) {
		t.Errorf("Show: %v", err)
	}
	if fb.Notification("guarded") != nil {
		t.Error("the notification reached the platform")
	}
	if err := Clipboard.Write(transfer.TextData("secret")); !errors.Is(err, ErrGuarded) {
		t.Errorf("Clipboard.Write: %v", err)
	}
	Clipboard.Clear()
	want := []string{"open-url https://example.com/", "open-path /tmp/x.ics", "show-in-folder /tmp/x", "trash /tmp/x", "trash /tmp/y",
		"launch-app /Applications/Mail.app", "notification guarded", "clipboard ", "clipboard "}
	if !slices.Equal(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
	// Allowed, it goes ahead.
	SetSystemGuard(func(string, string) bool { return true })
	if err := Clipboard.Write(transfer.TextData("ok")); err != nil {
		t.Errorf("allowed Clipboard.Write: %v", err)
	}
}
