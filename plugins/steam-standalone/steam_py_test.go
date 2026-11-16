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

// A stored password Steam refuses is dropped; an explicitly typed password
// that fails leaves the store alone, and public/no stored password is a
// no-op (issue #176, review finding S2).
func TestSteamRefusedPasswordDropRules(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", dir)
	got := pyRun(t, `
hs.save_beta_password("4321", "beta", "stored-pw")
hs.save_beta_password("4321", "other", "other-pw")
# The install sent the stored secret: Steam refused it, drop it.
hs.drop_refused_password("4321", {"branch": "beta", "branch_password": "stored-pw"})
# A typed password that failed is not the store's: keep everything.
hs.drop_refused_password("4321", {"branch": "other", "branch_password": "typed-pw"})
# Public and unknown channels never touch the store.
hs.drop_refused_password("4321", {"branch": "public", "branch_password": "stored-pw"})
hs.drop_refused_password("4321", {"branch": "beta"})
hs.drop_refused_password("9999", {"branch": "beta", "branch_password": "x"})
print(hs.beta_passwords())
`)
	if got != `{'4321': {'other': 'other-pw'}}` {
		t.Fatalf("beta_passwords = %s", got)
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

// Plugin state is private (issue #181): a standalone run creates its
// fallback data dir 0700 (and tightens a pre-fix 0755 one), and save_state
// writes 0600 over a pre-existing 0644 file.
func TestSteamStateFilesPrivate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PLATYPUS_PLUGIN_DATA", "")
	got := pyRun(t, `
import os, stat
d = hs.data_dir()
print(oct(stat.S_IMODE(os.stat(d).st_mode)))
`)
	if got != "0o700" {
		t.Fatalf("fallback data dir mode %s, want 0o700", got)
	}
	dirp := filepath.Join(home, ".platypus-steam-standalone")
	if err := os.Chmod(dirp, 0o755); err != nil {
		t.Fatal(err)
	}
	got = pyRun(t, `
import os, stat
p = hs.state_path("session.json")
with open(p, "w") as f:
    f.write("{}")
os.chmod(p, 0o644)
hs.save_state("session.json", {"account": "someone"})
print(oct(stat.S_IMODE(os.stat(hs.data_dir()).st_mode)),
      oct(stat.S_IMODE(os.stat(p).st_mode)))
`)
	if got != "0o700 0o600" {
		t.Fatalf("modes %q, want 0o700 0o600", got)
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

// The stats schema is binary KeyValues: nested dicts out, truncation and
// unknown value types refused as ValueError.
func TestSteamBinKVParse(t *testing.T) {
	got := pyRun(t, `
import struct
def w(out, node):
    for k, v in node.items():
        if isinstance(v, dict):
            out.append(0); out += k.encode() + b"\x00"; w(out, v)
        elif isinstance(v, str):
            out.append(1); out += k.encode() + b"\x00" + v.encode() + b"\x00"
        else:
            out.append(2); out += k.encode() + b"\x00" + struct.pack("<I", v)
    out.append(8)
o = bytearray(); o.append(0); o += b"4321\x00"
w(o, {"gamename": "G", "version": 1, "stats": {"0": {"type": "ACHIEVEMENTS"}}})
o.append(8)
print(hs.parse_bin_kv(bytes(o))["4321"]["stats"]["0"]["type"])
for bad in (b"\x02short", b"\x09k\x00", b"\x01k\x00v"):
    try:
        hs.parse_bin_kv(bad)
        print("kept")
    except ValueError:
        print("refused")
`)
	if got != "ACHIEVEMENTS\nrefused\nrefused\nrefused" {
		t.Fatalf("bin kv = %q", got)
	}
}

// achievements.list joins the schema (names, hidden, icons) with the unlock
// times and the global percentages; a bare filename maps to the community
// art path. Hidden counts "0" and "false" as shown, not just 0.
func TestSteamAchievementsJoin(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	got := pyRun(t, `
node = {"stats": {"0": {"type": "ACHIEVEMENTS", "bits": {
    "1": {"name": "A.ONE", "display": {"name": {"english": "One"},
        "desc": {"english": "First"}, "hidden": 0, "icon": "https://cdn/x.png"}},
    "2": {"name": "A.TWO", "display": "broken"},
    "3": {"name": "A.THREE", "display": {"name": {"english": "Three"},
        "desc": {"english": "Third"}, "hidden": "yes", "icon": "three.jpg"}},
    "4": {"name": "A.FOUR", "display": {"name": {}, "desc": "D",
        "hidden": "0", "icon": 7}}}}}}
defs = hs.schema_defs("4321", node)
print([(d["api"], d["bit"], d["name"], d["hidden"], d["icon"]) for d in defs])
hs.achievement_percents = lambda app: {"A.ONE": 84.1}
def fake_stats(app):
    return node, {("0", "1"): 1700000000}
hs.steam_user_stats = fake_stats
import json
print(json.dumps(hs.achievements_list({"id": "4321"}), sort_keys=True))
try:
    hs.achievements_list({})
except Exception as e:
    print("no-id:", getattr(e, "code", "?"))
`)
	icon3 := "https://cdn.cloudflare.steamstatic.com/steamcommunity/public/images/apps/4321/three.jpg"
	want := `[('A.ONE', '1', 'One', False, 'https://cdn/x.png'), ('A.TWO', '2', 'A.TWO', False, ''), ('A.THREE', '3', 'Three', True, 'three.jpg'), ('A.FOUR', '4', 'A.FOUR', False, '')]
{"list": [{"description": "First", "globalPercent": 84.1, "hidden": false, "icon": "https://cdn/x.png", "id": "A.ONE", "name": "One", "unlockedAt": 1700000000}, {"description": "", "hidden": false, "id": "A.TWO", "name": "A.TWO"}, {"description": "Third", "hidden": true, "icon": "` + icon3 + `", "id": "A.THREE", "name": "Three"}, {"description": "", "hidden": false, "id": "A.FOUR", "name": "A.FOUR"}], "total": 4, "unlocked": 1}
no-id: -32602`
	if got != want {
		t.Fatalf("join = %q", got)
	}
}

// A verify job pins the installed build (ledger target + stored password)
// and reports the helper's check payload on job.done.
func TestSteamVerifyRequestAndReport(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	got := pyRun(t, `
hs.refresh_token = lambda: "tok"
hs.notify = lambda m, p: sent.append((m, p))
sent, reqs = [], []
def fake_run(sub, req, job_id=None, game_dir=None, label="", err_code=-32000, phase="downloading"):
    reqs.append((sub, dict(req), phase))
    return {"checked": 10, "damaged": 1, "missing": 0, "repaired": 0}
hs.dl_run = fake_run
import json, os
dd = os.environ["PLATYPUS_PLUGIN_DATA"]
json.dump({"4321": {"path": "/g/4321", "channel": "beta", "build": "99",
    "archiveRef": {"depots": {"11": 4242}}}},
    open(os.path.join(dd, "installed.json"), "w"))
json.dump({"4321": {"beta": "s3cret"}}, open(os.path.join(dd, "beta_passwords.json"), "w"))
hs.follow_verify("jv", "4321", "/g/4321", True)
sub, req, phase = reqs[0]
print(sub, phase, req["repair"], req["target"], req.get("branch_passwords"))
print(sent[-1])
try:
    hs.start_verify({"id": "4321", "installDir": "relative", "jobId": "jv"})
except Exception as e:
    print("rel:", getattr(e, "code", "?"))
try:
    hs.start_verify({"id": "9999", "installDir": "/g/9999", "jobId": "jv"})
except Exception as e:
    print("unknown:", str(e)[:12].rstrip())
# No manifest pins in the ledger: a repair is refused, a check still starts.
json.dump({"4321": {"path": "/g/4321", "channel": "beta"}},
    open(os.path.join(dd, "installed.json"), "w"))
try:
    hs.start_verify({"id": "4321", "installDir": "/g/4321", "jobId": "jr", "repair": True})
except Exception as e:
    print("pinned:", str(e))
print(hs.start_verify({"id": "4321", "installDir": "/g/4321", "jobId": "jc"}))
`)
	want := `verify checking True {'archiveRef': {'depots': {'11': 4242}}, 'channel': 'beta', 'build': '99'} {'beta': 's3cret'}
('job.done', {'jobId': 'jv', 'verify': {'checked': 10, 'damaged': 1, 'missing': 0, 'repaired': 0}})
rel: -32000
unknown: app 9999 is
pinned: Update the game once to enable repair
{}`
	if got != want {
		t.Fatalf("verify = %q", got)
	}
}

// A second start with the same jobId while the first verify still runs is
// a no-op: one job.done, never two.
func TestSteamVerifyDuplicateJobId(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	got := pyRun(t, `
import threading, time
hs.refresh_token = lambda: "tok"
hs.notify = lambda m, p: sent.append((m, p))
sent = []
release = threading.Event()
def fake_run(sub, req, job_id=None, game_dir=None, label="", err_code=-32000, phase="downloading"):
    release.wait(10)
    return {"checked": 1, "damaged": 0, "missing": 0, "repaired": 0}
hs.dl_run = fake_run
import json, os
dd = os.environ["PLATYPUS_PLUGIN_DATA"]
json.dump({"4321": {"path": "/g/4321", "channel": "public",
    "build": "7", "archiveRef": {"depots": {"11": 1}}}},
    open(os.path.join(dd, "installed.json"), "w"))
first = threading.Thread(target=hs.start_verify,
    args=({"id": "4321", "installDir": "/g/4321", "jobId": "jdup"},))
first.start()
time.sleep(0.5)
print(hs.start_verify({"id": "4321", "installDir": "/g/4321", "jobId": "jdup"}))
release.set()
first.join()
# start_verify returns after spawning the worker: wait for its job.done.
t0 = time.time()
while time.time() - t0 < 10 and not [p for m, p in sent if m == "job.done"]:
    time.sleep(0.05)
print(len([p for m, p in sent if m == "job.done"]))
`)
	if got != "{}\n1" {
		t.Fatalf("duplicate start = %q", got)
	}
}
