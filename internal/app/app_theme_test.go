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
