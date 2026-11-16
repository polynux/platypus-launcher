// Cloud saves (issue #6) through the plugin's real JSON-RPC surface: param
// errors, unsupported, status/pull/push/resolve against the fake CM
// (testdata/fake_cm.py) Cloud handlers, with file bytes over its loopback
// HTTP server. No Valve network. Scratch dirs only: the CloudContext
// homes/prefixes/install dirs all live under the test root, never the real
// home or a real prefix.
package steam_test

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/types"
)

const cloudUFSAppInfo = `"appinfo"
{
	"appid"		"4321"
	"common" { "name" "Standalone Game" "type" "Game" }
	"ufs"
	{
		"quota" "524288"
		"maxnumfiles" "5"
		"savefiles"
		{
			"0" { "root" "GameInstall" "path" "saves" "pattern" "*.sav" }
			"1" { "root" "GameInstall" "path" "saves" "pattern" "*.cfg" }
		}
	}
}`

// cloudCtx builds a CloudContext rooted in the test's scratch dirs.
func (e *env) cloudCtx(t *testing.T) (map[string]any, string) {
	t.Helper()
	home := t.TempDir()
	install := filepath.Join(home, "install")
	if err := os.MkdirAll(filepath.Join(install, "saves"), 0o755); err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"os": "linux", "home": home, "installDir": install,
		"xdgData":   filepath.Join(home, ".local", "share"),
		"xdgConfig": filepath.Join(home, ".config"),
	}, install
}

