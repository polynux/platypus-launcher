package stores

import (
	"testing"

	"platypus/internal/plugins"
)

// A term without an id or a version can't be accepted, so it isn't listed;
// duplicates keep the first; an unknown title falls back to the id.
func TestTermsAreNormalized(t *testing.T) {
	got := termsFrom([]plugins.Term{
		{ID: "", Title: "no id", Version: "1"},
		{ID: "eula", Title: "  Licence  ", URL: "https://example.com/eula", Version: "2"},
		{ID: "eula", Title: "again", Version: "3"},
		{ID: "privacy", Version: "1", URL: "javascript:alert(1)"},
		{ID: "noversion", Title: "x"},
	})
	if len(got) != 2 {
		t.Fatalf("terms = %+v, want the licence and the privacy notice", got)
	}
	if got[0].Title != "Licence" || got[0].Version != "2" || got[0].URL != "https://example.com/eula" {
		t.Errorf("licence = %+v", got[0])
	}
	if got[1].Title != "privacy" || got[1].URL != "" {
		t.Errorf("privacy = %+v: the title falls back to the id and a non-web url is dropped", got[1])
	}
}

// Platypus reads a store's terms only when it asks for them: Terms sends
// `terms: true` (protocol 1.11) so a plugin can keep its expensive terms read
// for the install dialog and the gate, and a game page's Details must not.
func TestTermsAreReadOnlyWhenAsked(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	game := e.game(t, "licensed")
	var rec struct {
		DetailsAskedTerms bool `json:"detailsAskedTerms"`
	}
	asked := func() bool {
		t.Helper()
		if err := e.m.Call(e.ctx(t), "echo", "recorded", nil, &rec); err != nil {
			t.Fatal(err)
		}
		return rec.DetailsAskedTerms
	}

	if _, err := e.store.Details(e.ctx(t), game); err != nil {
		t.Fatal(err)
	}
	if asked() {
		t.Error("game.details asked for terms: the game page would wait on the store's terms read")
	}
	terms, err := e.store.Terms(e.ctx(t), game.StoreAppID)
	if err != nil || len(terms) == 0 {
		t.Fatalf("terms = %+v, %v", terms, err)
	}
	if !asked() {
		t.Error("Terms didn't send terms: true, so a plugin can't tell it apart from a game page")
	}
}
