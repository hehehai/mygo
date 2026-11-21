//go:build darwin && !ios

package darwin

import (
	"slices"
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// Notifications use UNUserNotificationCenter, which only works for apps
// running from a bundle with an identifier.

var (
	mainQueueMu sync.Mutex
	mainQueue   []func()
	// Registered by NewNotification so apps without senders do not link Intents.
	prepareCommunication func(*Backend, *platform.Notification, id, func(error)) bool
)

// runOnMain runs fn on the main thread; used by callbacks the system
// delivers on background queues.
func (b *Backend) runOnMain(fn func()) {
	if b.IsMainThread() {
		fn()
		return
	}
	b.post(fn)
}

// post runs fn on the main thread after the current event, even when
// called there.
func (b *Backend) post(fn func()) {
	mainQueueMu.Lock()
	mainQueue = append(mainQueue, fn)
	mainQueueMu.Unlock()
	b.Signal()
}

// drainMainQueue runs the functions posted, one at a time, as loop.drain
// does: one running a loop of its own, as a frame showing a modal dialog
// does (drawSoon), leaves those behind it to that loop, signaled again.
func drainMainQueue() {
	for {
		mainQueueMu.Lock()
		if len(mainQueue) == 0 {
			mainQueueMu.Unlock()
			return
		}
		fn := mainQueue[0]
		mainQueue[0] = nil
		mainQueue = mainQueue[1:]
		more := len(mainQueue) > 0
		mainQueueMu.Unlock()
		if more {
			theBackend.Signal()
		}
		fn()
	}
}

func registerNotificationDelegate() {
	if !hasClass("UNUserNotificationCenter") {
		return
	}
	classDef("MyGoNotificationDelegate", "NSObject", []string{"UNUserNotificationCenterDelegate"}, []objc.MethodDef{
		method("userNotificationCenter:willPresentNotification:withCompletionHandler:", func(self id, _ objc.SEL, center, n id, handler uintptr) {
			// Show banners even while the app is in the foreground.
			callBlock(handler, 1<<1|1<<3|1<<4) // sound | list | banner
		}),
		method("userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:", func(self id, _ objc.SEL, center, resp id, handler uintptr) {
			ident := goString(send(send(send(send(resp, "notification"), "request"), "identifier"), "self"))
			action := goString(send(resp, "actionIdentifier"))
			text := ""
			if respondsTo(resp, "userText") {
				text = goString(send(resp, "userText"))
			}
			switch action {
			case "", unDefaultAction:
				theBackend.runOnMain(func() { theBackend.h.NotificationClicked(ident) })
			case unDismissAction:
			default:
				theBackend.runOnMain(func() { theBackend.h.NotificationAction(ident, action, text) })
			}
			callBlock(handler)
		}),
	})
}

func (b *Backend) NotificationsSupported() bool {
	_, packaged := appController{b}.Package()
	return packaged && hasClass("UNUserNotificationCenter")
}

// notificationCenter returns the app's notification center. It throws
// when the app does not run from a bundle: check NotificationsSupported
// first.
func notificationCenter() id {
	return send(class("UNUserNotificationCenter"), "currentNotificationCenter")
}

// attachNotificationDelegate gives the notification center the delegate
// that receives clicks, at launch: the system delivers the click that
// launched the app before the app has finished launching, and drops it
// when the center has no delegate yet.
func (b *Backend) attachNotificationDelegate() {
	if !b.NotificationsSupported() {
		return
	}
	b.notifyDelegate = alloc("MyGoNotificationDelegate")
	withPool(func() {
		send(notificationCenter(), "setDelegate:", uintptr(b.notifyDelegate))
	})
}

// waitingNotification is a notification shown before the user has answered
// whether the app may show notifications.
type waitingNotification struct {
	n    *platform.Notification
	done func(error)
}

// notifyOptions are the permissions asked for: badge, sound and alert.
const notifyOptions = 1<<0 | 1<<1 | 1<<2

func (b *Backend) ShowNotification(n *platform.Notification, done func(error)) {
	if !b.NotificationsSupported() {
		done(platform.ErrUnsupported)
		return
	}
	if b.notifyAnswered {
		b.addNotification(n, done)
		return
	}
	// The system drops what it is given before the user has answered
	// whether the app may show notifications, so the first notification
	// asks and the ones shown meanwhile wait for the answer. The system
	// only prompts the first time; afterwards it answers at once, and
	// adding a notification fails when System Settings does not allow it.
	b.notifyWaiting = append(b.notifyWaiting, waitingNotification{n, done})
	if len(b.notifyWaiting) > 1 {
		return
	}
	withPool(func() {
		blk := newBlock(func(_ objc.Block, granted bool, err id) {
			b.runOnMain(func() {
				b.notifyAnswered = true
				waiting := b.notifyWaiting
				b.notifyWaiting = nil
				for _, w := range waiting {
					b.addNotification(w.n, w.done)
				}
			})
		})
		send(notificationCenter(), "requestAuthorizationWithOptions:completionHandler:",
			notifyOptions, uintptr(blk))
		blk.Release()
	})
}

// The response's actionIdentifier for a click on the notification itself
// and for its dismissal.
const (
	unDefaultAction = "com.apple.UNNotificationDefaultActionIdentifier"
	unDismissAction = "com.apple.UNNotificationDismissActionIdentifier"
)

// UNNotificationActionOptions.
const (
	unActionDestructive = 1 << 1
	unActionForeground  = 1 << 2
)

// declareCategory gives the center a notification's kind and its buttons
// (all the kinds known: setNotificationCategories: replaces the set),
// once and when its buttons change.
func (b *Backend) declareCategory(n *platform.Notification) {
	if n.Category == "" {
		return
	}
	if old, ok := b.notifyCategories[n.Category]; ok && slices.Equal(old, n.Actions) {
		return
	}
	if b.notifyCategories == nil {
		b.notifyCategories = map[string][]platform.NotificationAction{}
	}
	b.notifyCategories[n.Category] = slices.Clone(n.Actions)
	var cats []id
	for cid, actions := range b.notifyCategories {
		cats = append(cats, notificationCategory(cid, actions))
	}
	send(notificationCenter(), "setNotificationCategories:", uintptr(send(class("NSSet"), "setWithArray:", uintptr(nsArray(cats...)))))
}

// notificationCategory is a UNNotificationCategory (autoreleased) of the
// buttons actions.
func notificationCategory(cid string, actions []platform.NotificationAction) id {
	var as []id
	for _, a := range actions {
		opts := uintptr(0)
		if a.Destructive {
			opts |= unActionDestructive
		}
		if a.Foreground {
			opts |= unActionForeground
		}
		var act id
		if a.Input {
			button := a.InputButton
			if button == "" {
				button = a.Title
			}
			act = send(class("UNTextInputNotificationAction"), "actionWithIdentifier:title:options:textInputButtonTitle:textInputPlaceholder:",
				uintptr(nsString(a.ID)), uintptr(nsString(a.Title)), opts, uintptr(nsString(button)), uintptr(nsString(a.Placeholder)))
		} else {
			act = send(class("UNNotificationAction"), "actionWithIdentifier:title:options:",
				uintptr(nsString(a.ID)), uintptr(nsString(a.Title)), opts)
		}
		if act != 0 {
			as = append(as, act)
		}
	}
	return send(class("UNNotificationCategory"), "categoryWithIdentifier:actions:intentIdentifiers:options:",
		uintptr(nsString(cid)), uintptr(nsArray(as...)), uintptr(nsArray()), 0)
}

// addNotification gives the system a request to show a notification. Its
// trigger is nil, which shows it at once.
func (b *Backend) addNotification(n *platform.Notification, done func(error)) {
	b.cancelPreparingNotification(n.ID)
	withPool(func() {
		b.declareCategory(n)
		content := autorelease(alloc("UNMutableNotificationContent"))
		send(content, "setTitle:", uintptr(nsString(n.Title)))
		if n.Subtitle != "" {
			send(content, "setSubtitle:", uintptr(nsString(n.Subtitle)))
		}
		send(content, "setBody:", uintptr(nsString(n.Body)))
		if !n.Silent {
			send(content, "setSound:", uintptr(send(class("UNNotificationSound"), "defaultSound")))
		}
		if n.Group != "" {
			send(content, "setThreadIdentifier:", uintptr(nsString(n.Group)))
		}
		if n.Category != "" {
			send(content, "setCategoryIdentifier:", uintptr(nsString(n.Category)))
		}
		if len(n.Image) > 0 && attachNotificationImage != nil {
			done = attachNotificationImage(content, n.Image, done)
		}
		if n.Sender != nil && prepareCommunication != nil && prepareCommunication(b, n, content, done) {
			return
		}
		b.submitNotification(n, content, done)
	})
}

func (b *Backend) submitNotification(n *platform.Notification, content id, done func(error)) {
	req := send(class("UNNotificationRequest"), "requestWithIdentifier:content:trigger:",
		uintptr(nsString(n.ID)), uintptr(content), 0)
	blk := newBlock(func(_ objc.Block, nsErr id) {
		// The error is the caller's only while the block runs.
		var err error
		withPool(func() { err = notificationError(nsErr) })
		b.runOnMain(func() { done(err) })
	})
	send(notificationCenter(), "addNotificationRequest:withCompletionHandler:", uintptr(req), uintptr(blk))
	blk.Release()
}

func (b *Backend) cancelPreparingNotification(ident string) {
	if cancel := b.notifyPreparing[ident]; cancel != nil {
		delete(b.notifyPreparing, ident)
		cancel()
	}
}

// unErrorNotificationsNotAllowed is UNErrorCodeNotificationsNotAllowed.
const unErrorNotificationsNotAllowed = 1

// notificationError describes an error of the notification center.
func notificationError(err id) error {
	if err == 0 {
		return nil
	}
	if goString(send(err, "domain")) == "UNErrorDomain" && sendInt(err, "code") == unErrorNotificationsNotAllowed {
		return platform.ErrNotificationsDenied
	}
	return nsError(err)
}

func (b *Backend) RemoveNotification(ident string) {
	b.cancelPreparingNotification(ident)
	if !b.NotificationsSupported() {
		return
	}
	// One still waiting for the user's answer is not shown.
	waiting := b.notifyWaiting[:0]
	for _, w := range b.notifyWaiting {
		if w.n.ID == ident {
			w.done(nil)
		} else {
			waiting = append(waiting, w)
		}
	}
	b.notifyWaiting = waiting
	withPool(func() {
		center := notificationCenter()
		ids := nsArray(nsString(ident))
		send(center, "removeDeliveredNotificationsWithIdentifiers:", uintptr(ids))
		send(center, "removePendingNotificationRequestsWithIdentifiers:", uintptr(ids))
	})
}

func (b *Backend) RemoveAllNotifications() {
	preparing := b.notifyPreparing
	b.notifyPreparing = nil
	for _, cancel := range preparing {
		cancel()
	}
	if !b.NotificationsSupported() {
		return
	}
	waiting := b.notifyWaiting
	b.notifyWaiting = nil
	for _, w := range waiting {
		w.done(nil)
	}
	withPool(func() {
		center := notificationCenter()
		send(center, "removeAllDeliveredNotifications")
		send(center, "removeAllPendingNotificationRequests")
	})
}
