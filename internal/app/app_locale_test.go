package app

import (
	"path/filepath"
	"testing"

	"platypus/internal/settings"
	"platypus/internal/types"
)

func newLocaleApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	return &App{host: &artHost{}, settings: st, dataDir: root, optHome: root, optCacheDir: root}
}

// The effective locale is the setting, else the system list, else English.
func TestLocaleResolves(t *testing.T) {
	a := newLocaleApp(t)
	if got := a.Locale(); got != "en" {
		t.Fatalf("Locale = %q, want en", got)
	}
	a.SetSystemLanguages([]string{"fr-CA", "en"})
	if got := a.Locale(); got != "fr-CA" {
		t.Fatalf("Locale = %q, want fr-CA", got)
	}
	if _, err := a.SetLanguage("de"); err != nil {
		t.Fatal(err)
	}
	if got := a.Locale(); got != "de" {
		t.Fatalf("Locale = %q, want de (setting wins)", got)
	}
	if _, err := a.SetLanguage(""); err != nil {
		t.Fatal(err)
	}
	if got := a.Locale(); got != "fr-CA" {
		t.Fatalf("Locale = %q, want fr-CA again", got)
	}
}

// SetSystemLanguages keeps only well-formed tags and reports the locale.
func TestSetSystemLanguagesFilters(t *testing.T) {
	a := newLocaleApp(t)
	if got := a.SetSystemLanguages([]string{"!", " fr ", ""}); got != "fr" {
		t.Fatalf("SetSystemLanguages = %q, want fr", got)
	}
	if got := a.Locale(); got != "fr" {
		t.Fatalf("Locale = %q, want fr", got)
	}
}

// The game-info disk cache keys on language: an entry fetched in English is
// stale under French.
func TestGameInfoCacheKeysLanguage(t *testing.T) {
	a := newLocaleApp(t)
	a.infoCache = map[string]gameInfoEntry{}
	a.storeInfoEntry("g1", "steam:10", &types.GameDetails{Summary: "english"})
	e, ok := a.cachedInfoEntry("g1")
	if !ok || e.Language != "en" {
		t.Fatalf("cached entry = %+v, want Language en", e)
	}
	if _, err := a.SetLanguage("fr"); err != nil {
		t.Fatal(err)
	}
	// The entry is still stored, but getSideloadedDetails must not serve it
	// as fresh: the language differs. Check the freshness predicate directly
	// (the fetch itself is covered by the gameinfo suite).
	cached, hasCached := a.cachedInfoEntry("g1")
	cachedLang := cached.Language
	if cachedLang == "" {
		cachedLang = "en"
	}
	if !hasCached || cachedLang == a.Locale() {
		t.Fatalf("cached Language %q should differ from locale %q", cachedLang, a.Locale())
	}
}
