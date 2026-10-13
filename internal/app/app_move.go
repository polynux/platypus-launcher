package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/relocate"
	"platypus/internal/types"
)

// Move: the `move` capability (protocol 1.4) takes an installed game to
// another folder. A plugin that offers `move` does it itself (game.move). When
// it doesn't, core moves the folder (internal/relocate), records where the
// game went, and tells the plugin with game.relocated when it speaks 1.4.

// moveSpaceMargin is extra free space a move needs on top of the game's size.
const moveSpaceMargin = 1.02

// relocatedTimeout bounds the game.relocated call after a core move.
const relocatedTimeout = 30 * time.Second

var (
	errMoveStoppedEarly = apperr.New("move.canceled", "the move was canceled before it started", nil)
	errMoveStillStops   = apperr.New("move.stopping", "the earlier move of this game is still stopping; try again in a moment", nil)
)

// coreMoves tracks the core moves that run in a goroutine: one run per job,
// until the goroutine has unwound. The zero value is ready to use.
type coreMoves struct {
	mu      sync.Mutex
	runs    map[string]*coreRun
	stopped map[string]bool // canceled before its run started; the start refuses
}

// coreRun is one move core runs. done is closed once its goroutine is over.
type coreRun struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// begin registers a run of job id. A second run of the same job is refused
// while the first is still unwinding, so two runs never share the folders.
func (m *coreMoves) begin(id string, cancel context.CancelFunc) (*coreRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped[id] {
		delete(m.stopped, id)
		return nil, errMoveStoppedEarly
	}
	if _, busy := m.runs[id]; busy {
		return nil, errMoveStillStops
	}
	if m.runs == nil {
		m.runs = map[string]*coreRun{}
	}
	r := &coreRun{cancel: cancel, done: make(chan struct{})}
	m.runs[id] = r
	return r, nil
}

// end unregisters a run and wakes whoever waits for it.
func (m *coreMoves) end(id string, r *coreRun) {
	m.mu.Lock()
	if m.runs[id] == r {
		delete(m.runs, id)
	}
	m.mu.Unlock()
	close(r.done)
}

// stop cancels a running core move and waits, until ctx ends, for its
// goroutine to unwind. Without a run the job is marked, so a start that comes
// after the cancel refuses.
func (m *coreMoves) stop(ctx context.Context, id string) {
	m.mu.Lock()
	r, ok := m.runs[id]
	if !ok {
		if m.stopped == nil {
			m.stopped = map[string]bool{}
		}
		m.stopped[id] = true
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	r.cancel()
	select {
	case <-r.done:
	case <-ctx.Done():
	}
}

// active reports whether core is running the move of job id.
func (m *coreMoves) active(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.runs[id]
	return ok
}

// moveParams are the params of game.move for a job.
func moveParams(j types.Job) map[string]any {
	return map[string]any{"id": j.StoreGameID, "from": j.InstallDir, "to": j.Destination, "jobId": j.ID}
}

// currentFolder is the folder the game is installed in, as its store's library
// reports it (core's moves applied). The folder the UI holds is never
// trusted: moves, checks and repairs act on this one.
func (a *App) currentFolder(game types.Game) (string, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return "", apperr.New("game.notFromStore", fmt.Sprintf("%s doesn't come from a store plugin", game.Title), map[string]string{"title": game.Title})
	}
	// Through the library cache: planning a move doesn't re-list the store
	// on its own.
	lib, err := a.sourceLibrary(game.Source)
	if err != nil {
		return "", fmt.Errorf("reading %s's library: %w", p.Manifest.Name, err)
	}
	for _, g := range lib {
		if g.ID != game.ID {
			continue
		}
		if !g.IsInstalled {
			return "", apperr.New("game.notInstalled", fmt.Sprintf("%s is not installed", game.Title), map[string]string{"title": game.Title})
		}
		if g.InstallDir == "" {
			// The store says it is installed but names no folder (a store client,
			// a borrowed copy kept elsewhere): say what is known, not "not installed".
			return "", apperr.New("move.unknownLocation", fmt.Sprintf("%s is installed, but Platypus doesn't know which folder holds its files", game.Title), map[string]string{"title": game.Title})
		}
		return g.InstallDir, nil
	}
	return "", apperr.New("move.notInLibrary", fmt.Sprintf("%s is not in %s's library", game.Title, p.Manifest.Name), map[string]string{"title": game.Title, "plugin": p.Manifest.Name})
}

