//go:build darwin && !ios

package darwin

import (
	"maps"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// activityRecorder is an app handler that keeps the activities it is
// asked to continue.
type activityRecorder struct {
	platform.AppHandler
	got []platform.UserActivity
}

func (r *activityRecorder) ContinueActivity(a platform.UserActivity) { r.got = append(r.got, a) }

// TestContinueActivity: the app delegate takes the activities AppKit asks
// it to continue (a Spotlight result) and gives the handler the activity's
// type, title, page and userInfo's strings, numbers and URLs, leaving out
// other values and keys that are not strings.
func TestContinueActivity(t *testing.T) {
	load()
	// The app delegate's class alone (registerClasses would take the
	// classes other tests register).
	if objc.GetClass("MyGoAppDelegate") == 0 {
		registerAppDelegate()
	}
	rec := &activityRecorder{}
	saved := theBackend
	theBackend = &Backend{h: rec}
	defer func() { theBackend = saved }()
	withPool(func() {
		activity := autorelease(send(send(class("NSUserActivity"), "alloc"), "initWithActivityType:",
			uintptr(nsString("com.apple.corespotlightitem"))))
		send(activity, "setTitle:", uintptr(nsString("午饭？")))
		send(activity, "setWebpageURL:", uintptr(nsURL("https://example.com/m/42")))
		info := send(class("NSMutableDictionary"), "dictionary")
		set := func(k, v id) { send(info, "setObject:forKey:", uintptr(v), uintptr(k)) }
		set(nsString("kCSSearchableItemActivityIdentifier"), nsString("msg:42"))
		set(nsString("count"), nsNumberInt(7))
		set(nsString("flag"), nsBool(true))
		set(nsString("link"), nsURL("inboxly://open?m=42"))
		set(nsString("when"), send(class("NSDate"), "date"))
		set(nsNumberInt(1), nsString("key is not a string"))
		send(activity, "setUserInfo:", uintptr(info))

		delegate := autorelease(alloc("MyGoAppDelegate"))
		if !sendBool(delegate, "application:willContinueUserActivityWithType:", 0, uintptr(nsString("com.apple.corespotlightitem"))) {
			t.Error("willContinueUserActivityWithType: NO")
		}
		if !sendBool(delegate, "application:continueUserActivity:restorationHandler:", 0, uintptr(activity), 0) {
			t.Error("continueUserActivity: NO")
		}
	})
	if len(rec.got) != 1 {
		t.Fatalf("handler got %d activities, want 1", len(rec.got))
	}
	a := rec.got[0]
	if a.Type != "com.apple.corespotlightitem" || a.Title != "午饭？" || a.WebpageURL != "https://example.com/m/42" {
		t.Errorf("activity %+v", a)
	}
	want := map[string]string{
		"kCSSearchableItemActivityIdentifier": "msg:42",
		"count":                               "7",
		"flag":                                "1",
		"link":                                "inboxly://open?m=42",
	}
	if !maps.Equal(a.UserInfo, want) {
		t.Errorf("userInfo %q, want %q", a.UserInfo, want)
	}
}
