package app

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/jsonrpc"
	"platypus/internal/session"
	"platypus/internal/types"
)

// A runner failure keeps the code the core translates (#74): a missing runner
// and an unknown winetricks verb reach the UI as their own codes, with params.
func TestRunnerFailuresKeepTheirCodes(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.sessions = session.NewTracker(func(types.SessionEvent) {})
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg, err := a.settings.AddCustomGame("Win", exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSettings(cg.ID, map[string]any{"runner_id": "no-such-runner"}); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]

	_, err = a.prepareLaunch(g, true)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "game.runnerMissing" || ae.Params["id"] != "no-such-runner" {
		t.Fatalf("prepareLaunch = %v, want game.runnerMissing with id", err)
	}

	err = a.RunWinetricksVerb(g, "not-a-verb")
	if !errors.As(err, &ae) || ae.Code != "crash.badVerb" || ae.Params["verb"] != "not-a-verb" {
		t.Fatalf("RunWinetricksVerb = %v, want crash.badVerb with the verb", err)
	}
}

// A failure without a known code keeps the generic code of the call, and the
// plugin's message.
func TestRunnerFailureFallsBackToTheGenericCode(t *testing.T) {
	err := runnerFailure(errors.New("boom"), "runner.wrapFailed", map[string]string{"title": "G"})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "runner.wrapFailed" || ae.Params["title"] != "G" {
		t.Fatalf("plain error = %v, want runner.wrapFailed", err)
	}

	// A code the core doesn't know is not shown as is.
	rpc := &jsonrpc.Error{Code: -32003, Message: "x", Data: json.RawMessage(`{"code":"evil.code","params":{}}`)}
	err = runnerFailure(rpc, "crash.winetricksFailed", map[string]string{"verb": "vcrun2022"})
	if !errors.As(err, &ae) || ae.Code != "crash.winetricksFailed" || ae.Params["verb"] != "vcrun2022" {
		t.Fatalf("unknown code = %v, want the generic crash.winetricksFailed", err)
	}
}
