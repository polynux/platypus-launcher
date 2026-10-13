// Slow conformance for the Steam client plugin (issue #54): the real
// plugin path (manager, JSON-RPC, launch-spec validation) against a fake
// Steam tree in a temp HOME and a fake Store API. No network, no real
// Steam folders.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"platypus/internal/plugtest"
	"platypus/internal/testenv"
	"platypus/internal/types"
)

func TestConformanceSteam(t *testing.T) {
	testenv.RequireSlow(t)
	fakeHome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("appids") == "440" {
			fmt.Fprint(w, `{"440":{"success":true,"data":{
				"short_description":"Team-based multiplayer.",
				"categories":[{"id":23,"description":"Steam Cloud"}],
				"genres":[{"description":"Action"}]}}}`)
			return
		}
		fmt.Fprintf(w, `{%q:{"success":false}}`, r.URL.Query().Get("appids"))
	}))
	defer srv.Close()
	t.Setenv("PLATYPUS_STEAM_STORE_BASE", srv.URL)

	sess := plugtest.Suite{
		ID:       "steam",
		CopyDir:  ".",
		BuildCmd: []string{"go", "build", "-o", "plugins/steam/bin/platypus-steam", "platypus/plugins/steam"},
		BuildOut: "plugins/steam/bin/platypus-steam",
	}.Start(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	games, err := sess.Store.Library(ctx)
	if err != nil {
		t.Fatalf("library.list: %v", err)
	}
	if len(games) != 3 {
		ids := []string{}
		for _, g := range games {
			ids = append(ids, g.ID)
		}
		t.Fatalf("library = %v, want [steam_440 steam_570 steam_730]", ids)
	}
	byStore := map[string]types.Game{}
	for _, g := range games {
		byStore[g.StoreAppID] = g
		// Ids stay byte-identical across the move out of the core.
		if g.ID != sess.Store.WireID(g.StoreAppID) {
			t.Errorf("game id %q is not the gameIdPrefix form", g.ID)
		}
		if g.Source != "steam" || g.Store != "steam" {
			t.Errorf("%s source/store = %q/%q, want steam/steam", g.ID, g.Source, g.Store)
		}
		switch g.Offline {
		case "ok", "network", "unknown", "":
		default:
			t.Errorf("%s offline = %q", g.ID, g.Offline)
		}
	}
	if g := byStore["440"]; g.Title != "Team Fortress 2" || !g.IsInstalled || g.InstallDir == "" {
		t.Errorf("440 = %+v, want the installed Team Fortress 2", g)
	}
	// Local art is rewritten to the plugin art server and decodes to the
	// client's library cache; the CDN fallback passes through.
	cover := byStore["440"].CoverURL
	const prefix = "/platypus-art/steam/"
	if !strings.HasPrefix(cover, prefix) {
		t.Fatalf("440 cover = %q, want the art server", cover)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(cover, prefix))
	if err != nil || !strings.HasSuffix(string(raw), "appcache/librarycache/440/library_600x900.jpg") {
		t.Fatalf("440 cover decodes to %q (%v), want the library cache file", raw, err)
	}
	if got := byStore["570"].CoverURL; got != "https://steamcdn-a.akamaihd.net/steam/apps/570/library_600x900.jpg" {
		t.Errorf("570 cover = %q, want the CDN fallback", got)
	}

	// Details carry the store page; an unknown id errors.
	var rawDetails json.RawMessage
	if err := sess.Call("game.details", map[string]any{"id": "440"}, &rawDetails); err != nil {
		t.Fatalf("game.details: %v", err)
	}
	var det struct {
		Summary string `json:"summary"`
	}
	_ = json.Unmarshal(rawDetails, &det)
	if det.Summary != "Team-based multiplayer." {
		t.Errorf("440 summary = %q", det.Summary)
	}
	var junk json.RawMessage
	if err := sess.Call("game.details", map[string]any{"id": "no-such-game-0xdead"}, &junk); err == nil {
		t.Error("game.details on an unknown id succeeded, want an RPC error")
	}

	// The cached Cloud category shows as cloudSync on the next listing.
	games, err = sess.Store.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range games {
		if g.StoreAppID == "440" && !g.CloudSync {
			t.Error("440 cloudSync = false after cached Cloud details, want true")
		}
	}

	// Launch goes through the Steam client with the reaper match; the
	// executable override answers with a direct spec.
	l, err := sess.Store.Launch(ctx, byStore["440"], true)
	if err != nil || l.Client == nil || l.Client.Client != "Steam" || len(l.Client.Command) < 3 {
		t.Fatalf("launchSpec preview: %+v, %v", l, err)
	}
	var rawSpec struct {
		Delegate string   `json:"delegate"`
		Command  []string `json:"command"`
		Find     struct {
			Cmdline []string `json:"cmdline"`
		} `json:"find"`
	}
	if err := sess.Call("game.launchSpec", map[string]any{"id": "440", "preview": true}, &rawSpec); err != nil {
		t.Fatalf("game.launchSpec: %v", err)
	}
	if rawSpec.Delegate != "store-client" || len(rawSpec.Find.Cmdline) != 2 {
		t.Errorf("launchSpec = %+v, want a store-client delegate with find.cmdline", rawSpec)
	}
	if _, err := sess.Store.Launch(ctx, types.Game{StoreAppID: "no-such-game-0xdead"}, false); err == nil {
		t.Error("launchSpec for an unknown game succeeded")
	}
	// The executable override answers with a direct spec for that file
	// (core validates it against the plugin's folders on the way out).
	exePath := byStore["440"].InstallDir + "/hl2.sh"
	var rawExe struct {
		Exe string `json:"exe"`
	}
	if err := sess.Call("game.launchSpec", map[string]any{"id": "440", "executable": exePath}, &rawExe); err != nil {
		t.Fatalf("launchSpec executable: %v", err)
	}
	if rawExe.Exe != exePath {
		t.Errorf("executable spec = %+v, want exe %s", rawExe, exePath)
	}

	// Playtime comes from localconfig.vdf.
	var playtime struct {
		Games []struct {
			ID      string `json:"id"`
			Minutes int    `json:"minutes"`
		} `json:"games"`
	}
	if err := sess.Call("playtime.list", map[string]any{}, &playtime); err != nil {
		t.Fatalf("playtime.list: %v", err)
	}
	minutes := map[string]int{}
	for _, g := range playtime.Games {
		minutes[g.ID] = g.Minutes
	}
	if minutes["440"] != 21566 {
		t.Errorf("440 minutes = %d, want 21566", minutes["440"])
	}
}
