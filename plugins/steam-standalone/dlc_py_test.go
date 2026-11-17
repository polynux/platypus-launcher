package steam_test

// Unit-level checks of the DLC selection (protocol 1.5, fix round): the
// licence-based owned list with the owned-apps fallback, explicit choices,
// the remembered selection, the on-disk-plus-owned update rule (B1), the
// installed display rule, and the verify ledger. The CM and Store answer
// from mocks; nothing reaches Steam.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeDLCInstalled(t *testing.T, dir string, entry map[string]any) {
	t.Helper()
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "installed.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

const dlcMocks = `
hs.refresh_token = lambda: "tok"
catalog = {"dlc": [{"appid": 100, "depots": [12], "name": "Hunt", "sizeBytes": 2048},
                   {"appid": 101, "depots": [13]}],
           "depots": {"11": 0, "12": 100, "13": 101}, "base": [11],
           "by_os": {"linux": [11, 12], "windows": [11, 13]},
           "owned": ["100"]}
hs.dl_run = lambda sub, req, *a, **k: dict(catalog) if sub == "dlc" else (_ for _ in ()).throw(AssertionError("no " + sub))
hs.owned_games = lambda: (_ for _ in ()).throw(AssertionError("fallback not needed"))
hs.store_app = lambda *a, **k: (_ for _ in ()).throw(AssertionError("no store fetch"))
hs.store_cached = lambda app: {"101": {"name": "Cached Hunt"}}.get(str(app), {})
`

// An explicit choice passes through; unknown and unowned add-ons are
// refused. The owned list comes from the licences, not the owned-apps
// fallback (which must stay untouched here).
func TestDLCResolveSelection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", dir)
	got := pyRun(t, dlcMocks+`
print(hs.resolve_dlc_selection("4321", {"options": {"dlc": ["100"]}}, "install"))
for bad, why in [({"options": {"dlc": ["999"]}}, "unknown"), ({"options": {"dlc": ["101"]}}, "unowned"), ({"options": {"dlc": "100"}}, "shape")]:
    try:
        hs.resolve_dlc_selection("4321", bad, "install")
        print("accepted " + why)
    except hs.RPCError as e:
        print("refused " + why + ": " + str(e)[:60])
`)
	want := "['100']\nrefused unknown: Steam has no add-on 999 for app 4321\n" +
		"refused unowned: Cached Hunt is not owned: buy it in the store first\n" +
		"refused shape: invalid params: options.dlc is a list of value ids"
	if got != want {
		t.Fatalf("resolve = %q, want %q", got, want)
	}
	// A licence the CM never saw falls back to the owned-apps list.
	got = pyRun(t, dlcMocks+`
hs.dl_run = lambda sub, req, *a, **k: {"dlc": catalog["dlc"], "depots": catalog["depots"]}
hs.owned_games = lambda: {"response": {"games": [{"appid": 4321}, {"appid": 101}]}}
print(hs.resolve_dlc_selection("4321", {"options": {"dlc": ["101"]}}, "install"))
print(hs.resolve_dlc_selection("4321", {}, "install"))
`)
	if got != "['101']\n['101']" {
		t.Fatalf("fallback resolve = %q, want ['101'] twice", got)
	}
}

// A plain update never removes: the selection starts from what's on disk
// plus what's owned, even when ownership is gone (B1). An explicit choice
// still applies.
func TestDLCUpdateNeverRemoves(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", dir)
	// A pre-change install: archiveRef with the DLC depot, but no
	// remembered selection, and the DLC missing from ownership.
	writeDLCInstalled(t, dir, map[string]any{"4321": map[string]any{
		"archiveRef": map[string]any{"depots": map[string]string{"11": "1", "12": "2"}},
	}})
	got := pyRun(t, dlcMocks+`
hs.dl_run = lambda sub, req, *a, **k: dict(catalog, owned=[])
import os
print(hs.resolve_dlc_selection("4321", {}, "update"))
os.makedirs(hs.data_dir() + "/manifests/4321", exist_ok=True)
open(hs.data_dir() + "/manifests/4321/12_2", "w").write("x")
print(sorted(hs.saved_depot_ids("4321")))
print(hs.resolve_dlc_selection("4321", {}, "update"))
`)
	if got != "['100']\n['12']\n['100']" {
		t.Fatalf("update selection = %q, want on-disk DLC kept", got)
	}
}

// game.options marks owned/installed, ticks the store default, and never
// touches the Store: names come from PICS (or the disk cache).
func TestDLCOptionsShape(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", dir)
	writeDLCInstalled(t, dir, map[string]any{"4321": map[string]any{
		"dlc":        []string{"100", "101"},
		"depotDlc":   map[string]string{"12": "100", "13": "101"},
		"archiveRef": map[string]any{"depots": map[string]string{"11": "1", "12": "2"}},
		"channel":    "public",
		"build":      "9",
		"native":     true,
	}})
	got := pyRun(t, dlcMocks+`
import json
print(json.dumps(hs.game_options({"id": "4321"}), sort_keys=True))
hs.dl_run = lambda sub, req, *a, **k: {"dlc": [], "depots": {}, "base": []}
hs._dlc_cache.clear()
print(json.dumps(hs.game_options({"id": "4321"}), sort_keys=True))
try:
    hs.game_options({})
except hs.RPCError:
    print("params refused")
`)
	want := `{"groups": [{"default": ["100"], "dlc": true, "id": "dlc", "kind": "multi", "label": "Add-ons", "values": [` +
		`{"id": "100", "installed": true, "label": "Hunt", "owned": true, "sizeBytes": 2048}, ` +
		`{"id": "101", "label": "Cached Hunt", "owned": false, "storeUrl": "https://store.steampowered.com/app/101/"}]}]}` +
		"\n" + `{"groups": []}` + "\nparams refused"
	if got != want {
		t.Fatalf("options = %s, want %s", got, want)
	}
}

// The verify ledger passes the remembered selection to the helper (B2);
// the helper filters by it.
func TestDLCVerifyPassesSelection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", dir)
	writeDLCInstalled(t, dir, map[string]any{"4321": map[string]any{
		"dlc":        []string{"100"},
		"archiveRef": map[string]any{"depots": map[string]string{"11": "1", "12": "2"}},
	}})
	got := pyRun(t, dlcMocks+`
import json
print(json.dumps(hs.verify_version("4321"), sort_keys=True))
sent = []
orig = hs.dl_request
hs.dl_request = lambda p, e=None: sent.append(dict(p)) or orig(p, e)
hs.notify = lambda *a: None
hs.dl_run = lambda *a, **k: (_ for _ in ()).throw(AssertionError("no CM"))
try:
    hs.follow_verify("j1", "4321", "/tmp/g", False)
except AssertionError as ex:
    print("cm blocked:", ex)
print(json.dumps(sent[0].get("dlc")))
`)
	want := `{"target": {"archiveRef": {"depots": {"11": "1", "12": "2"}}}}` + "\ncm blocked: no CM\n" + `["100"]`
	if got != want {
		t.Fatalf("verify = %s, want %s", got, want)
	}
}
