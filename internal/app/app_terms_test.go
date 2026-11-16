package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
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
	t.Parallel()
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
		t.Fatalf("Platypus recorded %+v, want both versions with a date", accepted)
	}
	if pending, _ := e.a.GetGameTerms(game); len(types.Pending(pending)) != 0 {
		t.Fatalf("still pending after acceptance: %+v", pending)
	}
}

func TestAVersionBumpAsksAgainAndAStaleDialogCannotAccept(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// A store that can't list its terms because it isn't logged in refuses the
// install with a login problem, not a retry, and queues nothing: the login
// comes first, then the dialog asks the terms over again. (A job queued here
// would fail auth-required and be retried after the login without the gate.)
func TestTermsThatNeedALoginRefuseTheInstall(t *testing.T) {
	t.Parallel()
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	e.config(t, map[string]any{"termsAuth": true})
	_, err := e.a.InstallGame(game)
	if err == nil {
		t.Fatal("an install went through with the store logged out")
	}
	if code := apperr.CodeOf(err); code != "terms.needsLogin" {
		t.Fatalf("install with the store logged out: %v (code %q), want terms.needsLogin", err, code)
	}
	if !plugins.IsAuthRequired(err) {
		t.Fatalf("the refusal lost its auth code: %v", err)
	}
	if n := len(e.a.jobs.List()); n != 0 {
		t.Fatalf("%d jobs queued with the terms unreadable: %+v", n, e.a.jobs.List())
	}
}

// A job saved by an older build predates its terms acceptance: retrying it
// meets the same gate a new install would, instead of re-queuing silently.
func TestRetryRerunsTheTermsGate(t *testing.T) {
	t.Parallel()
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	saved := func() types.Job {
		t.Helper()
		j, err := e.a.jobs.AddWithOptions(jobs.KindInstall, game.Source, game.ID, game.StoreAppID,
			game.Title, filepath.Join(e.root, "Games", "licensed"), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.a.CancelJob(j.ID); err != nil {
			t.Fatal(err)
		}
		return j
	}
	j := saved()
	// Nothing accepted: the retry is refused like a new install.
	_, installErr := e.a.InstallGame(game)
	retryErr := e.a.RetryJob(j.ID)
	if retryErr == nil || apperr.CodeOf(retryErr) != apperr.CodeOf(installErr) {
		t.Fatalf("retry = %v (code %q), want the new-install refusal %v (code %q)",
			retryErr, apperr.CodeOf(retryErr), installErr, apperr.CodeOf(installErr))
	}
	if got := e.a.jobs.List(); len(got) != 1 || got[0].State != jobs.StateCanceled {
		t.Fatalf("refused retry re-queued the job: %+v", got)
	}
	// Accepted: the retry queues and installs.
	terms, err := e.a.GetGameTerms(game)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.a.AcceptGameTerms(game, termRefs(terms)); err != nil {
		t.Fatal(err)
	}
	if err := e.a.RetryJob(j.ID); err != nil {
		t.Fatalf("retry after acceptance: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	// Unreadable (logged out): the retry needs the login like a new install.
	j2 := saved()
	e.config(t, map[string]any{"termsAuth": true})
	_, installErr = e.a.InstallGame(game)
	if apperr.CodeOf(installErr) != "terms.needsLogin" {
		t.Fatalf("install logged out: %v (code %q), want terms.needsLogin", installErr, apperr.CodeOf(installErr))
	}
	if retryErr = e.a.RetryJob(j2.ID); apperr.CodeOf(retryErr) != "terms.needsLogin" {
		t.Fatalf("retry logged out: %v (code %q), want terms.needsLogin", retryErr, apperr.CodeOf(retryErr))
	}
}

func TestAStoreWithoutServerAcceptanceKeepsTheRecordInPlatypus(t *testing.T) {
	t.Setenv("ECHO_MISSING", "game.acceptTerms")
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	terms, _ := e.a.GetGameTerms(game)
	if err := e.a.AcceptGameTerms(game, termRefs(terms)); err != nil {
		t.Fatalf("a store without game.acceptTerms refused the local record: %v", err)
	}
	if n := len(e.a.settings.AcceptedTerms(game.ID)); n != 2 {
		t.Fatalf("Platypus recorded %d acceptances, want 2", n)
	}
}

func TestAcceptingAnUnlistedTermIsRefused(t *testing.T) {
	t.Parallel()
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

// Acceptance is kept per game and term version, in settings.json. A plugin
// reinstall and a fresh approval (new files, new hash) and a Platypus restart
// don't touch it: only a new version asks again.
func TestAcceptanceSurvivesAReinstallAndARestart(t *testing.T) {
	t.Parallel()
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	terms, err := e.a.GetGameTerms(game)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.a.AcceptGameTerms(game, termRefs(terms)); err != nil {
		t.Fatal(err)
	}

	pluginDir := filepath.Join(e.root, "plugins", "echo")
	if err := os.RemoveAll(pluginDir); err != nil {
		t.Fatal(err)
	}
	installEchoApp(t, filepath.Join(e.root, "plugins"))
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(fmt.Sprintf(versionsManifest, termsProtocols, termsCaps)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := e.a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.plugins.Initialize(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	after, err := e.a.GetGameTerms(game)
	if err != nil {
		t.Fatal(err)
	}
	if pending := types.Pending(after); len(pending) != 0 {
		t.Fatalf("after a reinstall pending = %+v, want none: the acceptance was kept", pending)
	}

	restarted, err := settings.Open(filepath.Join(e.root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	e.a.settings = restarted
	again, err := e.a.GetGameTerms(game)
	if err != nil {
		t.Fatal(err)
	}
	if pending := types.Pending(again); len(pending) != 0 {
		t.Fatalf("after a restart pending = %+v, want none", pending)
	}
}

// Every way to queue an install or an update is refused before acceptance:
// the dialog can't be bypassed by a folder, an options choice or a version.
// The ones that reach the gate name the terms; the others stop earlier (the
// game has no options, or no version is picked) and queue nothing either way.
func TestEveryQueueEntryPointWaitsForTheAcceptance(t *testing.T) {
	t.Parallel()
	e := versionsApp(t, termsProtocols, termsCaps)
	game := licensedGame()
	installed := game
	installed.IsInstalled = true
	folder := filepath.Join(t.TempDir(), "Licensed")
	attempts := map[string]struct {
		queue     func() error
		wantTerms bool
	}{
		"InstallGameTo":            {func() error { _, err := e.a.InstallGameTo(game, folder); return err }, true},
		"UpdateGame":               {func() error { _, err := e.a.UpdateGame(installed); return err }, true},
		"InstallGameWithOptions":   {func() error { _, err := e.a.InstallGameWithOptions(game, map[string]any{}); return err }, false},
		"InstallGameWithOptionsTo": {func() error { _, err := e.a.InstallGameWithOptionsTo(game, map[string]any{}, folder); return err }, false},
		"InstallGameVersion":       {func() error { _, err := e.a.InstallGameVersion(game, types.VersionTarget{}); return err }, false},
		"UpdateGameVersion":        {func() error { _, err := e.a.UpdateGameVersion(installed, types.VersionTarget{}); return err }, false},
	}
	for name, a := range attempts {
		err := a.queue()
		if err == nil {
			t.Errorf("%s before acceptance queued a job", name)
		} else if a.wantTerms && !strings.Contains(err.Error(), "accept the terms of Echo Licensed") {
			t.Errorf("%s before acceptance: %v, want a refusal that names the terms", name, err)
		}
	}
	if n := e.calls(t)["game.install"]; n != 0 {
		t.Fatalf("game.install reached echo %d times before the terms were accepted", n)
	}
}
