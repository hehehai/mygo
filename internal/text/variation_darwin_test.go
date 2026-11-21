//go:build darwin && !ios

package text

import (
	"os"
	"testing"
)

// TestRegisterFontVariation: a variable font registered at two points of
// its weight axis draws them apart (Core Text): the system's SF Pro at
// wght 100 and 900, the black wider; and the instances are listed under
// the names given, not the font's own. A System New makes (a Tester's
// window) takes the app's (RegisterAppFontVariation).
func TestRegisterFontVariation(t *testing.T) {
	data, err := os.ReadFile("/System/Library/Fonts/SFNS.ttf")
	if err != nil {
		t.Skip("no SF Pro (/System/Library/Fonts/SFNS.ttf)")
	}
	s := newSystem()
	for family, w := range map[string]float64{"MyGo Var Thin": 100, "MyGo Var Black": 900} {
		if err := s.RegisterFontVariation(data, family, map[string]float64{"wght": w}); err != nil {
			t.Fatal(err)
		}
	}
	width := func(s *System, family string) float32 {
		return s.Layout(Params{Text: "Hamburgefonstiv", Style: Style{Size: 40, Family: family}}).Width
	}
	thin, black := width(s, "MyGo Var Thin"), width(s, "MyGo Var Black")
	if thin <= 0 || black < thin*1.05 {
		t.Errorf("wght 100 is %.1f wide, 900 %.1f", thin, black)
	}
	listed := map[string]bool{}
	for _, f := range s.FontFamilies() {
		if f.Registered {
			listed[f.Name] = true
		}
	}
	if !listed["MyGo Var Thin"] || !listed["MyGo Var Black"] {
		t.Errorf("registered families: %v", listed)
	}
	if err := RegisterAppFontVariation(data, "MyGo App Var Black", map[string]float64{"wght": 900}); err != nil {
		t.Fatal(err)
	}
	apart := New()
	if w := width(apart, "MyGo App Var Black"); w < thin*1.05 || w > black*1.01 {
		t.Errorf("a System New makes draws the app's variation %.1f wide, want about %.1f", w, black)
	}
}
