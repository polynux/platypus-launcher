package gog_test

// Version selection (protocol 1.3, issue #4): versions.list shapes, target
// handling, the short-install check and the stall timeout. Unit-level checks
// load platypus-gog as a module with a stubbed gogdl; the job-level checks
// drive the real plugin path against testdata/gogdl like gog_test.go.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/types"
)

const versionsInfo = `{
 "size": {"*": {"download_size": 0, "disk_size": 0},
          "en-US": {"download_size": 8388608, "disk_size": 12582912}},
 "buildId": "8888", "versionName": "1.0",
 "languages": ["en-US"], "folder_name": "SuperGame", "dependencies": [],
 "available_branches": [null], "dlcs": [],
 "builds": {"total_count": 2, "count": 2, "items": [
   {"build_id": "8888", "version_name": "1.0", "date_published": "2024-01-08T12:35:53+0000", "branch": null, "generation": 2},
   {"build_id": "7777", "version_name": "0.9", "date_published": "2023-12-12T10:27:37+0000", "branch": null, "generation": 2}]}}`

func versionsList(t *testing.T, ledger string) map[string]any {
	t.Helper()
	got := pyRun(t, `
import json, os
infos = {"8888": json.loads(`+quotePy(versionsInfo)+`)}
infos["7777"] = {"size": {"en-US": {"download_size": 8388608, "disk_size": 12582912}},
 "buildId": "7777", "versionName": "0.9", "languages": ["en-US"],
 "folder_name": "SuperGame", "dependencies": [], "available_branches": [None],
 "dlcs": [], "builds": json.loads(json.dumps(infos["8888"]["builds"]))}
# The store counts more builds than it names; no manual box exists anymore.
for v in infos.values():
    v["builds"]["total_count"] = 7
def fake_gogdl(*a, **k):
    args = list(a)
    if args[0] == "info":
        # Like real gogdl, an unmatched --build keeps the default build.
        b = args[args.index("--build") + 1] if "--build" in args else "8888"
        if "--build" in args:
            raise AssertionError("versions.list must not probe builds")
        return (0, json.dumps(infos.get(b, infos["8888"])), "")
    raise AssertionError(args)
hg.gogdl = fake_gogdl
hg.platform_of = lambda app: "windows"
hg.credentials = lambda: {"access_token": "t"}
print(json.dumps(hg.versions_list({"id": "1234", "ledger": `+ledger+`})))
`)
	var out map[string]any
	if err := json.Unmarshal([]byte(got), &out); err != nil {
		t.Fatalf("versions.list printed no JSON: %v", got)
	}
	return out
}

