package app

import (
	"testing"

	"platypus/internal/types"
)

// A library scan must never ask a plugin for per-game details: with a real
// library that is hundreds of slow game.details calls, and the scan never ends.
// The game page asks; the library only reads what that left in the cache.
func TestLibraryScanAsksNoPluginForGameDetails(t *testing.T) {
	env, _ := compatEnv(t, `["1.0","1.1","1.10"]`)
	before := env.calls(t)["game.details"]
	games := []types.Game{env.game, env.game, env.game}
	env.a.applyFixes(games)
	if got := env.calls(t)["game.details"] - before; got != 0 {
		t.Fatalf("the library scan made %d game.details calls, want 0", got)
	}
	// After the game page has asked, the library uses the cached hint.
	if c := env.a.GetGameCompat(env.game); c.SteamAppID != "620" {
		t.Fatalf("game page AppID %q, want 620", c.SteamAppID)
	}
	asked := env.calls(t)["game.details"]
	env.a.applyFixes(games)
	if got := env.calls(t)["game.details"] - asked; got != 0 {
		t.Fatalf("the library asked again (%d calls) instead of using the cache", got)
	}
}
