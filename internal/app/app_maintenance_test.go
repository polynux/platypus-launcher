package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/jobs"
	"platypus/internal/session"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// Verify and move against the echo plugin, through the same App wiring as
// startup (see versionsApp). Protocol 1.4 adds verify and move; the tests that
// run on 1.3 check that nothing of them is sent.

const maintenanceCaps = `["library","install","launch","auth","discard","versions","verify","move"]`
const protocol14 = `["1.0","1.1","1.3","1.4"]`

// maintenanceEnv is a versionsEnv with a session tracker (the App's verify and
// move checks ask whether the game runs) and a library that applies core's
// move records, as the app's plugin stores do.
func maintenanceEnv(t *testing.T, protocols, caps string) *versionsEnv {
	t.Helper()
	e := versionsApp(t, protocols, caps)
	e.a.sessions = session.NewTracker(func(types.SessionEvent) {})
	return e
}

// installedTiny installs the echo game and returns it as the library lists it.
func (e *versionsEnv) installedTiny(t *testing.T) types.Game {
	t.Helper()
	e.install(t, nil)
	return e.libraryTiny(t)
}

// libraryTiny lists echo's library the way the app does, with the relocation
// records applied.
func (e *versionsEnv) libraryTiny(t *testing.T) types.Game {
	t.Helper()
	return e.libraryGame(t, "tiny")
}

// mustDir creates a folder for a test, failing the test when it can't.
func mustDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

// setDamage sets how many files echo reports damaged for tiny.
func (e *versionsEnv) setDamage(t *testing.T, n int) {
	t.Helper()
	if err := e.m.Call(context.Background(), "echo", "config", map[string]any{"damage": map[string]int{"tiny": n}}, nil); err != nil {
		t.Fatal(err)
	}
}

// waitJobDone waits for a job to end as done and returns it with its report.
func (e *versionsEnv) waitJobDone(t *testing.T, id string) types.Job {
	t.Helper()
	return e.waitJob(t, id, jobs.StateDone)
}

