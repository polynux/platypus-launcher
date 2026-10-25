package app

import (
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/themes"
)

// SetTheme saves a built-in theme, "" for the default, and refuses anything
// that isn't listed as ok. GetTheme("") resolves the picked one.
func TestSetThemePicksOnlyUsableThemes(t *testing.T) {
	a := newLocaleApp(t)
	if got := a.GetTheme("").ID; got != themes.DefaultID {
		t.Fatalf("GetTheme(\"\") = %q, want the default", got)
	}
	s, err := a.SetTheme("cool")
	if err != nil || s.Theme != "cool" {
		t.Fatalf("SetTheme(cool) = %q, %v", s.Theme, err)
	}
	if got := a.GetTheme("").ID; got != "cool" {
		t.Fatalf("GetTheme(\"\") after picking = %q, want cool", got)
	}
	if _, err := a.SetTheme("nope"); err == nil {
		t.Fatal("SetTheme(nope) succeeded, want an error")
	} else if ae, ok := err.(*apperr.Error); !ok || ae.Code != "theme.unknown" {
		t.Fatalf("SetTheme(nope) error = %v, want theme.unknown", err)
	}
	if a.settings.Theme() != "cool" {
		t.Fatalf("a refused pick changed the setting: %q", a.settings.Theme())
	}
	if s, err := a.SetTheme(""); err != nil || s.Theme != "" {
		t.Fatalf("SetTheme(\"\") = %q, %v", s.Theme, err)
	}
}

// SetGameTone saves the game-colour choice and answers the launcher settings
// with it; a value the UI never sends is refused and changes nothing.
func TestSetGameToneSavesTheChoice(t *testing.T) {
	a := newLocaleApp(t)
	if got := a.GetLauncherSettings().GameTone; got != "" {
		t.Fatalf("launcher settings game tone = %q, want empty (accent)", got)
	}
	s, err := a.SetGameTone("whole")
	if err != nil || s.GameTone != "whole" {
		t.Fatalf("SetGameTone(whole) = %q, %v", s.GameTone, err)
	}
	if got := a.GetLauncherSettings().GameTone; got != "whole" {
		t.Fatalf("launcher settings game tone = %q, want whole", got)
	}
	if s, err := a.SetGameTone("off"); err != nil || s.GameTone != "off" {
		t.Fatalf("SetGameTone(off) = %q, %v", s.GameTone, err)
	}
	if _, err := a.SetGameTone("purple"); err == nil {
		t.Fatal("SetGameTone(purple) succeeded, want an error")
	}
	if a.settings.GameTone() != "off" {
		t.Fatalf("a refused value changed the setting: %q", a.settings.GameTone())
	}
}

// SetColorScheme saves the colour-scheme choice (issue #134) and answers the
// launcher settings with it; a value the UI never sends is refused.
func TestSetColorSchemeSavesTheChoice(t *testing.T) {
	a := newLocaleApp(t)
	if got := a.GetLauncherSettings().ColorScheme; got != "" {
		t.Fatalf("launcher settings color scheme = %q, want empty (dark)", got)
	}
	for _, scheme := range []string{"dark", "light", "black"} {
		s, err := a.SetColorScheme(scheme)
		if err != nil || s.ColorScheme != scheme {
			t.Fatalf("SetColorScheme(%s) = %q, %v", scheme, s.ColorScheme, err)
		}
	}
	if _, err := a.SetColorScheme("sepia"); err == nil {
		t.Fatal("SetColorScheme(sepia) succeeded, want an error")
	}
	if a.settings.ColorScheme() != "black" {
		t.Fatalf("a refused value changed the setting: %q", a.settings.ColorScheme())
	}
}

func TestListThemesOffersBuiltIns(t *testing.T) {
	a := newLocaleApp(t)
	list := a.ListThemes()
	if len(list) < 2 || list[0].ID != themes.DefaultID || list[0].Status != themes.StatusOK {
		t.Fatalf("ListThemes = %+v", list)
	}
	for _, th := range list {
		if th.Warnings == nil || th.Supports == nil {
			t.Fatalf("%s: nil lists would reach the UI as null: %+v", th.ID, th)
		}
	}
}

// GetThemeLayout follows the picked theme: the default resolves the
// built-in arrangement, cool ships its own, and unknown views resolve to
// nothing.
func TestGetThemeLayoutFollowsPickedTheme(t *testing.T) {
	a := newLocaleApp(t)
	builtIn := a.GetThemeLayout("dashboard")
	if builtIn == nil || len(builtIn.Regions) != 4 || builtIn.Regions[0].Name != "top" {
		t.Fatalf("GetThemeLayout(dashboard) = %+v, want the built-in top/main/side/bottom arrangement", builtIn)
	}
	if _, err := a.SetTheme("cool"); err != nil {
		t.Fatalf("SetTheme(cool) = %v", err)
	}
	layout := a.GetThemeLayout("dashboard")
	if layout == nil || len(layout.Regions) != 2 || layout.Regions[1].Name != "side" {
		t.Fatalf("GetThemeLayout(dashboard) = %+v, want cool's two regions", layout)
	}
	if a.GetThemeLayout("attic") != nil {
		t.Fatal("an unknown view resolved a layout")
	}
}
