// Cloud saves uploads (issue #6 P3) through the plugin's real JSON-RPC
// surface: the Steam batch flow against the fake CM (testdata/fake_cm.py)
// upload handlers, with block bytes over its loopback HTTP server. No Valve
// network. Scratch dirs only, like the other cloud tests.
package steam_test

import (
	"context"
	"encoding/base64"
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
	// The proto's uint64 fields go out as varints, the way real Steam
	// reads them (B1).
	for _, want := range []string{"client_id=varint", "upload_batch_id=varint", "batch_id=varint"} {
		if !strings.Contains(log, want) {
			t.Fatalf("log misses %q: %s", want, log)
		}
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

// A batch whose second file fails is completed with eresult 2 and the
// baseline stays untouched; clearing the failure lets a retry upload (S2).
func TestCloudPushMultiFileFailAndRetry(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	for name, content := range map[string]string{"a.sav": "aaa", "b.sav": "bbb"} {
		if err := os.WriteFile(filepath.Join(install, "saves", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e.setCMState("cloud_fail_files", []any{"%GameInstall%saves/b.sav"})

	var st types.CloudStatus
	err := e.call("cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-multi"}, &st)
	if err == nil {
		t.Fatal("a batch with a failed file must fail")
	}
	log := e.waitCMLog("upload complete 1 2")
	if !strings.Contains(log, "upload begin 1 2 files") {
		t.Fatalf("log misses the begun batch: %s", log)
	}
	if _, statErr := os.Stat(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a failed push must leave the baseline untouched")
	}

	e.setCMState("cloud_fail_files", []any{})
	retried, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-multi-2"})
	if err != nil {
		t.Fatal(err)
	}
	if retried.State != "synced" || retried.Counts.Same != 2 {
		t.Fatalf("retry = %+v, want both files synced", retried)
	}
}

// A rejected completion fails the push and the baseline is NOT written (S2).
func TestCloudPushCompleteRejected(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte("doomed"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.setCMState("cloud_complete_eresult", float64(2))

	var st types.CloudStatus
	err := e.call("cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-complete-reject"}, &st)
	if err == nil || !strings.Contains(err.Error(), "complete") {
		t.Fatalf("err = %v, want the rejected completion", err)
	}
	e.waitCMLog("upload complete 1 1")
	if _, statErr := os.Stat(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a push with rejected completion must not write the baseline")
	}
}

// A resolve `local` whose upload fails keeps the choice, and a retry
// uploads; the success reports the choice (S2, S7).
func TestCloudResolveLocalFailKeepsChoice(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	write := func(content string) {
		if err := os.WriteFile(filepath.Join(install, "saves", "c.sav"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("local-version")
	e.cloudKnobs("%GameInstall%saves/c.sav", "cloud-version", false, 1700000000)
	e.setCMState("cloud_fail_files", []any{"%GameInstall%saves/c.sav"})

	var st types.CloudStatus
	err := e.call("cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "resolve-local-fail",
		"choices": map[string]string{"saves/c.sav": "local"}}, &st)
	if err == nil {
		t.Fatal("a resolve whose upload fails must fail")
	}
	raw, rerr := os.ReadFile(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json"))
	if rerr != nil {
		t.Fatalf("baseline missing: %v", rerr)
	}
	var base struct {
		Choices map[string]struct {
			Side string `json:"side"`
		} `json:"choices"`
	}
	if jerr := json.Unmarshal(raw, &base); jerr != nil {
		t.Fatal(jerr)
	}
	if base.Choices["saves/c.sav"].Side != "local" {
		t.Fatalf("baseline choices = %s, want the kept-local choice", raw)
	}

	e.setCMState("cloud_fail_files", []any{})
	// The kept choice replays on the next push: the retry uploads it.
	retried, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-after-failed-resolve"})
	if err != nil {
		t.Fatal(err)
	}
	if retried.State != "synced" || retried.Counts.Same != 1 {
		t.Fatalf("push retry = %+v, want the kept choice uploaded", retried)
	}
}

// A cloud at 5/5 files still accepts a changed save: only the delta counts (S1).
func TestCloudPushQuotaOverwriteAtFileLimit(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.cloudKnobs("%GameInstall%saves/q.sav", "qq", false, 1700000000)
	if err := os.WriteFile(filepath.Join(install, "saves", "q.sav"), []byte("qq"), 0o644); err != nil {
		t.Fatal(err)
	}
	same, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if same.State != "synced" {
		t.Fatalf("status = %+v, want the baseline recorded", same)
	}
	if err := os.WriteFile(filepath.Join(install, "saves", "q.sav"), []byte("qqx"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.setCMState("cloud_quota", map[string]any{
		"used_files": float64(5), "max_files": float64(5),
		"used_bytes": float64(100), "max_bytes": float64(1000),
	})
	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-overwrite"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" || pushed.Counts.Same != 1 {
		t.Fatalf("push = %+v, want the overwrite accepted", pushed)
	}
}

// Block drops that say POST go out as POST (F1).
func TestCloudPushPostBlocks(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	e.setCMState("cloud_post_blocks", true)
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte("via-post"), 0o644); err != nil {
		t.Fatal(err)
	}
	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-post"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" {
		t.Fatalf("push = %+v", pushed)
	}
	if log := e.waitCMLog("upload complete 1 1"); !strings.Contains(log, "upload http POST %GameInstall%saves/a.sav 8") {
		t.Fatalf("log misses the POST block: %s", log)
	}
}

// A block carrying explicit_body_data sends those exact bytes (F1).
func TestCloudPushExplicitBody(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	content := "explicit-bytes"
	e.setCMState("cloud_explicit", map[string]any{
		"%GameInstall%saves/e.sav": map[string]any{
			"body": base64.StdEncoding.EncodeToString([]byte(content)), "post": true,
		},
	})
	if err := os.WriteFile(filepath.Join(install, "saves", "e.sav"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-explicit"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" {
		t.Fatalf("push = %+v", pushed)
	}
	log := e.waitCMLog("upload complete 1 1")
	if !strings.Contains(log, "upload http POST %GameInstall%saves/e.sav") {
		t.Fatalf("log misses the explicit POST block: %s", log)
	}
	if err := os.Remove(filepath.Join(install, "saves", "e.sav")); err != nil {
		t.Fatal(err)
	}
	back, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-after-explicit"})
	if err != nil {
		t.Fatal(err)
	}
	if back.State != "synced" {
		t.Fatalf("pull = %+v", back)
	}
	if got, _ := os.ReadFile(filepath.Join(install, "saves", "e.sav")); string(got) != content {
		t.Fatalf("downloaded = %q, want the explicit bytes", got)
	}
}

// Blocks arriving out of order reassemble by offset, and the commit hash
// verifies the bytes (F1).
func TestCloudPushShuffledBlocks(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	e.setCMState("cloud_split_blocks", true)
	e.setCMState("cloud_shuffle_blocks", true)
	content := "shuffle-me-0123456789"
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-shuffle"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" {
		t.Fatalf("push = %+v", pushed)
	}
	if err := os.Remove(filepath.Join(install, "saves", "a.sav")); err != nil {
		t.Fatal(err)
	}
	back, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-after-shuffle"})
	if err != nil {
		t.Fatal(err)
	}
	if back.State != "synced" {
		t.Fatalf("pull = %+v", back)
	}
	if got, _ := os.ReadFile(filepath.Join(install, "saves", "a.sav")); string(got) != content {
		t.Fatalf("downloaded = %q, want the reassembled bytes", got)
	}
}

// The change number moving between the listing and the batch begin uploads
// nothing: the batch completes with failure and the push answers "try
// again" (S3).
func TestCloudPushChangeMoved(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	e.setCMState("cloud_begin_bump", true)
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte("stale-list"), 0o644); err != nil {
		t.Fatal(err)
	}
	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-bump"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "error" || !strings.Contains(pushed.Message, "try again") {
		t.Fatalf("push = %+v, want the try-again result", pushed)
	}
	log := e.waitCMLog("upload complete 1 2")
	if strings.Contains(log, "upload begin-file") || strings.Contains(log, "upload http") {
		t.Fatalf("a moved change number must upload nothing: %s", log)
	}
	if _, statErr := os.Stat(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a moved change number must leave the baseline untouched")
	}
}

// A stored `local` choice replays on a later push even when the cloud no
// longer lists the file: the upload recreates it under its recorded name.
func TestCloudPushHonoursKeptLocalOnDeletedCloudFile(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	write := func(content string) {
		if err := os.WriteFile(filepath.Join(install, "saves", "c.sav"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("local-version")
	e.cloudKnobs("%GameInstall%saves/c.sav", "cloud-version", false, 1700000000)
	e.setCMState("cloud_fail_files", []any{"%GameInstall%saves/c.sav"})
	var st types.CloudStatus
	err := e.call("cloud.resolve", map[string]any{"id": "4321", "ctx": ctx, "jobId": "resolve-local-fail",
		"choices": map[string]string{"saves/c.sav": "local"}}, &st)
	if err == nil {
		t.Fatal("a resolve whose upload fails must fail")
	}
	// The cloud drops the file while the choice is kept.
	e.setCMState("cloud_changelist", map[string]any{"current_change_number": float64(8)})
	e.setCMState("cloud_files", map[string]any{})
	e.setCMState("cloud_fail_files", []any{})
	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-kept-deleted"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" || pushed.Counts.Same != 1 {
		t.Fatalf("push = %+v, want the kept choice uploaded", pushed)
	}
	if err := os.Remove(filepath.Join(install, "saves", "c.sav")); err != nil {
		t.Fatal(err)
	}
	back, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-after-kept"})
	if err != nil {
		t.Fatal(err)
	}
	if back.State != "synced" {
		t.Fatalf("pull = %+v", back)
	}
	if got, _ := os.ReadFile(filepath.Join(install, "saves", "c.sav")); string(got) != "local-version" {
		t.Fatalf("downloaded = %q, want the recreated cloud copy", got)
	}
}

// A 0-byte file whose begin-file answers no block requests commits
// directly.
func TestCloudPushEmptyFileNoBlocks(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	e.setCMState("cloud_no_blocks", true)
	if err := os.WriteFile(filepath.Join(install, "saves", "empty.sav"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-empty"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" || pushed.Counts.Same != 1 {
		t.Fatalf("push = %+v", pushed)
	}
	log := e.waitCMLog("upload complete 1 1")
	if strings.Contains(log, "upload http") {
		t.Fatalf("no blocks means no HTTP: %s", log)
	}
	if err := os.Remove(filepath.Join(install, "saves", "empty.sav")); err != nil {
		t.Fatal(err)
	}
	back, err := e.cloudCall(t, "cloud.pull", map[string]any{"id": "4321", "ctx": ctx, "jobId": "pull-after-empty"})
	if err != nil {
		t.Fatal(err)
	}
	if back.State != "synced" {
		t.Fatalf("pull = %+v", back)
	}
	if got, _ := os.ReadFile(filepath.Join(install, "saves", "empty.sav")); len(got) != 0 {
		t.Fatalf("downloaded = %q, want the empty file", got)
	}
}

// A block drop naming an unknown HTTP method is refused, never guessed.
func TestCloudPushBadMethodRefused(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	e.setCMState("cloud_bad_method", true)
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var st types.CloudStatus
	err := e.call("cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-bad-method"}, &st)
	if err == nil || !strings.Contains(err.Error(), "HTTP method 1") {
		t.Fatalf("err = %v, want the refused method", err)
	}
	e.waitCMLog("upload complete 1 2")
	if _, statErr := os.Stat(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a refused method must leave the baseline untouched")
	}
}

// An encrypted begin-file answer is refused and reported with
// transfer_succeeded=false before the batch completes with failure.
func TestCloudPushEncryptRefused(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, install := e.cloudCtx(t)
	e.pushKnobs(t)
	e.setCMState("cloud_encrypt_files", []any{"%GameInstall%saves/a.sav"})
	if err := os.WriteFile(filepath.Join(install, "saves", "a.sav"), []byte("secret?no"), 0o644); err != nil {
		t.Fatal(err)
	}
	var st types.CloudStatus
	err := e.call("cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-encrypt"}, &st)
	if err == nil || !strings.Contains(err.Error(), "encrypted") {
		t.Fatalf("err = %v, want the encrypt refusal", err)
	}
	log := e.waitCMLog("upload complete 1 2")
	if !strings.Contains(log, "upload commit %GameInstall%saves/a.sav ok=0 refused") {
		t.Fatalf("log misses the failed-file report: %s", log)
	}
	if _, statErr := os.Stat(filepath.Join(e.pluginData(), "cloud", "4321", "baseline.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a refused upload must leave the baseline untouched")
	}
}

// A Windows-rooted rule uploads under its own root through the Linux
// override, and the next status reads it back as same (SUPERHOT shape).
func TestCloudPushWinRootOverride(t *testing.T) {
	e := setup(t)
	e.login()
	ctx, _ := e.cloudCtx(t)
	home := ctx["home"].(string)
	e.setCMState("appinfo_4321", `"appinfo"
{
	"appid"		"4321"
	"common" { "name" "Standalone Game" "type" "Game" }
	"ufs"
	{
		"quota" "524288"
		"maxnumfiles" "5"
		"savefiles"
		{
			"0" { "root" "WinAppDataLocalLow" "path" "SUPERHOT_Team/SUPERHOT" "pattern" "*.hot" }
		}
		"rootoverrides"
		{
			"0" { "os" "Linux" "oscompare" "=" "root" "WinAppDataLocalLow" "useinstead" "LinuxHome"
				"pathtransforms" { "0" { "find" "SUPERHOT_Team/SUPERHOT" "replace" ".config/unity3d/SUPERHOT_Team/SUPERHOT" } } }
		}
	}
}`)
	e.setCMState("cloud_changelist", map[string]any{"current_change_number": float64(5)})
	e.setCMState("cloud_files", map[string]any{})
	local := filepath.Join(home, ".config", "unity3d", "SUPERHOT_Team", "SUPERHOT", "super.hot")
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}
	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-winroot"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" || pushed.Counts.Same != 1 {
		t.Fatalf("push = %+v", pushed)
	}
	log := e.waitCMLog("upload complete 1 1")
	if !strings.Contains(log, "upload begin-file 1 %WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/super.hot 1 blocks") {
		t.Fatalf("log misses the platform-independent name: %s", log)
	}
	fresh, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.State != "synced" || fresh.Counts.Same != 1 {
		t.Fatalf("status after push = %+v", fresh)
	}
}

// A new save in a folder the cloud spells with the Windows client's casing
// uploads under that spelling (a folder neighbour lends it), and the baseline
// is keyed by the name the batch sent: the next status is same, and a later
// edit is an upload, never a conflict with its own earlier save.
func TestCloudPushNewSaveBorrowsFolderCase(t *testing.T) {
	e := setup(t)
	e.login()
	home := t.TempDir()
	prefix := filepath.Join(home, "prefix")
	docs := filepath.Join(prefix, "drive_c", "users", "steamuser", "Documents", "My Games", "Fallout4", "Saves")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := map[string]any{"os": "windows", "home": home, "installDir": filepath.Join(home, "install"),
		"prefixDir": prefix, "winUser": "steamuser"}
	e.setCMState("appinfo_4321", `"appinfo" { "appid" "4321" "common" { "name" "Standalone Game" } "ufs" { "savefiles" {
		"0" { "root" "WinMyDocuments" "path" "My Games/Fallout4/Saves" "pattern" "*.fos" } } } }`)
	const listedName = "%WinMyDocuments%my games/fallout4/saves/save1.fos"
	e.setCMState("cloud_changelist", map[string]any{
		"current_change_number": float64(9),
		"path_prefixes":         []any{"%WinMyDocuments%my games/fallout4/saves/"},
		"machine_names":         []any{"test-machine"},
		"files": []any{map[string]any{"name": "save1.fos", "sha": cloudSHA("old"),
			"time": float64(100), "size": float64(3), "persist": float64(0),
			"prefix": float64(0), "machine": float64(0)}},
	})
	e.setCMState("cloud_files", map[string]any{listedName: map[string]any{
		"body": base64.StdEncoding.EncodeToString([]byte("old")), "time": float64(100)}})
	for name, content := range map[string]string{"save1.fos": "old", "x.fos": "fresh"} {
		if err := os.WriteFile(filepath.Join(docs, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	st, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if st.Counts.Same != 1 || st.Counts.Upload != 1 || st.State != "pending" {
		t.Fatalf("status = %+v, want one same and one upload", st)
	}
	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-borrowed"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" {
		t.Fatalf("push = %+v", pushed)
	}
	log := e.waitCMLog("upload complete 1 1")
	if !strings.Contains(log, "upload begin-file 1 %WinMyDocuments%my games/fallout4/saves/x.fos 1 blocks") {
		t.Fatalf("log misses the borrowed folder spelling: %s", log)
	}
	pushBaselineSHA(t, e, "my games/fallout4/saves/x.fos", "fresh")

	next, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if next.State != "synced" || next.Counts.Same != 2 || next.Counts.Upload != 0 {
		t.Fatalf("status after push = %+v, want both same", next)
	}

	if err := os.WriteFile(filepath.Join(docs, "x.fos"), []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	edited, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": ctx})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Counts.Upload != 1 || edited.Counts.Conflict != 0 {
		t.Fatalf("status after edit = %+v, want an upload, not a conflict", edited)
	}
}

// An upload offers the matched rule's platforms (issue #188): a Windows-tagged
// rule sends platforms_to_sync 1, so the cloud keeps the save out of a Linux
// client's sync; an untagged rule sends All.
func TestCloudPushSendsRulePlatforms(t *testing.T) {
	e := setup(t)
	e.login()
	home := t.TempDir()
	prefix := filepath.Join(home, "prefix")
	saves := filepath.Join(prefix, "drive_c", "users", "steamuser", "Documents", "Saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := map[string]any{"os": "windows", "home": home, "installDir": filepath.Join(home, "install"),
		"prefixDir": prefix, "winUser": "steamuser"}
	e.setCMState("appinfo_4321", `"appinfo" { "appid" "4321" "common" { "name" "Standalone Game" } "ufs" { "savefiles" {
		"0" { "root" "WinMyDocuments" "path" "Saves" "pattern" "*.sav" "platforms" { "Windows" "1" } } } } }`)
	e.setCMState("cloud_changelist", map[string]any{"current_change_number": float64(5)})
	e.setCMState("cloud_files", map[string]any{})
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("win-only"), 0o644); err != nil {
		t.Fatal(err)
	}

	pushed, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-platforms"})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.State != "synced" || pushed.Counts.Same != 1 {
		t.Fatalf("push = %+v", pushed)
	}
	log := e.waitCMLog("upload complete 1 1")
	if !strings.Contains(log, "platforms=1") {
		t.Fatalf("log misses the rule's platforms mask: %s", log)
	}

	// The listing reads the mask back: a Linux context counts the save as
	// not synced on this device instead of comparing it.
	linux := map[string]any{"os": "linux", "home": home, "installDir": ctx["installDir"]}
	other, err := e.cloudCall(t, "cloud.status", map[string]any{"id": "4321", "ctx": linux})
	if err != nil {
		t.Fatal(err)
	}
	if other.Counts.Unmapped != 1 || other.Counts.Same != 0 {
		t.Fatalf("linux status = %+v, want the Windows save unmapped", other)
	}

	// An untagged rule offers every platform, like the client's default.
	e.setCMState("appinfo_4321", `"appinfo" { "appid" "4321" "common" { "name" "Standalone Game" } "ufs" { "savefiles" {
		"0" { "root" "WinMyDocuments" "path" "Saves" "pattern" "*.sav" } } } }`)
	if err := os.WriteFile(filepath.Join(saves, "b.sav"), []byte("both"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := e.cloudCall(t, "cloud.push", map[string]any{"id": "4321", "ctx": ctx, "jobId": "push-platforms-all"})
	if err != nil {
		t.Fatal(err)
	}
	if again.State != "synced" {
		t.Fatalf("push = %+v", again)
	}
	log = e.waitCMLog("upload complete 2 1")
	if !strings.Contains(log, "platforms=4294967295") {
		t.Fatalf("log misses the all-platforms mask: %s", log)
	}
}
