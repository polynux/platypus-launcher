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

func TestAutomaticUpdateQueuesAndWaitsForTheWindow(t *testing.T) {
	t.Parallel()
	e := autoUpdateEnv(t)
	holdUpdates(t, e)

	if _, err := e.a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	up := autoJobs(e)
	if len(up) != 1 {
		t.Fatalf("automatic updates queued = %+v", up)
	}
	j := up[0]
	if !j.Auto || j.State != jobs.StateQueued || j.GameID != e.game.ID {
		t.Fatalf("auto job = %+v", j)
	}

	// A second refresh doesn't queue another job for the same game.
	if _, err := e.a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	if got := autoJobs(e); len(got) != 1 {
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
			if _, err := e.a.GetLibrary(); err != nil {
				t.Fatal(err)
			}
			if got := autoJobs(e); len(got) != 0 {
				t.Fatalf("%s: automatic updates queued = %+v", c.name, got)
			}
		})
	}
}

func TestAutomaticUpdateNeedsAnUpdate(t *testing.T) {
	t.Parallel()
	e := versionsApp(t, allVersions, echoCaps)
	e.install(t, nil) // b300 is the newest build
	if _, err := e.a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	if got := autoJobs(e); len(got) != 0 {
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
