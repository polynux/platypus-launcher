package steam_test

// End-to-end DLC (protocol 1.5, fix round) through the real plugin path.
// Ownership comes from the CM licences mapped through PICS package info
// (the owned-apps list stays a fallback); the Store is never asked for
// names. Covered: owned vs unowned display with the store default ticked,
// the selection changing the install set, deselecting removing the DLC
// depot's files, plain updates never removing (B1), verify skipping
// deselected depots (B2), denied depot keys skipping depots (S2),
// Windows-only DLC on a Linux install, and adding DLC while pinned.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/plugtest"
	"platypus/plugins/steam-standalone/fakesteam"
)

func writeDLCAPI(t *testing.T) {
	t.Helper()
	api := os.Getenv("PLATYPUS_STEAM_API_FILE")
	data, _ := json.Marshal(map[string]any{
		"response": map[string]any{"games": []any{
			map[string]any{"appid": 4321, "name": "Standalone Game"},
			map[string]any{"appid": 4322, "name": "Tool Thing"},
		}},
		"_test": map[string]any{
			"4321": map[string]any{"success": true, "data": map[string]any{
				"type": "game", "name": "Standalone Game",
				"short_description": "A standalone game."}},
			"4322": map[string]any{"success": true, "data": map[string]any{
				"type": "tool", "name": "Tool Thing"}},
		},
	})
	if err := os.WriteFile(api, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// dlcEnv points the fake CM at the base depot plus a DLC depot (dlcappid
// 100, listofdlc 100 amd 101) and licenses the package holding the base
// game and DLC 100. DLC names come from per-app PICS info.
func dlcEnv(t *testing.T, e *env) (base, dlc *fakesteam.Fixture) {
	t.Helper()
	base = fakesteam.Build(fakesteam.ReadKnobs())
	dlc = fakesteam.BuildDLC()
	cdn := fakesteam.StartCDN(t, base, dlc)
	fakesteam.WriteCMStateDLC(t, e.cmState, cdn, base, dlc)
	e.setCMState("licenses", []any{7000})
	e.setCMState("package_apps", map[string]any{"7000": []any{4321, 100}})
	e.setCMState("appinfo_100", `"common" { "name" "Hunt" }`)
	e.setCMState("appinfo_101", `"common" { "name" "Gold" }`)
	writeDLCAPI(t)
	return base, dlc
}

type dlcValue struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Owned     bool   `json:"owned"`
	Installed bool   `json:"installed"`
	StoreURL  string `json:"storeUrl"`
	SizeBytes int64  `json:"sizeBytes"`
}

type dlcGroup struct {
	ID      string     `json:"id"`
	Kind    string     `json:"kind"`
	DLC     bool       `json:"dlc"`
	Default []string   `json:"default"`
	Values  []dlcValue `json:"values"`
}

func (e *env) dlcOptions(app string) dlcGroup {
	e.t.Helper()
	var out struct {
		Groups []dlcGroup `json:"groups"`
	}
	if err := e.call("game.options", map[string]any{"id": app}, &out); err != nil {
		e.t.Fatalf("game.options: %v", err)
	}
	if len(out.Groups) != 1 || out.Groups[0].ID != "dlc" || !out.Groups[0].DLC {
		e.t.Fatalf("game.options groups = %+v, want one dlc group", out.Groups)
	}
	return out.Groups[0]
}

func dlcByID(t *testing.T, values []dlcValue, id string) dlcValue {
	t.Helper()
	for _, v := range values {
		if v.ID == id {
			return v
		}
	}
	t.Fatalf("no DLC %q in %+v", id, values)
	return dlcValue{}
}

// waitPluginJob waits for the plugin's job.done for jobID, failing on its
// job.failed: direct game.install/game.update calls run outside the queue.
// It returns the done params.
func (e *env) waitPluginJob(jobID string) map[string]any {
	e.t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		for _, raw := range e.notes.forMethod("job.failed") {
			var p struct {
				JobID string `json:"jobId"`
				Error string `json:"error"`
			}
			if json.Unmarshal(raw, &p) == nil && p.JobID == jobID {
				e.t.Fatalf("job %s failed: %s", jobID, p.Error)
			}
		}
		for _, raw := range e.notes.forMethod("job.done") {
			var p map[string]any
			if json.Unmarshal(raw, &p) == nil && p["jobId"] == jobID {
				return p
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	e.t.Fatalf("no job.done for %s", jobID)
	return nil
}

// waitPluginJobFailed waits for the plugin's job.failed for jobID and
// returns its error.
func (e *env) waitPluginJobFailed(jobID string) string {
	e.t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		for _, raw := range e.notes.forMethod("job.failed") {
			var p struct {
				JobID string `json:"jobId"`
				Error string `json:"error"`
			}
			if json.Unmarshal(raw, &p) == nil && p.JobID == jobID {
				return p.Error
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	e.t.Fatalf("no job.failed for %s", jobID)
	return ""
}

func (e *env) installedEntry(app string) map[string]any {
	e.t.Helper()
	raw, err := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
	if err != nil {
		e.t.Fatal(err)
	}
	var installed map[string]map[string]any
	if err := json.Unmarshal(raw, &installed); err != nil {
		e.t.Fatal(err)
	}
	return installed[app]
}

func (e *env) setInstalledEntry(app string, fn func(map[string]any)) {
	e.t.Helper()
	path := filepath.Join(e.pluginData(), "installed.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatal(err)
	}
	var installed map[string]map[string]any
	if err := json.Unmarshal(raw, &installed); err != nil {
		e.t.Fatal(err)
	}
	fn(installed[app])
	data, _ := json.Marshal(installed)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		e.t.Fatal(err)
	}
}

// checkDLCKeys holds game.options to the plugtest shape: only documented
// keys, and a default naming known values.
func checkDLCKeys(t *testing.T, g dlcGroup) {
	t.Helper()
	var raw map[string]any
	data, _ := json.Marshal(g)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for k := range raw {
		found := false
		for _, allow := range plugtest.OptionGroupKeys {
			if k == allow {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("game.options group has undocumented key %q", k)
		}
	}
	ids := map[string]bool{}
	for _, v := range g.Values {
		ids[v.ID] = true
		data, _ := json.Marshal(v)
		var vm map[string]any
		if err := json.Unmarshal(data, &vm); err != nil {
			t.Fatal(err)
		}
		for k := range vm {
			found := false
			for _, allow := range plugtest.OptionValueKeys {
				if k == allow {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("game.options value has undocumented key %q", k)
			}
		}
	}
	for _, id := range g.Default {
		if !ids[id] {
			t.Fatalf("game.options default names unknown value %q", id)
		}
	}
}

func TestSteamDLCOwnershipAndPerDLCInstall(t *testing.T) {
	e := setup(t)
	e.login()
	base, _ := dlcEnv(t, e)

	// Owned vs unowned display comes from the licences: 100 is licensed,
	// 101 only catalogued. Names come from PICS (the API file knows no
	// DLC), and the fresh default ticks the owned add-on.
	g := e.dlcOptions("4321")
	checkDLCKeys(t, g)
	if len(g.Values) != 2 {
		t.Fatalf("DLC values = %+v, want 100 and 101", g.Values)
	}
	if v := dlcByID(t, g.Values, "100"); !v.Owned || v.Installed || v.Label != "Hunt" || v.SizeBytes <= 0 {
		t.Fatalf("DLC 100 = %+v, want owned Hunt with a size, not installed", v)
	}
	if v := dlcByID(t, g.Values, "101"); v.Owned || v.Installed || v.Label != "Gold" || v.StoreURL == "" {
		t.Fatalf("DLC 101 = %+v, want unowned Gold with a store link", v)
	}
	if len(g.Default) != 1 || g.Default[0] != "100" {
		t.Fatalf("default = %v, want the owned add-on ticked", g.Default)
	}

	// The size counts the selection: everything owned by default, the
	// base game alone when the DLC is switched off.
	var full, bare struct {
		DownloadBytes int64 `json:"downloadBytes"`
		DiskBytes     int64 `json:"diskBytes"`
	}
	if err := e.call("game.installSize", map[string]any{"id": "4321"}, &full); err != nil {
		t.Fatalf("size: %v", err)
	}
	if err := e.call("game.installSize", map[string]any{
		"id": "4321", "options": map[string]any{"dlc": []string{}},
	}, &bare); err != nil {
		t.Fatalf("size without DLC: %v", err)
	}
	if full.DiskBytes != int64(base.DiskBytes+fakesteam.BuildDLC().DiskBytes) {
		t.Fatalf("full size disk=%d, want base plus DLC", full.DiskBytes)
	}
	if bare.DiskBytes != int64(base.DiskBytes) {
		t.Fatalf("bare size disk=%d, want base %d", bare.DiskBytes, base.DiskBytes)
	}

	// Install with the DLC selected: both depots land, the selection and
	// the depot map persist, and the DLC shows installed.
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "dlc-install", "installDir": e.folder,
		"options": map[string]any{"dlc": []string{"100"}},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	e.waitPluginJob("dlc-install")
	entry := e.installedEntry("4321")
	gameDir, _ := entry["path"].(string)
	for _, name := range []string{"game.sh", "dlc.dat"} {
		if _, err := os.Stat(filepath.Join(gameDir, name)); err != nil {
			t.Fatalf("missing %s after install: %v", name, err)
		}
	}
	if sel, _ := entry["dlc"].([]any); len(sel) != 1 || sel[0] != "100" {
		t.Fatalf("remembered selection = %v, want [100]", entry["dlc"])
	}
	if m, _ := entry["depotDlc"].(map[string]any); m["12"] != "100" {
		t.Fatalf("depot map = %v, want depot 12 on DLC 100", entry["depotDlc"])
	}
	if v := dlcByID(t, e.dlcOptions("4321").Values, "100"); !v.Installed {
		t.Fatalf("DLC 100 = %+v, want installed after install", v)
	}

	// Deselecting on update removes only the DLC depot's files: dlc.dat
	// goes, the shared launcher stays, and the selection persists.
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "dlc-remove", "installDir": gameDir,
		"options": map[string]any{"dlc": []string{}},
	}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	e.waitPluginJob("dlc-remove")
	if _, err := os.Stat(filepath.Join(gameDir, "dlc.dat")); !os.IsNotExist(err) {
		t.Fatalf("dlc.dat survived the deselect: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "game.sh")); err != nil {
		t.Fatalf("shared game.sh went with the DLC: %v", err)
	}
	if entry := e.installedEntry("4321"); len(entry["dlc"].([]any)) != 0 {
		t.Fatalf("selection after deselect = %v, want []", entry["dlc"])
	}
	if v := dlcByID(t, e.dlcOptions("4321").Values, "100"); v.Installed {
		t.Fatalf("DLC 100 = %+v, want not installed after deselect", v)
	}

	// A plain update keeps the remembered selection: nothing comes back.
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "dlc-keep", "installDir": gameDir,
	}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	e.waitPluginJob("dlc-keep")
	if _, err := os.Stat(filepath.Join(gameDir, "dlc.dat")); !os.IsNotExist(err) {
		t.Fatalf("plain update restored the deselected DLC: %v", err)
	}

	// Reselecting downloads the DLC depot again.
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "dlc-back", "installDir": gameDir,
		"options": map[string]any{"dlc": []string{"100"}},
	}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	e.waitPluginJob("dlc-back")
	if _, err := os.Stat(filepath.Join(gameDir, "dlc.dat")); err != nil {
		t.Fatalf("dlc.dat missing after reselect: %v", err)
	}

	// Unknown and unowned add-ons are refused synchronously.
	for _, tc := range []struct {
		name string
		dlc  []string
	}{
		{"unknown", []string{"999"}},
		{"unowned", []string{"101"}},
	} {
		err := e.call("game.install", map[string]any{
			"id": "4321", "jobId": "dlc-refuse-" + tc.name, "installDir": e.folder,
			"options": map[string]any{"dlc": tc.dlc},
		}, nil)
		if err == nil {
			t.Fatalf("%s DLC accepted, want -32602", tc.name)
		}
		if code := e.rpcCode(err); code != -32602 {
			t.Fatalf("%s DLC error code = %d, want -32602 (%v)", tc.name, code, err)
		}
	}
}

// A pre-DLC install (no remembered selection) whose DLC left ownership
// keeps its files on a plain update (B1).
func TestSteamDLCPlainUpdateNeverRemoves(t *testing.T) {
	e := setup(t)
	e.login()
	dlcEnv(t, e)
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "dlc-b1-install", "installDir": e.folder,
		"options": map[string]any{"dlc": []string{"100"}},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	e.waitPluginJob("dlc-b1-install")
	gameDir, _ := e.installedEntry("4321")["path"].(string)
	// Backdate the ledger: no remembered selection or depot map.
	e.setInstalledEntry("4321", func(entry map[string]any) {
		delete(entry, "dlc")
		delete(entry, "depotDlc")
	})
	// Drop the DLC from ownership: the package now holds the base game.
	e.setCMState("package_apps", map[string]any{"7000": []any{4321}})
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "dlc-b1-update", "installDir": gameDir,
	}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	e.waitPluginJob("dlc-b1-update")
	if _, err := os.Stat(filepath.Join(gameDir, "dlc.dat")); err != nil {
		t.Fatalf("plain update removed the DLC: %v", err)
	}
	if sel, _ := e.installedEntry("4321")["dlc"].([]any); len(sel) != 1 || sel[0] != "100" {
		t.Fatalf("selection after plain update = %v, want [100]", e.installedEntry("4321")["dlc"])
	}
}

// Verify and repair follow the remembered selection (B2): a deselected
// DLC is not checked and not redownloaded.
func TestSteamDLCVerifySkipsDeselected(t *testing.T) {
	e := setup(t)
	e.login()
	dlcEnv(t, e)
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "dlc-b2-install", "installDir": e.folder,
		"options": map[string]any{"dlc": []string{"100"}},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	e.waitPluginJob("dlc-b2-install")
	gameDir, _ := e.installedEntry("4321")["path"].(string)
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "dlc-b2-remove", "installDir": gameDir,
		"options": map[string]any{"dlc": []string{}},
	}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	e.waitPluginJob("dlc-b2-remove")
	if err := e.call("game.verify", map[string]any{
		"id": "4321", "jobId": "dlc-b2-check", "installDir": gameDir,
	}, nil); err != nil {
		t.Fatalf("verify: %v", err)
	}
	done := e.waitPluginJob("dlc-b2-check")
	vfy, _ := done["verify"].(map[string]any)
	num := func(k string) int {
		f, _ := vfy[k].(float64)
		return int(f)
	}
	if num("checked") != 1 || num("damaged") != 0 || num("missing") != 0 {
		t.Fatalf("verify covered the deselected depot: %+v", vfy)
	}
	if err := e.call("game.verify", map[string]any{
		"id": "4321", "jobId": "dlc-b2-repair", "installDir": gameDir, "repair": true,
	}, nil); err != nil {
		t.Fatalf("repair: %v", err)
	}
	e.waitPluginJob("dlc-b2-repair")
	if _, err := os.Stat(filepath.Join(gameDir, "dlc.dat")); !os.IsNotExist(err) {
		t.Fatalf("repair restored the deselected DLC: %v", err)
	}
}

// A denied depot key skips the depot instead of failing the job (S2): the
// game installs without it, and the DLC never shows installed.
func TestSteamDLCDeniedKeySkipsDepot(t *testing.T) {
	e := setup(t)
	e.login()
	dlcEnv(t, e)
	e.setCMState("depot_key_deny", []any{12})
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "dlc-deny", "installDir": e.folder,
		"options": map[string]any{"dlc": []string{"100"}},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	e.waitPluginJob("dlc-deny")
	gameDir, _ := e.installedEntry("4321")["path"].(string)
	if _, err := os.Stat(filepath.Join(gameDir, "game.sh")); err != nil {
		t.Fatalf("base file missing after denied DLC key: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "dlc.dat")); !os.IsNotExist(err) {
		t.Fatalf("denied DLC depot downloaded anyway: %v", err)
	}
	if v := dlcByID(t, e.dlcOptions("4321").Values, "100"); v.Installed {
		t.Fatalf("DLC 100 = %+v, want not installed after a denied key", v)
	}
}

// The retire pass leaves files alone when the key fails (S2): a denied
// DLC depot keeps its files and forgets its manifests instead of failing
// the update.
func TestSteamDLCRetireDeniedKeyKeepsFiles(t *testing.T) {
	e := setup(t)
	e.login()
	base := fakesteam.Build(fakesteam.ReadKnobs())
	enc := fakesteam.BuildDLCEncrypted()
	cdn := fakesteam.StartCDN(t, base, enc)
	fakesteam.WriteCMStateDLC(t, e.cmState, cdn, base, enc)
	e.setCMState("licenses", []any{7000})
	e.setCMState("package_apps", map[string]any{"7000": []any{4321, 100}})
	e.setCMState("appinfo_100", `"common" { "name" "Hunt" }`)
	e.setCMState("appinfo_101", `"common" { "name" "Gold" }`)
	writeDLCAPI(t)
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "dlc-enc-install", "installDir": e.folder,
		"options": map[string]any{"dlc": []string{"100"}},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	e.waitPluginJob("dlc-enc-install")
	gameDir, _ := e.installedEntry("4321")["path"].(string)
	e.setCMState("depot_key_deny", []any{12})
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "dlc-enc-remove", "installDir": gameDir,
		"options": map[string]any{"dlc": []string{}},
	}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	e.waitPluginJob("dlc-enc-remove")
	if _, err := os.Stat(filepath.Join(gameDir, "dlc.dat")); err != nil {
		t.Fatalf("denied DLC files must stay: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(e.pluginData(), "manifests", "4321", "12_*"))
	if len(matches) != 0 {
		t.Fatalf("retired depot manifests must be forgotten: %v", matches)
	}
}

// A Windows-only DLC depot never lands on a Linux install (S5): the game
// installs, the DLC shows owned but never installed.
func TestSteamDLCWindowsOnlyDepot(t *testing.T) {
	e := setup(t)
	e.login()
	base := fakesteam.Build(fakesteam.ReadKnobs())
	cdn := fakesteam.StartCDN(t, base)
	fakesteam.WriteCMState(t, e.cmState, cdn, base)
	gid := base.GID
	e.setCMState("appinfo", `"extended"
{
"listofdlc" "100,101"
}
"depots"
{
"11"
{
"config" { "oslist" "linux" }
"manifests" { "public" { "gid" "`+itoa(gid)+`" } }
}
"12"
{
"config" { "oslist" "windows" }
"dlcappid" "100"
"manifests" { "public" { "gid" "1000" } }
}
}
`)
	e.setCMState("licenses", []any{7000})
	e.setCMState("package_apps", map[string]any{"7000": []any{4321, 100}})
	e.setCMState("appinfo_100", `"common" { "name" "Hunt" }`)
	e.setCMState("appinfo_101", `"common" { "name" "Gold" }`)
	writeDLCAPI(t)
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "dlc-winonly", "installDir": e.folder,
		"options": map[string]any{"dlc": []string{"100"}},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	e.waitPluginJob("dlc-winonly")
	gameDir, _ := e.installedEntry("4321")["path"].(string)
	if _, err := os.Stat(filepath.Join(gameDir, "game.sh")); err != nil {
		t.Fatalf("base file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "dlc.dat")); !os.IsNotExist(err) {
		t.Fatalf("windows-only depot downloaded on linux: %v", err)
	}
	g := e.dlcOptions("4321")
	checkDLCKeys(t, g)
	if v := dlcByID(t, g.Values, "100"); !v.Owned || v.Installed {
		t.Fatalf("DLC 100 = %+v, want owned but never installed", v)
	}
}

// Adding a DLC while the install is pinned is refused with a clear error:
// the pinned manifests name no build for the new depot.
func TestSteamDLCAddWhilePinnedRefused(t *testing.T) {
	e := setup(t)
	e.login()
	dlcEnv(t, e)
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "dlc-pin-install", "installDir": e.folder,
		"options": map[string]any{"dlc": []string{}},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	e.waitPluginJob("dlc-pin-install")
	gameDir, _ := e.installedEntry("4321")["path"].(string)
	arch := e.installedEntry("4321")["archiveRef"].(map[string]any)["depots"].(map[string]any)
	manual := "11: " + arch["11"].(string)
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "dlc-pin-add", "installDir": gameDir,
		"target":  map[string]any{"manual": map[string]string{"input": manual}},
		"options": map[string]any{"dlc": []string{"100"}},
	}, nil); err != nil {
		t.Fatalf("update start: %v", err)
	}
	if msg := e.waitPluginJobFailed("dlc-pin-add"); !strings.Contains(msg, "no pinned manifests") {
		t.Fatalf("pinned add-on error = %q, want the pinned-manifest refusal", msg)
	}
}

func itoa(n uint64) string {
	return fmt.Sprint(n)
}
