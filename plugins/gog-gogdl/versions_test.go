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
	"strings"
	"testing"
	"time"

	"platypus/internal/jobs"
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

func versionsList(t *testing.T, buildsFile, ledger string) map[string]any {
	t.Helper()
	if buildsFile != "" {
		t.Setenv("PLATYPUS_GOG_BUILDS_FILE", buildsFile)
	} else {
		t.Setenv("PLATYPUS_GOG_BUILDS_FILE", filepath.Join(t.TempDir(), "missing.json"))
	}
	got := pyRun(t, `
import json
infos = {"8888": `+quotePy(versionsInfo)+`}
infos["7777"] = {"size": {"en-US": {"download_size": 8388608, "disk_size": 12582912}},
 "buildId": "7777", "versionName": "0.9", "languages": ["en-US"],
 "folder_name": "SuperGame", "dependencies": [], "available_branches": [None],
 "dlcs": [], "builds": json.loads(infos["8888"])["builds"]}
def fake_gogdl(*a, **k):
    args = list(a)
    if args[0] == "info":
        b = args[args.index("--build") + 1] if "--build" in args else "8888"
        if b in infos:
            v = infos[b]
            return (0, v if isinstance(v, str) else json.dumps(v), "")
        return (1, "", "[cli] ERROR: unknown build")
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
	// info --build 5555 fails: the store no longer serves it.
	out := versionsList(t, "", ledger)
	byID := buildsByID(t, out)
	if byID["8888"]["source"] != "store" || byID["8888"]["name"] != "1.0" {
		t.Fatalf("latest build: %+v", byID["8888"])
	}
	if byID["7777"]["source"] != "store" || byID["7777"]["publishedAt"] != "2023-12-12" {
		t.Fatalf("older build: %+v", byID["7777"])
	}
	arch := byID["5555"]
	if arch == nil || arch["source"] != "archive" || arch["available"] != false {
		t.Fatalf("ledger-only build should be an unavailable archive entry: %+v", arch)
	}
	if out["manual"] != nil {
		t.Fatalf("complete list should offer no build-id box: %v", out["manual"])
	}
	if out["updateAvailable"] != false {
		t.Fatalf("nothing installed, no update: %v", out["updateAvailable"])
	}
}

func TestGogVersionsListMarksHiddenBuilds(t *testing.T) {
	hook := filepath.Join(t.TempDir(), "builds.json")
	os.WriteFile(hook, []byte(`{"total_count": 7, "count": 2, "has_private_branches": true,
 "items": [{"build_id": "8888"}, {"build_id": "7777"}]}`), 0o644)
	out := versionsList(t, hook, `[]`)
	manual, _ := out["manual"].(map[string]any)
	if manual == nil || manual["label"] != "GOG build id" || !strings.Contains(manual["hint"].(string), "5 more") {
		t.Fatalf("hidden builds should offer a build-id box naming the gap: %v", out["manual"])
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
