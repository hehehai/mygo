// Package sysguard holds the guard an app sets with mygo.SetSystemGuard,
// asked before MyGo changes something outside the app (package mygo and
// package ui both ask it).
package sysguard

import (
	"errors"
	"sync/atomic"
)

// The actions the guard is asked about (mygo re-exports them).
const (
	OpenURL      = "open-url"       // Shell.OpenExternal, a Link, Context.OpenURL
	OpenPath     = "open-path"      // Shell.OpenPath
	LaunchApp    = "launch-app"     // Shell.LaunchApp
	ShowItem     = "show-in-folder" // Shell.ShowItemInFolder
	Trash        = "trash"          // Shell.TrashItem, TrashItemResult
	Notification = "notification"   // Notification.Show
	Clipboard    = "clipboard"      // Clipboard writes; a text input's Copy and Cut
)

// ErrGuarded is what an action the guard refused returns.
var ErrGuarded = errors.New("mygo: refused by the system guard")

var guard atomic.Pointer[func(action, target string) bool]

// Set sets the guard (nil: none).
func Set(fn func(action, target string) bool) {
	if fn == nil {
		guard.Store(nil)
		return
	}
	guard.Store(&fn)
}

// Allows reports whether action on target may go ahead: always without a
// guard.
func Allows(action, target string) bool {
	p := guard.Load()
	return p == nil || (*p)(action, target)
}