// moveSpace is what a move from → to needs and what the destination has free.
func moveSpace(from, to string) (need, free int64, err error) {
	size, err := relocate.TreeSize(from)
	if err != nil {
		return 0, 0, fmt.Errorf("measuring the game: %w", err)
	}
	need = int64(float64(size) * moveSpaceMargin)
	dir := filepath.Dir(to)
	f, err := jobs.FreeSpace(dir)
	if err != nil {
		return need, 0, fmt.Errorf("free space in %s: %w", dir, err)
	}
	return need, int64(f), nil
}

// noRoom is the refusal for a move that doesn't fit.
func noRoom(dir string, need, free int64) error {
	needS, freeS := jobs.HumanBytes(need), jobs.HumanBytes(free)
	return apperr.New("move.noSpace", fmt.Sprintf("not enough space in %s: the game needs %s, %s is free",
		dir, needS, freeS), map[string]string{"dir": dir, "need": needS, "free": freeS})
}

// planMove checks a move of game into parent (the folder the user picked):
// the game is installed, not running, the target is free, outside the
// protected folders and not the game's own, and the destination has room.
// It returns the folder the game is in now. The game ends up in
// parent/<its folder name>.
func (a *App) planMove(game types.Game, parent string) (string, types.MoveCheck, error) {
	check := types.MoveCheck{PluginMoves: a.plugins.Has(game.Source, "move")}
	if !game.IsInstalled {
		return "", check, apperr.New("game.notInstalled", fmt.Sprintf("%s is not installed", game.Title), map[string]string{"title": game.Title})
	}
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return "", check, apperr.New("game.notFromStore", fmt.Sprintf("%s doesn't come from a store plugin", game.Title), map[string]string{"title": game.Title})
	}
	if p.Status != plugins.StatusReady {
		return "", check, apperr.New("plugin.notReady", fmt.Sprintf("%s is %s", p.Manifest.Name, p.Status), map[string]string{"name": p.Manifest.Name, "status": string(p.Status)})
	}
	if a.isRunning(game.ID) {
		return "", check, apperr.New("game.running", fmt.Sprintf("%s is running", game.Title), map[string]string{"title": game.Title})
	}
	if !filepath.IsAbs(parent) {
		return "", check, apperr.New("move.chooseFolder", fmt.Sprintf("choose a folder to move %s into", game.Title), map[string]string{"title": game.Title})
	}
	from, err := a.currentFolder(game)
	if err != nil {
		return "", check, err
	}
	if err := checkFolderFor(from, a.settings.GamesFolder(), "move", a.homeDir()); err != nil {
		return "", check, err
	}
	parent = filepath.Clean(parent)
	to := filepath.Join(parent, filepath.Base(from))
	check.To = to
	if strings.HasPrefix(to, from+string(filepath.Separator)) {
		return "", check, apperr.New("move.insideGame", fmt.Sprintf("%s is inside the game's folder", to), map[string]string{"path": to})
	}
	if err := checkFolderFor(to, a.settings.GamesFolder(), "move into", a.homeDir()); err != nil {
		return "", check, err
	}
	if _, err := os.Lstat(to); err == nil {
		return "", check, apperr.New("move.exists", fmt.Sprintf("%s already exists; choose another folder", to), map[string]string{"path": to})
	}
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return "", check, apperr.New("move.notFolder", fmt.Sprintf("%s is not a folder", parent), map[string]string{"path": parent})
	}
	if _, _, err := relocate.Check(from, to); err != nil {
		return "", check, err
	}
	need, free, err := moveSpace(from, to)
	if err != nil {
		return "", check, err
	}
	check.NeedBytes, check.FreeBytes = need, free
	if need > free {
		return "", check, noRoom(parent, need, free)
	}
	return from, check, nil
}

// CheckMove says where a game would go and whether there is room, before the
// user confirms. Refusals come back as the error.
func (a *App) CheckMove(game types.Game, parent string) (types.MoveCheck, error) {
	_, check, err := a.planMove(game, parent)
	return check, err
}

// MoveGame queues moving an installed game into parent (see CheckMove).
func (a *App) MoveGame(game types.Game, parent string) (types.Job, error) {
	from, check, err := a.planMove(game, parent)
	if err != nil {
		return types.Job{}, err
	}
	return a.jobs.AddMove(game.Source, game.ID, game.StoreAppID, game.Title, from, check.To)
}

