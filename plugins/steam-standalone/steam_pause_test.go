package steam_test

import (
	"context"
	"crypto/sha1"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/jsonrpc"
	"platypus/internal/types"
)

// Discard is the optional half of the jobs.Backend the real app wires; the
// tests' backend sends the same job.discard the app does.
func (b backend) Discard(ctx context.Context, j types.Job) error {
	return b.m.Call(ctx, j.Source, "job.discard", map[string]any{
		"jobId": j.ID, "id": j.StoreGameID, "kind": j.Kind, "installDir": j.InstallDir}, nil)
}

// slowDownload makes the fake CDN serve one small chunk at a time, so a
// download has a window in which it can be paused or canceled.
func (e *env) slowDownload() {
	e.t.Helper()
	e.setDDState("extra_files", 10)
	e.setDDState("chunk_delay_ms", 150)
	e.refreshCDN()
}

func (e *env) waitBytes(jobID string) types.Job {
	e.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, x := range e.q.List() {
			if x.ID != jobID {
				continue
			}
			if x.State == jobs.StateDone || x.State == jobs.StateFailed {
				e.t.Fatalf("job ended before it could be interrupted: %+v", x)
			}
			if x.State == jobs.StateRunning && x.Bytes > 0 {
				return x
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.t.Fatalf("job made no progress: %+v", e.q.List())
	return types.Job{}
}

// waitStaged waits until the downloader has staged n chunks for the job.
func (e *env) waitStaged(jobID string, n int) {
	e.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		staged, _ := os.ReadDir(filepath.Join(e.pluginData(), "jobs", jobID+".chunks"))
		if len(staged) >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.t.Fatalf("fewer than %d chunks staged", n)
}

func (e *env) waitJobState(jobID, state string) types.Job {
	e.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, x := range e.q.List() {
			if x.ID == jobID && x.State == state {
				return x
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.t.Fatalf("job never reached %s: %+v", state, e.q.List())
	return types.Job{}
}

func (e *env) installGame(game *types.Game) types.Job {
	e.t.Helper()
	j, err := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if err != nil {
		e.t.Fatal(err)
	}
	return j
}

func (e *env) jobFiles(jobID string) []string {
	matches, _ := filepath.Glob(filepath.Join(e.pluginData(), "jobs", jobID+"*"))
	return matches
}

// Everything the manifest lists is on disk with the right SHA-1.
func (e *env) checkInstalledFiles(dir string) {
	e.t.Helper()
	for _, f := range e.fix.Manifest.Files {
		if f.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Name)))
		if err != nil {
			e.t.Fatalf("missing %s: %v", f.Name, err)
		}
		if sum := sha1.Sum(data); string(sum[:]) != string(f.FileHash) {
			e.t.Fatalf("%s has the wrong SHA-1", f.Name)
		}
	}
}

func (e *env) gameDirs() []string {
	entries, _ := os.ReadDir(e.folder)
	var out []string
	for _, en := range entries {
		out = append(out, en.Name())
	}
	return out
}

// Pause mid-install keeps the staged chunks; resuming the same job id (right
// away, while the downloader is still winding down) finishes the install with
// every file intact.
func TestSteamPauseResumeFinishesTheInstall(t *testing.T) {
	e := setup(t)
	e.login()
	e.slowDownload()
	game := e.find(e.library(), "4321")

	j := e.installGame(game)
	e.waitBytes(j.ID)
	if err := e.q.Pause(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	e.waitJobState(j.ID, jobs.StatePaused)
	// Resume at once: the old downloader is still stopping.
	if err := e.q.Resume(j.ID); err != nil {
		t.Fatal(err)
	}
	done := e.waitJob(j.ID)
	if done.State != jobs.StateDone || done.ID != j.ID {
		t.Fatalf("resumed install: %+v", done)
	}
	dirs := e.gameDirs()
	if len(dirs) != 1 {
		t.Fatalf("one game folder expected, got %v", dirs)
	}
	e.checkInstalledFiles(filepath.Join(e.folder, dirs[0]))
	if left := e.jobFiles(j.ID); len(left) != 0 {
		t.Fatalf("finished job left state behind: %v", left)
	}
}

// A paused install, left for later, still has everything on disk.
func TestSteamPauseKeepsStagingAndFolder(t *testing.T) {
	e := setup(t)
	e.login()
	e.slowDownload()
	game := e.find(e.library(), "4321")
	j := e.installGame(game)
	e.waitBytes(j.ID)
	e.waitStaged(j.ID, 5)
	if err := e.q.Pause(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	e.waitJobState(j.ID, jobs.StatePaused)
	time.Sleep(2 * time.Second) // the downloader checkpoints on its way out
	if len(e.jobFiles(j.ID)) == 0 {
		t.Fatal("pausing deleted the job's staging")
	}
	if len(e.gameDirs()) != 1 {
		t.Fatalf("pausing removed the partial folder: %v", e.gameDirs())
	}
	// Cleanup passes never touch a paused job.
	e.q.SetCleanupAfter(0)
	e.q.Cleanup(context.Background())
	time.Sleep(200 * time.Millisecond)
	if len(e.jobFiles(j.ID)) == 0 || len(e.gameDirs()) != 1 {
		t.Fatal("cleanup deleted a paused download")
	}

	// Resuming fetches only what is missing.
	staged, _ := os.ReadDir(filepath.Join(e.pluginData(), "jobs", j.ID+".chunks"))
	if len(staged) == 0 {
		t.Fatal("nothing was staged before the pause")
	}
	hitsBefore := atomic.LoadInt64(e.cdn.Hits)
	if err := e.q.Resume(j.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("resumed install: %+v", got)
	}
	// One request per missing chunk, plus the manifest (fetched for the size
	// probe and again by the download, from either fake host).
	missing := int64(len(e.fix.Chunks) - len(staged))
	if fetched := atomic.LoadInt64(e.cdn.Hits) - hitsBefore; fetched > missing+5 {
		t.Fatalf("resume fetched %d times with %d chunks missing (%d staged): it started over", fetched, missing, len(staged))
	}
	e.checkInstalledFiles(filepath.Join(e.folder, e.gameDirs()[0]))
}

// Cancel keeps the data; discard deletes the staging and the partial folder
// and the plugin can install the game afresh afterwards.
func TestSteamDiscardRemovesStagingAndPartialFolder(t *testing.T) {
	e := setup(t)
	e.login()
	e.slowDownload()
	game := e.find(e.library(), "4321")
	j := e.installGame(game)
	e.waitBytes(j.ID)
	if err := e.q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	e.waitJobState(j.ID, jobs.StateCanceled)
	// Discard right away: it has to wait for the downloader to stop.
	if err := e.q.Discard(context.Background(), j.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if left := e.jobFiles(j.ID); len(left) != 0 {
		t.Fatalf("staging left behind: %v", left)
	}
	if dirs := e.gameDirs(); len(dirs) != 0 {
		t.Fatalf("partial install folder left behind: %v", dirs)
	}
	if st := e.q.List()[0]; st.CleanedAt == 0 || st.Leftover != "" {
		t.Fatalf("job should be marked cleaned: %+v", st)
	}
	// And a fresh install works.
	j2 := e.installGame(game)
	if got := e.waitJob(j2.ID); got.State != jobs.StateDone {
		t.Fatalf("install after discard: %+v", got)
	}
	e.checkInstalledFiles(filepath.Join(e.folder, e.gameDirs()[0]))
}

// A canceled update never takes the installed game with it, and neither does
// a discard that names an installed game's folder.
func TestSteamDiscardKeepsTheInstalledGame(t *testing.T) {
	e := setup(t)
	e.login()
	game := e.find(e.library(), "4321")
	if got := e.waitJob(e.installGame(game).ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	dir := filepath.Join(e.folder, e.gameDirs()[0])
	e.checkInstalledFiles(dir)

	// Staging of an update that was canceled halfway (written by hand: the
	// fake update finishes too fast to catch).
	for _, kind := range []string{jobs.KindUpdate, jobs.KindInstall} {
		jobID := "upd-" + kind
		os.MkdirAll(filepath.Join(e.pluginData(), "jobs", jobID+".chunks"), 0o755)
		os.WriteFile(filepath.Join(e.pluginData(), "jobs", jobID+".chunks", "aa"), []byte("x"), 0o644)
		os.WriteFile(filepath.Join(e.pluginData(), "jobs", jobID+".json"), []byte(`{}`), 0o644)
		err := e.call("job.discard", map[string]any{"jobId": jobID, "id": "4321", "kind": kind, "installDir": e.folder}, nil)
		if err != nil {
			t.Fatalf("discard %s: %v", kind, err)
		}
		if left := e.jobFiles(jobID); len(left) != 0 {
			t.Fatalf("%s: staging left behind: %v", kind, left)
		}
		e.checkInstalledFiles(dir)
	}
	lib := e.library()
	if g := e.find(lib, "4321"); !g.IsInstalled {
		t.Fatal("the game is no longer installed")
	}
}

// job.discard only ever touches the plugin's own job files and the job's own
// subfolder of the install folder.
func TestSteamDiscardRefusesPathTricks(t *testing.T) {
	e := setup(t)
	e.login()
	victim := filepath.Join(e.root, "victim")
	os.MkdirAll(victim, 0o755)
	os.WriteFile(filepath.Join(victim, "keep.txt"), []byte("x"), 0o644)
	// A job-state file outside jobs/ that a "../" job id would reach.
	outside := filepath.Join(e.pluginData(), "installed.json")
	os.MkdirAll(e.pluginData(), 0o755)
	os.WriteFile(outside, []byte(`{}`), 0o644)

	for _, bad := range []string{"../installed", "..", "a/b", "", "x y"} {
		err := e.call("job.discard", map[string]any{"jobId": bad, "id": "4321", "kind": "install", "installDir": e.folder}, nil)
		var rpcErr *jsonrpc.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != -32602 {
			t.Errorf("job id %q: got %v, want invalid params", bad, err)
		}
	}
	for _, dir := range []string{"", "relative/dir", "/", e.root, victim} {
		_ = e.call("job.discard", map[string]any{"jobId": "j-safe", "id": "4321", "kind": "install", "installDir": dir}, nil)
	}
	for _, p := range []string{outside, filepath.Join(victim, "keep.txt")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was deleted: %v", p, err)
		}
	}
}

// A discard for a job this plugin never heard of is not an error: Platypus
// forgets jobs whose data is already gone.
func TestSteamDiscardUnknownJobIsFine(t *testing.T) {
	e := setup(t)
	e.login()
	if err := e.call("job.discard", map[string]any{"jobId": "never-existed", "id": "4321", "kind": "install", "installDir": e.folder}, nil); err != nil {
		t.Fatal(err)
	}
}
