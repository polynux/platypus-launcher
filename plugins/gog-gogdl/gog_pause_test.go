package gog_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/types"
)

func (b backend) Discard(ctx context.Context, j types.Job) error {
	return b.m.Call(ctx, j.Source, "job.discard", map[string]any{
		"jobId": j.ID, "id": j.StoreGameID, "kind": j.Kind, "installDir": j.InstallDir}, nil)
}

func firstGame(t *testing.T, store interface {
	Library(context.Context) ([]types.Game, error)
}) types.Game {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	return lib[0]
}

func waitFor(t *testing.T, q *jobs.Queue, jobID string, ok func(types.Job) bool, what string) types.Job {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, x := range q.List() {
			if x.ID == jobID {
				if ok(x) {
					return x
				}
				if x.State == jobs.StateFailed {
					t.Fatalf("job failed while waiting for %s: %+v", what, x)
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job never reached %s: %+v", what, q.List())
	return types.Job{}
}

func inState(state string) func(types.Job) bool {
	return func(j types.Job) bool { return j.State == state }
}

func started(j types.Job) bool { return j.State == jobs.StateRunning && j.Bytes > 0 }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// A slow gogdl, so there is time to pause or cancel the download.
func slowGogdl(t *testing.T) { t.Setenv("FAKE_GOGDL_STEP", "0.5") }

func TestGogPauseResumeFinishesTheInstall(t *testing.T) {
	slowGogdl(t)
	m, q, store, folder := setup(t)
	login(t, m)
	game := firstGame(t, store)
	j, err := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, j.ID, started, "first progress")
	if err := q.Pause(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, j.ID, inState(jobs.StatePaused), "paused")
	if err := q.Resume(j.ID); err != nil {
		t.Fatal(err)
	}
	done := waitFor(t, q, j.ID, inState(jobs.StateDone), "done")
	if done.ID != j.ID {
		t.Fatalf("resumed under another id: %+v", done)
	}
	if !exists(filepath.Join(folder, "SuperGame", "start.sh")) {
		t.Fatal("the resumed install didn't finish")
	}
	if !firstGame(t, store).IsInstalled {
		t.Fatal("not installed after resume")
	}
}

func TestGogDiscardRemovesThePartialFolderOnly(t *testing.T) {
	slowGogdl(t)
	m, q, store, folder := setup(t)
	login(t, m)
	game := firstGame(t, store)
	// Another game's folder, there before the download started.
	os.MkdirAll(filepath.Join(folder, "Other"), 0o755)
	os.WriteFile(filepath.Join(folder, "Other", "keep.txt"), []byte("x"), 0o644)

	j, _ := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	waitFor(t, q, j.ID, started, "first progress")
	if err := q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, j.ID, inState(jobs.StateCanceled), "canceled")
	if !exists(filepath.Join(folder, "SuperGame")) {
		t.Fatal("cancel must keep the partial folder")
	}
	// Right after the cancel: gogdl is still stopping.
	if err := q.Discard(context.Background(), j.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if exists(filepath.Join(folder, "SuperGame")) {
		t.Fatal("the partial game folder is still there")
	}
	if !exists(filepath.Join(folder, "Other", "keep.txt")) || !exists(folder) {
		t.Fatal("discard deleted something that wasn't the job's")
	}
	// A fresh install works afterwards.
	j2, _ := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	waitFor(t, q, j2.ID, inState(jobs.StateDone), "done")
}

// A job whose plugin process crashed mid-install (no stop recorded) still
// discards exactly its own folder.
func TestGogDiscardAfterARestartedPlugin(t *testing.T) {
	slowGogdl(t)
	m, q, store, folder := setup(t)
	login(t, m)
	game := firstGame(t, store)
	j, _ := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	waitFor(t, q, j.ID, started, "first progress")
	// Cancel in Hydra's queue only, then call discard from a fresh plugin
	// process: nothing in memory knows the job.
	if err := q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, j.ID, inState(jobs.StateCanceled), "canceled")
	time.Sleep(time.Second)
	if pid := m.Running(id); pid > 0 {
		syscall.Kill(pid, syscall.SIGKILL)
	}
	time.Sleep(200 * time.Millisecond)
	if err := q.Discard(context.Background(), j.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if exists(filepath.Join(folder, "SuperGame")) {
		t.Fatal("the partial folder survived a plugin restart")
	}
}

func installOnce(t *testing.T, q *jobs.Queue, game types.Game, folder string) {
	t.Helper()
	j, _ := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	waitFor(t, q, j.ID, inState(jobs.StateDone), "done")
}

// Even if a job's record names an installed game's folder, a discard keeps it:
// the folder is checked against what is installed, and an update never loses
// the game.
func TestGogDiscardKeepsInstalledGamesAndStaysInsideTheFolder(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)
	game := firstGame(t, store)
	installOnce(t, q, game, folder)
	outside := filepath.Join(filepath.Dir(folder), "victim")
	os.MkdirAll(outside, 0o755)
	os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("x"), 0o644)

	meta := `{"bad": {"app": "1234", "installDir": "` + folder + `", "pre": [], "dirs": ["SuperGame", "../victim", "..", ".", "/", "a/b", ""]}}`
	data := filepath.Join(filepath.Dir(filepath.Dir(folder)), "data", id)
	os.MkdirAll(data, 0o755)
	if err := os.WriteFile(filepath.Join(data, "jobs-meta.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"install", "update"} {
		if err := call(t, m, "job.discard", map[string]any{"jobId": "bad", "id": "1234", "kind": kind, "installDir": folder}, nil); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	if !exists(filepath.Join(folder, "SuperGame", "start.sh")) {
		t.Fatal("an installed game was deleted")
	}
	if !exists(filepath.Join(outside, "keep.txt")) {
		t.Fatal("discard escaped the install folder")
	}
}

func TestGogDiscardRejectsBadJobIDs(t *testing.T) {
	m, _, _, folder := setup(t)
	for _, bad := range []string{"../x", "", "a/b", ".."} {
		err := call(t, m, "job.discard", map[string]any{"jobId": bad, "id": "1234", "kind": "install", "installDir": folder}, nil)
		if err == nil {
			t.Errorf("job id %q accepted", bad)
		}
	}
}