// startCoreMove runs a move core does itself. It returns at once; the copy
// runs in its own goroutine and reports through the queue like a plugin job.
// A move that starts fresh checks the room again: the queue checked when the
// game was queued, and the disk may have filled since.
func (a *App) startCoreMove(j types.Job) error {
	if !movePartlyDone(j.Destination) {
		need, free, err := moveSpace(j.InstallDir, j.Destination)
		if err != nil {
			return err
		}
		if need > free {
			return noRoom(filepath.Dir(j.Destination), need, free)
		}
	}
	ctx, cancel := context.WithCancel(a.ctx)
	run, err := a.moves.begin(j.ID, cancel)
	if err != nil {
		cancel()
		return err
	}
	go func() {
		defer a.moves.end(j.ID, run)
		defer cancel()
		a.runCoreMove(ctx, j)
	}()
	return nil
}

// movePartlyDone is true when a move to dest was started before (a copy, a
// staging folder or a placed game is there), so the room it needs is partly
// used already.
func movePartlyDone(dest string) bool {
	for _, p := range []string{dest, dest + relocate.StageSuffix} {
		if _, err := os.Lstat(p); !errors.Is(err, fs.ErrNotExist) {
			return true
		}
	}
	return false
}

func (a *App) runCoreMove(ctx context.Context, j types.Job) {
	res, err := relocate.Move(ctx, j.InstallDir, j.Destination, relocate.Options{
		Progress: func(done, total int64) {
			a.jobs.Notify(j.Source, "job.progress", plugins.JobUpdate{JobID: j.ID, Phase: "moving", Bytes: done, Total: total})
		},
		Placed: func(from, to string) error {
			return a.recordMove(j, from, to)
		},
	})
	if err != nil {
		// Platypus is stopping: the job stays running in the queue and resumes
		// from the same folders at the next start (relocate left no copy).
		if a.ctx.Err() != nil {
			return
		}
		// A cancel ends the job as canceled already; the note is for the rest.
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	if res.Leftover != "" {
		a.jobs.SetLeftover(j.ID, fmt.Sprintf("the game moved, but the old folder %s could not be removed", res.Leftover))
	}
	a.tellPluginRelocated(j)
	// The game is at its destination: the job is done whatever the queue
	// was told meanwhile (a cancel after the move was placed is too late).
	a.jobs.FinishMove(j.ID)
}

// recordMove writes where the game is now, before the old folder goes. A
// move from a folder core already moved keeps the first folder the plugin
// knows about, so the plugin's own (old) path still maps to the game.
func (a *App) recordMove(j types.Job, from, to string) error {
	rel := types.Relocation{From: from, To: to}
	if prev, ok := a.settings.Relocation(j.GameID); ok && prev.To == from {
		rel.From = prev.From
	}
	if err := a.settings.SetRelocation(j.GameID, rel); err != nil {
		return fmt.Errorf("the game is in %s but Platypus could not record it: %w", to, err)
	}
	// The game's folder changed: the cached listing still names the old one.
	a.invalidateLibrary(j.Source)
	return nil
}

// tellPluginRelocated tells a plugin that speaks 1.4 that core moved its game
// (game.relocated). When the plugin accepts, it knows the new folder, and
// Platypus's record is dropped. A plugin without the method keeps the record,
// which maps what it reports to the new folder.
func (a *App) tellPluginRelocated(j types.Job) {
	if !a.plugins.Runtime(j.Source).Speaks(plugins.Version{Major: 1, Minor: 4}) {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, relocatedTimeout)
	defer cancel()
	params := map[string]any{"id": j.StoreGameID, "from": j.InstallDir, "to": j.Destination}
	err := a.plugins.Call(ctx, j.Source, "game.relocated", params, nil)
	var rpcErr *jsonrpc.Error
	switch {
	case err == nil:
		if err := a.settings.ClearRelocation(j.GameID); err != nil {
			log.Printf("moves: clearing the record for %s: %v", j.Title, err)
		}
	case errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeMethodNotFound:
		// An older plugin doesn't track moves: the record stays.
	case errors.Is(err, plugins.ErrExited):
		// The plugin died: it restarts and reads its own records. Keep ours.
	default:
		log.Printf("moves: %s did not take the new folder: %v", j.Title, err)
	}
}

// PickMoveFolder asks where to move a game: the folder the game would go into.
// Returns "" when the user cancels.
func (a *App) PickMoveFolder(start string) (string, error) {
	return a.chooseFolder("Move the game into", start)
}
