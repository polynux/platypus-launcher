package steam

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The store lookup carries the Steam language for the locale, and the cache
// keys on app id + language so French and English never mix.
func TestStoreClientLocaleParamAndCache(t *testing.T) {
	var langs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		langs = append(langs, r.URL.Query().Get("l"))
		app := r.URL.Query().Get("appids")
		lang := r.URL.Query().Get("l")
		fmt.Fprintf(w, `{%q: {"success": true, "data": {"short_description": "desc-%s"}}}`,
			app, lang)
	}))
	defer srv.Close()

	c := NewStoreClient()
	c.Base = srv.URL
	c.HTTPClient = srv.Client()
	c.Locale = "fr-FR"
	d, err := c.Details("", "10")
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "desc-french" {
		t.Fatalf("Summary = %q, want desc-french", d.Summary)
	}
	c.Locale = "en"
	d, err = c.Details("", "10")
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "desc-english" {
		t.Fatalf("Summary = %q, want desc-english", d.Summary)
	}
	// Cached: one request per language, no more.
	if len(langs) != 2 || langs[0] != "french" || langs[1] != "english" {
		t.Fatalf("l params = %v, want [french english]", langs)
	}
	if _, err := c.Details("", "10"); err != nil {
		t.Fatal(err)
	}
	c.Locale = "fr-FR"
	if _, err := c.Details("", "10"); err != nil {
		t.Fatal(err)
	}
	if len(langs) != 2 {
		t.Fatalf("cached lookups hit the network: %v", langs)
	}
	if !strings.Contains(c.steamLang(), "english") && c.Locale == "en" {
		t.Fatal("unreachable")
	}
}
