package steam_test

// Unit-level checks of platypus-steam helpers, run by loading the script as a
// module (no Steam, no downloader).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const pyLoad = `
import importlib.machinery, importlib.util, sys
sys.path.insert(0, "bin")
loader = importlib.machinery.SourceFileLoader("hs", "bin/platypus-steam")
spec = importlib.util.spec_from_loader("hs", loader)
hs = importlib.util.module_from_spec(spec)
loader.exec_module(hs)
`

func pyRun(t *testing.T, script string) string {
	t.Helper()
	out, err := exec.Command("python3", "-c", pyLoad+script).CombinedOutput()
	if err != nil {
		t.Fatalf("python: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// STEAM-4: the beta branch and its password reach platypus-steam-dl's stdin
// request (never argv), for the size query and for install/update.
func TestSteamBranchPasswordInRequest(t *testing.T) {
	got := pyRun(t, `
import threading
hs.refresh_token = lambda: "tok"
reqs = []
def fake_run(sub, req, job_id=None, game_dir=None):
    reqs.append((sub, dict(req)))
    return {"os_used": "linux", "disk_bytes": 1, "download_bytes": 1}
hs.dl_run = fake_run
hs.record_install = lambda *a: None
hs.notify = lambda *a: None
hs.install_size({"id": "4321", "branch": "beta", "branchPassword": "s3cret"})
hs.follow_install("j1", "4321", "/tmp/g", "update", {"branch": "beta", "branchPassword": "s3cret"})
hs.follow_install("j2", "4321", "/tmp/g", "install")
print([(s, r["branch"], r.get("branch_password")) for s, r in reqs])
`)
	want := "[('size', 'beta', 's3cret'), ('update', 'beta', 's3cret'), ('install', 'public', None)]"
	if got != want {
		t.Fatalf("requests = %s, want %s", got, want)
	}
}

// The password never reaches the plugin's logs or job table.
func TestSteamBranchPasswordNotRetained(t *testing.T) {
	got := pyRun(t, `
hs.refresh_token = lambda: "tok"
hs.dl_run = lambda *a, **k: {"os_used": "linux"}
hs.record_install = lambda *a: None
sent = []
hs.notify = lambda m, p: sent.append((m, p))
hs.follow_install("j1", "4321", "/tmp/g", "install", {"branch": "beta", "branchPassword": "s3cret"})
print("s3cret" in repr(sent) or "s3cret" in repr(hs.jobs))
`)
	if got != "False" {
		t.Fatalf("branch password leaked into notifications or the job table")
	}
}

// STEAM-7: a token source persisted as JSON (list, mtime maybe an int) still
// matches the live account.config stamp, so the legacy fast path hits.
func TestSteamTokenSourceSurvivesJSONRoundTrip(t *testing.T) {
	got := pyRun(t, `
import json
src = ("/h/account.config", 1700000000.5)
rt = json.loads(json.dumps(src))
print(hs._source_matches(rt, "/h/account.config", 1700000000.5),
      hs._source_matches([ "/h/account.config", 1700000000], "/h/account.config", 1700000000.0),
      hs._source_matches(rt, "/h/other", 1700000000.5),
      hs._source_matches(rt, "/h/account.config", 1700000001.5),
      hs._source_matches(None, "/h/account.config", 1.0))
`)
	if got != "True True False False False" {
		t.Fatalf("_source_matches = %q", got)
	}
}

// STEAM-ART: newer apps (Aniimo 4126040) only have hashed asset paths, so
// the fixed CDN names 404. resolve_art_assets names the real paths via the
// keyless GetItems API, preferring the _2x file; numeric asset fields
// (last_modified etc.) must not break the parse.
func TestSteamArtAssetsHashed(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	got := pyRun(t, `
hs.art_cache = None
def fake_api(url, token=None):
    assert "GetItems" in url, url
    assert "access_token" not in url, "GetItems must stay keyless"
    return {"response": {"store_items": [
      {"appid": 4126040, "assets": {
        "asset_url_format": "steam/apps/4126040/${FILENAME}?t=111",
        "library_capsule": "a/cap.jpg", "library_capsule_2x": "a/cap2x.jpg",
        "library_hero": "a/hero.jpg", "library_hero_2x": "a/hero2x.jpg",
        "header": "a/hd.jpg", "header_2x": "a/hd2x.jpg",
        "last_modified": 1700000000, "flag": True, "nothing": None}},
      {"appid": 367520, "assets": {
        "asset_url_format": "steam/apps/367520/${FILENAME}?t=9",
        "library_capsule": "h/c.jpg", "library_hero": "h/h.jpg", "header": "h/hd.jpg"}}]}}
