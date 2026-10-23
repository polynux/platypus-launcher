// Tests the Steam client plugin against a fake Steam tree in a temp HOME:
// the same games, ids and fields the built-in scan found, playtime from
// localconfig.vdf, launches through the Steam client, and an empty library
// with no error when Steam isn't installed.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"platypus/internal/steam"
)

// fakeHome builds a temp Steam tree and points HOME at it:
//
//	440  Team Fortress 2, installed, library art + header, playtime 21566
//	570  Dota 2, installed, no local art (CDN fallback), no playtime
//	228980 Steam Linux Runtime (filtered as a system app)
//	an extra library folder via libraryfolders.vdf holding 730 (CS2)
func fakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// XDG_DATA_HOME must not point at a real Steam tree during the test.
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	root := filepath.Join(home, ".local", "share", "Steam")
	steamapps := filepath.Join(root, "steamapps")
	mustWrite := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	acf := func(appID, name, dir string) {
		mustWrite(filepath.Join(steamapps, "appmanifest_"+appID+".acf"),
			"\"AppState\"\n{\n\t\"appid\"\t\t\""+appID+"\"\n\t\"name\"\t\t\""+name+"\"\n\t\"installdir\"\t\t\""+dir+"\"\n}\n")
		if err := os.MkdirAll(filepath.Join(steamapps, "common", dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	acf("440", "Team Fortress 2", "Team Fortress 2")
	acf("570", "Dota 2", "dota 2 beta")
	acf("228980", "Steam Linux Runtime", "SteamLinuxRuntime")
	extra := filepath.Join(home, "extra-lib")
	extraApps := filepath.Join(extra, "steamapps")
	mustWrite(filepath.Join(extraApps, "appmanifest_730.acf"),
		"\"AppState\"\n{\n\t\"appid\"\t\t\"730\"\n\t\"name\"\t\t\"Counter-Strike 2\"\n\t\"installdir\"\t\t\"counter-strike 2\"\n}\n")
	if err := os.MkdirAll(filepath.Join(extraApps, "common", "counter-strike 2"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(filepath.Join(steamapps, "libraryfolders.vdf"),
		"libraryfolders\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\""+root+"\"\n\t}\n\t\"1\"\n\t{\n\t\t\"path\"\t\t\""+extra+"\"\n\t}\n}\n")
	art := func(appID, name string) {
		mustWrite(filepath.Join(root, "appcache/librarycache", appID, name), "fake-"+name)
	}
	art("440", "library_600x900.jpg")
	art("440", "header.jpg")
	art("440", "library_hero.jpg")
	mustWrite(filepath.Join(root, "userdata/12345/config/localconfig.vdf"),
		"\"UserLocalConfigStore\"\n{\n\t\"Software\" { \"Valve\" { \"Steam\" { \"apps\"\n\t{\n"+
			"\t\t\"440\"\n\t\t{\n\t\t\t\"LastPlayed\"\t\t\"1790282473\"\n\t\t\t\"Playtime\"\t\t\"21566\"\n\t\t}\n"+
			"\t}\n\t} } }\n}\n")
	return home
}

func libGames(t *testing.T, raw any) []map[string]any {
	t.Helper()
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Games []map[string]any `json:"games"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(res.Games, func(a, b map[string]any) int {
		return strings.Compare(a["id"].(string), b["id"].(string))
	})
	return res.Games
}

// The plugin lists the same set the built-in scan found: ids, titles,
// install dirs and art for the same files, offline ok, no system apps.
func TestLibraryMatchesBuiltinScan(t *testing.T) {
	home := fakeHome(t)
	root := filepath.Join(home, ".local", "share", "Steam")
	if err := os.MkdirAll(filepath.Join(root, "steamapps", "compatdata", "440", "pfx"), 0o755); err != nil {
		t.Fatal(err)
	}

	scanned, err := steam.ScanSteamLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if len(scanned) != 3 {
		ids := []string{}
		for _, g := range scanned {
			ids = append(ids, g.ID)
		}
		t.Fatalf("scan found %v, want [steam_440 steam_570 steam_730] (no system app)", ids)
	}

	raw, err := library()
	if err != nil {
		t.Fatal(err)
	}
	games := libGames(t, raw)
	if len(games) != len(scanned) {
		t.Fatalf("plugin listed %d games, built-in found %d", len(games), len(scanned))
	}
	byID := map[string]map[string]any{}
	for _, g := range games {
		byID[g["id"].(string)] = g
	}
	for _, g := range scanned {
		wire, ok := byID[g.StoreAppID]
		if !ok {
			t.Errorf("built-in game %s (%s) missing from the plugin listing", g.ID, g.Title)
			continue
		}
		if wire["title"] != g.Title {
			t.Errorf("%s title = %q, want %q", g.StoreAppID, wire["title"], g.Title)
		}
		if wire["installed"] != true {
			t.Errorf("%s installed = %v, want true", g.StoreAppID, wire["installed"])
		}
		if wire["installPath"] != g.InstallDir {
			t.Errorf("%s installPath = %q, want %q", g.StoreAppID, wire["installPath"], g.InstallDir)
		}
		if wire["offline"] != "ok" {
			t.Errorf("%s offline = %q, want ok", g.StoreAppID, wire["offline"])
		}
		if wire["clientLaunch"] != true {
			t.Errorf("%s clientLaunch = %v, want true", g.StoreAppID, wire["clientLaunch"])
		}
		// Last played reaches the library as before (the built-in source set it).
		if g.LastPlayed > 0 {
			if v, _ := wire["lastPlayed"].(float64); int64(v) != g.LastPlayed {
				t.Errorf("%s lastPlayed = %v, want %d", g.StoreAppID, wire["lastPlayed"], g.LastPlayed)
			}
		}
		if g.StoreAppID == "440" && g.LastPlayed == 0 {
			t.Error("fixture 440 should carry LastPlayed from localconfig.vdf")
		}
		// Art names the same files: local cache as file://, else the CDN
		// fallback the built-in source used.
		for _, kind := range []string{"coverUrl", "heroUrl", "bannerUrl"} {
			var scannedURL string
			switch kind {
			case "coverUrl":
				scannedURL = g.CoverURL
			case "heroUrl":
				scannedURL = g.HeroURL
			case "bannerUrl":
				scannedURL = g.BannerURL
			}
			got, _ := wire[kind].(string)
			if rel, ok := strings.CutPrefix(scannedURL, steam.ArtURLPrefix); ok {
				want := "file://" + filepath.Join(root, "appcache/librarycache", filepath.FromSlash(rel))
				if got != want {
					t.Errorf("%s %s = %q, want %q", g.StoreAppID, kind, got, want)
				}
			} else if got != scannedURL {
				t.Errorf("%s %s = %q, want CDN fallback %q", g.StoreAppID, kind, got, scannedURL)
			}
		}
	}
	ctxRaw, err := saveContext(params{ID: "440"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := ctxRaw.(map[string]any)
	if ctx["storeUserId"] != "12345" || ctx["prefix"] != filepath.Join(root, "steamapps", "compatdata", "440", "pfx") || ctx["proton"] != true {
		t.Fatalf("save context = %+v", ctx)
	}
	// 570 has no local art: all CDN fallbacks. 440's hero is local here.
	if m := byID["570"]; m["coverUrl"] != "https://steamcdn-a.akamaihd.net/steam/apps/570/library_600x900.jpg" {
		t.Errorf("570 cover = %q, want the CDN fallback", m["coverUrl"])
	}
	if m := byID["440"]; m["coverUrl"] != "file://"+filepath.Join(root, "appcache/librarycache/440/library_600x900.jpg") {
		t.Errorf("440 cover = %q, want the local cache file", m["coverUrl"])
	}
}

// The expected ids and fields, captured from the built-in scan and committed
// as testdata: the golden file the move must keep byte-identical (ids,
// Source via the manifest gameIdPrefix, art files, playtime).
func TestGoldenLibraryTestdata(t *testing.T) {
	fakeHome(t)
	raw, err := library()
	if err != nil {
		t.Fatal(err)
	}
	games := libGames(t, raw)
	// Strip the temp HOME prefix so the file is stable across runs.
	home := os.Getenv("HOME")
	stable := make([]map[string]any, 0, len(games))
	for _, g := range games {
		c := map[string]any{}
		for k, v := range g {
			if s, ok := v.(string); ok {
				c[k] = strings.ReplaceAll(s, home, "$HOME")
			} else {
				c[k] = v
			}
		}
		stable = append(stable, c)
	}
	data, _ := json.MarshalIndent(stable, "", "  ")
	path := "testdata/library.json"
	if os.Getenv("PLATYPUS_UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file missing (run with PLATYPUS_UPDATE_GOLDEN=1 to create it): %v", err)
	}
	var wantGames []map[string]any
	if err := json.Unmarshal(want, &wantGames); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%v", stable) != fmt.Sprintf("%v", wantGames) {
		t.Errorf("library differs from testdata/library.json:\ngot  %s\nwant %s", data, want)
	}
}

// Playtime comes from localconfig.vdf: minutes for 440, nothing for 570.
func TestPlaytimeFromLocalConfig(t *testing.T) {
	fakeHome(t)
	raw, err := playtime()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(raw)
	var res struct {
		Games []struct {
			ID      string `json:"id"`
			Minutes int    `json:"minutes"`
		} `json:"games"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatal(err)
	}
	byID := map[string]int{}
	for _, g := range res.Games {
		byID[g.ID] = g.Minutes
	}
	if byID["440"] != 21566 {
		t.Errorf("440 minutes = %d, want 21566", byID["440"])
	}
	if _, ok := byID["570"]; ok {
		t.Errorf("570 has no playtime but was reported: %v", byID)
	}
}

// No Steam install lists no games with no error.
func TestMissingInstallListsEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	if got := steam.Root(); got != "" {
		t.Fatalf("Root = %q in an empty HOME, want empty", got)
	}
	raw, err := library()
	if err != nil {
		t.Fatal(err)
	}
	if games := libGames(t, raw); len(games) != 0 {
		t.Fatalf("empty HOME listed %v, want no games", games)
	}
	ctxRaw, err := saveContext(params{ID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if roots := ctxRaw.(map[string]any)["roots"]; len(roots.(map[string]string)) != 0 {
		t.Fatalf("save context roots without a client = %v", roots)
	}
	raw, err = playtime()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(raw)
	var res struct {
		Games []any `json:"games"`
	}
	_ = json.Unmarshal(data, &res)
	if len(res.Games) != 0 {
		t.Fatalf("empty HOME playtime = %v, want none", res.Games)
	}
}

// Launch specs equal today's command: steam, then the flatpak client, then
// the steam:// fallback (which drops launch arguments, as before).
func TestLaunchSpecMatchesBuiltinCommand(t *testing.T) {
	fakeHome(t)
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeExe := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// With steam on PATH the spec asks the client directly.
	writeExe("steam", "#!/bin/sh\nexit 0\n")
	spec, err := launchSpec(params{ID: "440"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(spec)
	var got struct {
		Delegate string   `json:"delegate"`
		Client   string   `json:"client"`
		Command  []string `json:"command"`
		Find     struct {
			Cmdline []string `json:"cmdline"`
		} `json:"find"`
	}
	_ = json.Unmarshal(data, &got)
	if got.Delegate != "store-client" || got.Client != "Steam" {
		t.Fatalf("spec = %s, want a Steam store-client delegate", data)
	}
	if fmt.Sprintf("%q", got.Command) != `["steam" "-applaunch" "440"]` {
		t.Fatalf("command = %q, want today's steam command", got.Command)
	}
	if len(got.Find.Cmdline) != 2 || got.Find.Cmdline[0] != "SteamLaunch" || got.Find.Cmdline[1] != "AppId=440\x00" {
		t.Fatalf("find.cmdline = %q, want the Steam reaper match", got.Find.Cmdline)
	}
	// Without steam but with a flatpak Steam, the spec uses flatpak.
	if err := os.Remove(filepath.Join(bin, "steam")); err != nil {
		t.Fatal(err)
	}
	writeExe("flatpak", "#!/bin/sh\nif [ \"$1\" = info ]; then exit 0; fi\nexit 1\n")
	t.Setenv("PATH", bin)
	spec, err = launchSpec(params{ID: "440"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(spec)
	_ = json.Unmarshal(data, &got)
	if fmt.Sprintf("%q", got.Command) != `["flatpak" "run" "com.valvesoftware.Steam" "-applaunch" "440"]` {
		t.Fatalf("flatpak command = %q", got.Command)
	}
	// Without either, the steam:// fallback (no launch arguments).
	if err := os.Remove(filepath.Join(bin, "flatpak")); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	spec, err = launchSpec(params{ID: "440"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(spec)
	_ = json.Unmarshal(data, &got)
	if fmt.Sprintf("%q", got.Command) != `["xdg-open" "steam://rungameid/440"]` {
		t.Fatalf("fallback command = %q", got.Command)
	}
	// The executable override answers with a direct spec for that file.
	exe := filepath.Join(home(t), "game")
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec, err = launchSpec(params{ID: "440", Executable: exe})
	if err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(spec)
	var direct struct {
		Exe string `json:"exe"`
	}
	_ = json.Unmarshal(data, &direct)
	if direct.Exe != exe {
		t.Fatalf("executable spec = %s, want %s", data, exe)
	}
	if _, err := launchSpec(params{ID: "440", Executable: "relative/game"}); err == nil {
		t.Fatal("relative executable accepted, want an error")
	}
	if _, err := launchSpec(params{}); err == nil {
		t.Fatal("empty id accepted, want an error")
	}
}

func home(t *testing.T) string {
	t.Helper()
	h, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// Details merge the local cache with a fake Store API: summary, art and
// Cloud caching (library cloudSync appears once details were read).
func TestDetailsAndCloudSync(t *testing.T) {
	fakeHome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID := r.URL.Query().Get("appids")
		if appID == "440" {
			fmt.Fprint(w, `{"440":{"success":true,"data":{
				"short_description":"Team-based multiplayer.",
				"categories":[{"id":23,"description":"Steam Cloud"}],
				"genres":[{"description":"Action"}]}}}`)
			return
		}
		fmt.Fprintf(w, `{%q:{"success":false}}`, appID)
	}))
	defer srv.Close()
	t.Setenv("PLATYPUS_STEAM_STORE_BASE", srv.URL)

	raw, err := details(params{ID: "440"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(raw)
	var d struct {
		Summary string   `json:"summary"`
		Genres  []string `json:"genres"`
	}
	_ = json.Unmarshal(data, &d)
	if d.Summary != "Team-based multiplayer." {
		t.Fatalf("summary = %q", d.Summary)
	}
	// The cached Cloud category now shows as cloudSync in the library.
	raw, err = library()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range libGames(t, raw) {
		if g["id"] == "440" {
			found = true
			if g["cloudSync"] != true {
				t.Errorf("440 cloudSync = %v, want true after cached Cloud details", g["cloudSync"])
			}
		}
		if g["id"] == "570" {
			if _, ok := g["cloudSync"]; ok {
				t.Errorf("570 cloudSync present without cached details")
			}
		}
	}
	if !found {
		t.Fatal("440 missing from the library")
	}
	// Unknown ids without local data error (never data or an empty page).
	metaMu.Lock()
	meta = steam.NewStoreClient()
	metaMu.Unlock()
	if _, err := details(params{ID: "no-such-game-0xdead"}); err == nil {
		t.Fatal("unknown game details succeeded, want an error")
	}
	if _, err := details(params{}); err == nil {
		t.Fatal("empty id details succeeded, want an error")
	}
}
