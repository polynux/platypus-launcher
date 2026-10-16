package app

import (
	"slices"

	"platypus/internal/locale"
)

// Locale returns the effective BCP 47 locale for plugins and stores (issue
// #39, phase 4): the launcher setting when set, else the first well-formed
// system language the shell reported, else English. The UI maps this onto its
// shipped catalogs; stores get the full tag (e.g. "fr-FR").
func (a *App) Locale() string {
	var setting string
	if a.settings != nil {
		setting = a.settings.Language()
	}
	a.localeMu.Lock()
	system := slices.Clone(a.systemLangs)
	a.localeMu.Unlock()
	return locale.Resolve(setting, system)
}

// SetSystemLanguages records the desktop languages in preference order, as
// reported by the shell (Electron's app.getPreferredSystemLanguages). When
// the effective locale changes it notifies plugins that speak protocol 1.14
// and drops the localized store-data caches, so the next game page fetches
// in the new language. It returns the effective locale.
func (a *App) SetSystemLanguages(langs []string) string {
	before := a.Locale()
	clean := make([]string, 0, len(langs))
	for _, l := range langs {
		if locale.Valid(l) {
			clean = append(clean, locale.Canonical(l))
		}
	}
	a.localeMu.Lock()
	a.systemLangs = clean
	a.localeMu.Unlock()
	after := a.Locale()
	if after != before {
		a.applyLocaleChange(after)
	}
	return after
}

// applyLocaleChange pushes a new effective locale to running plugins and
// invalidates the localized store-data caches. Library titles wait for the
// next rescan.
func (a *App) applyLocaleChange(loc string) {
	if a.plugins != nil {
		a.plugins.SetLocale(loc)
	}
	if a.stores != nil {
		a.stores.SetLocale(loc)
	}
	// Plugin game.details facts may be localized: drop the per-game hint
	// cache so the next look asks again in the new language.
	a.compatHints.mu.Lock()
	a.compatHints.m = nil
	a.compatHints.mu.Unlock()
	// Steam store metadata is cached per language (see internal/steam), so
	// both languages can coexist; nothing to drop there. The sideloaded
	// game-info disk cache keys on game id + link + language (see
	// app_gameinfo.go) and refetches on a mismatch. Compat ratings
	// (ProtonDB, Deck, wiki) are not localized and stay in the compat-facts cache.
}