hs.api_get = fake_api
import json
print(json.dumps(hs.resolve_art_assets(["4126040", "367520"]), sort_keys=True))
`)
	want := `{"367520": {"bannerUrl": "https://shared.steamstatic.com/store_item_assets/steam/apps/367520/h/hd.jpg?t=9", "coverUrl": "https://shared.steamstatic.com/store_item_assets/steam/apps/367520/h/c.jpg?t=9", "heroUrl": "https://shared.steamstatic.com/store_item_assets/steam/apps/367520/h/h.jpg?t=9"}, "4126040": {"bannerUrl": "https://shared.steamstatic.com/store_item_assets/steam/apps/4126040/a/hd2x.jpg?t=111", "coverUrl": "https://shared.steamstatic.com/store_item_assets/steam/apps/4126040/a/cap2x.jpg?t=111", "heroUrl": "https://shared.steamstatic.com/store_item_assets/steam/apps/4126040/a/hero2x.jpg?t=111"}}`
	if got != want {
		t.Fatalf("resolve_art_assets =\n%s\nwant\n%s", got, want)
	}
}

// STEAM-ART: an app GetItems doesn't list, or a failed request, keeps the
// fixed CDN fallback: the resolver answers {} and never raises.
func TestSteamArtAssetsFallback(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	got := pyRun(t, `
hs.art_cache = None
hs.api_get = lambda url, token=None: {"response": {"store_items": []}}
print(hs.resolve_art_assets(["4321"]))
hs.api_get = lambda url, token=None: (_ for _ in ()).throw(IOError("down"))
print(hs.resolve_art_assets(["4321"]), hs.resolve_art_assets([]))
print(hs.art_urls("4321"))
`)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 || lines[0] != "{}" || lines[1] != "{} {}" {
		t.Fatalf("fallback = %q", got)
	}
	if !strings.Contains(lines[2], "cdn.cloudflare.steamstatic.com/steam/apps/4321/library_600x900.jpg") {
		t.Fatalf("fixed CDN fallback changed: %q", lines[2])
	}
}

// STEAM-ART: ids resolve in batches of ART_BATCH (one request per batch).
func TestSteamArtAssetsBatched(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	got := pyRun(t, `
import json, urllib.parse
hs.art_cache = None
calls = []
def fake_api(url, token=None):
    calls.append(url)
    q = urllib.parse.parse_qs(urllib.parse.urlsplit(url).query)
    ids = [str(i["appid"]) for i in json.loads(q["input_json"][0])["ids"]]
    return {"response": {"store_items": [
      {"appid": int(a), "assets": {
        "asset_url_format": "steam/apps/%s/${FILENAME}?t=1" % a,
        "library_capsule": "h/c.jpg", "library_hero": "h/h.jpg", "header": "h/hd.jpg"}}
      for a in ids]}}
hs.api_get = fake_api
hs.ART_BATCH = 2
out = hs.resolve_art_assets(["11", "22", "33", "44", "55"])
print(len(calls), len(out), sorted(out))
`)
	if got != "3 5 ['11', '22', '33', '44', '55']" {
		t.Fatalf("batching = %q", got)
	}
}

// STEAM-ART: the second listing uses the cache (no request); entries older
// than the TTL are refreshed; a spent budget or failed request still uses
// stale cache entries instead of failing.
func TestSteamArtAssetsCacheTTL(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	got := pyRun(t, `
import time
hs.art_cache = None
calls = []
def fake_api(url, token=None):
    calls.append(1)
    return {"response": {"store_items": [{"appid": 77, "assets": {
      "asset_url_format": "steam/apps/77/${FILENAME}?t=AAA",
      "library_capsule": "h/c.jpg", "library_hero": "h/h.jpg", "header": "h/hd.jpg"}}]}}
