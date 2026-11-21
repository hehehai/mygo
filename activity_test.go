//go:build !js

package mygo

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestContinueActivity: an activity the platform asks to continue (a
// Spotlight result opened) reaches every OnContinueActivity listener on
// the main thread, with its values; a listener removed hears no more.
func TestContinueActivity(t *testing.T) {
	var got []UserActivity
	off := App.OnContinueActivity(func(a UserActivity) {
		if !isMainThread() {
			t.Error("listener off the main thread")
		}
		got = append(got, a)
	})
	var second []UserActivity
	offSecond := App.OnContinueActivity(func(a UserActivity) { second = append(second, a) })
	defer offSecond()
	onMain(func() {
		fb.ContinueActivity(platform.UserActivity{
			Type:     SpotlightActivityType,
			UserInfo: map[string]string{SpotlightItemIDKey: "msg:42"},
			Title:    "Lunch?",
		})
	})
	off()
	onMain(func() {
		fb.ContinueActivity(platform.UserActivity{Type: "dev.example.reading", WebpageURL: "https://example.com/r"})
	})
	if len(got) != 1 {
		t.Fatalf("got %d activities, want 1: %+v", len(got), got)
	}
	if a := got[0]; a.Type != SpotlightActivityType || a.UserInfo[SpotlightItemIDKey] != "msg:42" || a.Title != "Lunch?" || a.WebpageURL != "" {
		t.Errorf("activity %+v", a)
	}
	if len(second) != 2 {
		t.Fatalf("second listener heard %d, want 2", len(second))
	}
	// Without values, UserInfo is empty, not nil.
	if a := second[1]; a.Type != "dev.example.reading" || a.WebpageURL != "https://example.com/r" || a.UserInfo == nil || len(a.UserInfo) != 0 {
		t.Errorf("activity %+v", a)
	}
}
