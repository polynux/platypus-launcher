package epic_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/types"
)

func (b backend) Discard(ctx context.Context, j types.Job) error {
	return b.m.Call(ctx, j.Source, "job.discard", map[string]any{
		"jobId": j.ID, "id": j.StoreGameID, "kind": j.Kind, "installDir": j.InstallDir}, nil)
}

func (e *env) login(t *testing.T) {
	t.Helper()
	var st types.AuthStatus
	good := map[string]any{"fields": map[string]string{"code": "abcdef0123456789abcdef0123456789"}}
	if err := e.call(t, "auth.complete", good, &st); err != nil || !st.LoggedIn {
		t.Fatalf("login: %+v, %v", st, err)
	}
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

// A slow legendary and a private config folder (where its resume file goes).
func slowLegendary(t *testing.T) string {
	t.Helper()
	t.Setenv("FAKE_LEGENDARY_STEP", "0.5")
	cfg := t.TempDir()
	t.Setenv("LEGENDARY_CONFIG_PATH", cfg)
	return cfg
}

func TestEpicPauseResumeFinishesTheInstall(t *testing.T) {
	slowLegendary(t)
	e := setup(t)
	e.login(t)
	game := e.library(t)["Goose"]
	j, err := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, e.q, j.ID, started, "first progress")
	if err := e.q.Pause(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e.q, j.ID, inState(jobs.StatePaused), "paused")
	if err := e.q.Resume(j.ID); err != nil {
		t.Fatal(err)
	}
	if done := waitFor(t, e.q, j.ID, inState(jobs.StateDone), "done"); done.ID != j.ID {
		t.Fatalf("resumed under another id: %+v", done)
	}
	if !e.library(t)["Goose"].IsInstalled {
		t.Fatal("not installed after resume")
	}
}

func TestEpicDiscardRemovesPartialFolderAndResumeFile(t *testing.T) {
	cfg := slowLegendary(t)
	e := setup(t)
	e.login(t)
	game := e.library(t)["Goose"]
	os.MkdirAll(filepath.Join(e.folder, "Other"), 0o755)
	os.WriteFile(filepath.Join(e.folder, "Other", "keep.txt"), []byte("x"), 0o644)
	resume := filepath.Join(cfg, "tmp", "Goose.resume")

	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	waitFor(t, e.q, j.ID, started, "first progress")
	if err := e.q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e.q, j.ID, inState(jobs.StateCanceled), "canceled")
	partial := filepath.Join(e.folder, "UntitledGooseGame")
	if !exists(filepath.Join(partial, ".cache")) || !exists(resume) {
		t.Fatalf("cancel must keep legendary's partial data (folder %v, resume %v)", exists(partial), exists(resume))
	}
	if err := e.q.Discard(context.Background(), j.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if exists(partial) || exists(resume) {
		t.Fatalf("leftovers remain (folder %v, resume %v)", exists(partial), exists(resume))
	}
	if !exists(filepath.Join(e.folder, "Other", "keep.txt")) || !exists(e.folder) {
		t.Fatal("discard deleted something that wasn't the job's")
	}
	// The other, preinstalled game outside Hydra's folder is untouched.
	if !e.library(t)["Pre"].IsInstalled {
		t.Fatal("an installed game was affected")
	}
	// Installing afterwards works.
	j2, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	waitFor(t, e.q, j2.ID, inState(jobs.StateDone), "done")
}

// Canceling an update and discarding it leaves the installed game alone.
func TestEpicDiscardOfAnUpdateKeepsTheGame(t *testing.T) {
	cfg := slowLegendary(t)
	e := setup(t)
	e.login(t)
	game := e.library(t)["Goose"]
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	waitFor(t, e.q, j.ID, inState(jobs.StateDone), "done")
	gameDir := filepath.Join(e.folder, "UntitledGooseGame")
	os.WriteFile(filepath.Join(cfg, "dummy"), nil, 0o644)

	// Hand-written record of a canceled update job, as the plugin would have
	// left it: no folders are ever attributed to an update.
	data := filepath.Join(filepath.Dir(e.games), "data", id)
	os.MkdirAll(data, 0o755)
	meta := `{"upd": {"app": "Goose", "installDir": "` + e.folder + `", "pre": [], "dirs": ["UntitledGooseGame", "../victim", "/"]}}`
	os.WriteFile(filepath.Join(data, "jobs-meta.json"), []byte(meta), 0o644)
	for _, kind := range []string{"update", "install"} {
		if err := e.call(t, "job.discard", map[string]any{"jobId": "upd", "id": "Goose", "kind": kind, "installDir": e.folder}, nil); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if !exists(filepath.Join(gameDir, "Binaries", "Game.exe")) {
			t.Fatalf("a %s discard deleted the installed game", kind)
		}
	}
}

func TestEpicDiscardRejectsBadJobIDs(t *testing.T) {
	e := setup(t)
	for _, bad := range []string{"../x", "", "a/b", ".."} {
		if err := e.call(t, "job.discard", map[string]any{"jobId": bad, "id": "Goose", "kind": "install", "installDir": e.folder}, nil); err == nil {
			t.Errorf("job id %q accepted", bad)
		}
	}
}
