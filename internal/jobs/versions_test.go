package jobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

type installedCall struct {
	job types.Job
	in  types.Installed
}

func recordInstalled(q *Queue) func() []installedCall {
	var mu sync.Mutex
	var got []installedCall
	q.SetOnInstalled(func(j types.Job, in types.Installed) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, installedCall{j, in})
	})
	return func() []installedCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]installedCall{}, got...)
	}
}

func TestTargetIsKeptWithTheJobAndSurvivesARestart(t *testing.T) {
	b := &fakeBackend{}
	q, path := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	target := &types.VersionTarget{Build: "b200", Manual: map[string]string{"depot-481": "7845"}}
	j, err := q.AddTargeted(KindInstall, "echo", "g", "tiny", "Tiny", dir, target)
	if err != nil {
		t.Fatal(err)
	}
	target.Build = "mutated after the call" // the queue keeps its own copy
	if j.Target == nil || j.Target.Build != "b200" {
		t.Fatalf("returned job lost its target: %+v", j.Target)
	}
	waitState(t, q, j.ID, StateRunning)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Jobs) != 1 || f.Jobs[0].Target == nil || f.Jobs[0].Target.Build != "b200" || f.Jobs[0].Target.Manual["depot-481"] != "7845" {
		t.Fatalf("saved queue lost the target: %s", raw)
	}

	// A new run of Hydra resumes the job with the same target.
	again, err := Open(path, &fakeBackend{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	list := again.List()
	if len(list) != 1 || list[0].Target == nil || list[0].Target.Build != "b200" {
		t.Fatalf("reopened queue: %+v", list)
	}
}

func TestAnUninstallTakesNoTarget(t *testing.T) {
	q, _ := newQueue(t, &fakeBackend{})
	if _, err := q.AddTargeted(KindUninstall, "echo", "g", "tiny", "Tiny", "", &types.VersionTarget{Build: "b1"}); err == nil {
		t.Fatal("an uninstall with a version target was queued")
	}
}

func TestInstalledReachesTheHookOnceWithTheFinishedJob(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	calls := recordInstalled(q)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	j, _ := q.AddTargeted(KindUpdate, "echo", "p.echo.tiny", "tiny", "Tiny", dir, &types.VersionTarget{Channel: "beta"})
	waitState(t, q, j.ID, StateRunning)
	b.waitStarts(1, 3*time.Second)

	in := &types.Installed{Build: "b310", Channel: "beta", VersionName: "3.1.0 beta", ArchiveRef: json.RawMessage(`{"m":1}`)}
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID, Installed: in})
	waitState(t, q, j.ID, StateDone)
	// A repeated job.done is ignored: the job is no longer running.
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID, Installed: in})

	got := calls()
	if len(got) != 1 {
		t.Fatalf("hook called %d times, want 1: %+v", len(got), got)
	}
	if got[0].job.ID != j.ID || got[0].job.GameID != "p.echo.tiny" || got[0].job.Source != "echo" || got[0].job.Target.Channel != "beta" {
		t.Errorf("hook job = %+v", got[0].job)
	}
	if got[0].in.Build != "b310" || got[0].in.Channel != "beta" || string(got[0].in.ArchiveRef) != `{"m":1}` {
		t.Errorf("hook installed = %+v", got[0].in)
	}
}

func TestNoHookWithoutInstalledOrFromAnotherPluginOrOnFailure(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	calls := recordInstalled(q)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	in := &types.Installed{Build: "b1"}

	plain, _ := q.Add(KindInstall, "echo", "g1", "a", "A", dir)
	waitState(t, q, plain.ID, StateRunning)
	b.waitStarts(1, 3*time.Second)
	q.Notify("evil", "job.done", plugins.JobUpdate{JobID: plain.ID, Installed: in}) // not its job
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: plain.ID})                // no `installed`
	waitState(t, q, plain.ID, StateDone)

	failing, _ := q.Add(KindInstall, "echo", "g2", "b", "B", dir)
	waitState(t, q, failing.ID, StateRunning)
	b.waitStarts(2, 3*time.Second)
	q.Notify("echo", "job.failed", plugins.JobUpdate{JobID: failing.ID, Error: "boom", Installed: in})
	waitState(t, q, failing.ID, StateFailed)

	if got := calls(); len(got) != 0 {
		t.Fatalf("hook called for %+v", got)
	}
}

// Whoever sees a job Done must also see what it left behind: the ledger entry
// is written before the job reads as done, not after.
func TestTheHookRunsBeforeTheJobReadsDone(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	hookRan, readsDone := false, false
	q.SetOnInstalled(func(job types.Job, _ types.Installed) {
		hookRan = true
		for _, j := range q.List() {
			if j.ID == job.ID && j.State == StateDone {
				readsDone = true
			}
		}
	})
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	j, _ := q.AddTargeted(KindInstall, "echo", "g", "tiny", "Tiny", dir, nil)
	waitState(t, q, j.ID, StateRunning)
	b.waitStarts(1, 3*time.Second)

	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID, Installed: &types.Installed{Build: "b1"}})
	if !hookRan || readsDone {
		t.Fatalf("hook ran = %v, job read as done first = %v", hookRan, readsDone)
	}
	waitState(t, q, j.ID, StateDone)
}

// A job's saved record can never hold a password: the target has no field for
// one and the unlock goes to the plugin alone. This pins the saved shape.
func TestSavedJobCarriesNoSecret(t *testing.T) {
	q, path := newQueue(t, &fakeBackend{})
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	j, _ := q.AddTargeted(KindInstall, "echo", "g", "tiny", "Tiny", dir, &types.VersionTarget{Channel: "beta"})
	waitState(t, q, j.ID, StateRunning)
	raw, _ := os.ReadFile(path)
	if strings.Contains(strings.ToLower(string(raw)), "password") {
		t.Fatalf("saved queue mentions a password: %s", raw)
	}
}
