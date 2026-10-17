package app

import (
	"context"
	"fmt"
	"slices"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/plugins"
	"platypus/internal/types"
)

// runnerWrapResult is what runner.wrap returns. The prefix facts are what cloud
// saves and backups need; the core doesn't read the prefix layout itself.
type runnerWrapResult struct {
	Command    []string           `json:"command"`
	Env        map[string]string  `json:"env"`
	Cwd        string             `json:"cwd"`
	Runner     plugins.RunnerInfo `json:"runner"`
	PrefixDir  string             `json:"prefixDir"`
	PrefixUser string             `json:"prefixUser"`
}

type runnerDetectResult struct {
	Native bool   `json:"native"`
	Kind   string `json:"kind"`
}

// runnerPlugin reports whether a runner plugin is installed. Installed but not
// ready is an error: a launch must not quietly fall back to the direct spec.
func (a *App) runnerPlugin() (bool, error) {
	if a.plugins == nil {
		return false, nil
	}
	plugin, found := a.plugins.Plugin("runner")
	if !found {
		return false, nil
	}
	if plugin.Status != plugins.StatusReady || !slices.Contains(a.plugins.Runtime("runner").Capabilities, "runner") {
		return false, apperr.New("runner.unavailable", "the bundled runner plugin is unavailable", map[string]string{"plugin": "runner"})
	}
	return true, nil
}

// detectExecutable asks the runner plugin what kind of file path is. With no
// runner plugin nothing is detected, and the file runs as its direct spec.
func (a *App) detectExecutable(path string) (runnerDetectResult, error) {
	present, err := a.runnerPlugin()
	if err != nil || !present {
		return runnerDetectResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), launchTimeout)
	defer cancel()
	var result runnerDetectResult
	if err := a.plugins.Call(ctx, "runner", "runner.detectExecutable", map[string]any{"path": path}, &result); err != nil {
		return runnerDetectResult{}, err
	}
	return result, nil
}

// wrapLaunch has the runner plugin wrap a launch spec, one runner.wrap per
// launch or preview. With no runner plugin the spec runs as it is.
func (a *App) wrapLaunch(game types.Game, command []string, env map[string]string, cwd string, needsWine, preview bool, redact []string) (*prepared, error) {
	raw := &prepared{args: command, env: env, dir: cwd, redact: redact}
	present, err := a.runnerPlugin()
	if err != nil {
		return nil, err
	}
	if !present {
		return raw, nil
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), launchTimeout)
	defer cancel()
	params := map[string]any{
		"spec": map[string]any{"command": command, "env": env, "cwd": cwd, "needsWine": needsWine},
		"game": game, "runnerId": game.Settings.RunnerID,
		"options": game.SettingsExtra, "tweaks": a.GetTweaks(),
		"prefixesFolder": a.effectivePrefixesFolder(), "preview": preview,
	}
	var result runnerWrapResult
	if err := a.plugins.Call(ctx, "runner", "runner.wrap", params, &result); err != nil {
		return nil, apperr.Wrap("runner.wrapFailed", fmt.Errorf("runner plugin couldn't prepare %s: %w", game.Title, err), map[string]string{"title": game.Title})
	}
	if len(result.Command) == 0 {
		return nil, apperr.New("runner.invalidSpec", "the runner plugin returned an empty command", map[string]string{"title": game.Title})
	}
	p := &prepared{args: result.Command, env: result.Env, dir: result.Cwd, redact: redact,
		runner: result.Runner, prefixDir: result.PrefixDir, prefixUser: result.PrefixUser}
	// The stop hook hands the wrap result back unread: the runner plugin
	// finds the wineserver from it.
	p.stop = func(force bool) {
		ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 6*time.Second)
		defer cancel()
		_ = a.plugins.Call(ctx, "runner", "runner.runAction", map[string]any{
			"id": "stop", "force": force, "launch": result,
		}, nil)
	}
	return p, nil
}

// runnerList is the installed runners, or none without a runner plugin.
func (a *App) runnerList() ([]plugins.RunnerInfo, error) {
	present, err := a.runnerPlugin()
	if err != nil || !present {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), launchTimeout)
	defer cancel()
	var result struct {
		Runners []plugins.RunnerInfo `json:"runners"`
	}
	if err := a.plugins.Call(ctx, "runner", "runner.list", map[string]any{}, &result); err != nil {
		return nil, err
	}
	return result.Runners, nil
}
