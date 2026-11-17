package gog_test

// The GOG APIs the plugin reads besides the store's own: achievements and
// playtime on GOG's gameplay service, install options from gogdl info, and the
// install record after a move. The HTTP ones answer from a loopback test
// server (PLATYPUS_GOG_API_BASE), never the network.

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeDepot serves the content-system depot list a Windows install's stored
// gogdl manifest points at: the fake gogdl's data.bin (12 MiB of zeros) as one
// chunk, zlib-compressed like GOG's CDN.
func fakeDepot(t *testing.T) string {
	t.Helper()
	content := make([]byte, 12<<20)
	sum := md5.Sum(content)
	body, err := json.Marshal(map[string]any{"version": 2, "depot": map[string]any{"items": []any{
		map[string]any{"type": "DepotFile", "path": "data.bin", "md5": hex.EncodeToString(sum[:]),
			"chunks": []any{map[string]any{"md5": hex.EncodeToString(sum[:]), "size": len(content)}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	zw.Write(body)
	zw.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/content-system/v2/meta/") {
			http.NotFound(w, r)
			return
		}
		w.Write(compressed.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// storeAPI starts a loopback server for the plugin's API calls and points the
// plugin at it.
func storeAPI(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) {
	t.Helper()
	t.Setenv("PLATYPUS_GOG_API_FILE", "")
	srv := httptest.NewServer(http.HandlerFunc(handle))
	t.Cleanup(srv.Close)
	t.Setenv("PLATYPUS_GOG_API_BASE", srv.URL)
}

// pyJSON runs a plugin script through pyRun and decodes what it printed.
func pyJSON(t *testing.T, script string, out any) {
	t.Helper()
	if err := json.Unmarshal([]byte(pyRun(t, script)), out); err != nil {
		t.Fatalf("script printed no JSON: %v", err)
	}
}

func TestGogAchievementsList(t *testing.T) {
	var path, auth, lang string
	storeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		path, auth, lang = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-Gog-Lc")
		w.Write([]byte(`{"total_count": 2, "items": [
		  {"achievement_id": "1", "achievement_key": "FIRST", "visible": true, "name": "First",
		   "description": "Do it", "image_url_locked": "https://img.example/l.png",
		   "image_url_unlocked": "https://img.example/u.png",
		   "date_unlocked": "2024-11-05T10:27:37.000Z", "rarity": 12.5},
		  {"achievement_id": "2", "achievement_key": "SECRET", "visible": false, "name": "Secret",
		   "description": "?", "image_url_locked": "https://img.example/l2.png",
		   "image_url_unlocked": "https://img.example/u2.png", "date_unlocked": null}]}`))
	})
	var out struct {
		Total    int `json:"total"`
		Unlocked int `json:"unlocked"`
		List     []struct {
			ID            string  `json:"id"`
			Name          string  `json:"name"`
			Description   string  `json:"description"`
			Icon          string  `json:"icon"`
			Hidden        bool    `json:"hidden"`
			UnlockedAt    int64   `json:"unlockedAt"`
			GlobalPercent float64 `json:"globalPercent"`
		} `json:"list"`
	}
	pyJSON(t, `
import json
hg.credentials = lambda: {"access_token": "tok", "user_id": "46899977096215655"}
print(json.dumps(hg.achievements_list({"id": "1234"})))
`, &out)
	want := time.Date(2024, 11, 5, 10, 27, 37, 0, time.UTC).Unix()
	if path != "/clients/1234/users/46899977096215655/achievements" {
		t.Fatalf("requested %q", path)
	}
	if auth != "Bearer tok" || lang != "en-US" {
		t.Fatalf("headers: %q %q", auth, lang)
	}
	if out.Total != 2 || out.Unlocked != 1 || len(out.List) != 2 {
		t.Fatalf("list: %+v", out)
	}
	first, secret := out.List[0], out.List[1]
	if first.ID != "FIRST" || first.Name != "First" || first.Hidden ||
		first.Icon != "https://img.example/u.png" || first.UnlockedAt != want ||
		first.GlobalPercent != 12.5 {
		t.Fatalf("unlocked achievement: %+v", first)
	}
	if secret.ID != "SECRET" || !secret.Hidden || secret.Icon != "https://img.example/l2.png" ||
		secret.UnlockedAt != 0 || secret.GlobalPercent != 0 {
		t.Fatalf("hidden achievement: %+v", secret)
	}
}

func TestGogAchievementsNeedLogin(t *testing.T) {
	if got := pyRun(t, `
hg.credentials = lambda: None
try:
    hg.achievements_list({"id": "1234"})
except hg.RPCError as e:
    print(e.code)
`); got != "-32001" {
		t.Fatalf("logged out: %s", got)
	}
}

func TestGogPlaytimeList(t *testing.T) {
	data := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", data)
	var sessions []string
	hits := map[string]int{}
	storeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		switch r.URL.Path {
		case "/account/getFilteredProducts":
			w.Write([]byte(`{"totalPages": 1, "products": [{"id": 1}, {"id": 2}, {"id": 3}, {"id": 4}]}`))
		case "/games/1/users/42/sessions":
			sessions = append(sessions, r.URL.Path)
			w.Write([]byte(`{"time_sum": 90}`))
		case "/games/2/users/42/sessions":
			w.Write([]byte(`{"time_sum": 0}`))
		case "/games/3/users/42/sessions":
			w.WriteHeader(http.StatusInternalServerError)
		case "/games/4/users/42/sessions":
			w.Write([]byte(`{"time_sum": 45}`))
		default:
			http.NotFound(w, r)
		}
	})
	script := `
import json
hg.credentials = lambda: {"access_token": "tok", "user_id": "42"}
print(json.dumps(hg.playtime_list()))
`
	var out struct {
		Games []struct {
			ID      string `json:"id"`
			Minutes int    `json:"minutes"`
		} `json:"games"`
	}
	pyJSON(t, script, &out)
	if len(out.Games) != 2 || out.Games[0].ID != "1" || out.Games[0].Minutes != 90 ||
		out.Games[1].ID != "4" || out.Games[1].Minutes != 45 {
		t.Fatalf("playtime: %+v", out.Games)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions calls: %v", sessions)
	}
	// A second call inside the cache window asks GOG nothing new.
	before := len(sessions)
	pyJSON(t, script, &out)
	if len(sessions) != before {
		t.Fatalf("cached playtime still hit the sessions API: %v", sessions)
	}
	if len(out.Games) != 2 || out.Games[0].Minutes != 90 {
		t.Fatalf("cached playtime: %+v", out.Games)
	}
	// A stale entry is refetched; a 429 keeps the cached time instead of 0.
	cache := filepath.Join(data, "playtime.json")
	stale := `{"1": {"minutes": 90, "fetched": 0}, "2": {"minutes": 0, "fetched": 0}}`
	if err := os.WriteFile(cache, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	hits = map[string]int{}
	storeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		switch r.URL.Path {
		case "/account/getFilteredProducts":
			w.Write([]byte(`{"totalPages": 1, "products": [{"id": 1}, {"id": 2}]}`))
		case "/games/1/users/42/sessions":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/games/2/users/42/sessions":
			w.Write([]byte(`{"time_sum": 7}`))
		default:
			http.NotFound(w, r)
		}
	})
	pyJSON(t, script, &out)
	if len(out.Games) != 2 || out.Games[0].ID != "1" || out.Games[0].Minutes != 90 ||
		out.Games[1].ID != "2" || out.Games[1].Minutes != 7 {
		t.Fatalf("playtime after 429: %+v", out.Games)
	}
	if hits["/games/1/users/42/sessions"] != 1 || hits["/games/2/users/42/sessions"] != 1 {
		t.Fatalf("stale entries not refetched: %v", hits)
	}
}

func TestGogInstallOptionsAndSizes(t *testing.T) {
	data := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", data)
	manifests := filepath.Join(data, "gogdl", "heroic_gogdl", "manifests")
	if err := os.MkdirAll(manifests, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"version": 2, "baseProductId": "1234", "HGLInstallLanguage": "de-DE",
	  "HGLdlcs": [{"id": "111", "title": "Season Pass"}], "depots": []}`
	if err := os.WriteFile(filepath.Join(manifests, "1234"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Groups []struct {
			ID      string `json:"id"`
			Kind    string `json:"kind"`
			DLC     bool   `json:"dlc"`
			Default string `json:"default"`
			Values  []struct {
				ID        string `json:"id"`
				Owned     bool   `json:"owned"`
				Installed bool   `json:"installed"`
				SizeBytes int64  `json:"sizeBytes"`
			} `json:"values"`
		} `json:"groups"`
	}
	pyJSON(t, `
import json
infos = {"size": {"*": {"download_size": 1, "disk_size": 2},
                  "en-US": {"download_size": 3, "disk_size": 4},
                  "de-DE": {"download_size": 5, "disk_size": 6}},
         "languages": ["en-US", "de-DE"], "buildId": "8888", "versionName": "1.0",
         "dlcs": [{"id": "111", "title": "Season Pass",
                   "size": {"de-DE": {"download_size": 7, "disk_size": 8}}},
                  {"id": "222", "title": "Deluxe",
                   "size": {"de-DE": {"download_size": 9, "disk_size": 10}}}],
         "available_branches": [None], "builds": {"items": []}}
hg.gogdl = lambda *a, **k: (0, json.dumps(infos), "")
hg.platform_of = lambda app: "windows"
print(json.dumps(hg.game_options({"id": "1234"})))
`, &out)
	if len(out.Groups) != 2 {
		t.Fatalf("groups: %+v", out.Groups)
	}
	language, dlc := out.Groups[0], out.Groups[1]
	if language.ID != "language" || language.Kind != "select" || language.Default != "de-DE" ||
		len(language.Values) != 2 || language.Values[0].ID != "en-US" || language.Values[1].ID != "de-DE" {
		t.Fatalf("language group: %+v", language)
	}
	if dlc.ID != "dlc" || dlc.Kind != "multi" || !dlc.DLC || len(dlc.Values) != 2 {
		t.Fatalf("dlc group: %+v", dlc)
	}
	if v := dlc.Values[0]; v.ID != "111" || !v.Owned || !v.Installed || v.SizeBytes != 8 {
		t.Fatalf("installed dlc: %+v", v)
	}
	if v := dlc.Values[1]; v.ID != "222" || !v.Owned || v.Installed || v.SizeBytes != 10 {
		t.Fatalf("uninstalled dlc: %+v", v)
	}

	raw := pyRun(t, `
import json
infos = {"size": {"*": {"download_size": 1, "disk_size": 2},
                  "en-US": {"download_size": 3, "disk_size": 4},
                  "de-DE": {"download_size": 5, "disk_size": 6}},
         "dlcs": [{"id": "111", "title": "Season Pass",
                   "size": {"de-DE": {"download_size": 7, "disk_size": 8}}}],
         "available_branches": [None], "builds": {"items": []}}
hg.gogdl = lambda *a, **k: (0, json.dumps(infos), "")
hg.platform_of = lambda app: "windows"
a = hg.install_size({"id": "1234", "options": {"language": "de-DE", "dlc": ["111"]}})
b = hg.install_size({"id": "1234"})
print(json.dumps([a, b]))
`)
	var both []map[string]int64
	if err := json.Unmarshal([]byte(raw), &both); err != nil {
		t.Fatalf("sizes: %v (%s)", err, raw)
	}
	if both[0]["downloadBytes"] != 13 || both[0]["diskBytes"] != 16 {
		t.Fatalf("sizes with options: %+v", both[0])
	}
	if both[1]["downloadBytes"] != 3 || both[1]["diskBytes"] != 4 {
		t.Fatalf("sizes without options: %+v", both[1])
	}
}

func TestGogRelocated(t *testing.T) {
	data := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", data)
	installed := `{"1234": {"path": "/games/Old", "executable": "/games/Old/start.sh",
	  "platform": "windows", "title": "Super Game"}}`
	if err := os.WriteFile(filepath.Join(data, "installed.json"), []byte(installed), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := pyRun(t, `
import json, os
ap = hg.game_relocated({"id": "1234", "from": "/games/Old", "to": "/games/New"})
rec = json.load(open(os.path.join(os.environ["PLATYPUS_PLUGIN_DATA"], "installed.json")))["1234"]
codes = []
for params in ({"id": "1234", "from": "/games/Elsewhere", "to": "/games/X"}, {"id": "1234"}):
    try:
        hg.game_relocated(params)
    except hg.RPCError as e:
        codes.append(e.code)
print(json.dumps({"out": ap, "path": rec["path"], "exe": rec["executable"], "codes": codes}))
`)
	var out struct {
		Out   map[string]any `json:"out"`
		Path  string         `json:"path"`
		Exe   string         `json:"exe"`
		Codes []int          `json:"codes"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("relocated: %v (%s)", err, raw)
	}
	if out.Path != "/games/New" || out.Exe != "/games/New/start.sh" {
		t.Fatalf("rebased record: %+v", out)
	}
	if len(out.Codes) != 2 || out.Codes[0] != -32602 || out.Codes[1] != -32602 {
		t.Fatalf("error codes: %v", out.Codes)
	}
}

// gogdl compares a depot's language with the installed one by code, English
// name or legacy code, and nothing else (no prefix matching).
func TestGogLanguageMatching(t *testing.T) {
	raw := pyRun(t, `
import json
cases = [
    (["*"], "en-US", True),
    (["en"], "en-US", True),
    (["English"], "en-US", True),
    (["en_GB"], "en-GB", True),
    (["de"], "de-DE", True),
    (["German"], "de-DE", True),
    (["es"], "es-ES", True),
    (["Spanish"], "es-ES", True),
    (["es_mx"], "es-MX", True),
    (["pt"], "pt-PT", True),
    (["br"], "pt-BR", True),
    (["cn"], "zh-Hans", True),
    (["zh"], "zh-Hans", True),
    (["en"], "en-GB", False),
    (["en-US"], "en-GB", False),
    (["de"], "de-AT", False),
    (["English"], "de-DE", False),
]
print(json.dumps([[hg.lang_matches(depots, installed) for depots, installed, _ in cases],
                  [want for _, _, want in cases],
                  [hg.gog_locale(l) for l in ("fr", "pt-br", "en", "de-DE", "xx", "")]]))
`)
	var all []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &all); err != nil || len(all) != 3 {
		t.Fatalf("languages: %v (%s)", err, raw)
	}
	var matches, wants []bool
	var locales []string
	if err := json.Unmarshal(all[0], &matches); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(all[1], &wants); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(all[2], &locales); err != nil {
		t.Fatal(err)
	}
	for i, want := range wants {
		if matches[i] != want {
			t.Fatalf("case %d: got %v, want %v", i, matches[i], want)
		}
	}
	gotLocales := []string{"fr-FR", "pt-BR", "en-US", "de-DE", "en-US", "en-US"}
	for i, want := range gotLocales {
		if locales[i] != want {
			t.Fatalf("locale %d: got %q, want %q", i, locales[i], want)
		}
	}
}

