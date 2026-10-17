package app

import (
	"context"
	"fmt"
	"slices"

	"platypus/internal/apperr"
	"platypus/internal/plugins"
	"platypus/internal/runner"
	"platypus/internal/types"
)

type runnerWrapResult struct {
	Command []string            `json:"command"`
	Env     map[string]string   `json:"env"`
	Cwd     string              `json:"cwd"`
	Runner  types.ProtonVersion `json:"runner"`
}

func (a *App) wrapLaunch(game types.Game, command []string, env map[string]string, cwd string, needsWine, preview bool, redact []string) (*prepared, error) {
	raw := &prepared{args: command, env: env, dir: cwd, redact: redact}
	if a.plugins == nil {
		var selected types.ProtonVersion
		if needsWine {
			var err error
			selected, err = a.selectRunner(game)
			if err != nil {
				return nil, err
			}
		}
		target := runner.Target{Exe: command[0], Args: command[1:], Env: env, Native: !needsWine}
		if runner.IsUmu(selected) {
			args, wrappedEnv, err := a.umuCommand(game, selected, target, preview)
			if err != nil {
				return nil, err
			}
			return &prepared{args: args, env: wrappedEnv, dir: cwd, redact: redact, runner: selected}, nil
		}
		args, wrappedEnv, err := runner.BuildCommandFor(game, selected, target, preview, a.prefixesFolder(), a.homeDir())
		if err != nil {
			return nil, err
		}
		return &prepared{args: args, env: wrappedEnv, dir: cwd, redact: redact, runner: selected}, nil
	}
	plugin, found := a.plugins.Plugin("runner")
	if !found {
		return raw, nil
	}
	if plugin.Status != plugins.StatusReady || !slices.Contains(a.plugins.Runtime("runner").Capabilities, "runner") {
		return nil, apperr.New("runner.unavailable", "the bundled runner plugin is unavailable", map[string]string{"plugin": "runner"})
	}
	ctx, cancel := context.WithTimeout(a.ctx, launchTimeout)
	defer cancel()
	params := map[string]any{
		"spec": map[string]any{"command": command, "env": env, "cwd": cwd, "needsWine": needsWine},
		"game": game, "runnerId": game.Settings.RunnerID,
		"options": game.SettingsExtra, "tweaks": a.GetTweaks(),
		"prefixesFolder": a.prefixesFolder(), "preview": preview,
	}
	var result runnerWrapResult
	if err := a.plugins.Call(ctx, "runner", "runner.wrap", params, &result); err != nil {
		return nil, apperr.Wrap("runner.wrapFailed", fmt.Errorf("runner plugin couldn't prepare %s: %w", game.Title, err), map[string]string{"title": game.Title})
	}
	if len(result.Command) == 0 {
		return nil, apperr.New("runner.invalidSpec", "the runner plugin returned an empty command", map[string]string{"title": game.Title})
	}
	return &prepared{args: result.Command, env: result.Env, dir: result.Cwd, redact: redact, runner: result.Runner}, nil
}
