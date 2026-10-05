package stores

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

type fakeStore string

func (f fakeStore) ID() string                                              { return string(f) }
func (fakeStore) Library(context.Context) ([]types.Game, error)             { return nil, nil }
func (fakeStore) Launch(context.Context, types.Game, bool) (*Launch, error) { return nil, nil }
func (fakeStore) Details(context.Context, types.Game) (*types.GameDetails, error) {
	return nil, nil
}

func TestRegistryGetBySource(t *testing.T) {
	r := NewRegistry(func() []Store { return []Store{fakeStore("epic-legendary")} }, NewSteam())
	if s, ok := r.Get(types.SourceSteam); !ok || s.ID() != "steam" {
		t.Fatalf("Get(steam) = %v, %v", s, ok)
	}
	if _, ok := r.Get("epic-legendary"); !ok {
		t.Fatal("plugin store not found")
	}
	if _, ok := r.Get(types.SourceCustom); ok {
		t.Fatal("custom games must have no store")
	}
}

func TestSteamLaunch(t *testing.T) {
	s := NewSteam()
	game := types.Game{Source: "steam", Store: types.StoreSteam, StoreAppID: "620"}
	l, err := s.Launch(context.Background(), game, false)
	if err != nil || l == nil || l.Client == nil || l.Client.Client != "Steam" || len(l.Client.Command) == 0 {
		t.Fatalf("installed Steam app should launch through the client, got %+v, %v", l, err)
	}

	// An executable override means Hydra runs the game itself.
	game.Executable = "/games/portal2/portal2.exe"
	if l, _ := s.Launch(context.Background(), game, false); l != nil {
		t.Fatalf("override should bypass the client, got %+v", l)
	}
}

func TestGameID(t *testing.T) {
	if got := GameID("epic-legendary", "Fortnite"); got != "p.epic-legendary.Fortnite" {
		t.Errorf("plain id: %s", got)
	}
	a, b := GameID("x", "a/b"), GameID("x", "a_b")
	if a == b || strings.ContainsAny(a, "/") {
		t.Errorf("unsafe ids must stay unique and path-safe: %s %s", a, b)
	}
	if GameID("steam", "620") == "steam_620" {
		t.Error("plugin ids must not collide with built-in Steam ids")
	}
}

func TestRedactArgs(t *testing.T) {
	got := RedactArgs([]string{"-AUTH_PASSWORD=abc", "-AUTH_TYPE=exchangecode", "-AUTH_PASSWORD="}, []string{"-AUTH_PASSWORD="})
	want := []string{"-AUTH_PASSWORD=***", "-AUTH_TYPE=exchangecode", "-AUTH_PASSWORD="}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %v", got)
	}
}

func TestValidateSpec(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "Games", "epic")
	game := filepath.Join(install, "Game")
	os.MkdirAll(game, 0o755)
	exe := filepath.Join(game, "game.exe")
	os.WriteFile(exe, []byte("MZ"), 0o644)
	native := filepath.Join(game, "run.sh")
	os.WriteFile(native, []byte("#!/bin/sh\n"), 0o755)
	outside := filepath.Join(root, "elsewhere.exe")
	os.WriteFile(outside, []byte("MZ"), 0o644)
	link := filepath.Join(game, "escape.exe")
	os.Symlink(outside, link)

	man := plugins.Manifest{ID: "epic-legendary", Name: "Epic", Requires: []plugins.Requirement{{Bin: "sh"}}}
	folders := []string{install}

	ok := []plugins.LaunchSpec{
		{Exe: exe, Args: []string{"-epicportal"}, NeedsWine: true, Env: map[string]string{"DXVK_ASYNC": "1"}},
		{Exe: native, Cwd: game},
		{Delegate: "store-client", Command: []string{"sh", "-c", "true"}, Find: &plugins.FindSpec{Cmdline: []string{"AppId=42"}}},
	}
	for i, spec := range ok {
		if _, err := ValidateSpec(spec, man, folders); err != nil {
			t.Errorf("ok[%d]: %v", i, err)
		}
	}

	bad := map[string]plugins.LaunchSpec{
		"exe outside":         {Exe: outside, NeedsWine: true},
		"symlink escape":      {Exe: link, NeedsWine: true},
		"relative exe":        {Exe: "Game/game.exe", NeedsWine: true},
		"cwd outside":         {Exe: exe, Cwd: root, NeedsWine: true},
		"native not runnable": {Exe: exe},
		"LD_PRELOAD":          {Exe: exe, NeedsWine: true, Env: map[string]string{"LD_PRELOAD": "/tmp/x.so"}},
		"dll overrides":       {Exe: exe, NeedsWine: true, Env: map[string]string{"WINEDLLOVERRIDES": "steam_api64=n"}},
		"PATH":                {Exe: exe, NeedsWine: true, Env: map[string]string{"PATH": "/tmp"}},
		"bad key":             {Exe: exe, NeedsWine: true, Env: map[string]string{"A=B": "1"}},
		"delegate other bin":  {Delegate: "store-client", Command: []string{"rm", "-rf", "/"}, Find: &plugins.FindSpec{Cmdline: []string{"AppId=42"}}},
		"delegate no find":    {Delegate: "store-client", Command: []string{"sh"}},
		"delegate weak find":  {Delegate: "store-client", Command: []string{"sh"}, Find: &plugins.FindSpec{Cmdline: []string{"a"}}},
		"unknown delegate":    {Delegate: "magic"},
	}
	for name, spec := range bad {
		if _, err := ValidateSpec(spec, man, folders); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// A folder the user allowed later works too.
	if _, err := ValidateSpec(plugins.LaunchSpec{Exe: outside, NeedsWine: true}, man, append(folders, root)); err != nil {
		t.Errorf("allowed folder: %v", err)
	}
}
