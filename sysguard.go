package mygo

import "github.com/egoist/mygo/internal/sysguard"

// The actions SetSystemGuard's guard is asked about.
const (
	GuardOpenURL      = sysguard.OpenURL      // Shell.OpenExternal, a Link, Context.OpenURL
	GuardOpenPath     = sysguard.OpenPath     // Shell.OpenPath
	GuardLaunchApp    = sysguard.LaunchApp    // Shell.LaunchApp (target: the app's path)
	GuardShowItem     = sysguard.ShowItem     // Shell.ShowItemInFolder
	GuardTrash        = sysguard.Trash        // Shell.TrashItem, TrashItemResult
	GuardNotification = sysguard.Notification // Notification.Show (macOS asks the user the first time)
	GuardClipboard    = sysguard.Clipboard    // Clipboard.Write and the writes built on it, Clear; a text input's Copy and Cut
)

// ErrGuarded is what an action the guard refused returns.
var ErrGuarded = sysguard.ErrGuarded

// SetSystemGuard sets fn to be asked before MyGo changes something outside
// the app: opening a URL or a file in another app, launching an app,
// revealing or trashing a file, showing a notification, writing the
// clipboard. fn gets the action (GuardOpenURL…) and its target (the URL,
// the path, the notification's ID, "" for the clipboard), on any
// goroutine; false skips the action, which returns ErrGuarded where it
// returns an error. Scripted checks of an app use it to keep the user's
// other apps, notifications and clipboard out of their way. nil (the
// default) allows everything.
func SetSystemGuard(fn func(action, target string) bool) { sysguard.Set(fn) }
