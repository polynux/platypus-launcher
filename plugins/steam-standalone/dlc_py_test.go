package steam_test

// Unit-level checks of the DLC selection (protocol 1.5): explicit choices,
// the remembered selection, the owned default, and the install ledger. The
// CM and Store answer from mocks; nothing reaches Steam.
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
catalog = {"dlc": [{"appid": 100, "depots": [12]}, {"appid": 101, "depots": [13]}],
           "depots": {"11": 0, "12": 100, "13": 101}, "base": [11]}
hs.dl_run = lambda sub, req, *a, **k: dict(catalog) if sub == "dlc" else (_ for _ in ()).throw(AssertionError("no " + sub))
hs.owned_games = lambda: {"response": {"games": [{"appid": 4321}, {"appid": 100}]}}
hs.store_app = lambda app, fetch=True: {"100": {"name": "Hunt", "type": "dlc"}, "101": {"name": "Gold", "type": "dlc"}}.get(str(app), {})
`

// An explicit choice passes through; unknown and unowned add-ons are
// refused; the remembered selection needs no CM round trip at all.
func TestDLCResolveSelection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", dir)
	got := pyRun(t, dlcMocks+`
calls = []
orig = hs.dl_run
def counting(sub, req, *a, **k):
    calls.append(sub)
    return orig(sub, req, *a, **k)
hs.dl_run = counting
print(hs.resolve_dlc_selection("4321", {"options": {"dlc": ["100"]}}))
print(hs.resolve_dlc_selection("4321", {}))
print([c for c in calls])
for bad, why in [({"options": {"dlc": ["999"]}}, "unknown"), ({"options": {"dlc": ["101"]}}, "unowned"), ({"options": {"dlc": "100"}}, "shape")]:
    try:
        hs.resolve_dlc_selection("4321", bad)
        print("accepted " + why)
    except hs.RPCError as e:
        print("refused " + why)
`)
	want := "['100']\n['100']\n['dlc', 'dlc']\nrefused unknown\nrefused unowned\nrefused shape"
	if got != want {
		t.Fatalf("resolve = %q, want %q", got, want)
	}
	// The remembered selection answers without touching the CM: a strict
	// mock fails the test if dl_run runs.
	writeDLCInstalled(t, dir, map[string]any{"4321": map[string]any{"dlc": []string{}}})
	got = pyRun(t, dlcMocks+`
hs.dl_run = lambda *a, **k: (_ for _ in ()).throw(AssertionError("CM touched"))
print(hs.resolve_dlc_selection("4321", {}))
`)
	if got != "[]" {
		t.Fatalf("stored selection = %q, want []", got)
	}
}

// game.options marks owned/installed and links the store page of unowned
// add-ons; a game without DLC has no groups.
func TestDLCOptionsShape(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", dir)
	writeDLCInstalled(t, dir, map[string]any{"4321": map[string]any{
		"dlc":        []string{"100"},
		"depotDlc":   map[string]string{"12": "100", "13": "101"},
		"archiveRef": map[string]any{"depots": map[string]string{"11": "1", "12": "2"}},
		"channel":    "public",
		"build":      "9",
	}})
	got := pyRun(t, dlcMocks+`
import json
print(json.dumps(hs.game_options({"id": "4321"}), sort_keys=True))
hs.dl_run = lambda sub, req, *a, **k: {"dlc": [], "depots": {}, "base": [11]}
print(json.dumps(hs.game_options({"id": "4321"}), sort_keys=True))
try:
    hs.game_options({})
except hs.RPCError as e:
    print("params refused")
`)
	want := `{"groups": [{"dlc": true, "id": "dlc", "kind": "multi", "label": "Add-ons", "values": [` +
		`{"id": "100", "installed": true, "label": "Hunt", "owned": true}, ` +
		`{"id": "101", "label": "Gold", "owned": false, "storeUrl": "https://store.steampowered.com/app/101/"}]}]}` +
		"\n" + `{"groups": []}` + "\nparams refused"
	if got != want {
		t.Fatalf("options = %s, want %s", got, want)
	}
}

// The remembered depot map filters the verify pins: a deselected add-on's
// depots leave the target, unmapped depots stay in.
func TestDLCVerifyFilter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", dir)
	writeDLCInstalled(t, dir, map[string]any{"4321": map[string]any{
		"dlc":        []string{"100"},
		"depotDlc":   map[string]string{"12": "100", "13": "101"},
		"archiveRef": map[string]any{"depots": map[string]string{"11": "1", "12": "2", "13": "3", "14": "4"}},
	}})
	got := pyRun(t, dlcMocks+`
import json
print(json.dumps(hs.verify_version("4321"), sort_keys=True))
`)
	want := `{"target": {"archiveRef": {"depots": {"11": "1", "12": "2", "14": "4"}}}}`
	if got != want {
		t.Fatalf("verify target = %s, want %s", got, want)
	}
}
