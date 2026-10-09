package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
)

// Terms (protocol 1.11) around the echo test plugin: the game "licensed" lists
// an end user licence and a privacy notice. The echo manifest offers terms;
// the protocol list decides whether they are on.

const termsProtocols = `["1.0","1.1","1.3","1.11"]`
const termsCaps = `["library","install","auth","discard","versions","terms"]`

func licensedGame() types.Game {
	return types.Game{
		ID: stores.GameID("echo", "licensed"), Source: "echo", Store: "echo", StoreAppID: "licensed",
		Title: "Echo Licensed", Installable: true,
	}
}

// config sets echo's knobs with a `config` request.
func (e *versionsEnv) config(t *testing.T, knobs map[string]any) {
	t.Helper()
	if err := e.m.Call(context.Background(), "echo", "config", knobs, nil); err != nil {
		t.Fatal(err)
	}
}

// termRefs names every term in the list at the version it has.
func termRefs(terms []types.GameTerm) []types.TermRef {
	var refs []types.TermRef
	for _, t := range terms {
		refs = append(refs, types.TermRef{ID: t.ID, Version: t.Version})
	}
	return refs
}

func TestInstallWaitsForTheUsersAcceptance(t *testing.T) {
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	terms, err := e.a.GetGameTerms(game)
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 2 || terms[0].ID != "eula" || terms[0].URL == "" || terms[0].AcceptedAt != 0 {
		t.Fatalf("terms = %+v, want the licence and the privacy notice, none accepted", terms)
	}
	if _, err := e.a.InstallGame(game); err == nil || !strings.Contains(err.Error(), "accept the terms of Echo Licensed") {
		t.Fatalf("install before acceptance: %v, want a refusal that names the terms", err)
	}
	if n := e.calls(t)["game.install"]; n != 0 {
		t.Fatalf("game.install reached echo %d times before the terms were accepted", n)
	}

	if err := e.a.AcceptGameTerms(game, termRefs(terms)); err != nil {
		t.Fatal(err)
	}
	job, err := e.a.InstallGame(game)
	if err != nil {
		t.Fatalf("install after acceptance: %v", err)
	}
	e.waitJob(t, job.ID, jobs.StateDone)

	if n := e.calls(t)["game.acceptTerms"]; n != 1 {
		t.Fatalf("game.acceptTerms reached echo %d times, want 1", n)
	}
	var rec struct {
		AcceptedTerms map[string][]string `json:"acceptedTerms"`
	}
	if err := e.m.Call(context.Background(), "echo", "recorded", nil, &rec); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(rec.AcceptedTerms["licensed"], ","); got != "eula@2026.1,privacy@3" {
		t.Fatalf("echo recorded %q, want both terms at their versions", got)
	}
	accepted := e.a.settings.AcceptedTerms(game.ID)
	if len(accepted) != 2 || accepted[0].At == 0 {
		t.Fatalf("Hydra recorded %+v, want both versions with a date", accepted)
	}
	if pending, _ := e.a.GetGameTerms(game); len(types.Pending(pending)) != 0 {
		t.Fatalf("still pending after acceptance: %+v", pending)
	}
}

