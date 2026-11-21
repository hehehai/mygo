package mygo

import (
	"maps"

	"github.com/egoist/mygo/internal/platform"
)

// UserActivity is something the user was doing that the system asks the
// app to continue: one of the app's Spotlight results opened (macOS:
// CoreSpotlight's CSSearchableItem), or a Handoff from another device
// (NSUserActivity).
type UserActivity struct {
	// Type is the activity's type: SpotlightActivityType for a Spotlight
	// result, the app's own (from NSUserActivityTypes in its Info.plist)
	// for a Handoff.
	Type string
	// UserInfo holds the activity's values that are strings, numbers (as
	// text, a bool as "1" or "0") or URLs (as text); others are left out.
	// For a Spotlight result, SpotlightItemIDKey holds the item's
	// uniqueIdentifier.
	UserInfo map[string]string
	// WebpageURL is the page to open when the app cannot continue it, if
	// any.
	WebpageURL string
	// Title is the activity's title, if any.
	Title string
}

// Activity types and keys of Spotlight's (CoreSpotlight).
const (
	// SpotlightActivityType is the Type of a Spotlight result the user
	// opened (CSSearchableItemActionType).
	SpotlightActivityType = "com.apple.corespotlightitem"
	// SpotlightItemIDKey is the UserInfo key of the opened item's
	// uniqueIdentifier (CSSearchableItemActivityIdentifier).
	SpotlightItemIDKey = "kCSSearchableItemActivityIdentifier"
	// SpotlightQueryActivityType is the Type of "Search in App" from
	// Spotlight (CSQueryContinuationActionType), for apps whose Info.plist
	// sets CoreSpotlightContinuation; SpotlightQueryKey holds what the user
	// typed (CSSearchQueryString).
	SpotlightQueryActivityType = "com.apple.corespotlightquery"
	SpotlightQueryKey          = "kCSSearchQueryString"
)

// OnContinueActivity is called on the main thread when the system asks
// the app to continue an activity: the user opened one of the app's
// Spotlight results (Type SpotlightActivityType, the item's ID under
// SpotlightItemIDKey) or took a Handoff. macOS brings the app to the
// front. Register it before Run to receive the activity the app was
// launched with, as OnOpenURL. Only macOS has them; elsewhere it is never
// called.
func (a *Application) OnContinueActivity(fn func(a UserActivity)) (off func()) {
	return a.onContinueActivity.add(fn, false)
}

func userActivity(a platform.UserActivity) UserActivity {
	info := maps.Clone(a.UserInfo)
	if info == nil {
		info = map[string]string{}
	}
	return UserActivity{Type: a.Type, UserInfo: info, WebpageURL: a.WebpageURL, Title: a.Title}
}