hs.api_get = fake_api
first = hs.resolve_art_assets(["77"])
def boom(url, token=None):
    raise IOError("network down")
hs.api_get = boom
second = hs.resolve_art_assets(["77"])  # fresh cache: no request
ok_cache = (first == second and len(calls) == 1)
hs.ART_TTL = -1  # everything is stale now
def fake2(url, token=None):
    calls.append(1)
    return {"response": {"store_items": [{"appid": 77, "assets": {
      "asset_url_format": "steam/apps/77/${FILENAME}?t=BBB",
      "library_capsule": "h/c.jpg", "library_hero": "h/h.jpg", "header": "h/hd.jpg"}}]}}
hs.api_get = fake2
third = hs.resolve_art_assets(["77"])  # stale: refetched
ok_refresh = ("BBB" in third["77"]["coverUrl"] and len(calls) == 2)
hs.ART_TTL = -1
hs.api_get = boom
stale = hs.resolve_art_assets(["77"], time.time() - 1)  # spent budget, network down
ok_stale = (stale == third)
print(ok_cache, ok_refresh, ok_stale, len(calls))
`)
	if got != "True True True 2" {
		t.Fatalf("cache/ttl = %q", got)
	}
}

// STEAM-9: the game's exe often sits in a subfolder of the install.
func TestSteamFindExecutableSearchesSubfolders(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "bin", "x64")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "Game.exe"), make([]byte, 4000), 0o755)
	os.WriteFile(filepath.Join(dir, "setup.exe"), make([]byte, 9000), 0o755)
	os.MkdirAll(filepath.Join(dir, "redist"), 0o755)
	os.WriteFile(filepath.Join(dir, "redist", "Big.exe"), make([]byte, 50000), 0o755)
	cmd := exec.Command("python3", "-c", pyLoad+`print(hs.find_executable(sys.argv[1]))`, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != filepath.Join(sub, "Game.exe") {
		t.Fatalf("find_executable = %q, want bin/x64/Game.exe", got)
	}
}

// I18N-4: with locale fr, the store details request carries l=french and the
// launch-mode note is French; English is the default and the fallback.
func TestSteamLocaleDetailsAndModeNote(t *testing.T) {
	got := pyRun(t, `
hs.set_locale("fr")
urls = []
def fake_api(url, token=None):
    urls.append(url)
    return {"367520": {"success": True, "data": {
        "short_description": "x", "genres": [], "developers": [],
        "publishers": [], "release_date": {}, "screenshots": []}}}
hs.api_get = fake_api
hs.resolve_art_assets = lambda apps, deadline: {}
hs.game_details({"id": "367520"})
print("l=french" in urls[0], hs.mode_note("none").startswith("Mode de lancement"))
hs.set_locale("en")
print(hs.mode_note("client").startswith("Launch mode"), hs.steam_lang())
hs.set_locale("de")
print(hs.tr("mode_none").startswith("Launch mode"))
`)
	if got != "True True\nTrue english\nTrue" {
		t.Fatalf("locale fr/en = %q", got)
	}
}

// I18N-4: user-facing login errors are French under fr.
func TestSteamLocaleErrors(t *testing.T) {
	got := pyRun(t, `
hs.set_locale("fr")
print(hs.tr("need_login"), "|", hs.tr("not_installed", app="10"))
hs.set_locale("en")
print(hs.tr("need_login"))
`)
	if got != "connectez-vous à Steam d'abord | l'application 10 n'est pas installée\nlog in to Steam first" {
		t.Fatalf("locale errors = %q", got)
	}
}
