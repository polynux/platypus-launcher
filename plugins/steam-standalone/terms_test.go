package steam_test

// Terms (protocol 1.11): the plugin lists a game's EULAs from PICS app info
// (common/eulas), read over the CM session the sizes already use. The fixture
// is a real Trackmania 2225070 app info excerpt. Steam records the acceptance
// itself, so the plugin offers no game.acceptTerms and Platypus keeps its own
// record.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/stores"
	"platypus/internal/types"
)

func TestSteamTermsFromPICS(t *testing.T) {
	e := setup(t)
	e.login()
	// The fake CM answers with depot-only app info by default: no terms.
	if terms, err := e.store.Terms(context.Background(), "4321"); err != nil || len(terms) != 0 {
		t.Fatalf("terms with no eulas = %+v, %v", terms, err)
	}
	fixture, err := os.ReadFile(filepath.Join("cmd", "platypus-steam-dl", "testdata", "appinfo_2225070.vdf"))
	if err != nil {
		t.Fatal(err)
	}
	e.setCMState("appinfo", string(fixture))

	terms, err := e.store.Terms(context.Background(), "4321")
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 1 {
		t.Fatalf("terms = %+v, want the fixture's EULA", terms)
	}
	got := terms[0]
	if got.ID != "2225070_eula_0" || got.Title != "Trackmania EULA" ||
		got.Version != "4" || got.URL != "https://store.steampowered.com//eula/2225070_eula_0" {
		t.Fatalf("term = %+v", got)
	}
	// Optional server-side acceptance: Steam keeps it itself, so the plugin
	// must answer "not offered" and Platypus keeps acceptance alone.
	if err := e.store.AcceptTerms(context.Background(), "4321", []types.TermRef{{ID: got.ID, Version: got.Version}}); !stores.IsNotOffered(err) {
		t.Fatalf("game.acceptTerms = %v, want -32601", err)
	}
}
