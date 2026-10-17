package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/dataplug"
	"platypus/internal/testenv"
	"platypus/internal/types"
	"platypus/internal/winetricks"
)

func request(method string, params any) message {
	raw, _ := json.Marshal(params)
	return message{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: method, Params: raw}
}

func TestCrashHintsReturnsSteamClientLogFindings(t *testing.T) {
	got := handle(request("runner.crashHints", map[string]any{
		"game":     map[string]any{"id": "steam_620", "source": "steam", "store": "steam"},
		"runnerId": "auto", "log": `err:module:import_dll Library vcruntime140_1.dll not found`,
	}))
	if got.Error != nil {
		t.Fatal(got.Error)
	}
	var hints []types.CrashHint
	raw, _ := json.Marshal(got.Result)
	if err := json.Unmarshal(raw, &hints); err != nil {
		t.Fatal(err)
	}
	if len(hints) != 1 || hints[0].Verb != "vcrun2022" {
		t.Fatalf("crash hints = %+v, want the winetricks fix for Steam-client log", hints)
	}
}

func TestRunWinetricksActionForUmuGame(t *testing.T) {
	bin := testenv.SetupTestBin(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	pluginData := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", pluginData)
	umuDir := t.TempDir()
	umuPath := filepath.Join(umuDir, "umu-run")
	if err := os.WriteFile(umuPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", umuDir+string(os.PathListSeparator)+bin.Dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	compat := filepath.Join(home, ".local/share/Steam/compatibilitytools.d/FakeProton")
	if err := os.MkdirAll(filepath.Dir(compat), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "-r", bin.ProtonDir(), compat).CombinedOutput(); err != nil {
		t.Fatalf("copy fake Proton: %v: %s", err, out)
	}
	prefix := filepath.Join(home, "prefixes", "steam_620")
	if err := os.MkdirAll(filepath.Join(prefix, "pfx"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := actionParams{ID: "winetricks", Verb: "d3dx9", RunnerID: "umu", PrefixesFolder: filepath.Dir(prefix),
		Game: types.Game{ID: "steam_620", Title: "Game", Store: "steam", StoreAppID: "620", Settings: types.GameSettings{WinePrefix: prefix}}}
	if err := runWinetricks(p); err != nil {
		t.Fatalf("runWinetricks: %v", err)
	}
	wantPrefix := filepath.Join(prefix, "pfx")
	wantWine := filepath.Join(compat, "files/bin/wine")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got := bin.Read(t)
		if strings.Contains(got, "WINEPREFIX="+wantPrefix) && strings.Contains(got, "WINE="+wantWine) && strings.Contains(got, "d3dx9") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("testbin call log = %q, want umu winetricks to use %s and %s", bin.Read(t), wantPrefix, wantWine)
}

// A native game runs with no runner and no prefix: the wrap result must not
// name one, or cloud saves would treat the game as Windows.
func TestWrapNativeGameHasNoRunnerOrPrefix(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resp := handle(request("runner.wrap", map[string]any{
		"spec": map[string]any{"command": []string{"/games/a"}, "env": map[string]string{}, "cwd": "/games", "needsWine": false},
		"game": map[string]any{"id": "g", "title": "G"}, "runnerId": "auto",
	}))
	if resp.Error != nil {
		t.Fatalf("wrap: %+v", resp.Error)
	}
	out, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result = %T, want an object", resp.Result)
	}
	for _, k := range []string{"runner", "prefixDir", "prefixUser"} {
		if _, ok := out[k]; ok {
			t.Errorf("native wrap result has %q: %v", k, out[k])
		}
	}
}

// protonPrefix is the pfx folder of a Proton prefix; a prefix already named
// pfx is used as it is (issue #60).
func TestProtonPrefixNormalisesOnce(t *testing.T) {
	t.Parallel()
	if got := protonPrefix("/x"); got != "/x/pfx" {
		t.Errorf("protonPrefix(/x) = %q, want /x/pfx", got)
	}
	if got := protonPrefix("/x/pfx"); got != "/x/pfx" {
		t.Errorf("protonPrefix(/x/pfx) = %q, want /x/pfx", got)
	}
}

// Every verb the shipped fixes name must be a real winetricks verb, so a bad
// entry fails here instead of at install time. The data loader only checks a
// verb's shape: the runner owns the catalogue.
func TestBuiltinFixVerbsAreKnown(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, f := range dataplug.Builtin().Fixes {
		for _, w := range f.Winetricks {
			seen[w] = true
			if !winetricks.KnownVerb(w) {
				t.Errorf("%q lists unknown winetricks verb %q", f.Title, w)
			}
		}
	}
	if !seen["d3dx9"] || !seen["faudio"] {
		t.Fatalf("built-in verbs = %v, want the shipped d3dx9 and faudio entries covered", seen)
	}
}
