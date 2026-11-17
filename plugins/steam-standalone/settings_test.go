// The `settings` capability (protocol 1.8) through the plugin's real
// surface: the schema core draws, a save that persists, and the chosen
// download region reaching platypus-steam-dl as its server-list cell id.
package steam_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/jobs"
)

func TestSteamSettingsSchema(t *testing.T) {
	e := setup(t)
	var schema struct {
		Groups []struct {
			ID     string `json:"id"`
			Label  string `json:"label"`
			Fields []struct {
				ID      string `json:"id"`
				Kind    string `json:"kind"`
				Default any    `json:"default"`
				Options []struct {
					ID    string `json:"id"`
					Label string `json:"label"`
				} `json:"options"`
			} `json:"fields"`
		} `json:"groups"`
	}
	if err := e.call("settings.schema", map[string]any{}, &schema); err != nil {
		t.Fatalf("settings.schema: %v", err)
	}
	if len(schema.Groups) != 1 || len(schema.Groups[0].Fields) != 2 {
		t.Fatalf("schema: %+v", schema)
	}
	field := schema.Groups[0].Fields[0]
	if field.ID != "region" || field.Kind != "select" || field.Default != "auto" {
		t.Fatalf("region field: %+v", field)
	}
	ids := map[string]bool{}
	for _, o := range field.Options {
		ids[o.ID] = true
	}
	if !ids["auto"] || !ids["14"] || !ids[field.Default.(string)] {
		t.Fatalf("region options: %+v", field.Options)
	}
	shader := schema.Groups[0].Fields[1]
	if shader.ID != "shader" || shader.Kind != "bool" || shader.Default != false {
		t.Fatalf("shader field: %+v", shader)
	}
	// An unknown region is refused, like a value core refuses before saving.
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "9999"}}, nil); err == nil {
		t.Fatal("unknown region accepted")
	}
	// A non-boolean shader value is refused too.
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"shader": "yes"}}, nil); err == nil {
		t.Fatal("non-boolean shader accepted")
	}
}

// A chosen region is persisted and used as the cell id of the next install's
// server-list request, over any Steam client config.
func TestSteamDownloadRegionSetting(t *testing.T) {
	t.Setenv("PLATYPUS_STEAM_ROOT", t.TempDir()) // no Steam client config
	t.Setenv("PLATYPUS_STEAM_CELLID", "")        // no inherited override
	e := setup(t)
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "14"}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	stored, err := os.ReadFile(filepath.Join(e.root, "data", id, "settings.json"))
	if err != nil || !strings.Contains(string(stored), `"14"`) {
		t.Fatalf("settings not persisted: %s %v", stored, err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	if log := e.cmLog(); !strings.Contains(log, "servers cell=14") {
		t.Fatalf("region setting ignored: %s", log)
	}
}
