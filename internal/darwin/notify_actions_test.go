//go:build darwin && !ios

package darwin

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestNotificationCategory: a kind's buttons become UNNotificationActions,
// a reply a UNTextInputNotificationAction with its button and hint, with
// the options asked for.
func TestNotificationCategory(t *testing.T) {
	load()
	if !hasClass("UNNotificationCategory") {
		t.Skip("no UserNotifications")
	}
	withPool(func() {
		cat := notificationCategory("mail", []platform.NotificationAction{
			{ID: "archive", Title: "Archive"},
			{ID: "reply", Title: "Reply", Input: true, Placeholder: "Message"},
			{ID: "delete", Title: "Delete", Destructive: true, Foreground: true},
		})
		if got := goString(send(cat, "identifier")); got != "mail" {
			t.Errorf("identifier %q", got)
		}
		actions := arrayItems(send(cat, "actions"))
		if len(actions) != 3 {
			t.Fatalf("%d actions", len(actions))
		}
		if got := goString(send(actions[0], "identifier")); got != "archive" {
			t.Errorf("first action %q", got)
		}
		reply := actions[1]
		if !sendBool(reply, "isKindOfClass:", uintptr(class("UNTextInputNotificationAction"))) {
			t.Error("the reply takes no text")
		} else if b, p := goString(send(reply, "textInputButtonTitle")), goString(send(reply, "textInputPlaceholder")); b != "Reply" || p != "Message" {
			t.Errorf("reply button %q, hint %q", b, p)
		}
		if opts := uintptr(send(actions[2], "options")); opts != unActionDestructive|unActionForeground {
			t.Errorf("delete options %#x", opts)
		}
	})
}
