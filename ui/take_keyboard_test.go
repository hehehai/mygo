package ui

import "testing"

// TestTextInputTakesKeyboard has a press on a text input ask the window
// for the keyboard, which a panel that takes it only for text
// (mygo.PanelKeyboardForText) does not take by itself, and a press on a
// button leave it where it is.
func TestTextInputTakesKeyboard(t *testing.T) {
	value := ""
	tt := NewTester(func(c *Context) {
		Column(c).Children(func() {
			Button(c, "Go")
			TextInput(c, &value).Label("Input")
		})
	}, 300, 160)
	if err := tt.Click("Go"); err != nil {
		t.Fatal(err)
	}
	if n := tt.h.keyboardTakes; n != 0 {
		t.Fatalf("a button asked for the keyboard %d times", n)
	}
	if err := tt.Click("Input"); err != nil {
		t.Fatal(err)
	}
	if n := tt.h.keyboardTakes; n != 1 {
		t.Fatalf("a text input asked for the keyboard %d times, want 1", n)
	}
}
