package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/session"
	"platypus/internal/settings"
	"platypus/internal/types"
)

// Removal of sideloaded games (issue #43). Every test works in t.TempDir()
// trees only: deleting files is dangerous, so the real removal never runs on
// a path outside a temp dir.

type removeEnv struct {
	a    *App
	home string // == root: home, settings, data and cache all live below it
}

func newRemoveEnv(t *testing.T) *removeEnv {
	t.Helper()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetGamesFolder(filepath.Join(root, "Games")); err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, ctx: context.Background(), optHome: root}
	a.plugins = plugins.NewManager(plugins.Options{})
	a.sessions = session.NewTracker(func(types.SessionEvent) {})
	a.sessions.SetLogDir(filepath.Join(root, "logs"))
	if a.jobs, err = jobs.Open(filepath.Join(root, "jobs.json"), jobBackend{a}, nil); err != nil {
		t.Fatal(err)
	}
	a.jobs.SetOnFinished(a.jobFinished)
	return &removeEnv{a: a, home: root}
}

// addGame registers a sideloaded game with its executable at exeRel (below
// the temp home) and returns it as the library holds it.
func (e *removeEnv) addGame(t *testing.T, title, exeRel string) types.Game {
	t.Helper()
	exe := filepath.Join(e.home, exeRel)
	mustDir(t, filepath.Dir(exe))
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := e.a.settings.AddCustomGame(title, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range e.a.customGames() {
		if c.ID == g.ID {
			return c
		}
	}
	t.Fatalf("added game %s missing from the library", g.ID)
	return types.Game{}
}

// setPrefix pins the game's Wine prefix to dir (created) and returns it.
func (e *removeEnv) setPrefix(t *testing.T, game types.Game, dir string, marker bool) string {
	t.Helper()
	mustDir(t, dir)
	if marker {
		if err := os.WriteFile(filepath.Join(dir, "system.reg"), []byte("REGEDIT4\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	saved := e.a.settings.GameSettings()[game.ID]
	if saved == nil {
		saved = map[string]any{}
	}
	saved["wine_prefix"] = dir
	if err := e.a.settings.SetGameSettings(game.ID, saved); err != nil {
		t.Fatal(err)
	}
	for _, c := range e.a.customGames() {
		if c.ID == game.ID {
			return dir
		}
	}
	t.Fatalf("game %s missing from the library", game.ID)
	return ""
}

// refresh returns the game as the library holds it now.
func (e *removeEnv) refresh(t *testing.T, id string) (types.Game, bool) {
	t.Helper()
	for _, c := range e.a.customGames() {
		if c.ID == id {
			return c, true
		}
	}
	return types.Game{}, false
}

// runQueue runs the job queue until id ends (done, failed or canceled) and
// returns it.
func (e *removeEnv) runQueue(t *testing.T, id string) types.Job {
	t.Helper()
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); e.a.jobs.Run(runCtx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range e.a.jobs.List() {
			if j.ID == id && (j.State == jobs.StateDone || j.State == jobs.StateFailed || j.State == jobs.StateCanceled) {
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never ended: %+v", id, e.a.jobs.List())
	return types.Job{}
}

func removeJob(t *testing.T, e *removeEnv, game types.Game, opts types.RemoveOptions) types.Job {
	t.Helper()
	j, err := e.a.RemoveGame(game.ID, opts)
	if err != nil {
		t.Fatal(err)
	}
	if j.Kind != jobs.KindRemove {
		t.Fatalf("job kind = %q, want remove", j.Kind)
	}
	return e.runQueue(t, j.ID)
}

func both() types.RemoveOptions { return types.RemoveOptions{DeletePrefix: true, DeleteFiles: true} }

func TestRemoveCombinations(t *testing.T) {
	t.Parallel()
	newGame := func(t *testing.T) (*removeEnv, types.Game, string, string) {
		e := newRemoveEnv(t)
		game := e.addGame(t, "Gem", filepath.Join("Games", "Gem", "gem.sh"))
		prefix := e.setPrefix(t, game, filepath.Join(e.home, "prefixes", game.ID), true)
		if err := os.WriteFile(filepath.Join(prefix, "save.dat"), []byte("saves"), 0o644); err != nil {
			t.Fatal(err)
		}
		files := filepath.Dir(game.Executable)
		return e, game, prefix, files
	}

	t.Run("prefix only", func(t *testing.T) {
		t.Parallel()
		e, game, prefix, files := newGame(t)
		got := removeJob(t, e, game, types.RemoveOptions{DeletePrefix: true})
		if got.State != jobs.StateDone {
			t.Fatalf("job = %s (%s)", got.State, got.Error)
		}
		if _, err := os.Stat(prefix); !os.IsNotExist(err) {
			t.Fatalf("prefix still there: %v", err)
		}
		if _, err := os.Stat(filepath.Join(files, "gem.sh")); err != nil {
			t.Fatalf("game files touched: %v", err)
		}
		if _, ok := e.refresh(t, game.ID); ok {
			t.Fatal("library entry outlived the removal")
		}
	})

	t.Run("files only", func(t *testing.T) {
		t.Parallel()
		e, game, prefix, files := newGame(t)
		got := removeJob(t, e, game, types.RemoveOptions{DeleteFiles: true})
		if got.State != jobs.StateDone {
			t.Fatalf("job = %s (%s)", got.State, got.Error)
		}
		if _, err := os.Stat(files); !os.IsNotExist(err) {
			t.Fatalf("game files still there: %v", err)
		}
		if _, err := os.Stat(filepath.Join(prefix, "system.reg")); err != nil {
			t.Fatalf("prefix touched: %v", err)
		}
		if _, ok := e.refresh(t, game.ID); ok {
			t.Fatal("library entry outlived the removal")
		}
	})

	t.Run("both", func(t *testing.T) {
		t.Parallel()
		e, game, prefix, files := newGame(t)
		// A nested symlink inside the prefix must go with it without taking
		// what it points at: RemoveAll never follows links.
		outside := filepath.Join(e.home, "outside")
		mustDir(t, outside)
		keep := filepath.Join(outside, "keep.dat")
		if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(keep, filepath.Join(prefix, "link.dat")); err != nil {
			t.Fatal(err)
		}
		got := removeJob(t, e, game, both())
		if got.State != jobs.StateDone {
			t.Fatalf("job = %s (%s)", got.State, got.Error)
		}
		if _, err := os.Stat(prefix); !os.IsNotExist(err) {
			t.Fatalf("prefix still there: %v", err)
		}
		if _, err := os.Stat(files); !os.IsNotExist(err) {
			t.Fatalf("game files still there: %v", err)
		}
		if _, ok := e.refresh(t, game.ID); ok {
			t.Fatal("library entry outlived the removal")
		}
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("the deletion followed a nested symlink: %v", err)
		}
	})

	t.Run("neither flag is refused", func(t *testing.T) {
		t.Parallel()
		e, game, _, _ := newGame(t)
		if _, err := e.a.RemoveGame(game.ID, types.RemoveOptions{}); apperr.CodeOf(err) != "remove.nothingToDelete" {
			t.Fatalf("err = %v, want remove.nothingToDelete", err)
		}
		if _, err := e.a.CheckRemove(game.ID, types.RemoveOptions{}); err != nil {
			t.Fatalf("an empty check was refused: %v", err)
		}
	})
}

func TestRemoveCheckMeasuresTargets(t *testing.T) {
	t.Parallel()
	e := newRemoveEnv(t)
	game := e.addGame(t, "Gem", filepath.Join("Games", "Gem", "gem.sh"))
	prefix := e.setPrefix(t, game, filepath.Join(e.home, "prefixes", game.ID), true)
	if err := os.WriteFile(filepath.Join(prefix, "save.dat"), []byte("12345678"), 0o644); err != nil {
		t.Fatal(err)
	}
	game, _ = e.refresh(t, game.ID)

	check, err := e.a.CheckRemove(game.ID, both())
	if err != nil {
		t.Fatal(err)
	}
	if len(check.Targets) != 2 {
		t.Fatalf("targets = %+v, want prefix and files", check.Targets)
	}
	byKind := map[string]types.RemoveTarget{}
	for _, tg := range check.Targets {
		byKind[tg.Kind] = tg
	}
	pre, files := byKind["prefix"], byKind["files"]
	if !pre.Allowed || !pre.Exists || pre.Path != prefix || pre.Bytes <= 0 {
		t.Fatalf("prefix target = %+v", pre)
	}
	if !files.Allowed || !files.Exists || files.Path != filepath.Dir(game.Executable) || files.Bytes <= 0 {
		t.Fatalf("files target = %+v", files)
	}
}

func TestRemoveRefusals(t *testing.T) {
	t.Parallel()
	prefixesRoot := func(e *removeEnv) string {
		return settings.DefaultPrefixesFolderFor(settings.DataDirFor(e.home))
	}
	cases := map[string]struct {
		// arrange the game; return which target kind carries the refusal.
		arrange func(t *testing.T, e *removeEnv, game types.Game) (kind, wantCode string)
	}{
		"home": {func(t *testing.T, e *removeEnv, game types.Game) (string, string) {
			return "files", "remove.home" // exe directly in home in arrange below
		}},
		"filesystem root": {func(t *testing.T, e *removeEnv, game types.Game) (string, string) {
			e.setPrefix(t, game, "/", false)
			return "prefix", "remove.root"
		}},
		"games folder root": {func(t *testing.T, e *removeEnv, game types.Game) (string, string) {
			return "files", "remove.gamesFolder"
		}},
		"prefixes folder root": {func(t *testing.T, e *removeEnv, game types.Game) (string, string) {
			e.setPrefix(t, game, prefixesRoot(e), false)
			return "prefix", "remove.prefixesRoot"
		}},
		"exe in Downloads": {func(t *testing.T, e *removeEnv, game types.Game) (string, string) {
			return "files", "remove.userDir"
		}},
		"prefix in an XDG dir": {func(t *testing.T, e *removeEnv, game types.Game) (string, string) {
			dir := filepath.Join(e.home, "Documents", "pfx")
			e.setPrefix(t, game, dir, true)
			return "prefix", "remove.userDir"
		}},
		"symlink target": {func(t *testing.T, e *removeEnv, game types.Game) (string, string) {
			real := filepath.Join(e.home, "real")
			mustDir(t, real)
			if err := os.WriteFile(filepath.Join(real, "system.reg"), []byte("REGEDIT4\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(e.home, "linkprefix")
			if err := os.Symlink(real, link); err != nil {
				t.Fatal(err)
			}
			e.setPrefix(t, game, link, false)
			return "prefix", "remove.symlink"
		}},
		"symlinked parent into an XDG dir": {func(t *testing.T, e *removeEnv, game types.Game) (string, string) {
			docs := filepath.Join(e.home, "Documents")
			mustDir(t, filepath.Join(docs, "pfx"))
			if err := os.WriteFile(filepath.Join(docs, "pfx", "system.reg"), []byte("REGEDIT4\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(docs, filepath.Join(e.home, "via")); err != nil {
				t.Fatal(err)
			}
			e.setPrefix(t, game, filepath.Join(e.home, "via", "pfx"), false)
			return "prefix", "remove.userDir"
		}},
		"prefix without a Wine marker": {func(t *testing.T, e *removeEnv, game types.Game) (string, string) {
			e.setPrefix(t, game, filepath.Join(e.home, "prefixes", game.ID), false)
			return "prefix", "remove.noMarker"
		}},
	}
	exeFor := map[string]string{
		"home":                             filepath.Join("run.sh"),
		"games folder root":                filepath.Join("Games", "run.sh"),
		"exe in Downloads":                 filepath.Join("Downloads", "run.sh"),
		"filesystem root":                  filepath.Join("Games", "Gem", "gem.sh"),
		"prefixes folder root":             filepath.Join("Games", "Gem", "gem.sh"),
		"prefix in an XDG dir":             filepath.Join("Games", "Gem", "gem.sh"),
		"symlink target":                   filepath.Join("Games", "Gem", "gem.sh"),
		"prefix without a Wine marker":     filepath.Join("Games", "Gem", "gem.sh"),
		"symlinked parent into an XDG dir": filepath.Join("Games", "Gem", "gem.sh"),
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newRemoveEnv(t)
			game := e.addGame(t, "Gem", exeFor[name])
			kind, want := c.arrange(t, e, game)
			game, _ = e.refresh(t, game.ID)

			check, err := e.a.CheckRemove(game.ID, both())
			if err != nil {
				t.Fatalf("check was refused outright: %v", err)
			}
			found := false
			for _, tg := range check.Targets {
				if tg.Kind != kind {
					continue
				}
				found = true
				if tg.Allowed || tg.ReasonCode != want {
					t.Fatalf("%s target = %+v, want disallowed with %s", kind, tg, want)
				}
			}
			if !found {
				t.Fatalf("no %s target in %+v", kind, check.Targets)
			}
			var opts types.RemoveOptions
			if kind == "prefix" {
				opts = types.RemoveOptions{DeletePrefix: true}
			} else {
				opts = types.RemoveOptions{DeleteFiles: true}
			}
			if _, err := e.a.RemoveGame(game.ID, opts); apperr.CodeOf(err) != want {
				t.Fatalf("RemoveGame err = %v, want %s", err, want)
			}
			// Nothing was touched: the executable is still there.
			if _, err := os.Stat(game.Executable); err != nil {
				t.Fatalf("a refused removal touched the game: %v", err)
			}
		})
	}
}

func TestRemoveKeepsASharedPrefixAndNamesTheOtherGame(t *testing.T) {
	t.Parallel()
	e := newRemoveEnv(t)
	first := e.addGame(t, "First", filepath.Join("Games", "First", "first.sh"))
	shared := filepath.Join(e.home, "prefixes", "shared")
	e.setPrefix(t, first, shared, true)
	second := e.addGame(t, "Second", filepath.Join("Games", "Second", "second.sh"))
	e.setPrefix(t, second, shared, true)
	second, _ = e.refresh(t, second.ID)

	check, err := e.a.CheckRemove(second.ID, both())
	if err != nil {
		t.Fatalf("check was refused outright: %v", err)
	}
	for _, tg := range check.Targets {
		if tg.Kind != "prefix" {
			continue
		}
		if tg.Allowed || tg.ReasonCode != "remove.shared" || !strings.Contains(tg.Reason, "First") {
			t.Fatalf("shared prefix target = %+v, want a refusal naming First", tg)
		}
	}
	if _, err := e.a.RemoveGame(second.ID, both()); apperr.CodeOf(err) != "remove.shared" || !strings.Contains(err.Error(), "First") {
		t.Fatalf("err = %v, want remove.shared naming First", err)
	}
	if _, err := os.Stat(filepath.Join(shared, "system.reg")); err != nil {
		t.Fatalf("the shared prefix was touched: %v", err)
	}
}

func TestRemoveRefusesARunningGameAndAHeldOne(t *testing.T) {
	t.Parallel()
	e := newRemoveEnv(t)
	game := e.addGame(t, "Gem", filepath.Join("Games", "Gem", "gem.sh"))
	e.setPrefix(t, game, filepath.Join(e.home, "prefixes", game.ID), true)
	game, _ = e.refresh(t, game.ID)

	e.a.sessionRunning = func(id string) bool { return id == game.ID }
	if _, err := e.a.CheckRemove(game.ID, both()); apperr.CodeOf(err) != "game.running" {
		t.Fatalf("check err = %v, want game.running", err)
	}
	if _, err := e.a.RemoveGame(game.ID, both()); apperr.CodeOf(err) != "game.running" {
		t.Fatalf("remove err = %v, want game.running", err)
	}
	e.a.sessionRunning = nil

	if _, err := e.a.jobs.AddVerify("custom", game.ID, game.StoreAppID, game.Title, filepath.Dir(game.Executable), false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.CheckRemove(game.ID, both()); apperr.CodeOf(err) != "remove.busy" {
		t.Fatalf("check err = %v, want remove.busy", err)
	}
	if _, err := e.a.RemoveGame(game.ID, both()); apperr.CodeOf(err) != "remove.busy" {
		t.Fatalf("remove err = %v, want remove.busy", err)
	}
	if _, err := os.Stat(game.Executable); err != nil {
		t.Fatalf("a refused removal touched the game: %v", err)
	}
}

func TestLaunchWaitsForARemoval(t *testing.T) {
	t.Parallel()
	e := newRemoveEnv(t)
	game := e.addGame(t, "Gem", filepath.Join("Games", "Gem", "gem.sh"))
	j, err := e.a.jobs.AddRemove("custom", game.ID, game.StoreAppID, game.Title, true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = j
	if err := e.a.heldByJob(game); err == nil || !strings.Contains(err.Error(), "being removed") {
		t.Fatalf("heldByJob = %v, want a refusal about being removed", err)
	}
	if _, err := e.a.LaunchGame(game); err == nil || !strings.Contains(err.Error(), "being removed") {
		t.Fatalf("launch during a removal: %v", err)
	}
}

func TestRemoveGateFailureDeletesNothing(t *testing.T) {
	t.Parallel()
	e := newRemoveEnv(t)
	game := e.addGame(t, "Gem", filepath.Join("Games", "Gem", "gem.sh"))
	prefix := e.setPrefix(t, game, filepath.Join(e.home, "prefixes", game.ID), true)
	game, _ = e.refresh(t, game.ID)

	e.a.backupGateForTests = func(ctx context.Context, g types.Game, trigger string, allowNoSaves bool) (types.SaveGate, error) {
		if trigger != "remove" || g.ID != game.ID {
			t.Errorf("gate called with game %q trigger %q", g.ID, trigger)
		}
		return types.SaveGate{}, errors.New("the vault is sealed")
	}
	got := removeJob(t, e, game, both())
	if got.State != jobs.StateFailed || !strings.Contains(got.Error, "the vault is sealed") {
		t.Fatalf("job = %s (%s), want a failed job naming the gate error", got.State, got.Error)
	}
	if _, err := os.Stat(filepath.Join(prefix, "system.reg")); err != nil {
		t.Fatalf("the prefix was deleted although the gate failed: %v", err)
	}
	if _, err := os.Stat(game.Executable); err != nil {
		t.Fatalf("the files were deleted although the gate failed: %v", err)
	}
	if _, ok := e.refresh(t, game.ID); !ok {
		t.Fatal("the library entry was forgotten although the gate failed")
	}
}

func TestRemovePartialFailureKeepsTheEntryAndARetryFinishes(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("removal failure needs file permissions, which root ignores")
	}
	e := newRemoveEnv(t)
	game := e.addGame(t, "Gem", filepath.Join("Games", "Gem", "gem.sh"))
	prefix := e.setPrefix(t, game, filepath.Join(e.home, "prefixes", game.ID), true)
	game, _ = e.refresh(t, game.ID)
	files := filepath.Dir(game.Executable)

	// The files folder loses its write bit: deleting the prefix works, the
	// files fail halfway.
	if err := os.Chmod(files, 0o555); err != nil {
		t.Fatal(err)
	}
	j, err := e.a.RemoveGame(game.ID, both())
	if err != nil {
		t.Fatal(err)
	}
	got := e.runQueue(t, j.ID)
	if got.State != jobs.StateFailed {
		t.Fatalf("job = %s (%s), want a failed job", got.State, got.Error)
	}
	if _, err := os.Stat(prefix); !os.IsNotExist(err) {
		t.Fatalf("prefix still there after a partial removal: %v", err)
	}
	if _, ok := e.refresh(t, game.ID); !ok {
		t.Fatal("the library entry was forgotten although files remain")
	}
	if !strings.Contains(got.Leftover, files) {
		t.Fatalf("leftover = %q, want it to list the files folder", got.Leftover)
	}

	// A retry counts the missing prefix as done and finishes.
	if err := os.Chmod(files, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := e.a.jobs.Retry(j.ID); err != nil {
		t.Fatal(err)
	}
	finished := e.runQueue(t, j.ID)
	if finished.State != jobs.StateDone {
		t.Fatalf("retry = %s (%s), want done", finished.State, finished.Error)
	}
	if _, err := os.Stat(files); !os.IsNotExist(err) {
		t.Fatalf("game files still there after the retry: %v", err)
	}
	if _, ok := e.refresh(t, game.ID); ok {
		t.Fatal("library entry outlived the finished removal")
	}
}

func TestRemoveCanceledBeforeDeletionDeletesNothing(t *testing.T) {
	t.Parallel()
	e := newRemoveEnv(t)
	game := e.addGame(t, "Gem", filepath.Join("Games", "Gem", "gem.sh"))
	prefix := e.setPrefix(t, game, filepath.Join(e.home, "prefixes", game.ID), true)
	game, _ = e.refresh(t, game.ID)

	j, err := e.a.RemoveGame(game.ID, both())
	if err != nil {
		t.Fatal(err)
	}
	if err := e.a.CancelJob(j.ID); err != nil {
		t.Fatal(err)
	}
	got := e.runQueue(t, j.ID)
	if got.State != jobs.StateCanceled {
		t.Fatalf("job = %s (%s), want canceled", got.State, got.Error)
	}
	if _, err := os.Stat(filepath.Join(prefix, "system.reg")); err != nil {
		t.Fatalf("a canceled removal deleted the prefix: %v", err)
	}
	if _, err := os.Stat(game.Executable); err != nil {
		t.Fatalf("a canceled removal deleted the files: %v", err)
	}
	if _, ok := e.refresh(t, game.ID); !ok {
		t.Fatal("a canceled removal forgot the library entry")
	}
}

func TestRemoveUnknownGame(t *testing.T) {
	t.Parallel()
	e := newRemoveEnv(t)
	if _, err := e.a.CheckRemove("custom-nope", both()); apperr.CodeOf(err) != "sideload.unknown" {
		t.Fatalf("check err = %v, want sideload.unknown", err)
	}
	if _, err := e.a.RemoveGame("custom-nope", both()); apperr.CodeOf(err) != "sideload.unknown" {
		t.Fatalf("remove err = %v, want sideload.unknown", err)
	}
}

func TestDiscardRefusesARemove(t *testing.T) {
	t.Parallel()
	games := t.TempDir()
	j := types.Job{ID: "j", Kind: jobs.KindRemove, Source: "custom", GameID: "custom-1"}
	if _, err := discardParams(j, games); !errors.Is(err, jobs.ErrNoData) {
		t.Fatalf("discardParams = %v, want ErrNoData", err)
	}
	if err := func() error {
		q, err := jobs.Open(filepath.Join(t.TempDir(), "jobs.json"), nil, nil)
		if err != nil {
			return err
		}
		jj, err := q.AddRemove("custom", "custom-1", "custom", "Gem", true, false, false)
		if err != nil {
			return err
		}
		// A remove works on the installed game, so there is nothing of
		// theirs to delete, whatever state the job is in.
		if err := q.Discard(context.Background(), jj.ID); !errors.Is(err, jobs.ErrNoData) {
			return errors.New("remove discard did not report ErrNoData")
		}
		return nil
	}(); err != nil {
		t.Fatalf("queue discard: %v", err)
	}
}

func TestXdgUserDirs(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if got := xdgUserDirs(home); len(got) != len(xdgUserDirKeys) {
		t.Fatalf("defaults = %v, want one dir per key", got)
	}
	cfg := "XDG_DESKTOP_DIR=\"$HOME/Desktop\"\nXDG_DOWNLOAD_DIR=\"$HOME/dl\"\n# a comment\nXDG_DOCUMENTS_DIR=/srv/docs\nbogus\n"
	mustDir(t, filepath.Join(home, ".config"))
	if err := os.WriteFile(filepath.Join(home, ".config", "user-dirs.dirs"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs := xdgUserDirs(home)
	want := map[string]bool{
		filepath.Join(home, "Desktop"):  true,
		filepath.Join(home, "dl"):       true,
		"/srv/docs":                     true,
		filepath.Join(home, "Music"):    true,
		filepath.Join(home, "Pictures"): true,
		filepath.Join(home, "Videos"):   true,
		filepath.Join(home, "Public"):   true,
	}
	if len(dirs) != len(want) {
		t.Fatalf("dirs = %v, want %v", dirs, want)
	}
	for _, d := range dirs {
		if !want[d] {
			t.Fatalf("dirs = %v, want %v", dirs, want)
		}
	}
}

// A game folder reached through a symlinked parent (a games folder on another
// drive) is deleted normally: the resolved folder passes every check.
func TestRemoveThroughSymlinkedParent(t *testing.T) {
	t.Parallel()
	e := newRemoveEnv(t)
	drive := t.TempDir()
	mustDir(t, filepath.Join(drive, "Gem"))
	if err := os.Symlink(drive, filepath.Join(e.home, "hdd")); err != nil {
		t.Fatal(err)
	}
	game := e.addGame(t, "Gem", filepath.Join("hdd", "Gem", "gem.sh"))
	check, err := e.a.CheckRemove(game.ID, types.RemoveOptions{DeleteFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range check.Targets {
		if tg.Kind == "files" && !tg.Allowed {
			t.Fatalf("files target refused: %+v", tg)
		}
	}
	job, err := e.a.RemoveGame(game.ID, types.RemoveOptions{DeleteFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if j := e.runQueue(t, job.ID); j.State != jobs.StateDone {
		t.Fatalf("job = %+v", j)
	}
	if _, err := os.Stat(filepath.Join(drive, "Gem")); !os.IsNotExist(err) {
		t.Fatalf("game folder still there: %v", err)
	}
	if _, err := os.Stat(drive); err != nil {
		t.Fatalf("the drive folder went too: %v", err)
	}
}