// quotePy inlines JSON into a python string literal.
func quotePy(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func buildsByID(t *testing.T, out map[string]any) map[string]map[string]any {
	t.Helper()
	byID := map[string]map[string]any{}
	raw, _ := out["builds"].([]any)
	for _, b := range raw {
		m, _ := b.(map[string]any)
		byID[m["id"].(string)] = m
	}
	return byID
}
func TestGogParseProgressBothFormats(t *testing.T) {
	got := pyRun(t, `
print(hg.parse_progress("Progress: 42.13% (123.45/292.88 MiB) @ 5.61 MiB/s"))
print(hg.parse_progress("[PROGRESS] INFO: = Progress: 22.36 16802208/75133843, Running for: 00:17:14, ETA: 00:59:59"))
print(hg.parse_progress("[V2] INFO: Nothing to do"))
`)
	want := `(42.13, 129446707, 307106938, 5882511.36)
(22.36, 16802208, 75133843, None)
None`
	if got != want {
		t.Fatalf("parse_progress:\n%s\nwant:\n%s", got, want)
	}
}

func TestGogVersionsListBuildsAndArchive(t *testing.T) {
	ledger := `[{"store": "gog-gogdl", "game": "p.gog-gogdl.1234", "build": "5555",
 "versionName": "0.5", "date": "2023-01-01T00:00:00Z"}]`
	// A ledger build the store doesn't list is unavailable directly: no
	// probing (gogdl would answer the default build for it anyway).
	out := versionsList(t, ledger)
	byID := buildsByID(t, out)
	if byID["8888"]["source"] != "store" || byID["8888"]["name"] != "1.0" {
		t.Fatalf("latest build: %+v", byID["8888"])
	}
	if byID["7777"]["source"] != "store" || byID["7777"]["publishedAt"] != "2023-12-12" {
		t.Fatalf("older build: %+v", byID["7777"])
	}
	arch := byID["5555"]
	if arch == nil || arch["source"] != "archive" || arch["available"] != false ||
		arch["name"] != "0.5" {
		t.Fatalf("ledger-only build should be an unavailable archive entry with the ledger name: %+v", arch)
	}
	if out["manual"] != nil {
		t.Fatalf("no manual box: %v", out["manual"])
	}
	if out["updateAvailable"] != false {
		t.Fatalf("nothing installed, no update: %v", out["updateAvailable"])
	}
}

func TestGogVersionsListHasNoManualBox(t *testing.T) {
	// The stub counts more builds than it names; unservable ids get no box.
	out := versionsList(t, `[]`)
	if out["manual"] != nil {
		t.Fatalf("private-branch ids are uninstallable, no box: %v", out["manual"])
	}
}

func TestGogResolveTarget(t *testing.T) {
	got := pyRun(t, `
hg.platform_of = lambda app: "windows"
print(hg.resolve_target(None))
print(hg.resolve_target({"build": "57122283917433645"}))
print(hg.resolve_target({"manual": {"input": " 57122283917433645\n"}}))
try:
    hg.resolve_target({"build": "latest-ish"})
except hg.RPCError as e:
    print("error:", e.code, e)
`)
	lines := strings.Split(got, "\n")
	if len(lines) != 4 || lines[0] != "('', '')" || lines[1] != "('57122283917433645', '')" ||
		lines[1] != lines[2] || !strings.HasPrefix(lines[3], "error: -32000") {
		t.Fatalf("resolve_target: %q", got)
	}
}

func waitState(t *testing.T, q *jobs.Queue, jobID string, states ...string) types.Job {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		for _, x := range q.List() {
			if x.ID != jobID {
				continue
			}
			for _, s := range states {
				if x.State == s {
					return x
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %s never reached %v: %+v", jobID, states, q.List())
	return types.Job{}
}

// installTargeted queues an install carrying a Versions target, which the
// test backend forwards to the plugin like production does (installParams).
func installTargeted(t *testing.T, q *jobs.Queue, game types.Game, folder string, target *types.VersionTarget) types.Job {
	t.Helper()
	j, err := q.AddTargeted(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder, target)
	if err != nil {
		t.Fatal(err)
	}
	return waitState(t, q, j.ID, jobs.StateDone, jobs.StateFailed)
}

func TestGogInstallTargetBuild(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	j := installTargeted(t, q, lib[0], folder, &types.VersionTarget{Build: "7777"})
	if j.State != jobs.StateDone {
		t.Fatalf("targeted install: %+v", j)
	}
	readCalls(t).check(t, "--build 7777")
	// The plugin recorded the requested build, so versions.list marks it
	// current and offers the newer latest as an update.
	var vi types.VersionInfo
	if err := call(t, m, "versions.list", map[string]any{"id": "1234"}, &vi); err != nil {
		t.Fatalf("versions.list: %v", err)
	}
	current := map[string]bool{}
	for _, b := range vi.Builds {
		current[b.ID] = b.Current
	}
	if !current["7777"] || current["8888"] {
		t.Fatalf("current build: %+v", current)
	}
	if !vi.UpdateAvailable {
		t.Fatal("old build installed, latest known: update should be offered")
	}
}

// A run gogdl reports as done but leaves far short on disk fails loudly
// instead of recording a broken install (issue #4: the old-build download).
func TestGogShortInstallFails(t *testing.T) {
	t.Setenv("FAKE_GOGDL_SHORT", "1")
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	j := installTargeted(t, q, lib[0], folder, nil)
	if j.State != jobs.StateFailed {
		t.Fatalf("short install: %+v", j)
	}
	if !strings.Contains(j.Error, "incomplete") {
		t.Fatalf("short install error should say incomplete: %q", j.Error)
	}
}

// A run that stops moving fails with a stall message instead of hanging
// forever (issue #4: the update stuck at 22%).
func TestGogStalledDownloadFails(t *testing.T) {
	t.Setenv("FAKE_GOGDL_STALL", "1")
	t.Setenv("PLATYPUS_GOG_STALL_BYTES_S", "2")
	t.Setenv("PLATYPUS_GOG_STALL_QUIET_S", "60")
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	j := installTargeted(t, q, lib[0], folder, nil)
	if j.State != jobs.StateFailed {
		t.Fatalf("stalled download: %+v", j)
	}
	if !strings.Contains(j.Error, "stalled") {
		t.Fatalf("stall error should say stalled: %q", j.Error)
	}
}

// Progress lines keep arriving but the byte count never grows (the live
// stall: `= Progress: 22.36 16802208/…` every second at 0.00 MiB/s). The job
// must still fail on the bytes stall, not wait for silence that never comes.
func TestGogFrozenProgressFails(t *testing.T) {
	t.Setenv("FAKE_GOGDL_FROZEN", "1")
	t.Setenv("PLATYPUS_GOG_STALL_BYTES_S", "2")
	t.Setenv("PLATYPUS_GOG_STALL_QUIET_S", "60")
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	j := installTargeted(t, q, lib[0], folder, nil)
	if j.State != jobs.StateFailed {
		t.Fatalf("frozen download: %+v", j)
	}
	if !strings.Contains(j.Error, "no progress") {
		t.Fatalf("frozen error should name the stall: %q", j.Error)
	}
}

// An unlisted build id silently installs the default (real gogdl keeps the
// default on an unmatched --build). The plugin must fail loudly instead of
// recording the requested build.
func TestGogInstallUnknownBuildFails(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	j := installTargeted(t, q, lib[0], folder, &types.VersionTarget{Build: "9999"})
	if j.State != jobs.StateFailed {
		t.Fatalf("unlisted build install: %+v", j)
	}
	if !strings.Contains(j.Error, "not the requested build") {
		t.Fatalf("unlisted build error should name the mismatch: %q", j.Error)
	}
}

// installSize honours a target, and refuses an unserved build id.
func TestGogInstallSizeTarget(t *testing.T) {
	m, _, _, _ := setup(t)
	login(t, m)
	var size plugins.InstallSize
	if err := call(t, m, "game.installSize",
		map[string]any{"id": "1234", "target": map[string]any{"build": "7777"}}, &size); err != nil {
		t.Fatalf("targeted size: %v", err)
	}
	if size.DiskBytes != 12<<20 {
		t.Fatalf("targeted size: %+v", size)
	}
	var junk plugins.InstallSize
	if err := call(t, m, "game.installSize",
		map[string]any{"id": "1234", "target": map[string]any{"build": "9999"}}, &junk); err == nil {
		t.Fatal("unserved build size should fail")
	} else if !strings.Contains(err.Error(), "doesn't offer build 9999") {
		t.Fatalf("unserved build size error: %v", err)
	}
}

// A channel install passes --branch, records the channel, and versions.list
// marks the channel current with the channel head installed.
func TestGogChannelInstall(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	j := installTargeted(t, q, lib[0], folder,
		&types.VersionTarget{Channel: "beta", Build: "7776"})
	if j.State != jobs.StateDone {
		t.Fatalf("channel install: %+v", j)
	}
	readCalls(t).check(t, "--branch beta")
	var vi types.VersionInfo
	if err := call(t, m, "versions.list", map[string]any{"id": "1234"}, &vi); err != nil {
		t.Fatalf("versions.list: %v", err)
	}
	beta := false
	for _, c := range vi.Channels {
		if c.ID == "beta" && c.Current {
			beta = true
		}
	}
	if !beta {
		t.Fatalf("beta channel should be current: %+v", vi.Channels)
	}
	current := map[string]bool{}
	for _, b := range vi.Builds {
		current[b.ID] = b.Current
	}
	if !current["7776"] || current["8888"] {
		t.Fatalf("channel head build should be current: %+v", current)
	}
	if vi.UpdateAvailable {
		t.Fatal("channel head installed: no update should be offered")
	}
}

// An update can name a build: the plugin converges the installed game to it.
func TestGogUpdateTargetBuild(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	if j := installTargeted(t, q, lib[0], folder, nil); j.State != jobs.StateDone {
		t.Fatalf("latest install: %+v", j)
	}
	u, err := q.AddTargeted(jobs.KindUpdate, id, lib[0].ID, lib[0].StoreAppID,
		lib[0].Title, folder, &types.VersionTarget{Build: "7777"})
	if err != nil {
		t.Fatal(err)
	}
	u = waitState(t, q, u.ID, jobs.StateDone, jobs.StateFailed)
	if u.State != jobs.StateDone {
		t.Fatalf("targeted update: %+v", u)
	}
	readCalls(t).check(t, "update 1234")
	readCalls(t).check(t, "--build 7777")
	var vi types.VersionInfo
	if err := call(t, m, "versions.list", map[string]any{"id": "1234"}, &vi); err != nil {
		t.Fatalf("versions.list: %v", err)
	}
	for _, b := range vi.Builds {
		if b.ID == "7777" && !b.Current {
			t.Fatalf("downgraded build should be current: %+v", vi.Builds)
		}
	}
}

// Slow but moving downloads must not trip the stall detector: growing bytes,
// growing downloaded MiB and non-zero throughput all count as liveness.
func TestGogSlowDownloadSucceeds(t *testing.T) {
	t.Setenv("FAKE_GOGDL_SLOW", "1")
	t.Setenv("PLATYPUS_GOG_STALL_BYTES_S", "2")
	t.Setenv("PLATYPUS_GOG_STALL_QUIET_S", "60")
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	if j := installTargeted(t, q, lib[0], folder, nil); j.State != jobs.StateDone {
		t.Fatalf("slow download: %+v", j)
	}
}

// Chatter without any progress line trips the quiet timer, not the bytes one.
func TestGogQuietStallFails(t *testing.T) {
	t.Setenv("FAKE_GOGDL_QUIET", "1")
	t.Setenv("PLATYPUS_GOG_STALL_BYTES_S", "60")
	t.Setenv("PLATYPUS_GOG_STALL_QUIET_S", "2")
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	j := installTargeted(t, q, lib[0], folder, nil)
	if j.State != jobs.StateFailed {
		t.Fatalf("quiet download: %+v", j)
	}
	if !strings.Contains(j.Error, "went quiet") {
		t.Fatalf("quiet error should say quiet: %q", j.Error)
	}
}

// gogdl's parent may exit while a forked child still holds stdout: the job
// must finish from the reaped parent instead of hanging on EOF.
func TestGogOrphanedStdoutFinishes(t *testing.T) {
	t.Setenv("FAKE_GOGDL_ORPHAN", "1")
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	if j := installTargeted(t, q, lib[0], folder, nil); j.State != jobs.StateDone {
		t.Fatalf("orphan-stdout download: %+v", j)
	}
}

// The first real stall kills the run and retries once with a fresh process;
// only the second stall fails the job.
func TestGogStallRecoveryRetriesOnce(t *testing.T) {
	t.Setenv("FAKE_GOGDL_STALL_ONCE", "1")
	t.Setenv("PLATYPUS_GOG_STALL_BYTES_S", "2")
	t.Setenv("PLATYPUS_GOG_STALL_QUIET_S", "60")
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	if j := installTargeted(t, q, lib[0], folder, nil); j.State != jobs.StateDone {
		t.Fatalf("stall-recovery download: %+v", j)
	}
	body := readCalls(t).body
	if n := strings.Count(body, "download 1234"); n != 2 {
		t.Fatalf("want one retry (2 download runs), calls:\n%s", body)
	}
}

// Cancel mid-install, then install again: the resume converges and finishes.
func TestGogCancelResumeFinishesTheInstall(t *testing.T) {
	slowGogdl(t)
	m, q, store, folder := setup(t)
	login(t, m)
	game := firstGame(t, store)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	j, err := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, j.ID, started, "first progress")
	if err := q.Cancel(ctx, j.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, j.ID, inState(jobs.StateCanceled), "canceled")
	r, err := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitFor(t, q, r.ID, inState(jobs.StateDone), "resumed done"); done.ID != r.ID {
		t.Fatalf("resumed under another id: %+v", done)
	}
	if !firstGame(t, store).IsInstalled {
		t.Fatal("not installed after resume")
	}
}

// Expected size prefers en-US (what gogdl installs), then *, then the max:
// the max overstates multilingual games.
func TestGogBestSizesPrefersEnUS(t *testing.T) {
	got := pyRun(t, `
print(hg.best_sizes({"size": {"en-US": {"download_size": 100, "disk_size": 200},
                              "de-DE": {"download_size": 900, "disk_size": 1800}}}))
print(hg.best_sizes({"size": {"*": {"download_size": 0, "disk_size": 0},
                              "de-DE": {"download_size": 900, "disk_size": 1800}}}))
print(hg.best_sizes({"download_size": 5, "disk_size": 6}))
print(hg.best_sizes({"size": {"en-US": {"download_size": 0, "disk_size": 0}}}))
`)
	if got != "(100, 200)\n(900, 1800)\n(5, 6)\n(0, 0)" {
		t.Fatalf("best_sizes:\n%s", got)
	}
}

// A store failure listing versions is -32002; only login stays -32001.
func TestGogVersionsListFailureCodes(t *testing.T) {
	got := pyRun(t, `
hg.platform_of = lambda app: "windows"
hg.credentials = lambda: {"access_token": "t"}
hg.gogdl = lambda *a, **k: (1, "", "boom")
try:
    hg.versions_list({"id": "1"})
except hg.RPCError as e:
    print(e.code)
hg.gogdl = lambda *a, **k: (1, "not logged in", "")
hg.credentials = lambda: None
try:
    hg.versions_list({"id": "1"})
except hg.RPCError as e:
    print(e.code)
`)
	if got != "-32002\n-32001" {
		t.Fatalf("versions.list codes:\n%s", got)
	}
}

// installed.json from 0.1.0 has no build: versions.list fills current from
// the marker, and an installed build the store doesn't list claims no update.
func TestGogVersionsListOlderInstallNoUpdate(t *testing.T) {
	data := t.TempDir()
	game := t.TempDir()
	os.WriteFile(filepath.Join(game, "goggame-1234.info"), []byte(`{"buildId": "5555", "version": "0.5"}`), 0o644)
	os.WriteFile(filepath.Join(data, "installed.json"),
		[]byte(`{"1234": {"path": "`+game+`", "platform": "windows", "version": "0.5"}}`), 0o644)
	t.Setenv("PLATYPUS_PLUGIN_DATA", data)
	out := versionsList(t, `[{"store": "gog-gogdl", "game": "g", "build": "5555",
 "versionName": "0.5", "date": "2023-01-01T00:00:00Z"}]`)
	byID := buildsByID(t, out)
	if byID["5555"]["current"] != true {
		t.Fatalf("marker build should be current: %+v", byID["5555"])
	}
	if out["updateAvailable"] != false {
		t.Fatalf("unlisted installed build claims no update: %v", out["updateAvailable"])
	}
}

// A Linux installer has no builds and no goggame marker: the install
// succeeds through gameinfo + the linux manifest and records no version,
// and the game offers no version selection.
func TestGogLinuxInstallSkipsVersions(t *testing.T) {
	m, q, store, folder := setup(t)
	// The fake account game works on Linux and the fake product API offers
	// a Linux installer, so the plugin picks the linux platform.
	api := os.Getenv("PLATYPUS_GOG_API_FILE")
	raw, err := os.ReadFile(api)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["downloads"] = map[string]any{"installers": []any{map[string]any{"os": "linux"}}}
	products := doc["products"].([]any)
	products[0].(map[string]any)["worksOn"] = map[string]any{"Windows": false, "Mac": false, "Linux": true}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(api, out, 0o644); err != nil {
		t.Fatal(err)
	}
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	for _, f := range lib[0].Features {
		if f == "versions" {
			t.Fatalf("linux installer should not offer versions: %+v", lib[0].Features)
		}
	}
	j := installTargeted(t, q, lib[0], folder, nil)
	if j.State != jobs.StateDone {
		t.Fatalf("linux install: %+v", j)
	}
	raw, err = os.ReadFile(filepath.Join(m.DataDirOf(id), "installed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	if rec["1234"]["build"] != "" || rec["1234"]["path"] == "" {
		t.Fatalf("linux install records no build: %+v", rec["1234"])
	}
	if !strings.HasSuffix(rec["1234"]["path"].(string), "SuperGame") {
		t.Fatalf("linux game dir not resolved: %+v", rec["1234"])
	}
}

// stop_proc kills the whole job process group: a child that ignores SIGINT
// must be gone after the call, not just the parent. Unix-only: without
// killpg the plugin can only signal the parent.
func TestGogStopProcKillsSignallgnorantGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs killpg")
	}
	got := pyRun(t, `
import subprocess, sys, time, os
kid_code = "import signal, time\nsignal.signal(signal.SIGINT, signal.SIG_IGN)\ntime.sleep(60)"
par_code = ("import subprocess, sys, time\n"
            "c = subprocess.Popen([sys.executable, '-c', sys.argv[1]])\n"
            "print(c.pid, flush=True)\n"
            "time.sleep(60)")
p = hg.spawn_gogdl([sys.executable, "-c", par_code, kid_code])
child = int(p.stdout.readline().strip())
hg.stop_proc(p, grace=1)
time.sleep(1)
print("parent-dead:", p.poll() is not None)
try:
    os.kill(child, 0)
    print("child: alive")
except OSError:
    print("child: gone")
`)
	if !strings.Contains(got, "parent-dead: True") || !strings.Contains(got, "child: gone") {
		t.Fatalf("group kill:\n%s", got)
	}
}

// The stall retry deletes the chunk cache in folders this job created (from
// jobs-meta) and never globs the install folder.
func TestGogClearDownloadCacheUsesClaimedDirs(t *testing.T) {
	data := t.TempDir()
	installDir := t.TempDir()
	claimed := filepath.Join(installDir, "NewGame", ".gogdl-download-cache")
	if err := os.MkdirAll(claimed, 0o755); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(installDir, ".gogdl-download-cache")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(map[string]any{"job9": map[string]any{
		"app": "1234", "installDir": installDir, "pre": []string{}, "dirs": []string{}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "jobs-meta.json"), meta, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLATYPUS_PLUGIN_DATA", data)
	got := pyRun(t, `
import os
hg.jobs["job9"] = {"id": "1234", "path": "`+installDir+`", "platform": "windows"}
hg.clear_download_cache("job9")
print("claimed-gone:", not os.path.isdir("`+claimed+`"))
print("stray-kept:", os.path.isdir("`+stray+`"))
`)
	if !strings.Contains(got, "claimed-gone: True") || !strings.Contains(got, "stray-kept: True") {
		t.Fatalf("claimed dirs:\n%s", got)
	}
}

// On update, a matching legacy shared manifest is copied into the isolated
// cache; a mismatch (or nothing) only warns.
func TestGogPrimeLegacyManifest(t *testing.T) {
	data := t.TempDir()
	legacy := t.TempDir()
	game := t.TempDir()
	os.WriteFile(filepath.Join(game, "goggame-1234.info"), []byte(`{"buildId": "8888"}`), 0o644)
	os.MkdirAll(filepath.Join(legacy, "heroic_gogdl", "manifests"), 0o755)
	os.WriteFile(filepath.Join(legacy, "heroic_gogdl", "manifests", "1234"),
		[]byte(`{"buildId": "8888"}`), 0o644)
	t.Setenv("PLATYPUS_PLUGIN_DATA", data)
	t.Setenv("XDG_CONFIG_HOME", legacy)
	script := `
import os
hg.prime_legacy_manifest("1234", "` + game + `")
base = os.path.join("` + data + `", "gogdl")
print("copied:", os.path.isfile(os.path.join(base, "manifests", "1234")),
      os.path.isfile(os.path.join(base, "heroic_gogdl", "manifests", "1234")))
`
	if got := pyRun(t, script); !strings.Contains(got, "copied: True True") {
		t.Fatalf("matching legacy manifest should be copied:\n%s", got)
	}
	os.WriteFile(filepath.Join(game, "goggame-1234.info"), []byte(`{"buildId": "7777"}`), 0o644)
	if got := pyRun(t, script); !strings.Contains(got, "copied: True True") {
		t.Fatalf("existing isolated copy stays put:\n%s", got)
	}
}

// An update naming a build the store doesn't serve fails before gogdl
// starts (no wasted download of the default).
func TestGogUpdateUnlistedBuildFailsFast(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	if j := installTargeted(t, q, lib[0], folder, nil); j.State != jobs.StateDone {
		t.Fatalf("latest install: %+v", j)
	}
	before := readCalls(t).body
	err = call(t, m, "game.update",
		map[string]any{"id": "1234", "installDir": folder, "jobId": "up-bad",
			"target": map[string]any{"build": "9999"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "doesn't offer build 9999") {
		t.Fatalf("unlisted update target should fail fast: %v", err)
	}
	if after := readCalls(t).body; strings.Count(after, "update 1234") != strings.Count(before, "update 1234") {
		t.Fatalf("gogdl must not start for an unserved build")
	}
}
