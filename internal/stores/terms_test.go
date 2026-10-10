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
