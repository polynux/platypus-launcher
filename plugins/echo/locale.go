package main

// Localized plugin text (issue #39, phase 4): the recommended path is that
// the plugin returns already-localized text for the locale it was given in
// initialize {locale} (protocol 1.14), kept current through the
// locale.changed {locale} notification. Echo keeps a per-plugin
// locales/<lang>.json catalog (embedded) and falls back to English key by
// key — the helper pattern docs/plugins/protocol.md recommends to every
// plugin author. It localizes two strings to prove it end to end: the
// game.details summary and the auth instructions.

import (
	"embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFiles embed.FS

var (
	localeMu      sync.Mutex
	currentLocale = "en"
	localeTables  = map[string]map[string]string{}
)

func loadLocales() {
	entries, err := localeFiles.ReadDir("locales")
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		lang := strings.TrimSuffix(name, ".json")
		data, err := localeFiles.ReadFile("locales/" + name)
		if err != nil {
			continue
		}
		var table map[string]string
		if err := json.Unmarshal(data, &table); err != nil {
			continue
		}
		localeTables[strings.ToLower(lang)] = table
	}
}

func setLocale(tag string) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "en"
	}
	localeMu.Lock()
	currentLocale = tag
	localeMu.Unlock()
}

func localeBase() string {
	localeMu.Lock()
	tag := currentLocale
	localeMu.Unlock()
	tag = strings.TrimSpace(tag)
	if i := strings.Index(tag, "-"); i >= 0 {
		tag = tag[:i]
	}
	return strings.ToLower(tag)
}

// text returns the localized string for key, falling back to English.
func text(key string) string {
	base := localeBase()
	if table, ok := localeTables[base]; ok {
		if v, ok := table[key]; ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	if table, ok := localeTables["en"]; ok {
		if v, ok := table[key]; ok {
			return v
		}
	}
	return key
}
