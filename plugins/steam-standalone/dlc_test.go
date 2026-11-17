package steam_test

// End-to-end DLC (protocol 1.5) through the real plugin path: owned vs
// unowned add-ons in game.options, the selection changing the install set,
// deselecting removing the DLC depot's files, updates keeping the
// selection, and refusals for unknown or unowned add-ons. PICS, depot keys
// and content come from the fake CM and a two-depot fake CDN; the Store and
// owned list from the API hook file. No network, no Valve.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/plugins/steam-standalone/fakesteam"
)

func writeDLCAPI(t *testing.T) {
	t.Helper()
	api := os.Getenv("PLATYPUS_STEAM_API_FILE")
	data, _ := json.Marshal(map[string]any{
		"response": map[string]any{"games": []any{
			map[string]any{"appid": 4321, "name": "Standalone Game"},
			map[string]any{"appid": 100, "name": "Hunt"},
			map[string]any{"appid": 4322, "name": "Tool Thing"},
		}},
		"_test": map[string]any{
			"4321": map[string]any{"success": true, "data": map[string]any{
				"type": "game", "name": "Standalone Game",
				"short_description": "A standalone game."}},
			"4322": map[string]any{"success": true, "data": map[string]any{
				"type": "tool", "name": "Tool Thing"}},
			"100": map[string]any{"success": true, "data": map[string]any{
				"type": "dlc", "name": "Hunt"}},
			"101": map[string]any{"success": true, "data": map[string]any{
				"type": "dlc", "name": "Gold"}},
		},
	})
	if err := os.WriteFile(api, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

type dlcValue struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Owned     bool   `json:"owned"`
	Installed bool   `json:"installed"`
	StoreURL  string `json:"storeUrl"`
}

func (e *env) dlcOptions(app string) []dlcValue {
	e.t.Helper()
	var out struct {
		Groups []struct {
			ID     string     `json:"id"`
			Kind   string     `json:"kind"`
			DLC    bool       `json:"dlc"`
			Values []dlcValue `json:"values"`
		} `json:"groups"`
	}
	if err := e.call("game.options", map[string]any{"id": app}, &out); err != nil {
		e.t.Fatalf("game.options: %v", err)
	}
	if len(out.Groups) != 1 || out.Groups[0].ID != "dlc" || !out.Groups[0].DLC {
		e.t.Fatalf("game.options groups = %+v, want one dlc group", out.Groups)
	}
	return out.Groups[0].Values
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
func (e *env) waitPluginJob(jobID string) {
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
			var p struct {
				JobID string `json:"jobId"`
			}
			if json.Unmarshal(raw, &p) == nil && p.JobID == jobID {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	e.t.Fatalf("no job.done for %s", jobID)
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

func TestSteamDLCOwnershipAndPerDLCInstall(t *testing.T) {
	e := setup(t)
	e.login()
	base := fakesteam.Build(fakesteam.ReadKnobs())
	dlc := fakesteam.BuildDLC()
	cdn := fakesteam.StartCDN(t, base, dlc)
	fakesteam.WriteCMStateDLC(t, e.cmState, cdn, base, dlc)
	writeDLCAPI(t)

	// Owned vs unowned display: 100 is in the owned list, 101 only in the
	// catalog; nothing is installed yet.
	values := e.dlcOptions("4321")
	if len(values) != 2 {
		t.Fatalf("DLC values = %+v, want 100 and 101", values)
	}
	if v := dlcByID(t, values, "100"); !v.Owned || v.Installed || v.Label != "Hunt" {
		t.Fatalf("DLC 100 = %+v, want owned Hunt, not installed", v)
	}
	if v := dlcByID(t, values, "101"); v.Owned || v.Installed || v.StoreURL == "" {
		t.Fatalf("DLC 101 = %+v, want unowned with a store link", v)
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
	if full.DiskBytes != int64(base.DiskBytes+dlc.DiskBytes) {
		t.Fatalf("full size disk=%d, want base %d + dlc %d",
			full.DiskBytes, base.DiskBytes, dlc.DiskBytes)
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
	if v := dlcByID(t, e.dlcOptions("4321"), "100"); !v.Installed {
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
	if v := dlcByID(t, e.dlcOptions("4321"), "100"); v.Installed {
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
		if !strings.Contains(err.Error(), tc.dlc[0]) && tc.name == "unknown" {
			t.Fatalf("unknown DLC error names nothing: %v", err)
		}
	}
}