func cloudSHA(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// cloudKnobs serves one cloud file through the fake: the changelist entry
// plus the download body (plain or single-entry zip).
func (e *env) cloudKnobs(name, content string, zipIt bool, mtime int64) {
	e.t.Helper()
	var spec map[string]any
	if zipIt {
		spec = map[string]any{"zip": map[string]any{"save.sav": base64.StdEncoding.EncodeToString([]byte(content))}}
	} else {
		spec = map[string]any{"body": base64.StdEncoding.EncodeToString([]byte(content))}
	}
	spec["time"] = mtime
	e.setCMState("cloud_changelist", map[string]any{
		"current_change_number": 7,
		"path_prefixes":         []any{"%GameInstall%saves/"},
		"machine_names":         []any{"test-machine"},
		"files": []any{map[string]any{
			"name": strings.TrimPrefix(name, "%GameInstall%saves/"),
			"sha":  cloudSHA(content), "time": mtime,
			"size": float64(len(content)), "persist": float64(0),
			"prefix": float64(0), "machine": float64(0),
		}},
	})
	e.setCMState("cloud_files", map[string]any{name: spec})
	e.setCMState("appinfo_4321", cloudUFSAppInfo)
}

func (e *env) cloudCall(t *testing.T, method string, params map[string]any) (types.CloudStatus, error) {
	t.Helper()
	var st types.CloudStatus
	err := e.call(method, params, &st)
	return st, err
}

// Parameter errors come before any CM work, the way plugtest drives them.
func TestCloudParamErrors(t *testing.T) {
	e := setup(t)
	ctx, _ := e.cloudCtx(t)
	for _, tc := range []struct {
		method string
		params map[string]any
	}{
		{"cloud.status", map[string]any{}},
		{"cloud.status", map[string]any{"id": "4321"}},
		{"cloud.status", map[string]any{"id": "4321", "ctx": map[string]any{"os": "plan9"}}},
		{"cloud.status", map[string]any{"id": "4321", "ctx": map[string]any{"os": "linux"}}},
		{"cloud.status", map[string]any{"id": "nope", "ctx": ctx}},
		{"cloud.pull", map[string]any{"id": "4321", "ctx": ctx}},
		{"cloud.push", map[string]any{"id": "4321", "ctx": ctx}},
		{"cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "j"}},
		{"cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "j", "choices": map[string]string{"a": "sideways"}}},
	} {
		var st types.CloudStatus
		if err := e.call(tc.method, tc.params, &st); e.rpcCode(err) != -32602 {
			t.Fatalf("%s %+v: want -32602, got %v", tc.method, tc.params, err)
		}
	}
	// A choice for a path nobody listed is refused without touching the CM.
	e.login()
	e.setCMState("appinfo_4321", cloudUFSAppInfo)
	e.setCMState("cloud_changelist", map[string]any{})
	var st types.CloudStatus
	err := e.call("cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "j-bad", "choices": map[string]string{"no/such/file": "local"}}, &st)
	if e.rpcCode(err) != -32602 {
		t.Fatalf("resolve of an unlisted path: want -32602, got %v", err)
	}
}

// No ufs rules (the fixture appinfo without ufs) is unsupported, not an
// error: the game launches as is.
func TestCloudUnsupportedWithoutRules(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, _ := e.cloudCtx(t)
	e.setCMState("appinfo_4321", `"appinfo" { "appid" "4321" "common" { "name" "Standalone Game" } }`)
	st, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "unsupported" || st.Message == "" {
		t.Fatalf("status = %+v, want unsupported with a reason", st)
	}
	if st.Counts != (types.CloudCounts{}) {
		t.Fatalf("counts = %+v", st.Counts)
	}
	// Push on an unsupported game stays unsupported and uploads nothing.
	push, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-unsupported"})
	if err != nil {
		t.Fatal(err)
	}
	if push.State != "unsupported" {
		t.Fatalf("push = %+v", push)
	}
}

// Pull downloads what changed only in the cloud, verifies SHA-1, stamps the
// mtime, and records the baseline: the second pull (new jobId) is synced,
// and a repeated jobId answers the first run's result.
func TestCloudPullEndToEnd(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.cloudKnobs("%GameInstall%saves/a.sav", "cloud-save", false, 1700000000)

	st, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "pending" || st.Counts.Download != 1 {
		t.Fatalf("status = %+v, want pending with one download", st)
	}

	pulled, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-1"})
	if err != nil {
		t.Fatal(err)
	}
	if pulled.State != "synced" {
		t.Fatalf("pull = %+v, want synced", pulled)
	}
	got, err := os.ReadFile(filepath.Join(install, "saves", "a.sav"))
	if err != nil || string(got) != "cloud-save" {
		t.Fatalf("downloaded = %q, %v", got, err)
	}
	if st, _ := os.Stat(filepath.Join(install, "saves", "a.sav")); st.ModTime().Unix() != 1700000000 {
		t.Fatalf("mtime = %v", st.ModTime())
	}
	// The baseline now pins the file: a fresh pull is synced, and the
	// repeated jobId answers the first run's stored result.
	again, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-1"})
	if err != nil {
		t.Fatal(err)
	}
	if again.State != pulled.State || again.Counts != pulled.Counts {
		t.Fatalf("repeated jobId: %+v vs %+v", again, pulled)
	}
	fresh, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-2"})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.State != "synced" || fresh.Counts.Same != 1 {
		t.Fatalf("second pull = %+v", fresh)
	}
	raw, err := os.ReadFile(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json"))
	if err != nil {
		t.Fatalf("baseline missing: %v", err)
	}
	var base struct {
		Files map[string]struct {
			SHA string `json:"sha"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatal(err)
	}
	if base.Files["saves/a.sav"].SHA != cloudSHA("cloud-save") {
		t.Fatalf("baseline = %s", raw)
	}
}

// A zipped cloud body unpacks before the SHA check.
func TestCloudPullZip(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.cloudKnobs("%GameInstall%saves/z.sav", "zipped-save", true, 1700000001)
	pulled, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-zip"})
	if err != nil {
		t.Fatal(err)
	}
	if pulled.State != "synced" {
		t.Fatalf("pull = %+v", pulled)
	}
	if got, _ := os.ReadFile(filepath.Join(install, "saves", "z.sav")); string(got) != "zipped-save" {
		t.Fatalf("unzipped = %q", got)
	}
}

// A SHA mismatch writes nothing and fails the pull.
func TestCloudPullBadSHA(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.setCMState("appinfo_4321", cloudUFSAppInfo)
	e.setCMState("cloud_changelist", map[string]any{
		"current_change_number": 7,
		"path_prefixes":         []any{"%GameInstall%saves/"},
		"machine_names":         []any{"test-machine"},
		"files": []any{map[string]any{
			"name": "bad.sav", "sha": cloudSHA("original"),
			"time": float64(1700000000), "size": float64(8),
			"persist": float64(0), "prefix": float64(0), "machine": float64(0),
		}},
	})
	e.setCMState("cloud_files", map[string]any{
		"%GameInstall%saves/bad.sav": map[string]any{
			"body": base64.StdEncoding.EncodeToString([]byte("tampered")),
			"sha":  cloudSHA("original"),
			"time": float64(1700000000),
		},
	})
	var st types.CloudStatus
	if err := e.call("cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-bad"}, &st); err == nil {
		t.Fatal("a SHA mismatch must fail the pull")
	}
	if _, err := os.Stat(filepath.Join(install, "saves", "bad.sav")); !os.IsNotExist(err) {
		t.Fatal("a failed download must write nothing")
	}
}

// Conflicts are never overwritten: resolve remote downloads with a backup,
// resolve local uploads this device's copy.
func TestCloudConflictResolve(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	write := func(content string) {
		if err := os.MkdirAll(filepath.Join(install, "saves"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(install, "saves", "c.sav"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("local-version")
	e.cloudKnobs("%GameInstall%saves/c.sav", "cloud-version", false, 1700000000)

	st, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "conflict" || len(st.Conflicts) != 1 || st.Conflicts[0].Path != "saves/c.sav" {
		t.Fatalf("status = %+v, want one conflict", st)
	}
	if st.Conflicts[0].Remote.Machine != "test-machine" || st.Conflicts[0].Local.Size != int64(len("local-version")) {
		t.Fatalf("conflict sides = %+v", st.Conflicts[0])
	}

	// Pull alone leaves the conflict in place.
	pulled, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-conflict"})
	if err != nil {
		t.Fatal(err)
	}
	if pulled.State != "conflict" {
		t.Fatalf("pull with a conflict = %+v", pulled)
	}

	// Remote: download with a backup of the local file.
	resolved, err := e.cloudCall(t, "cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "resolve-remote",
		"choices": map[string]string{"saves/c.sav": "remote"}})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.State != "synced" {
		t.Fatalf("resolve remote = %+v", resolved)
	}
	if got, _ := os.ReadFile(filepath.Join(install, "saves", "c.sav")); string(got) != "cloud-version" {
		t.Fatalf("resolved = %q", got)
	}
	if resolved.BackupDir == "" {
		t.Fatal("a replaced local file must report its backup dir")
	}
	entries, err := os.ReadDir(resolved.BackupDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("backups = %v, %v", entries, err)
	}
	backup, err := os.ReadFile(filepath.Join(resolved.BackupDir, entries[0].Name(), "saves", "c.sav"))
	if err != nil || string(backup) != "local-version" {
		t.Fatalf("backup = %q, %v", backup, err)
	}

	// Local: change both sides again (a newer cloud save and a newer local
	// file) so the status is a conflict the choices can settle. A local
	// choice uploads this device's copy with the push path.
	write("local-newer")
	e.cloudKnobs("%GameInstall%saves/c.sav", "cloud-newer", false, 1700000002)
	st, err = e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "conflict" {
		t.Fatalf("changed on both sides again = %+v", st)
	}
	kept, err := e.cloudCall(t, "cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "resolve-local",
		"choices": map[string]string{"saves/c.sav": "local"}})
	if err != nil {
		t.Fatal(err)
	}
	if kept.State != "synced" || kept.Counts.Same != 1 {
		t.Fatalf("resolve local = %+v, want the upload applied", kept)
	}
	// The kept-local choice is reported the plan's way.
	if log := e.cmLog(); !strings.Contains(log, "conflict-resolution 4321 1") {
		t.Fatalf("log misses the kept-local report: %s", log)
	}
	// The repeated jobId answers the same stored result.
	keptAgain, err := e.cloudCall(t, "cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "resolve-local",
		"choices": map[string]string{"saves/c.sav": "local"}})
	if err != nil {
		t.Fatal(err)
	}
	if keptAgain.State != kept.State || keptAgain.Counts != kept.Counts {
		t.Fatalf("repeated resolve jobId: %+v vs %+v", keptAgain, kept)
	}
	// The fake holds the uploaded bytes: removing the local file and
	// pulling brings back exactly what the local choice uploaded.
	if err := os.Remove(filepath.Join(install, "saves", "c.sav")); err != nil {
		t.Fatal(err)
	}
	back, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-after-resolve-local"})
	if err != nil {
		t.Fatal(err)
	}
	if back.State != "synced" {
		t.Fatalf("pull after resolve local = %+v", back)
	}
	if got, _ := os.ReadFile(filepath.Join(install, "saves", "c.sav")); string(got) != "local-newer" {
		t.Fatalf("downloaded = %q, want the uploaded copy", got)
	}
}

// Encrypted cloud files are refused: the pull fails and writes nothing.
func TestCloudPullEncrypted(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.setCMState("appinfo_4321", cloudUFSAppInfo)
	e.setCMState("cloud_changelist", map[string]any{
		"current_change_number": 7,
		"path_prefixes":         []any{"%GameInstall%saves/"},
		"machine_names":         []any{"test-machine"},
		"files": []any{map[string]any{
			"name": "enc.sav", "sha": cloudSHA("secret"),
			"time": float64(1700000000), "size": float64(6),
			"persist": float64(0), "prefix": float64(0), "machine": float64(0),
		}},
	})
	e.setCMState("cloud_files", map[string]any{
		"%GameInstall%saves/enc.sav": map[string]any{
			"body": base64.StdEncoding.EncodeToString([]byte("secret")),
			"time": float64(1700000000), "encrypted": true,
		},
	})
	var st types.CloudStatus
	if err := e.call("cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-enc"}, &st); err == nil {
		t.Fatal("an encrypted file must fail the pull")
	}
	if _, err := os.Stat(filepath.Join(install, "saves", "enc.sav")); !os.IsNotExist(err) {
		t.Fatal("a refused download must write nothing")
	}
}

// Resolve remote on a file the cloud deleted: the local file is backed up
// and removed, its baseline entry dropped — no conflict loop afterwards.
func TestCloudResolveRemoteOnDeleted(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	write := func(content string) {
		if err := os.MkdirAll(filepath.Join(install, "saves"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(install, "saves", "d.sav"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("doomed-local")
	e.setCMState("appinfo_4321", cloudUFSAppInfo)
	e.setCMState("cloud_changelist", map[string]any{
		"current_change_number": 7,
		"path_prefixes":         []any{"%GameInstall%saves/"},
		"machine_names":         []any{"test-machine"},
		"files": []any{map[string]any{
			"name": "d.sav", "sha": cloudSHA("x"),
			"time": float64(1700000000), "size": float64(1),
			"persist": float64(2), "prefix": float64(0), "machine": float64(0),
		}},
	})
	st, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "conflict" {
		t.Fatalf("deleted in the cloud but present here must ask: %+v", st)
	}
	resolved, err := e.cloudCall(t, "cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "resolve-deleted",
		"choices": map[string]string{"saves/d.sav": "remote"}})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.State != "synced" {
		t.Fatalf("resolve remote on a deletion = %+v", resolved)
	}
	if _, err := os.Stat(filepath.Join(install, "saves", "d.sav")); !os.IsNotExist(err) {
		t.Fatal("taking the cloud deletion must remove the local file")
	}
	if resolved.BackupDir == "" {
		t.Fatal("a removed local file must be backed up first")
	}
	st, err = e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "synced" {
		t.Fatalf("no conflict loop afterwards: %+v", st)
	}
}

// Forgotten cloud entries (persist_state 1) are not ours to sync: the local
// file counts as its own upload candidate instead of a conflict.
func TestCloudForgottenEntrySkipped(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	if err := os.MkdirAll(filepath.Join(install, "saves"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "saves", "f.sav"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.setCMState("appinfo_4321", cloudUFSAppInfo)
	e.setCMState("cloud_changelist", map[string]any{
		"current_change_number": 7,
		"path_prefixes":         []any{"%GameInstall%saves/"},
		"machine_names":         []any{"test-machine"},
		"files": []any{map[string]any{
			"name": "f.sav", "sha": cloudSHA("other"),
			"time": float64(1700000000), "size": float64(5),
			"persist": float64(1), "prefix": float64(0), "machine": float64(0),
		}},
	})
	st, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "pending" || st.Counts.Upload != 1 {
		t.Fatalf("a forgotten entry must not conflict: %+v", st)
	}
}

// Resolve on a game without rules refuses every choice path before the
// unsupported answer; an empty choice set stays unsupported.
func TestCloudResolveWithoutRules(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, _ := e.cloudCtx(t)
	e.setCMState("appinfo_4321", `"appinfo" { "appid" "4321" "common" { "name" "Standalone Game" } }`)
	var st types.CloudStatus
	err := e.call("cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "resolve-norules",
		"choices": map[string]string{"a.sav": "remote"}}, &st)
	if e.rpcCode(err) != -32602 {
		t.Fatalf("a choice path with no rules: want -32602, got %v", err)
	}
	st, err = e.cloudCall(t, "cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "resolve-norules-empty",
		"choices": map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "unsupported" {
		t.Fatalf("empty choices with no rules = %+v", st)
	}
}

// A client-mode game syncs itself through the real client: Platypus answers
// unsupported without touching the saves.
func TestCloudClientModeUnsupported(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	data, _ := json.Marshal(map[string]any{"4321": map[string]any{"mode": "client", "path": install}})
	if err := os.WriteFile(filepath.Join(e.pluginData(), "installed.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "unsupported" || !strings.Contains(st.Message, "Steam syncs this game itself") {
		t.Fatalf("client-mode game = %+v", st)
	}
}
