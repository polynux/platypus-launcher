// Cloud saves uploads (issue #6 P3) through the plugin's real JSON-RPC
// surface: the Steam batch flow against the fake CM (testdata/fake_cm.py)
// upload handlers, with block bytes over its loopback HTTP server. No Valve
// network. Scratch dirs only, like the other cloud tests.
package steam_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/types"
)

// pushKnobs serves an empty cloud listing, so a local save uploads as new.
func (e *env) pushKnobs(t *testing.T) {
	t.Helper()
	e.setCMState("cloud_changelist", map[string]any{"current_change_number": float64(5)})
	e.setCMState("cloud_files", map[string]any{})
	e.setCMState("appinfo_4321", cloudUFSAppInfo)
}

func pushBaselineSHA(t *testing.T, e *env, rel, content string) {
	t.Helper()
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
	if base.Files[rel].SHA != cloudSHA(content) {
		t.Fatalf("baseline = %s, want sha of %q", raw, content)
	}
}

// Upload round trip: status, push, status says same, and the fake holds the
// bytes with the right sha (a later pull brings back exactly what went up).
// Split blocks prove the client honors block_offset/block_length.
func TestCloudPushRoundTrip(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	e.setCMState("cloud_split_blocks", true)
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte("hello-saves"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "pending" || st.Counts.Upload != 1 || !strings.Contains(st.Message, "waiting to upload") {
		t.Fatalf("status = %+v, want one pending upload", st)
	}

	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-1"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" || pushed.Counts.Same != 1 {
		t.Fatalf("push = %+v, want synced with the upload counted same", pushed)
	}
	log := e.waitCMLog("upload complete 1 1")
	if !strings.Contains(log, "upload begin 1 1 files") || !strings.Contains(log, "2 blocks") {
		t.Fatalf("log misses the batch: %s", log)
	}
	pushBaselineSHA(t, e, "saves/a.sav", "hello-saves")

	// The repeated jobId answers the first run's stored result.
	again, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-1"})
	if err != nil {
		t.Fatal(err)
	}
	if again.State != pushed.State || again.Counts != pushed.Counts {
		t.Fatalf("repeated push jobId: %+v vs %+v", again, pushed)
	}

	// A fresh status reads back the upload: same, not an upload again.
	fresh, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.State != "synced" || fresh.Counts.Same != 1 {
		t.Fatalf("status after push = %+v", fresh)
	}

	// The fake holds the bytes: removing the local file and pulling
	// brings back exactly what went up.
	if err := os.Remove(filepath.Join(install, "saves", "a.sav")); err != nil {
		t.Fatal(err)
	}
	back, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-after-push"})
	if err != nil {
		t.Fatal(err)
	}
	if back.State != "synced" {
		t.Fatalf("pull after push = %+v", back)
	}
	if got, _ := os.ReadFile(filepath.Join(install, "saves", "a.sav")); string(got) != "hello-saves" {
		t.Fatalf("downloaded = %q", got)
	}
}

// A batch that fails mid-flight (the file open is refused) is completed
// with eresult 2, and the baseline stays untouched for the retry.
func TestCloudPushFailsMidBatch(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte("doomed"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.setCMState("cloud_upload_eresult", float64(2))

	var st types.CloudStatus
	err := e.call("cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-fail"}, &st)
	if err == nil || !strings.Contains(err.Error(), "upload") {
		t.Fatalf("err = %v, want the failed upload", err)
	}
	log := e.waitCMLog("upload complete 1 2")
	if !strings.Contains(log, "upload begin 1 1 files") {
		t.Fatalf("log misses the begun batch: %s", log)
	}
	if _, statErr := os.Stat(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a failed push must leave the baseline untouched")
	}
}

// The cloud changing since the baseline is a conflict: the push lists it,
// uploads nothing, and writes nothing.
func TestCloudPushConflictWhenRemoteChanged(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.cloudKnobs("%GameInstall%saves/c.sav", "base", false, 1700000000)
	if err := os.WriteFile(filepath.Join(install, "saves", "c.sav"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "synced" {
		t.Fatalf("status = %+v, want the baseline recorded", st)
	}
	if err := os.WriteFile(filepath.Join(install, "saves", "c.sav"), []byte("local2"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.cloudKnobs("%GameInstall%saves/c.sav", "cloud2", false, 1700000002)
	st, err = e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "conflict" {
		t.Fatalf("status = %+v, want a conflict", st)
	}

	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-conflict"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "conflict" || pushed.Counts.Conflict != 1 {
		t.Fatalf("push = %+v, want the conflict listed", pushed)
	}
	if log := e.cmLog(); strings.Contains(log, "upload begin") {
		t.Fatalf("a conflicted push named a batch: %s", log)
	}
	if got, _ := os.ReadFile(filepath.Join(install, "saves", "c.sav")); string(got) != "local2" {
		t.Fatalf("local file = %q, must be untouched", got)
	}
}

// Over quota fails before anything is named, with a clear message.
func TestCloudPushQuotaExceeded(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	big := strings.Repeat("x", 200)
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	e.setCMState("cloud_quota", map[string]any{
		"used_bytes": float64(900), "max_bytes": float64(1000),
		"used_files": float64(1), "max_files": float64(5),
	})

	var st types.CloudStatus
	err := e.call("cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-quota"}, &st)
	if err == nil || !strings.Contains(err.Error(), "full") {
		t.Fatalf("err = %v, want the quota refusal", err)
	}
	if log := e.cmLog(); strings.Contains(log, "upload begin") {
		t.Fatalf("an over-quota push named a batch: %s", log)
	}
	if _, statErr := os.Stat(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json")); !os.IsNotExist(statErr) {
		t.Fatalf("an over-quota push must leave the baseline untouched")
	}

	e.setCMState("cloud_quota", map[string]any{
		"used_files": float64(5), "max_files": float64(5),
	})
	err = e.call("cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-quota-files"}, &st)
	if err == nil || !strings.Contains(err.Error(), "full") {
		t.Fatalf("err = %v, want the file-count refusal", err)
	}
}

// Canceling mid-upload completes the begun batch with failure and leaves
// the baseline untouched.
func TestCloudPushCancelMidUpload(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte("cancel-me"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.setCMState("cloud_upload_hang", true)

	pushDone := make(chan error, 1)
	go func() {
		callCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var st types.CloudStatus
		pushDone <- e.m.Call(callCtx, id, "cloud.push",
			map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-cancel"}, &st)
	}()
	// The block PUT has landed when the fake logs it; the hang keeps the
	// batch begun, so cancel must complete it with failure.
	e.waitCMLog("upload http")
	var canceled map[string]any
	if err := e.call("job.cancel", map[string]any{"jobId": "push-cancel"}, &canceled); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-pushDone:
		if err == nil {
			t.Fatal("a canceled push must fail")
		}
	case <-time.After(60 * time.Second):
		t.Fatal("the canceled push never returned")
	}
	e.waitCMLog("upload complete 1 2")
	if _, statErr := os.Stat(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a canceled push must leave the baseline untouched")
	}
}
