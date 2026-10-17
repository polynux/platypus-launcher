package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"platypus/internal/apperr"
	"platypus/internal/plugins"
	"platypus/internal/types"
)

type runnerAction struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// GameCrashHints asks the runner plugin to interpret the game's last abnormal
// run. The session log path stays in the core; only its contents are sent.
func (a *App) GameCrashHints(game types.Game) *types.CrashHints {
	if a.sessions == nil || !a.runnerAvailable() {
		return nil
	}
	path := a.sessions.CrashLog(game.ID)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), launchTimeout)
	defer cancel()
	params := map[string]any{"game": game, "runnerId": game.Settings.RunnerID, "log": string(raw)}
	var hints []types.CrashHint
	if err := a.plugins.Call(ctx, "runner", "runner.crashHints", params, &hints); err != nil || len(hints) == 0 {
		return nil
	}
	var actions struct {
		Actions []runnerAction `json:"actions"`
	}
	winetricks := a.plugins.Call(ctx, "runner", "runner.actions", map[string]any{
		"game": game, "runnerId": game.Settings.RunnerID, "prefixesFolder": a.effectivePrefixesFolder(),
	}, &actions) == nil
	if winetricks {
		winetricks = false
		for _, action := range actions.Actions {
			if action.ID == "winetricks" {
				winetricks = true
				break
			}
		}
	}
	return &types.CrashHints{Log: path, Winetricks: winetricks, Hints: hints}
}

// RunWinetricksVerb asks the runner plugin to install a known crash fix. The
// core enforces the generic rule that maintenance cannot alter a running game.
func (a *App) RunWinetricksVerb(game types.Game, verb string) error {
	if a.isRunning(game.ID) {
		return apperr.New("game.running", fmt.Sprintf("%s is already running", game.Title), map[string]string{"title": game.Title})
	}
	if !a.runnerAvailable() {
		return apperr.New("runner.unavailable", "the bundled runner plugin is unavailable", map[string]string{"plugin": "runner"})
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), launchTimeout)
	defer cancel()
	params := map[string]any{
		"id": "winetricks", "verb": verb, "game": game,
		"runnerId": game.Settings.RunnerID, "prefixesFolder": a.effectivePrefixesFolder(),
	}
	if err := a.plugins.Call(ctx, "runner", "runner.runAction", params, nil); err != nil {
		return apperr.Wrap("crash.winetricksFailed", err, map[string]string{"verb": verb})
	}
	return nil
}

func (a *App) runnerAvailable() bool {
	if a.plugins == nil {
		return false
	}
	p, ok := a.plugins.Plugin("runner")
	if !ok || p.Status != plugins.StatusReady {
		return false
	}
	for _, capability := range a.plugins.Runtime("runner").Capabilities {
		if capability == "runner" {
			return true
		}
	}
	return false
}

func (a *App) ctxOrBackground() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// OpenCrashLog opens the log of the game's last abnormal run in the default
// viewer (issue #46).
func (a *App) OpenCrashLog(game types.Game) error {
	path := a.sessions.CrashLog(game.ID)
	if path == "" {
		return apperr.New("crash.noLog", fmt.Sprintf("%s has no failed launch to show", game.Title), map[string]string{"title": game.Title})
	}
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return apperr.Wrap("crash.openLogFailed", err, map[string]string{"title": game.Title})
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
