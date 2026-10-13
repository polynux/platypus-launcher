// Package locale resolves the effective BCP 47 locale for plugins and
// stores (issue #39, phase 4): the launcher setting wins, else the first
// well-formed system language, else English. It mirrors the UI's
// resolveLanguage (apps/ui/src/lib/i18n.ts) in precedence — explicit setting,
// then the system list in order, then English — but returns the full tag
// (e.g. "fr-FR") for store APIs, while the UI maps it onto its shipped
// catalogs (e.g. "fr-FR" ships "fr"). The shared fixture table
// testdata/cases.json pins both sides so they can't drift: each case names
// the UI language (ui) and the core locale (locale).
package locale

import (
	"regexp"
	"strings"
)

var tagRe = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// Valid reports whether tag is a well-formed BCP 47 tag like "fr" or "fr-CA".
func Valid(tag string) bool {
	return tagRe.MatchString(strings.TrimSpace(tag))
}

// Canonical normalizes a well-formed tag to BCP 47 casing: the language
// lower, a two-letter region upper, other subtags as-is ("FR-fr" → "fr-FR").
func Canonical(tag string) string {
	tag = strings.TrimSpace(tag)
	parts := strings.Split(tag, "-")
	if len(parts) == 0 {
		return tag
	}
	parts[0] = strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) == 2 {
			parts[i] = strings.ToUpper(parts[i])
		} else if len(parts[i]) == 4 {
			parts[i] = strings.ToLower(parts[i])
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "-")
}

// Base returns the lower-cased language subtag ("fr-CA" → "fr").
func Base(tag string) string {
	tag = strings.TrimSpace(tag)
	if i := strings.Index(tag, "-"); i >= 0 {
		tag = tag[:i]
	}
	return strings.ToLower(tag)
}

// Resolve returns the effective locale: the explicit setting when
// well-formed, else the first well-formed system language, else "en".
func Resolve(setting string, system []string) string {
	if s := strings.TrimSpace(setting); s != "" && Valid(s) {
		return Canonical(s)
	}
	for _, t := range system {
		if Valid(t) {
			return Canonical(strings.TrimSpace(t))
		}
	}
	return "en"
}

// steamLanguages maps a base language to the Steam store's `l=` value.
// Unknown bases fall back to English.
var steamLanguages = map[string]string{
	"en": "english",
	"fr": "french",
	"de": "german",
	"es": "spanish",
	"it": "italian",
	"ja": "japanese",
	"ko": "korean",
	"pt": "portuguese",
	"ru": "russian",
	"pl": "polish",
	"nl": "dutch",
	"da": "danish",
	"no": "norwegian",
	"nb": "norwegian",
	"nn": "norwegian",
	"sv": "swedish",
	"fi": "finnish",
	"cs": "czech",
	"hu": "hungarian",
	"tr": "turkish",
	"ar": "arabic",
	"th": "thai",
	"uk": "ukrainian",
	"el": "greek",
	"bg": "bulgarian",
	"ro": "romanian",
	"vi": "vietnamese",
	"zh": "schinese",
	"he": "english",
	"ca": "spanish",
	"eu": "spanish",
	"gl": "spanish",
}

// SteamLanguage maps a BCP 47 locale to the Steam store's `l=` value
// ("fr-FR" → "french"), with special cases for regional variants.
func SteamLanguage(locale string) string {
	locale = strings.TrimSpace(locale)
	lower := strings.ToLower(locale)
	switch {
	case lower == "pt-br" || lower == "pt_br":
		return "brazilian"
	case lower == "es-419" || lower == "es-mx" || strings.HasPrefix(lower, "es-"):
		// Steam uses "latam" for Latin American Spanish.
		if lower != "es-es" && strings.HasPrefix(lower, "es-") {
			return "latam"
		}
	case lower == "zh-cn" || lower == "zh-sg" || lower == "zh-my":
		return "schinese"
	case lower == "zh-tw" || lower == "zh-hk" || lower == "zh-mo":
		return "tchinese"
	}
	if v, ok := steamLanguages[Base(locale)]; ok {
		return v
	}
	return "english"
}

// gogDefaultRegion maps a base language to its default GOG catalog locale.
var gogDefaultRegion = map[string]string{
	"en": "en-US",
	"fr": "fr-FR",
	"de": "de-DE",
	"es": "es-ES",
	"it": "it-IT",
	"pl": "pl-PL",
	"ru": "ru-RU",
	"ja": "ja-JP",
	"ko": "ko-KR",
	"pt": "pt-BR",
	"nl": "nl-NL",
	"cs": "cs-CZ",
	"hu": "hu-HU",
	"tr": "tr-TR",
	"ar": "ar-SA",
	"uk": "uk-UA",
	"zh": "zh-CN",
}

// GOGLocale maps a BCP 47 locale to the GOG catalog's `locale=` value
// ("fr" → "fr-FR"), preserving an explicit region when present.
func GOGLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" || !Valid(locale) {
		return "en-US"
	}
	c := Canonical(locale)
	if strings.Contains(c, "-") {
		return c
	}
	if v, ok := gogDefaultRegion[Base(c)]; ok {
		return v
	}
	return "en-US"
}
