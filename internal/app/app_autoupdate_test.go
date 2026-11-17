package app

import (
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/types"
)

// Automatic updates (issue #209): a library refresh queues an automatic
// update for an installed game whose store reports one, and the update
// window holds it like any other auto job.

// autoUpdateEnv installs echo's tiny game at b200, so the store reports an
// update to the latest build (b300).
func autoUpdateEnv(t *testing.T) *versionsEnv {
	t.Helper()
	e := versionsApp(t, allVersions, echoCaps)
	e.install(t, &types.VersionTarget{Build: "b200"})
	return e
}

// holdUpdates closes the window around now, so an automatic update is queued
// without starting.
func holdUpdates(t *testing.T, e *versionsEnv) {
	t.Helper()
	start := time.Now().Add(5 * time.Minute)
	end := start.Add(time.Hour)
	if _, err := e.a.SetUpdateWindow(true, start.Format("15:04"), end.Format("15:04")); err != nil {
		t.Fatal(err)
	}
}

// checkLibrary refreshes the library and waits for the automatic-update
// check the refresh schedules, so a test sees its queues.
func checkLibrary(t *testing.T, e *versionsEnv) {
	t.Helper()
	if _, err := e.a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	e.a.autoChecks.Wait()
}

// autoJobs are the queue's update jobs, the kind automatic updates use.
func autoJobs(e *versionsEnv) []types.Job {
	var out []types.Job
	for _, j := range e.a.jobs.List() {
		if j.Kind == jobs.KindUpdate {
			out = append(out, j)
		}
	}
	return out
}

// activeAutoJobs are automatic updates that are queued or running.
func activeAutoJobs(e *versionsEnv) []types.Job {
	var out []types.Job
	for _, j := range autoJobs(e) {
		if j.Auto && (j.State == jobs.StateQueued || j.State == jobs.StateRunning) {
			out = append(out, j)
		}
	}
	return out
}

