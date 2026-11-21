//go:build darwin && !ios

package darwin

import "os"

// launchFront brings the app to the front as it finishes launching, unless
// LaunchServices started it and left it in the background, or `mygo dev
// -background` launched it (AppOptions.Background).
//
// LaunchServices activates an app it opens (the Finder, the Dock, open)
// before applicationDidFinishLaunching: (NSApp isActive is YES already in
// applicationWillFinishLaunching:), and not one opened with open -g (or
// any launch told not to switch): it is not active then, and the app in
// front stays. MyGo activated every regular app there, for plain
// executables (go run, mygo dev), which nothing activates, and so took
// the front from the user on an open -g too; and its first window's Show
// activated it again. Started by launchd (LaunchServices) and not active,
// the app stays in the background, whatever its activation policy (an app
// that hides its Dock icon by a setting is an accessory), and the core
// shows its first windows without activating it
// (platform.BackgroundLaunch). Only a regular app is activated otherwise,
// as before: an accessory is left as LaunchServices made it.
func (b *Backend) launchFront(regular, byLaunchd, active bool) {
	switch {
	case b.background:
	case byLaunchd && !active:
		b.launchedBehind = true
	case regular:
		// Plain executables (go run) do not come to the front on their
		// own.
		send(b.app, "activateIgnoringOtherApps:", 1)
	}
}

// LaunchedInBackground reports whether LaunchServices started the app and
// left it in the background (platform.BackgroundLaunch).
func (a appController) LaunchedInBackground() bool { return a.b.launchedBehind }

// launchedByLaunchd reports whether launchd started the process: an app
// LaunchServices opened. A plain executable's parent is the shell or
// mygo dev.
func launchedByLaunchd() bool { return os.Getppid() == 1 }