func TestAVersionBumpAsksAgainAndAStaleDialogCannotAccept(t *testing.T) {
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	terms, _ := e.a.GetGameTerms(game)
	if err := e.a.AcceptGameTerms(game, termRefs(terms)); err != nil {
		t.Fatal(err)
	}

	e.config(t, map[string]any{"termsBump": true})
	game.IsInstalled = true
	now, err := e.a.GetGameTerms(game)
	if err != nil {
		t.Fatal(err)
	}
	pending := types.Pending(now)
	if len(pending) != 1 || pending[0].ID != "eula" || pending[0].Version != "2026.2" {
		t.Fatalf("after the bump pending = %+v, want the licence at 2026.2 only", pending)
	}
	if err := e.a.AcceptGameTerms(game, []types.TermRef{{ID: "eula", Version: "2026.1"}}); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("accepting the old version: %v, want a refusal (the terms changed)", err)
	}
	if _, err := e.a.UpdateGame(game); err == nil || !strings.Contains(err.Error(), "Echo end user licence") {
		t.Fatalf("update with a new licence: %v, want the update refused", err)
	}
	if err := e.a.AcceptGameTerms(game, []types.TermRef{{ID: "eula", Version: "2026.2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.UpdateGame(game); err != nil {
		t.Fatalf("update after accepting the new version: %v", err)
	}
}

func TestNothingIsAcceptedOnTheUsersBehalf(t *testing.T) {
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	if _, err := e.a.InstallGame(game); err == nil {
		t.Fatal("install without a dialog went through")
	}
	if n := e.a.settings.AcceptedTerms(game.ID); len(n) != 0 {
		t.Fatalf("acceptance recorded without the user: %+v", n)
	}
	if n := e.calls(t)["game.acceptTerms"]; n != 0 {
		t.Fatalf("game.acceptTerms sent %d times without the user", n)
	}
}

func TestAResumedJobIsNotAskedAgain(t *testing.T) {
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	terms, _ := e.a.GetGameTerms(game)
	if err := e.a.AcceptGameTerms(game, termRefs(terms)); err != nil {
		t.Fatal(err)
	}
	// A job queued before the term changed starts after it: Start doesn't gate.
	e.config(t, map[string]any{"termsBump": true})
	job := types.Job{
		ID: "resumed-licence", Kind: jobs.KindInstall, Source: "echo", GameID: game.ID,
		StoreGameID: "licensed", Title: game.Title, InstallDir: filepath.Join(e.root, "Games", "licensed"),
	}
	if err := (jobBackend{e.a}).Start(context.Background(), job); err != nil {
		t.Fatalf("a resumed job was asked to accept again: %v", err)
	}
	if n := e.calls(t)["game.install"]; n != 1 {
		t.Fatalf("game.install reached echo %d times, want the resumed job to start", n)
	}
}

func TestTermsStayOffBelowProtocol111(t *testing.T) {
	e := versionsApp(t, `["1.0","1.1","1.3","1.9"]`, termsCaps)
	game := licensedGame()
	if terms, err := e.a.GetGameTerms(game); err != nil || len(terms) != 0 {
		t.Fatalf("terms over 1.9 = %+v, %v: want none", terms, err)
	}
	job, err := e.a.InstallGame(game)
	if err != nil {
		t.Fatalf("install over 1.9: %v: nothing is asked", err)
	}
	e.waitJob(t, job.ID, jobs.StateDone)
	calls := e.calls(t)
	if calls["game.acceptTerms"] != 0 || calls["game.details"] != 0 {
		t.Fatalf("terms calls sent under 1.11: %v", calls)
	}
}

func TestAStoreWithoutServerAcceptanceKeepsTheRecordInHydra(t *testing.T) {
	t.Setenv("ECHO_MISSING", "game.acceptTerms")
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	terms, _ := e.a.GetGameTerms(game)
	if err := e.a.AcceptGameTerms(game, termRefs(terms)); err != nil {
		t.Fatalf("a store without game.acceptTerms refused the local record: %v", err)
	}
	if n := len(e.a.settings.AcceptedTerms(game.ID)); n != 2 {
		t.Fatalf("Hydra recorded %d acceptances, want 2", n)
	}
}

func TestAcceptingAnUnlistedTermIsRefused(t *testing.T) {
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	err := e.a.AcceptGameTerms(game, []types.TermRef{{ID: "invented", Version: "1"}})
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("accepting a term the store doesn't list: %v", err)
	}
	if n := len(e.a.settings.AcceptedTerms(game.ID)); n != 0 {
		t.Fatalf("recorded %d acceptances for a term that isn't listed", n)
	}
}
