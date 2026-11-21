//go:build darwin && !ios

package darwin

import "github.com/egoist/mygo/internal/platform"

// continueActivity gives the app an activity to continue (a Spotlight
// result opened, a Handoff), bringing it to the front as a click on a
// result means to.
func (b *Backend) continueActivity(activity id) {
	a := userActivity(activity)
	if b.app != 0 && !sendBool(b.app, "isActive") {
		send(b.app, "activateIgnoringOtherApps:", 1)
	}
	b.h.ContinueActivity(a)
}

// userActivity reads an NSUserActivity: its userInfo's values that are
// strings, numbers (their description: a bool is "1" or "0") or URLs
// (absoluteString), under keys that are strings; others are left out.
func userActivity(activity id) platform.UserActivity {
	var a platform.UserActivity
	withPool(func() {
		a.Type = goString(send(activity, "activityType"))
		a.Title = goString(send(activity, "title"))
		if u := send(activity, "webpageURL"); u != 0 {
			a.WebpageURL = goString(send(u, "absoluteString"))
		}
		a.UserInfo = map[string]string{}
		info := send(activity, "userInfo")
		if info == 0 {
			return
		}
		str, num, url := class("NSString"), class("NSNumber"), class("NSURL")
		for _, k := range arrayItems(send(info, "allKeys")) {
			if !sendBool(k, "isKindOfClass:", uintptr(str)) {
				continue
			}
			v := send(info, "objectForKey:", uintptr(k))
			switch {
			case sendBool(v, "isKindOfClass:", uintptr(str)):
				a.UserInfo[goString(k)] = goString(v)
			case sendBool(v, "isKindOfClass:", uintptr(num)):
				a.UserInfo[goString(k)] = goString(send(v, "description"))
			case sendBool(v, "isKindOfClass:", uintptr(url)):
				a.UserInfo[goString(k)] = goString(send(v, "absoluteString"))
			}
		}
	})
	return a
}
