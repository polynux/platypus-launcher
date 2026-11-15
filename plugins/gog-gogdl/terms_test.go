package gog_test

// Terms (protocol 1.11): GOG exposes none. The live check of 2026-11-15
// (read-only, Cursed to Golf 1930721905, fixtures in testdata) found no
// EULA/terms/licence data in either endpoint the plugin can use, so the plugin
// lists no terms in game.details and doesn't declare the capability. Platypus
// then gates nothing on GOG installs.

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// useProductFixture points the plugin's API hook at the captured product
// response instead of setup's inline document.
func useProductFixture(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile("testdata/product_1930721905.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("PLATYPUS_GOG_API_FILE"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// termsKey matches the key names a store would put EULA data under. Store
// names are camelCase or snake_case, so a case-insensitive substring match is
// enough; it can't match account or catalogue fields ("license" appears in
// neither endpoint).
var termsKey = regexp.MustCompile(`(?i)eula|terms|licen[cs]e|agreement`)

// noTermKeys walks a decoded JSON document and reports the paths whose key
// names a term (empty for both captured responses).
func noTermKeys(t *testing.T, doc map[string]any) []string {
	t.Helper()
	var hits []string
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				p := path + "/" + k
				if termsKey.MatchString(k) {
					hits = append(hits, p)
				}
				walk(p, child)
			}
		case []any:
			for _, child := range x {
				walk(path, child)
			}
		}
	}
	walk("", doc)
	return hits
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestGogGameDetailsCarryNoTerms drives game.details with the captured product
// response: the metadata is there and no terms are listed.
func TestGogGameDetailsCarryNoTerms(t *testing.T) {
	m, _, _, _ := setup(t)
	login(t, m)
	useProductFixture(t)
	var d map[string]any
	if err := call(t, m, "game.details", map[string]any{"id": "1930721905"}, &d); err != nil {
		t.Fatal(err)
	}
	if _, ok := d["terms"]; ok {
		t.Fatalf("game.details listed terms: %v", d["terms"])
	}
	if summary, _ := d["summary"].(string); !strings.Contains(summary, "Cursed to Golf") {
		t.Fatalf("details didn't come from the fixture: %+v", d)
	}
}

// TestGogFixturesHaveNoTerms pins the live finding itself: neither the account
// gameDetails (where a EULA would live) nor the product response names a term,
// and the account's text fields are empty.
func TestGogFixturesHaveNoTerms(t *testing.T) {
	account := readJSON(t, "testdata/account_game_details_1930721905.json")
	product := readJSON(t, "testdata/product_1930721905.json")
	for name, doc := range map[string]map[string]any{"account": account, "product": product} {
		if hits := noTermKeys(t, doc); len(hits) > 0 {
			t.Fatalf("%s capture has term keys: %v", name, hits)
		}
	}
	if text, _ := account["textInformation"].(string); text != "" {
		t.Fatalf("account gameDetails has textInformation: %q", text)
	}
	if messages, _ := account["messages"].([]any); len(messages) != 0 {
		t.Fatalf("account gameDetails has messages: %v", messages)
	}
}

// TestGogManifestOffersNoTerms keeps the manifest honest with the finding: the
// plugin can't list terms, so it doesn't promise the capability.
func TestGogManifestOffersNoTerms(t *testing.T) {
	var manifest struct {
		Capabilities []string `json:"capabilities"`
	}
	raw, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, c := range manifest.Capabilities {
		if c == "terms" {
			t.Fatalf("manifest declares terms without a terms source: %v", manifest.Capabilities)
		}
	}
}
