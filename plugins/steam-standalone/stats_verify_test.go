// New 1.4/1.7 capabilities through the plugin's real surface: playtime.list,
// achievements.list (CM stats + cache + global percentages), game.verify
// (check, corrupt, repair) and game.relocated. No Valve network: the Web API
// answers from the hook file and stats from the fake CM's canned userstats.
package steam_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/jobs"
	"platypus/internal/jsonrpc"
)

// rewriteAPI replaces the Web API hook file: owned games stay the fixture's
// (with playtime fields), _routes answers the global percentages endpoint,
// and _test keeps the store/appdetails entries game.details reads.
func rewriteAPI(t *testing.T, games any) {
	t.Helper()
	api := os.Getenv("PLATYPUS_STEAM_API_FILE")
	data, err := os.ReadFile(api)
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	file["response"] = map[string]any{"games": games}
	file["_routes"] = map[string]any{
		"IPlayerService/GetOwnedGames/v1": file["response"],
		"ISteamUserStats/GetGlobalAchievementPercentagesForApp/v2": map[string]any{
			"achievementpercentages": map[string]any{"achievements": []any{
				map[string]any{"name": "ACH_FIRST", "percent": 84.1},
			}},
		},
	}
	out, _ := json.Marshal(file)
	if err := os.WriteFile(api, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedUserStats adds the canned stats schema + unlock state for 4321 to the
// fake CM state: two achievements, the first unlocked with a real icon URL,
// the second hidden and locked with a bare filename (never sent as an icon).
// Every other app answers eresult 2 (no stats), like a game without any.
func seedUserStats(t *testing.T, e *env) {
	t.Helper()
	data, err := os.ReadFile(e.cmState)
	if err != nil {
		t.Fatal(err)
	}
	var st map[string]any
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	st["userstats"] = map[string]any{"4321": map[string]any{"achievements": []any{
		map[string]any{"api": "ACH_FIRST", "name": "First steps", "desc": "Start something.",
			"icon": "https://cdn.steam/ach_first.png", "unlocked": true},
		map[string]any{"api": "ACH_SECRET", "name": "Secret room", "desc": "Find it.",
			"hidden": true, "icon": "secret.jpg"},
	}}}
	out, _ := json.Marshal(st)
	if err := os.WriteFile(e.cmState, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSteamPlaytimeList(t *testing.T) {
	e := setup(t)
	e.login()
	rewriteAPI(t, []any{
		map[string]any{"appid": float64(4321), "name": "Standalone Game",
			"playtime_forever": float64(150), "rtime_last_played": float64(1700000000)},
		map[string]any{"appid": float64(4322), "name": "Tool Thing"},
	})

	var list struct {
		Games []struct {
			ID         string `json:"id"`
			Minutes    int    `json:"minutes"`
			LastPlayed int64  `json:"lastPlayed"`
		} `json:"games"`
	}
	if err := e.call("playtime.list", map[string]any{}, &list); err != nil {
		t.Fatalf("playtime.list: %v", err)
	}
	if len(list.Games) != 2 {
		t.Fatalf("games = %+v, want 2 entries", list.Games)
	}
	got := map[string]struct {
		minutes int
		last    int64
	}{}
	for _, g := range list.Games {
		got[g.ID] = struct {
			minutes int
			last    int64
		}{g.Minutes, g.LastPlayed}
	}
	if got["4321"].minutes != 150 || got["4321"].last != 1700000000 {
		t.Fatalf("4321 = %+v, want 150 minutes and a lastPlayed", got["4321"])
	}
	if got["4322"].minutes != 0 || got["4322"].last != 0 {
		t.Fatalf("4322 = %+v, want zero minutes and no lastPlayed", got["4322"])
	}

	// Logged out, the store playtime is unavailable, like the library.
	e2 := setup(t)
	var junk json.RawMessage
	if err := e2.call("playtime.list", map[string]any{}, &junk); e.rpcCode(err) != -32001 {
		t.Fatalf("logged out playtime.list = %v, want -32001", err)
	}
}

func TestSteamAchievementsList(t *testing.T) {
	e := setup(t)
	e.login()
	rewriteAPI(t, []any{
		map[string]any{"appid": float64(4321), "name": "Standalone Game"},
		map[string]any{"appid": float64(4322), "name": "Tool Thing"},
	})
	seedUserStats(t, e)

	var res struct {
		Total    int `json:"total"`
		Unlocked int `json:"unlocked"`
		List     []struct {
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			Description   string   `json:"description"`
			Icon          string   `json:"icon"`
			Hidden        bool     `json:"hidden"`
			UnlockedAt    int64    `json:"unlockedAt"`
			GlobalPercent *float64 `json:"globalPercent"`
		} `json:"list"`
	}
	if err := e.call("achievements.list", map[string]any{"id": "4321"}, &res); err != nil {
		t.Fatalf("achievements.list: %v", err)
	}
	if res.Total != 2 || res.Unlocked != 1 || len(res.List) != 2 {
		t.Fatalf("result = %+v", res)
	}
	first, second := res.List[0], res.List[1]
	if first.ID != "ACH_FIRST" || first.Name != "First steps" || first.Description != "Start something." {
		t.Fatalf("first = %+v", first)
	}
	if first.Hidden || first.UnlockedAt != 1700000000 || first.Icon != "https://cdn.steam/ach_first.png" {
		t.Fatalf("first state = %+v", first)
	}
	if first.GlobalPercent == nil || *first.GlobalPercent != 84.1 {
		t.Fatalf("first percent = %+v", first.GlobalPercent)
	}
	if second.ID != "ACH_SECRET" || !second.Hidden || second.UnlockedAt != 0 {
		t.Fatalf("second = %+v", second)
	}
	if second.Icon != "" || second.GlobalPercent != nil {
		t.Fatalf("second leaks icon/percent: %+v", second)
	}

	// A game the fake CM reports no stats for answers an empty list.
	var empty struct {
		Total int               `json:"total"`
		List  []json.RawMessage `json:"list"`
	}
	if err := e.call("achievements.list", map[string]any{"id": "4322"}, &empty); err != nil {
		t.Fatalf("achievements.list 4322: %v", err)
	}
	if empty.Total != 0 || len(empty.List) != 0 {
		t.Fatalf("4322 = %+v, want empty", empty)
	}

	// A missing id is -32602, like every other parameterized method.
	var junk json.RawMessage
	if err := e.call("achievements.list", map[string]any{}, &junk); e.rpcCode(err) != jsonrpc.CodeInvalidParams {
		t.Fatalf("achievements.list without id = %v, want -32602", err)
	}

	// The schema cache fills in, and the library only flags the game that
	// has achievements.
	raw, err := os.ReadFile(filepath.Join(e.pluginData(), "stats-cache.json"))
	if err != nil {
		t.Fatalf("no stats cache: %v", err)
	}
	var cache map[string]struct {
		Achievements []struct {
			API string `json:"api"`
		} `json:"achievements"`
	}
	if err := json.Unmarshal(raw, &cache); err != nil {
		t.Fatal(err)
	}
	if len(cache["4321"].Achievements) != 2 {
		t.Fatalf("cache = %s", raw)
	}
	var lib struct {
		Games []struct {
			ID       string   `json:"id"`
			Features []string `json:"features"`
		} `json:"games"`
	}
	if err := e.call("library.list", nil, &lib); err != nil {
		t.Fatalf("library.list: %v", err)
	}
	feats := map[string][]string{}
	for _, g := range lib.Games {
		feats[g.ID] = g.Features
	}
	if len(feats["4321"]) != 1 || feats["4321"][0] != "achievements" {
		t.Fatalf("4321 features = %v", feats["4321"])
	}
	if len(feats["4322"]) != 0 {
		t.Fatalf("4322 features = %v, want none", feats["4322"])
	}
}

func TestSteamVerifyCheckCorruptRepair(t *testing.T) {
	e := setup(t)
	e.login()
	lib := e.library()
	game := e.find(lib, "4321")

	j, err := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(j.ID); done.State != jobs.StateDone {
		t.Fatalf("install: %+v", done)
	}
	game = e.find(e.library(), "4321")
	if !game.IsInstalled || game.InstallDir == "" {
		t.Fatalf("not installed: %+v", game)
	}

	check := func(repair bool) (checked, damaged, missing, repaired int) {
		t.Helper()
		v, err := e.q.AddVerify(id, game.ID, game.StoreAppID, game.Title, game.InstallDir, repair)
		if err != nil {
			t.Fatal(err)
		}
		done := e.waitJob(v.ID)
		if done.State != jobs.StateDone || done.Verify == nil {
			t.Fatalf("verify(repair=%v): %+v", repair, done)
		}
		r := done.Verify
		return r.Checked, r.Damaged, r.Missing, r.Repaired
	}
	if c, d, m, r := check(false); c <= 0 || d != 0 || m != 0 || r != 0 {
		t.Fatalf("clean check = %d/%d/%d/%d, want all good", c, d, m, r)
	}

	// Damage one installed file: the next check must name exactly it.
	var victim string
	entries, err := os.ReadDir(game.InstallDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range entries {
		if !de.IsDir() {
			victim = filepath.Join(game.InstallDir, de.Name())
			break
		}
	}
	if victim == "" {
		t.Fatal("no installed file to damage")
	}
	f, err := os.OpenFile(victim, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("platypus-verify-probe"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if c, d, m, r := check(false); d != 1 || m != 0 || r != 0 || c <= 0 {
		t.Fatalf("damaged check = %d/%d/%d/%d, want 1 damaged", c, d, m, r)
	}

	// Repair restores it; a check after is clean again.
	if c, d, m, r := check(true); d != 0 || m != 0 || r != 1 || c <= 0 {
		t.Fatalf("repair = %d/%d/%d/%d, want 1 repaired", c, d, m, r)
	}
	if c, d, m, _ := check(false); c <= 0 || d != 0 || m != 0 {
		t.Fatalf("after repair = %d/%d/%d, want clean", c, d, m)
	}

	// Unknown games and missing params are refused before any work.
	if err := e.rawCallErr("game.verify",
		map[string]any{"id": "9999", "installDir": "/tmp/nope", "jobId": "vx"}); err == nil {
		t.Fatal("game.verify on an unknown game succeeded")
	}
	if err := e.rawCallErr("game.verify", map[string]any{}); e.rpcCode(err) != jsonrpc.CodeInvalidParams {
		t.Fatalf("game.verify without params = %v, want -32602", err)
	}
}

func TestSteamRelocated(t *testing.T) {
	e := setup(t)
	e.login()
	installed := `{"4321": {"path": "/old/games/g", "dir_name": "g"}}`
	if err := os.WriteFile(filepath.Join(e.pluginData(), "installed.json"), []byte(installed), 0o644); err != nil {
		t.Fatal(err)
	}

	var ok map[string]any
	if err := e.call("game.relocated",
		map[string]any{"id": "4321", "from": "/old/games/g", "to": "/new/games/g2"}, &ok); err != nil {
		t.Fatalf("game.relocated: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
	var after map[string]map[string]any
	if err := json.Unmarshal(raw, &after); err != nil {
		t.Fatal(err)
	}
	if after["4321"]["path"] != "/new/games/g2" || after["4321"]["dir_name"] != "g2" {
		t.Fatalf("record = %s", raw)
	}

	// A `from` the record doesn't point at is -32602 and changes nothing.
	if err := e.rawCallErr("game.relocated",
		map[string]any{"id": "4321", "from": "/elsewhere", "to": "/new/games/g3"}); e.rpcCode(err) != jsonrpc.CodeInvalidParams {
		t.Fatalf("relocated mismatch = %v, want -32602", err)
	}
	raw, _ = os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
	if !strings.Contains(string(raw), "/new/games/g2") {
		t.Fatalf("record moved on a refused call: %s", raw)
	}
}
