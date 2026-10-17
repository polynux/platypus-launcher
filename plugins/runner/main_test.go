package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/testenv"
	"platypus/internal/types"
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
