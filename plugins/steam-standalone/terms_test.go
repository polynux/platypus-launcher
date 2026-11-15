package steam_test

// Terms (protocol 1.11): the plugin lists a game's EULAs from PICS app info
// (common/eulas), read over the CM session the sizes already use. The fixture
// is a real Trackmania 2225070 app info excerpt. Steam records the acceptance
// itself, so the plugin offers no game.acceptTerms and Platypus keeps its own
// record.
//
// Reading terms is the only reason to log on to the CM here: a game page's
// game.details must not pay for it, and a terms read that can't happen (no
// login, a refused token, a CM that won't answer) fails rather than answering
// "no terms" — a gate that reads an empty list would install without the EULA.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/plugins"
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

// A game page's game.details must not log on to the CM: the page shows the
// store metadata even when the terms source (PICS) can't be reached, and the
// terms read then fails with the reason instead of answering "none".
func TestSteamDetailsNeedNoCM(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("logon_eresult", 20) // ServiceUnavailable
	game := types.Game{
		ID: stores.GameID(id, "4321"), Source: id, Store: "steam",
		StoreAppID: "4321", Title: "Standalone Game",
	}
	logons := func() int { return strings.Count(e.cmLog(), "logon ") }
	before := logons()
	d, err := e.store.Details(context.Background(), game)
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "A standalone game." {
		t.Fatalf("summary = %q, want the store's description", d.Summary)
	}
	if n := logons(); n != before {
		t.Fatalf("game.details logged on to the CM (%d logons before, %d after)", before, n)
	}
	_, err = e.store.Terms(context.Background(), "4321")
	if err == nil {
		t.Fatal("a terms read over a CM that refuses the logon went through")
	}
	if !strings.Contains(err.Error(), "couldn't read the terms") || !strings.Contains(err.Error(), "Steam app info") {
		t.Fatalf("terms error = %v, want it to say the app info read failed", err)
	}
}

// A logged-out plugin can't read the terms, and says so: the install asks for
// the login instead of installing without the EULA ever shown.
func TestSteamTermsLoggedOutIsAuthRequired(t *testing.T) {
	e := setup(t)
	if _, err := e.store.Terms(context.Background(), "4321"); !plugins.IsAuthRequired(err) {
		t.Fatalf("terms while logged out = %v, want -32001", err)
	}
}

// AccessDenied (15) at logon is the CM refusing a revoked token, not a CM
// failure: the caller must read it as "log in again", like client.go does.
func TestSteamTermsRefusedTokenIsAuthRequired(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("logon_eresult", 15) // AccessDenied
	if _, err := e.store.Terms(context.Background(), "4321"); !plugins.IsAuthRequired(err) {
		t.Fatalf("terms with a refused token = %v, want -32001", err)
	}
}
