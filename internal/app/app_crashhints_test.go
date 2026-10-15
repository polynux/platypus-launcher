package app

import (
	"errors"
	"os/exec"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/session"
	"platypus/internal/types"
)

// A game whose last run was fine has no crash hints.
func TestGameCrashHintsIsNilWithoutAFailedLaunch(t *testing.T) {
	a := &App{sessions: session.NewTracker(func(types.SessionEvent) {})}
	if got := a.GameCrashHints(types.Game{ID: "fine", Title: "Fine"}); got != nil {
		t.Fatalf("GameCrashHints = %+v, want nil", got)
	}
}

// A launch that exits non-zero leaves its log, and the hints read it back.
func TestGameCrashHintsReadsTheFailedLaunch(t *testing.T) {
	events := make(chan types.SessionEvent, 16)
	tr := session.NewTracker(func(e types.SessionEvent) { events <- e })
	tr.SetLogDir(t.TempDir())
	a := &App{sessions: tr}

	cmd := exec.Command("sh", "-c", "echo 'err:module:import_dll Library bar.dll (which is needed by L\"x.exe\") not found'; exit 1")
	if _, err := tr.StartDirect("crashy", cmd, "sh", nil); err != nil {
		t.Fatalf("StartDirect: %v", err)
	}
	deadline := time.After(10 * time.Second)
wait:
	for {
		select {
		case e := <-events:
			if e.State == "failed" {
				break wait
			}
		case <-deadline:
			t.Fatal("timed out waiting for the failed launch")
		}
	}
	got := a.GameCrashHints(types.Game{ID: "crashy", Title: "Crashy"})
	if got == nil || len(got.Hints) != 1 {
		t.Fatalf("GameCrashHints = %+v, want one hint", got)
	}
	if got.Hints[0].Rule != "missingDll" || got.Hints[0].Param != "bar.dll" {
		t.Fatalf("hint = %+v, want the missing bar.dll", got.Hints[0])
	}
	if got.Log == "" {
		t.Fatal("hints name no log")
	}
}

// Only verbs in the pinned winetricks list are ever run.
func TestRunWinetricksVerbRefusesAnUnknownVerb(t *testing.T) {
	a := &App{sessions: session.NewTracker(func(types.SessionEvent) {})}
	err := a.RunWinetricksVerb(types.Game{ID: "g", Title: "G"}, "not-a-verb")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "crash.badVerb" {
		t.Fatalf("RunWinetricksVerb err = %v, want crash.badVerb", err)
	}
}
