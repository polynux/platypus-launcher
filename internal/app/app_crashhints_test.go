package app

import (
	"errors"
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/session"
	"platypus/internal/types"
)

func TestGameCrashHintsIsNilWithoutARunnerPlugin(t *testing.T) {
	a := &App{sessions: session.NewTracker(func(types.SessionEvent) {})}
	if got := a.GameCrashHints(types.Game{ID: "fine", Title: "Fine"}); got != nil {
		t.Fatalf("GameCrashHints = %+v, want nil without a runner plugin", got)
	}
}

func TestRunWinetricksVerbRequiresARunnerPlugin(t *testing.T) {
	a := &App{sessions: session.NewTracker(func(types.SessionEvent) {})}
	err := a.RunWinetricksVerb(types.Game{ID: "g", Title: "G"}, "d3dx9")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "runner.unavailable" {
		t.Fatalf("RunWinetricksVerb err = %v, want runner.unavailable", err)
	}
}