func TestAutomaticUpdateQueuesAndWaitsForTheWindow(t *testing.T) {
	t.Parallel()
	e := autoUpdateEnv(t)
	holdUpdates(t, e)

	checkLibrary(t, e)
	up := autoJobs(e)
	if len(up) != 1 {
		t.Fatalf("automatic updates queued = %+v", up)
	}
	j := up[0]
	if !j.Auto || j.State != jobs.StateQueued || j.GameID != e.game.ID {
		t.Fatalf("auto job = %+v", j)
	}

	// A second refresh doesn't queue another job for the same game.
	checkLibrary(t, e)
	if got := activeAutoJobs(e); len(got) != 1 {
		t.Fatalf("a second refresh queued again: %+v", got)
	}

	// Opening the window lets the queued job start, and it installs the update.
	if _, err := e.a.SetUpdateWindow(false, "03:00", "06:00"); err != nil {
		t.Fatal(err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if h := e.a.ledger.Entries(e.game.ID); h[len(h)-1].Build != "b300" {
		t.Fatalf("the automatic update installed %+v", h[len(h)-1])
	}
	// A finished auto update leaves no attempt behind, so a later update is
	// queued again.
	if _, ok := e.a.autoTry(e.game.ID); ok {
		t.Fatal("a finished automatic update kept its attempt")
	}
}

func TestAutomaticUpdateRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, e *versionsEnv)
	}{
		{"pinned game", func(t *testing.T, e *versionsEnv) {
			if err := e.a.PinGameVersion(e.game, types.VersionPin{Kind: types.PinBuild, ID: "b200"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"excluded game", func(t *testing.T, e *versionsEnv) {
			if err := e.a.SetGameAutoUpdate(e.game.ID, false); err != nil {
				t.Fatal(err)
			}
		}},
		{"running game", func(t *testing.T, e *versionsEnv) {
			e.a.sessionRunning = func(id string) bool { return id == e.game.ID }
		}},
		{"switch off", func(t *testing.T, e *versionsEnv) {
			if _, err := e.a.SetAutoUpdate(false); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := autoUpdateEnv(t)
			c.setup(t, e)
			checkLibrary(t, e)
			if got := activeAutoJobs(e); len(got) != 0 {
				t.Fatalf("%s: automatic updates queued = %+v", c.name, got)
			}
		})
	}
}

func TestAutomaticUpdateNeedsAnUpdate(t *testing.T) {
	t.Parallel()
	e := versionsApp(t, allVersions, echoCaps)
	e.install(t, nil) // b300 is the newest build
	checkLibrary(t, e)
	if got := activeAutoJobs(e); len(got) != 0 {
		t.Fatalf("automatic updates queued without one = %+v", got)
	}
}

func TestAutomaticUpdateSettingDefaultsOn(t *testing.T) {
	t.Parallel()
	e := versionsApp(t, allVersions, echoCaps)
	if !e.a.settings.AutoUpdate() || !e.a.settings.GameAutoUpdate(e.game.ID) {
		t.Fatal("automatic updates should be on, for every game, until switched off")
	}
	if s := e.a.GetLauncherSettings(); !s.AutoUpdate {
		t.Fatal("launcher settings hide the automatic-update switch")
	}
}

func TestCanceledAutomaticUpdateSticks(t *testing.T) {
	t.Parallel()
	e := autoUpdateEnv(t)
	holdUpdates(t, e)
	checkLibrary(t, e)
	up := activeAutoJobs(e)
	if len(up) != 1 {
		t.Fatalf("automatic updates queued = %+v", up)
	}
	if err := e.a.CancelJob(up[0].ID); err != nil {
		t.Fatal(err)
	}
	// The canceled attempt is remembered: the same offered update isn't
	// queued again.
	checkLibrary(t, e)
	if got := activeAutoJobs(e); len(got) != 0 {
		t.Fatalf("a canceled update was queued again: %+v", got)
	}
	if try, ok := e.a.autoTry(e.game.ID); !ok || try.outcome != autoCanceled {
		t.Fatalf("attempt after cancel = %+v, %v", try, ok)
	}
}

func TestAutomaticUpdateOffCancelsQueued(t *testing.T) {
	t.Parallel()
	e := autoUpdateEnv(t)
	holdUpdates(t, e)
	checkLibrary(t, e)
	up := activeAutoJobs(e)
	if len(up) != 1 {
		t.Fatalf("automatic updates queued = %+v", up)
	}
	// Excluding the game cancels its waiting update.
	if err := e.a.SetGameAutoUpdate(e.game.ID, false); err != nil {
		t.Fatal(err)
	}
	for _, j := range autoJobs(e) {
		if j.ID == up[0].ID && j.State != jobs.StateCanceled {
			t.Fatalf("job state after exclusion = %s", j.State)
		}
	}
	if got := activeAutoJobs(e); len(got) != 0 {
		t.Fatalf("exclusion left an active auto update: %+v", got)
	}

	// The global switch does the same for every game.
	e = autoUpdateEnv(t)
	holdUpdates(t, e)
	checkLibrary(t, e)
	if len(activeAutoJobs(e)) != 1 {
		t.Fatal("no automatic update to cancel")
	}
	if _, err := e.a.SetAutoUpdate(false); err != nil {
		t.Fatal(err)
	}
	if got := activeAutoJobs(e); len(got) != 0 {
		t.Fatalf("the switch-off left an active auto update: %+v", got)
	}
}

func TestFailedAutomaticUpdateWaitsBeforeRetry(t *testing.T) {
	t.Parallel()
	e := autoUpdateEnv(t)
	holdUpdates(t, e)
	// The last attempt failed just now: no retry inside the cooldown.
	e.a.autoEnded(types.Job{Kind: jobs.KindUpdate, Auto: true, GameID: e.game.ID, State: jobs.StateFailed})
	checkLibrary(t, e)
	if got := activeAutoJobs(e); len(got) != 0 {
		t.Fatalf("a failed update retried inside its cooldown: %+v", got)
	}
	// Once the cooldown has passed, the next check tries again.
	e.a.autoMu.Lock()
	try := e.a.autoTries[e.game.ID]
	try.at = time.Now().Add(-autoRetryAfter - time.Minute)
	e.a.autoTries[e.game.ID] = try
	e.a.autoMu.Unlock()
	checkLibrary(t, e)
	if got := activeAutoJobs(e); len(got) != 1 {
		t.Fatalf("no retry after the cooldown: %+v", got)
	}
}

func TestManualUpdateReplacesWaitingAuto(t *testing.T) {
	t.Parallel()
	e := autoUpdateEnv(t)
	holdUpdates(t, e)
	checkLibrary(t, e)
	auto := activeAutoJobs(e)
	if len(auto) != 1 {
		t.Fatalf("automatic updates queued = %+v", auto)
	}
	j, err := e.a.UpdateGame(e.game)
	if err != nil {
		t.Fatal(err)
	}
	// The waiting auto update is canceled, and the manual one runs even
	// while the window is closed (the window only holds auto jobs).
	for _, old := range autoJobs(e) {
		if old.ID == auto[0].ID && old.State != jobs.StateCanceled {
			t.Fatalf("auto job state = %s, want canceled", old.State)
		}
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if _, ok := e.a.autoTry(e.game.ID); ok {
		t.Fatal("a manual update left an automatic attempt behind")
	}
}