func (e *versionsEnv) queueVerify(t *testing.T, game types.Game) types.Job {
	t.Helper()
	j, err := e.a.VerifyGame(game)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestVerifyReportsDamageThenRepairRestoresIt(t *testing.T) {
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	e.setDamage(t, 3)

	if _, err := e.a.RepairGame(game); err == nil {
		t.Fatal("a repair ran before any check")
	}
	check := e.waitJobDone(t, e.queueVerify(t, game).ID)
	if check.Verify == nil || check.Verify.Damaged != 3 || check.Verify.Repaired != 0 || check.Verify.Checked == 0 {
		t.Fatalf("check report = %+v", check.Verify)
	}
	repair, err := e.a.RepairGame(game)
	if err != nil {
		t.Fatalf("repair after a damaged check: %v", err)
	}
	fixed := e.waitJobDone(t, repair.ID)
	if fixed.Verify == nil || fixed.Verify.Repaired != 3 || fixed.Verify.Damaged != 0 {
		t.Fatalf("repair report = %+v", fixed.Verify)
	}
	clean := e.waitJobDone(t, e.queueVerify(t, game).ID)
	if clean.Verify == nil || clean.Verify.Damaged != 0 {
		t.Fatalf("after the repair the check still finds damage: %+v", clean.Verify)
	}
	if _, err := e.a.RepairGame(game); err == nil {
		t.Fatal("a repair was allowed after a clean check")
	}
	if n := e.calls(t)["game.verify"]; n != 3 {
		t.Fatalf("game.verify reached the plugin %d times, want 3 (two checks and a repair)", n)
	}
}

func TestVerifyCanBeCanceledAndLeavesNoReport(t *testing.T) {
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	e.setDamage(t, 2)
	j := e.queueVerify(t, game)
	if err := e.a.CancelJob(j.ID); err != nil {
		t.Fatal(err)
	}
	for _, got := range e.a.ListJobs() {
		if got.ID == j.ID && (got.State != jobs.StateCanceled || got.Verify != nil) {
			t.Fatalf("canceled check: %+v", got)
		}
	}
}

func TestVerifyNeedsTheCapabilityAndProtocol14(t *testing.T) {
	e := maintenanceEnv(t, `["1.0","1.1","1.3"]`, maintenanceCaps)
	game := e.installedTiny(t)
	if _, err := e.a.VerifyGame(game); err == nil {
		t.Fatal("a check was queued over protocol 1.3")
	}
	if _, err := e.a.RepairGame(game); err == nil {
		t.Fatal("a repair was queued over protocol 1.3")
	}
	for _, m := range []string{"game.verify", "game.move", "game.relocated"} {
		if n := e.calls(t)[m]; n != 0 {
			t.Fatalf("%s reached a 1.3 plugin %d times", m, n)
		}
	}
}

func TestMoveByThePluginWhenItOffersMove(t *testing.T) {
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	parent := filepath.Join(t.TempDir(), "Moved")
	mustDir(t, parent)

	check, err := e.a.CheckMove(game, parent)
	if err != nil {
		t.Fatal(err)
	}
	if !check.PluginMoves || check.To != filepath.Join(parent, "tiny") || check.FreeBytes <= 0 {
		t.Fatalf("check = %+v", check)
	}
	j, err := e.a.MoveGame(game, parent)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJobDone(t, j.ID)
	if _, err := os.Stat(filepath.Join(parent, "tiny", "game.sh")); err != nil {
		t.Fatalf("the plugin didn't move the game: %v", err)
	}
	if _, ok := e.a.settings.Relocation(game.ID); ok {
		t.Fatal("Platypus recorded a core move for a move the plugin made")
	}
	if got := e.libraryTiny(t); got.InstallDir != filepath.Join(parent, "tiny") {
		t.Fatalf("library shows %q after the move", got.InstallDir)
	}
	if n := e.calls(t)["game.relocated"]; n != 0 {
		t.Fatalf("game.relocated sent after the plugin's own move: %d", n)
	}
}

func TestMoveByCoreWhenThePluginHasNoMoveAndTellsIt(t *testing.T) {
	t.Setenv("ECHO_OFF", "move")
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	from := game.InstallDir
	parent := filepath.Join(t.TempDir(), "Elsewhere")
	mustDir(t, parent)

	j, err := e.a.MoveGame(game, parent)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJobDone(t, j.ID)
	to := filepath.Join(parent, "tiny")
	if _, err := os.Stat(filepath.Join(to, "game.sh")); err != nil {
		t.Fatalf("core didn't move the game: %v", err)
	}
	if _, err := os.Stat(from); !os.IsNotExist(err) {
		t.Fatalf("the old folder is still there: %v", err)
	}
	// echo accepts game.relocated, so it knows the new folder and Platypus's
	// record is dropped.
	if _, ok := e.a.settings.Relocation(game.ID); ok {
		t.Fatal("the record stayed although the plugin took the new folder")
	}
	if n := e.calls(t)["game.relocated"]; n != 1 {
		t.Fatalf("game.relocated sent %d times, want 1", n)
	}
	if got := e.libraryTiny(t); got.InstallDir != to {
		t.Fatalf("library shows %q", got.InstallDir)
	}
}

func TestMoveByCoreKeepsItsRecordForAPluginThatDoesNotRebase(t *testing.T) {
	t.Setenv("ECHO_OFF", "move")
	t.Setenv("ECHO_MISSING", "game.relocated")
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	from := game.InstallDir
	parent := filepath.Join(t.TempDir(), "Elsewhere")
	mustDir(t, parent)
	to := filepath.Join(parent, "tiny")

	j, err := e.a.MoveGame(game, parent)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJobDone(t, j.ID)
	rel, ok := e.a.settings.Relocation(game.ID)
	if !ok || rel.From != from || rel.To != to {
		t.Fatalf("record = %+v (%v), want %s → %s", rel, ok, from, to)
	}
	if got := e.libraryTiny(t); got.InstallDir != to {
		t.Fatalf("library shows %q, want the new folder %q", got.InstallDir, to)
	}
	// The plugin still answers from its own records, which name the old
	// folder: the launch spec is rebased into the new one.
	p, _ := e.m.Plugin("echo")
	s := stores.NewPlugin(e.m, p, func() []string { return e.m.Folders("echo", e.a.settings.GamesFolder()) }).
		WithRelocations(e.a.settings.Relocation)
	launch, err := s.Launch(context.Background(), game, true)
	if err != nil {
		t.Fatalf("launch after the move: %v", err)
	}
	if launch.Direct == nil || !strings.HasPrefix(launch.Direct.Exe, to+string(filepath.Separator)) {
		t.Fatalf("launch runs %+v, want a script inside %s", launch.Direct, to)
	}
}

func TestMoveByCoreWorksOverProtocol13WithoutAnyNewCalls(t *testing.T) {
	// An older plugin: no verify, no move, no relocated, but core can still
	// move the game it installed.
	e := maintenanceEnv(t, `["1.0","1.1","1.3"]`, maintenanceCaps)
	game := e.installedTiny(t)
	parent := filepath.Join(t.TempDir(), "Old")
	mustDir(t, parent)
	j, err := e.a.MoveGame(game, parent)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJobDone(t, j.ID)
	if _, err := os.Stat(filepath.Join(parent, "tiny", "game.sh")); err != nil {
		t.Fatalf("core didn't move the game: %v", err)
	}
	if n := e.calls(t)["game.move"] + e.calls(t)["game.relocated"]; n != 0 {
		t.Fatalf("a 1.3 plugin received %d move calls", n)
	}
}

func TestMoveRefusals(t *testing.T) {
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	root := t.TempDir()
	taken := filepath.Join(root, "taken")
	mustDir(t, filepath.Join(taken, "tiny"))
	inside := filepath.Join(game.InstallDir, "nested")
	mustDir(t, inside)

	cases := map[string]struct {
		parent string
		want   string
	}{
		"a relative folder":      {"relative", "choose a folder"},
		"a folder that is taken": {taken, "already exists"},
		"a system folder":        {"/usr", "system folder"},
		"inside the game":        {inside, "inside the game's folder"},
	}
	for name, c := range cases {
		_, err := e.a.MoveGame(game, c.parent)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
	if _, err := os.Stat(filepath.Join(game.InstallDir, "game.sh")); err != nil {
		t.Fatalf("a refused move touched the game: %v", err)
	}
}

func TestMoveNeedsAnInstalledGameFromAPlugin(t *testing.T) {
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	notInstalled := types.Game{ID: "p.echo.tiny", Source: "echo", StoreAppID: "tiny", Title: "Echo Tiny"}
	if _, err := e.a.MoveGame(notInstalled, t.TempDir()); err == nil {
		t.Fatal("a game that isn't installed was moved")
	}
	if _, err := e.a.MoveGame(types.Game{Source: "steam", IsInstalled: true, InstallDir: "/x"}, t.TempDir()); err == nil {
		t.Fatal("a game from another source was moved")
	}
}
