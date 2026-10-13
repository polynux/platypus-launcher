package onlineart

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// The Steam search/appdetails and GOG catalog carry the locale: French under
// fr, English by default.
func TestLocaleParams(t *testing.T) {
	var searchL, detailsL, gogLocale string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/storesearch/":
			searchL = r.URL.Query().Get("l")
			fmt.Fprint(w, `{"items":[]}`)
		case r.URL.Path == "/api/appdetails":
			detailsL = r.URL.Query().Get("l")
			fmt.Fprint(w, `{"10":{"success":true,"data":{"name":"X"}}}`)
		case r.URL.Path == "/v1/catalog":
			gogLocale = r.URL.Query().Get("locale")
			fmt.Fprint(w, `{"products":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Client{SearchBase: srv.URL, GOGBase: srv.URL}
	c.Locale = "fr-FR"
	if _, err := c.Search("jeu"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.AppInfo(10); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SearchGOG("jeu"); err != nil {
		t.Fatal(err)
	}
	if searchL != "french" || detailsL != "french" || gogLocale != "fr-FR" {
		t.Fatalf("fr params = search l=%q details l=%q gog locale=%q", searchL, detailsL, gogLocale)
	}

	c.Locale = ""
	var query url.Values
	_ = query
	if _, err := c.Search("game"); err != nil {
		t.Fatal(err)
	}
	if searchL != "english" {
		t.Fatalf("default search l = %q, want english", searchL)
	}
}
