package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/types"
)

// The folder a move, check or repair acts on is the one the library reports,
// never the one the frontend sent along with the game.
func TestMoveAndCheckUseTheLibrariesFolderNotTheCallersCopy(t *testing.T) {
	t.Parallel()
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	decoy := game
	decoy.InstallDir = filepath.Join(t.TempDir(), "Decoy")
	parent := filepath.Join(t.TempDir(), "Moved")
	mustDir(t, parent)

	check, err := e.a.CheckMove(decoy, parent)
	if err != nil {
		t.Fatal(err)
	}
	if check.To != filepath.Join(parent, "tiny") {
		t.Fatalf("the move targets %q: the caller's folder was used", check.To)
	}
	j, err := e.a.VerifyGame(decoy)
	if err != nil {
		t.Fatal(err)
	}
	if j.InstallDir != game.InstallDir {
		t.Fatalf("the check runs in %q, want the library's %q", j.InstallDir, game.InstallDir)
	}
	e.waitJobDone(t, j.ID)
}

// A game that is running, or whose folder a move would put inside itself, is
// refused before anything is queued.
func TestMoveRefusesARunningGameAndAnAliasIntoIt(t *testing.T) {
	t.Parallel()
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	parent := filepath.Join(t.TempDir(), "Moved")
	mustDir(t, parent)

	e.a.sessionRunning = func(id string) bool { return id == game.ID }
	if _, err := e.a.CheckMove(game, parent); err == nil || !strings.Contains(err.Error(), "is running") {
		t.Fatalf("a running game was moved: %v", err)
	}
	if _, err := e.a.VerifyGame(game); err == nil || !strings.Contains(err.Error(), "is running") {
		t.Fatalf("a running game was checked: %v", err)
	}
	e.a.sessionRunning = nil

	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(game.InstallDir, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.MoveGame(game, alias); err == nil {
		t.Fatal("a move into the game through a link was queued")
	}
	if _, err := os.Stat(filepath.Join(game.InstallDir, "game.sh")); err != nil {
		t.Fatalf("the game was touched: %v", err)
	}
}

// A paused or queued check or move holds the game: it can't be launched from
// the folder that is being checked or moved.
func TestLaunchWaitsForAHeldMoveOrCheck(t *testing.T) {
	t.Parallel()
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	q, err := jobs.Open(filepath.Join(t.TempDir(), "jobs.json"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	e.a.jobs = q // no runner: the check stays queued and holds the game
	if _, err := q.AddVerify("echo", game.ID, game.StoreAppID, game.Title, game.InstallDir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.LaunchGame(game); err == nil || !strings.Contains(err.Error(), "being checked") {
		t.Fatalf("launch during a check: %v", err)
	}
	if _, err := e.a.LaunchGameViaClient(game); err == nil || !strings.Contains(err.Error(), "being checked") {
		t.Fatalf("launch through the client during a check: %v", err)
	}
}

// Discard never reaches the plugin for a verify or move.
func TestDiscardParamsRefuseVerifyAndMove(t *testing.T) {
	t.Parallel()
	games := t.TempDir()
	for _, kind := range []string{jobs.KindVerify, jobs.KindMove} {
		j := types.Job{ID: "j", Kind: kind, Source: "echo", InstallDir: filepath.Join(games, "echo")}
		if _, err := discardParams(j, games); err == nil {
			t.Errorf("%s produced discard params", kind)
		}
	}
}

// Two runs of one core move never overlap, and a cancel waits for the run to
// unwind before it returns.
func TestCoreMoveRunsAreSerialAndCancelWaits(t *testing.T) {
	t.Parallel()
	var m coreMoves
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run, err := m.begin("j", cancel)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.begin("j", cancel); err == nil {
		t.Fatal("a second run of a running move was started")
	}
	var unwound atomic.Bool
	go func() {
		defer m.end("j", run)
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
		unwound.Store(true)
	}()
	m.stop(context.Background(), "j")
	if !unwound.Load() {
		t.Fatal("cancel returned before the move unwound")
	}
	if m.active("j") {
		t.Fatal("the move is still registered after it unwound")
	}
	if _, err := m.begin("j", cancel); err != nil {
		t.Fatalf("a new run after the old one unwound was refused: %v", err)
	}
}

// A cancel that lands before a move starts stops the start, so the move
// doesn't run for a job the user already canceled.
func TestCancelBeforeTheStartStopsIt(t *testing.T) {
	t.Parallel()
	var m coreMoves
	m.stop(context.Background(), "j")
	if _, err := m.begin("j", func() {}); err == nil {
		t.Fatal("a move canceled before its start ran")
	}
	if _, err := m.begin("j", func() {}); err != nil {
		t.Fatalf("the next start was refused: %v", err)
	}
}

// Uninstalling a moved game forgets where core put it: a later install at the
// plugin's own folder must not be sent to the old copy.
func TestUninstallClearsAMovedGamesRecord(t *testing.T) {
	t.Setenv("ECHO_OFF", "move")
	t.Setenv("ECHO_MISSING", "game.relocated") // the record stays for the plugin's old path
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	parent := filepath.Join(t.TempDir(), "Elsewhere")
	mustDir(t, parent)
	j, err := e.a.MoveGame(game, parent)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJobDone(t, j.ID)
	moved := e.libraryTiny(t)
	if _, ok := e.a.settings.Relocation(game.ID); !ok {
		t.Fatal("the move left no record")
	}
	u, err := e.a.UninstallGameWithoutBackup(moved)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJobDone(t, u.ID)
	if _, ok := e.a.settings.Relocation(game.ID); ok {
		t.Fatal("the record outlived the uninstall")
	}
}

// An update of an installed game goes to the folder the game is in now: after
// a move, that is the new folder, not the plugin's default install folder.
func TestAnUpdateOfAMovedGameStaysInItsNewFolder(t *testing.T) {
	t.Parallel()
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	game := e.installedTiny(t)
	parent := filepath.Join(t.TempDir(), "Moved")
	mustDir(t, parent)
	m, err := e.a.MoveGame(game, parent)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJobDone(t, m.ID)

	moved := game
	moved.InstallDir = filepath.Join(parent, "tiny")
	u, err := e.a.UpdateGame(moved)
	if err != nil {
		t.Fatal(err)
	}
	if u.InstallDir != moved.InstallDir {
		t.Fatalf("the update runs in %q, want the moved game's %q", u.InstallDir, moved.InstallDir)
	}
	e.waitJobDone(t, u.ID)
}