func TestGogCaseInsensitivePath(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "Data", "X.TXT")
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := pyRun(t, `
import json, os, sys
base = sys.argv[1]
print(json.dumps([hg.case_insensitive_path(os.path.join(base, *parts))
                  for parts in (("data", "x.txt"), ("Data", "x.txt"), ("data", "X.TXT"))]))
`, dir)
	var got []string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("paths: %v (%s)", err, raw)
	}
	for _, p := range got {
		if p != want {
			t.Fatalf("case-insensitive path = %q, want %q", p, want)
		}
	}
}

func TestGogIsoUnlockTimes(t *testing.T) {
	raw := pyRun(t, `
import json
print(json.dumps([hg.iso_unix("2024-11-05T10:27:37+0000"),
                  hg.iso_unix("2024-11-05T10:27:37.000Z"),
                  hg.iso_unix("not a date")]))
`)
	var got []*int64
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("dates: %v (%s)", err, raw)
	}
	want := time.Date(2024, 11, 5, 10, 27, 37, 0, time.UTC).Unix()
	if len(got) != 3 || got[0] == nil || *got[0] != want || got[1] == nil || *got[1] != want || got[2] != nil {
		t.Fatalf("unlock times: %v", got)
	}
}

func TestGogVerifyCancelsMidHash(t *testing.T) {
	dir := t.TempDir()
	game := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(game, bytes.Repeat([]byte{7}, 2<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := pyRun(t, `
import hashlib, json, sys
path, gamedir = sys.argv[1], sys.argv[2]
with open(path, "rb") as f:
    chunks = [{"size": 1 << 20, "md5": hashlib.md5(f.read(1 << 20)).hexdigest()} for _ in range(2)]
calls = {"n": 0}
hg.canceled = lambda job: calls.__setitem__("n", calls["n"] + 1) or calls["n"] > 1
mid = hg.hashes_match("j", path, chunks)
calls["n"] = 0
fetches = []
hg.jobs = {"j": {"canceled": False}}
hg.stored_manifest = lambda app: {"version": 2, "baseProductId": "1",
    "HGLInstallLanguage": "en-US", "HGLdlcs": [], "depots": [
        {"manifest": "aa", "productId": "1", "languages": ["*"]},
        {"manifest": "bb", "productId": "1", "languages": ["*"]}]}
hg.api_get_zjson = lambda url: fetches.append(url) or {"depot": {"items": []}}
report = hg.verify_files("j", "1", gamedir, "windows")
print(json.dumps({"mid": mid, "report": report, "fetches": len(fetches)}))
`, game, dir)
	var out struct {
		Mid     *bool `json:"mid"`
		Report  any   `json:"report"`
		Fetches int   `json:"fetches"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("cancel: %v (%s)", err, raw)
	}
	if out.Mid != nil {
		t.Fatalf("hash of a canceled job = %v, want null", *out.Mid)
	}
	if out.Report != nil || out.Fetches != 1 {
		t.Fatalf("depot fetches after cancel: %+v", out)
	}
}

func TestGogOptionRefusals(t *testing.T) {
	raw := pyRun(t, `
import json
infos = {"languages": ["en-US", "de-DE"],
         "dlcs": [{"id": "111", "title": "Season Pass",
                   "size": {"en-US": {"download_size": 1, "disk_size": 2}}}]}
hg.credentials = lambda: {"access_token": "t"}
hg.gogdl = lambda *a, **k: (0, json.dumps(infos), "")
hg.platform_of = lambda app: "windows"
hg.installed_games = lambda: {}
codes = []
for options in ({"language": "xx-XX"}, {"dlc": ["999"]}):
    try:
        hg.check_options("1234", "windows", options)
    except hg.RPCError as e:
        codes.append(e.code)
hg.check_options("1234", "windows", {"language": "de-DE", "dlc": ["111"]})
hg.gogdl = lambda *a, **k: (1, "", "[cli] ERROR: no builds")
hg.api_get = lambda url, token="", headers=None: {"downloads": {"installers": []}}
nothing = hg.game_options({"id": "999"})
sizes = {"*": {"download_size": 4, "disk_size": 5},
         "en-US": {"download_size": 1, "disk_size": 2}}
print(json.dumps({"codes": codes, "nothing": nothing, "disk": hg.dlc_disk_bytes(sizes, "en-US")}))
`)
	var out struct {
		Codes   []int          `json:"codes"`
		Nothing map[string]any `json:"nothing"`
		Disk    int64          `json:"disk"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("options: %v (%s)", err, raw)
	}
	if len(out.Codes) != 2 || out.Codes[0] != -32602 || out.Codes[1] != -32602 {
		t.Fatalf("refusal codes: %v", out.Codes)
	}
	if groups, ok := out.Nothing["groups"].([]any); !ok || len(groups) != 0 {
		t.Fatalf("non-installable product options: %+v", out.Nothing)
	}
	if out.Disk != 7 {
		t.Fatalf("dlc disk bytes = %d, want 7 (its language plus *)", out.Disk)
	}
}

func TestGogExtraWithInstallersListsNoOptions(t *testing.T) {
	// Some goodies collections list catalog installers gogdl's content
	// system cannot install; options must answer empty groups, not an error.
	raw := pyRun(t, `
import json
hg.credentials = lambda: {"access_token": "t"}
hg.gogdl = lambda *a, **k: (1, "", "[GENERIC DOWNLOAD_MANAGER] ERROR: Game doesn't support content system api, unable to proceed using platform windows")
hg.api_get = lambda url, token="", headers=None: {"downloads": {"installers": [{"os": "windows"}]}}
hg.platform_of = lambda app: "windows"
print(json.dumps(hg.game_options({"id": "1980301910"})))
`)
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("options: %v (%s)", err, raw)
	}
	if groups, ok := out["groups"].([]any); !ok || len(groups) != 0 {
		t.Fatalf("extra product options: %+v", out)
	}
}
