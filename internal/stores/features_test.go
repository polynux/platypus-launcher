package stores

import (
	"slices"
	"testing"
)

func TestGameFeaturesAbsentMeansEveryRuntimeCapability(t *testing.T) {
	t.Setenv("ECHO_OFF", "discard")
	e := newEchoEnv(t)
	e.login(t)
	lib, err := e.store.Library(e.ctx(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"library", "install", "launch", "auth", "cloud-saves", "versions", "verify", "move"}
	for _, g := range lib {
		if !slices.Equal(g.Features, want) {
			t.Fatalf("%s features = %v, want every runtime capability %v", g.StoreAppID, g.Features, want)
		}
	}
}

func TestGameFeaturesNarrowAndAreClampedToTheRuntimeSet(t *testing.T) {
	t.Setenv("ECHO_OFF", "launch")
	// tiny: a subset, with a capability that is off and one Hydra never heard of.
	// slow: explicitly none. The other games say nothing.
	t.Setenv("ECHO_GAME_FEATURES", `{"tiny":["install","launch","hologram","install"],"slow":[]}`)
	e := newEchoEnv(t)
	e.login(t)
	lib, err := e.store.Library(e.ctx(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := e.game(t, "tiny"); !slices.Equal(got.Features, []string{"install"}) {
		t.Fatalf("tiny features = %v", got.Features)
	}
	slow := e.game(t, "slow")
	if slow.Features == nil || len(slow.Features) != 0 {
		t.Fatalf("an explicit empty list must stay empty (not nil, which means all): %#v", slow.Features)
	}
	if got := e.game(t, "flaky"); len(got.Features) != 8 { // library, install, auth, discard, cloud-saves, versions, verify, move
		t.Fatalf("flaky features = %v", got.Features)
	}
	_ = lib
}

func TestInstallableFollowsTheRuntimeSet(t *testing.T) {
	t.Setenv("ECHO_OFF", "install")
	e := newEchoEnv(t)
	e.login(t)
	if _, err := e.store.Library(e.ctx(t)); err != nil {
		t.Fatal(err)
	}
	if g := e.game(t, "tiny"); g.Installable {
		t.Fatalf("install is off at runtime but the game is installable: %+v", g)
	}
}

func TestDetailsOfASwitchedOffLibraryCapabilityIsEmptyNotAnError(t *testing.T) {
	t.Setenv("ECHO_MISSING", "game.details")
	e := newEchoEnv(t)
	e.login(t)
	lib, _ := e.store.Library(e.ctx(t))
	tiny := e.game(t, "tiny")
	for i := 0; i < 2; i++ {
		d, err := e.store.Details(e.ctx(t), tiny)
		if err != nil || d == nil || d.Summary != "" {
			t.Fatalf("details with game.details missing: %+v, %v", d, err)
		}
	}
	_ = lib
}
