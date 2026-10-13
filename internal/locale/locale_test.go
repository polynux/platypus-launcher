package locale

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type fixtureCase struct {
	Setting string   `json:"setting"`
	System  []string `json:"system"`
	UI      string   `json:"ui"`
	Locale  string   `json:"locale"`
}

func loadFixtures(t *testing.T) []fixtureCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []fixtureCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// The shared fixture table with the TS resolveLanguage: the Go side checks
// the locale column (the TS side checks ui in locale-resolve.test.ts).
func TestResolveMatchesFixtures(t *testing.T) {
	for _, c := range loadFixtures(t) {
		if got := Resolve(c.Setting, c.System); got != c.Locale {
			t.Errorf("Resolve(%q, %v) = %q, want %q", c.Setting, c.System, got, c.Locale)
		}
	}
}

func TestResolveFallsBackToEnglish(t *testing.T) {
	if got := Resolve("", []string{"!", " "}); got != "en" {
		t.Fatalf("Resolve with garbage system = %q, want en", got)
	}
	if got := Resolve("!", []string{"fr"}); got != "fr" {
		t.Fatalf("Resolve with garbage setting = %q, want fr", got)
	}
	if got := Resolve("", nil); got != "en" {
		t.Fatalf("Resolve empty = %q, want en", got)
	}
}

func TestSteamLanguageMapping(t *testing.T) {
	cases := map[string]string{
		"en":    "english",
		"en-US": "english",
		"fr":    "french",
		"fr-FR": "french",
		"fr-CA": "french",
		"de":    "german",
		"pt-BR": "brazilian",
		"pt":    "portuguese",
		"zh-CN": "schinese",
		"zh-TW": "tchinese",
		"es-MX": "latam",
		"es-ES": "spanish",
		"ja":    "japanese",
		"xx":    "english",
		"":      "english",
	}
	for in, want := range cases {
		if got := SteamLanguage(in); got != want {
			t.Errorf("SteamLanguage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGOGLocaleMapping(t *testing.T) {
	cases := map[string]string{
		"en":    "en-US",
		"fr":    "fr-FR",
		"fr-FR": "fr-FR",
		"de":    "de-DE",
		"pt-BR": "pt-BR",
		"xx":    "en-US",
		"":      "en-US",
	}
	for in, want := range cases {
		if got := GOGLocale(in); got != want {
			t.Errorf("GOGLocale(%q) = %q, want %q", in, got, want)
		}
	}
}
