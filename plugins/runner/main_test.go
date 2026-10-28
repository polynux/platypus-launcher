package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"platypus/internal/dataplug"
	"platypus/internal/plugins"
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

// The core models only the settings it launches with (types.GameSettings). The
// runner's choices are plugin keys: none of them may be a core field, so the core
// stays free of Wine and Proton knowledge. A new core field is classed here on
// purpose.
func TestRunnerChoicesAreNotCoreSettings(t *testing.T) {
	t.Parallel()
	core := map[string]bool{
		"runner_id": true, "wine_prefix": true, "launch_arguments": true, "env_vars": true,
		"use_gamemode": true, "use_mangohud": true, "use_gamescope": true,
		"gamescope_w": true, "gamescope_h": true, "gamescope_r": true, "gamescope_args": true,
		"eac_runtime": true, "battleye_runtime": true, "working_dir": true, "run_mode": true,
	}
	gs := reflect.TypeOf(types.GameSettings{})
	for i := range gs.NumField() {
		key := strings.Split(gs.Field(i).Tag.Get("json"), ",")[0]
		if !core[key] {
			t.Errorf("GameSettings.%s (%q) is not a core setting: a runner choice belongs under runner.options", gs.Field(i).Name, key)
		}
	}
	// A key the schema offers that the core models is a core setting on purpose.
	// Every other offered key is a runner choice, stored under its own name.
	offered := 0
	for _, g := range runnerSchema("")["groups"].([]any) {
		for _, f := range g.(map[string]any)["fields"].([]any) {
			offered++
			_ = f.(map[string]any)["id"].(string)
		}
	}
	if offered < 20 {
		t.Errorf("runner.options offers %d choices, want the full set", offered)
	}
}

// runner.prefix answers from the game, its settings and the disk: the folder
// its saves are read from, and the login inside it.
func TestRunnerPrefixFollowsTheLaunchLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	folder := filepath.Join(home, "prefixes")
	game := types.Game{ID: "g1", Title: "G"}
	prefixOf := func(g types.Game, runnerID string) (string, string) {
		t.Helper()
		resp := handle(request("runner.prefix", map[string]any{"game": g, "runnerId": runnerID, "prefixesFolder": folder}))
		if resp.Error != nil {
			t.Fatalf("runner.prefix: %+v", resp.Error)
		}
		out := resp.Result.(map[string]any)
		dir, _ := out["dir"].(string)
		user, _ := out["user"].(string)
		return dir, user
	}

	// "missing" names no installed runner, so the layout on disk decides (this
	// host may have system wine, which "auto" would pick).
	if dir, user := prefixOf(game, "missing"); dir != filepath.Join(folder, "g1") || user != "" {
		t.Fatalf("no runner: %q, %q; want the folder of the game, no login", dir, user)
	}

	// A Proton-made prefix (pfx folder) with no runner installed is still Proton's.
	if err := os.MkdirAll(filepath.Join(folder, "g1", "pfx", "drive_c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if dir, user := prefixOf(game, "missing"); dir != filepath.Join(folder, "g1", "pfx") || user != "steamuser" {
		t.Fatalf("pfx layout, no runner: %q, %q; want pfx and steamuser", dir, user)
	}

	// A pinned prefix that already is the pfx folder is not nested again (#60).
	pinned := types.Game{ID: "g2", Title: "P", Settings: types.GameSettings{WinePrefix: filepath.Join(home, "pinned", "pfx")}}
	if dir, _ := prefixOf(pinned, "auto"); dir != filepath.Join(home, "pinned", "pfx") {
		t.Fatalf("pinned pfx = %q, want it unchanged", dir)
	}

	// An installed Proton build: its prefix is the pfx folder, run as steamuser.
	proton := filepath.Join(home, ".local/share/Steam/compatibilitytools.d", "FakeTest")
	if err := os.MkdirAll(proton, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proton, "proton"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	protonID := ""
	for _, r := range handle(request("runner.list", map[string]any{})).Result.(map[string]any)["runners"].([]plugins.RunnerInfo) {
		if strings.HasSuffix(r.Path, "proton") {
			protonID = r.ID
		}
	}
	if protonID == "" {
		t.Fatal("the fake Proton build is not listed")
	}
	if dir, user := prefixOf(game, protonID); dir != filepath.Join(folder, "g1", "pfx") || user != "steamuser" {
		t.Fatalf("proton: %q, %q; want pfx and steamuser", dir, user)
	}

	// A native game has no prefix at all.
	native := types.Game{ID: "g3", Title: "N", Native: true}
	resp := handle(request("runner.prefix", map[string]any{"game": native, "runnerId": "auto", "prefixesFolder": folder}))
	if out, _ := resp.Result.(map[string]any); resp.Error != nil || len(out) != 0 {
		t.Fatalf("native: %+v, %+v; want an empty result", resp.Result, resp.Error)
	}
}

// A game a store client started that ended too soon: the Proton log decides the
// hints. Without a log since the launch, the one hint names where it appears.
func TestClientCrashReadsTheProtonLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PROTON_LOG_DIR", "")
	game := types.Game{ID: "steam_620", Source: "steam", Store: "steam", StoreAppID: "620"}
	since := time.Now()
	logPath := filepath.Join(home, "steam-620.log")
	answer := func() (string, []types.CrashHint) {
		t.Helper()
		resp := handle(request("runner.clientCrash", map[string]any{"game": game, "since": since}))
		if resp.Error != nil {
			t.Fatalf("runner.clientCrash: %+v", resp.Error)
		}
		raw, _ := json.Marshal(resp.Result)
		var out struct {
			Log   string            `json:"log"`
			Hints []types.CrashHint `json:"hints"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out.Log, out.Hints
	}

	// No log yet: the hint says to turn it on, and where it will appear.
	if log, hints := answer(); log != logPath || len(hints) != 1 || hints[0].Rule != "protonLog" || hints[0].Param != logPath {
		t.Fatalf("no log: %q, %+v; want the protonLog hint for %s", log, hints, logPath)
	}

	// A log from before the launch is not this run's.
	if err := os.WriteFile(logPath, []byte("err:module:import_dll Library vcruntime140_1.dll not found"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := since.Add(-time.Hour)
	if err := os.Chtimes(logPath, old, old); err != nil {
		t.Fatal(err)
	}
	if _, hints := answer(); len(hints) != 1 || hints[0].Rule != "protonLog" {
		t.Fatalf("a log from before the launch was read: %+v", hints)
	}

	// A log written since the launch is read like any other, with the usual hints.
	if err := os.Chtimes(logPath, since.Add(time.Second), since.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, hints := answer(); len(hints) == 0 || hints[0].Verb != "vcrun2022" {
		t.Fatalf("the Proton log's hints = %+v, want the vcrun2022 fix", hints)
	}
}
