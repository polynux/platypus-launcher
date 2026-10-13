package main

import (
	"strings"
	"testing"
)

// The echo reference plugin returns French text under fr, English otherwise,
// falling back to English key by key.
func TestEchoLocaleText(t *testing.T) {
	loadLocales()
	setLocale("en")
	if got := text("details_summary"); !strings.Contains(got, "tiny fake game") {
		t.Fatalf("en summary = %q", got)
	}
	setLocale("fr-FR")
	if got := text("details_summary"); !strings.Contains(got, "faux jeu") {
		t.Fatalf("fr summary = %q", got)
	}
	if got := text("auth_instructions"); !strings.Contains(got, "connexion") {
		t.Fatalf("fr instructions = %q", got)
	}
	// Unknown locales and unknown keys fall back to English / the key.
	setLocale("de")
	if got := text("details_summary"); !strings.Contains(got, "tiny fake game") {
		t.Fatalf("de summary = %q, want English fallback", got)
	}
	setLocale("fr")
	if got := text("no_such_key"); got != "no_such_key" {
		t.Fatalf("unknown key = %q, want the key", got)
	}
}
