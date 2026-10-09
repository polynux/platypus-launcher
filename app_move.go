package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/relocate"
	"hydra-launcher/internal/types"
)

// Move: the `move` capability (protocol 1.4) takes an installed game to
// another folder. A plugin that offers `move` does it itself (game.move). When
// it doesn't, core moves the folder (internal/relocate), records where the
// game went, and tells the plugin with game.relocated when it speaks 1.4.

// moveSpaceMargin is extra free space a move needs on top of the game's size.
const moveSpaceMargin = 1.02

// relocatedTimeout bounds the game.relocated call after a core move.
const relocatedTimeout = 30 * time.Second

// coreMoves tracks the core moves that are running, so a cancel can stop
// them. The zero value is ready to use.
type coreMoves struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func (m *coreMoves) add(id string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancels == nil {
		m.cancels = map[string]context.CancelFunc{}
	}
	m.cancels[id] = cancel
}

// stop cancels a running core move and reports whether there was one.
func (m *coreMoves) stop(id string) bool {
	m.mu.Lock()
	cancel, ok := m.cancels[id]
	m.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (m *coreMoves) forget(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cancels, id)
}

// moveParams are the params of game.move for a job.
func moveParams(j types.Job) map[string]any {
	return map[string]any{"id": j.StoreGameID, "from": j.InstallDir, "to": j.Destination, "jobId": j.ID}
}

// moveTarget checks a move of game into parent (the folder the user picked):
// the game is installed, not running, the target is free, outside the
// protected folders and not the game's own, and the destination has room.
// The game ends up in parent/<its folder name>.
func (a *App) moveTarget(game types.Game, parent string) (types.MoveCheck, error) {
	check := types.MoveCheck{PluginMoves: a.plugins.Has(game.Source, "move")}
	if !game.IsInstalled || game.InstallDir == "" {
		return check, fmt.Errorf("%s is not installed", game.Title)
	}
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return check, fmt.Errorf("%s doesn't come from a store plugin", game.Title)
	}
	if p.Status != plugins.StatusReady {
		return check, fmt.Errorf("%s is %s", p.Manifest.Name, p.Status)
	}
	if a.sessions.Running(game.ID) {
		return check, fmt.Errorf("%s is running", game.Title)
	}
	if !filepath.IsAbs(parent) {
		return check, fmt.Errorf("choose a folder to move %s into", game.Title)
	}
	from := filepath.Clean(game.InstallDir)
	to := filepath.Join(filepath.Clean(parent), filepath.Base(from))
	check.To = to
	if strings.HasPrefix(to, from+string(filepath.Separator)) {
		return check, fmt.Errorf("%s is inside the game's folder", to)
	}
	if err := checkFolder(to, a.settings.GamesFolder(), "move into"); err != nil {
		return check, err
	}
	if _, err := os.Lstat(to); err == nil {
		return check, fmt.Errorf("%s already exists; choose another folder", to)
	}
	if info, err := os.Stat(filepath.Clean(parent)); err != nil || !info.IsDir() {
		return check, fmt.Errorf("%s is not a folder", parent)
	}
	size, err := relocate.TreeSize(game.InstallDir)
	if err != nil {
		return check, fmt.Errorf("measuring %s: %w", game.Title, err)
	}
	check.NeedBytes = int64(float64(size) * moveSpaceMargin)
	free, err := jobs.FreeSpace(filepath.Clean(parent))
	if err != nil {
		return check, fmt.Errorf("free space in %s: %w", parent, err)
	}
	check.FreeBytes = int64(free)
	if check.NeedBytes > check.FreeBytes {
		return check, fmt.Errorf("not enough space in %s: the game needs %s, %s is free",
			parent, jobs.HumanBytes(check.NeedBytes), jobs.HumanBytes(check.FreeBytes))
	}
	return check, nil
}

// CheckMove says where a game would go and whether there is room, before the
// user confirms. Refusals come back as the error.
func (a *App) CheckMove(game types.Game, parent string) (types.MoveCheck, error) {
	return a.moveTarget(game, parent)
}

// MoveGame queues moving an installed game into parent (see CheckMove).
func (a *App) MoveGame(game types.Game, parent string) (types.Job, error) {
	check, err := a.moveTarget(game, parent)
	if err != nil {
		return types.Job{}, err
	}
	return a.jobs.AddMove(game.Source, game.ID, game.StoreAppID, game.Title, game.InstallDir, check.To)
}

// startCoreMove runs a move core does itself. It returns at once; the copy
// runs in its own goroutine and reports through the queue like a plugin job.
func (a *App) startCoreMove(j types.Job) {
	ctx, cancel := context.WithCancel(a.ctx)
	a.moves.add(j.ID, cancel)
	go func() {
		defer a.moves.forget(j.ID)
		defer cancel()
		a.runCoreMove(ctx, j)
	}()
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
		// A cancel ends the job as canceled already; the note is for the rest.
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	if res.Leftover != "" {
		a.jobs.SetLeftover(j.ID, fmt.Sprintf("the game moved, but the old folder %s could not be removed", res.Leftover))
	}
	a.tellPluginRelocated(j)
	a.jobs.Notify(j.Source, "job.done", plugins.JobUpdate{JobID: j.ID})
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
		return fmt.Errorf("the game is in %s but Hydra could not record it: %w", to, err)
	}
	return nil
}

// tellPluginRelocated tells a plugin that speaks 1.4 that core moved its game
// (game.relocated). When the plugin accepts, it knows the new folder, and
// Hydra's record is dropped. A plugin without the method keeps the record,
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
