package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/savebackup"
	"platypus/internal/types"
)

// runnerWrapResult is what runner.wrap returns.
type runnerWrapResult struct {
	Command []string           `json:"command"`
	Env     map[string]string  `json:"env"`
	Cwd     string             `json:"cwd"`
	Runner  plugins.RunnerInfo `json:"runner"`
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
		"options": gameOptions(game), "tweaks": a.GetTweaks(),
		"prefixesFolder": a.effectivePrefixesFolder(), "preview": preview,
	}
	var result runnerWrapResult
	if err := a.plugins.Call(ctx, "runner", "runner.wrap", params, &result); err != nil {
		return nil, runnerFailure(err, "runner.wrapFailed", map[string]string{"title": game.Title})
	}
	if len(result.Command) == 0 {
		return nil, apperr.New("runner.invalidSpec", "the runner plugin returned an empty command", map[string]string{"title": game.Title})
	}
	p := &prepared{args: result.Command, env: result.Env, dir: result.Cwd, redact: redact, runner: result.Runner}
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

// gamePrefix collects the runner's Wine identity and the store's save roots.
// Native games and store-client launches can still need store-owned roots.
func (a *App) gamePrefix(game types.Game) (savebackup.Prefix, error) {
	a.applySavedSettings(&game)
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), launchTimeout)
	defer cancel()
	var result savebackup.Prefix
	if !game.Native && !game.ClientLaunch {
		present, err := a.runnerPlugin()
		if err != nil {
			return savebackup.Prefix{}, err
		}
		if present {
			params := map[string]any{"game": game, "runnerId": game.Settings.RunnerID, "prefixesFolder": a.effectivePrefixesFolder()}
			if err := a.plugins.Call(ctx, "runner", "runner.prefix", params, &result); err != nil {
				return savebackup.Prefix{}, apperr.Wrap("runner.wrapFailed", fmt.Errorf("runner plugin couldn't prepare %s: %w", game.Title, err), map[string]string{"title": game.Title})
			}
		}
	}
	if a.plugins != nil && a.plugins.Has(game.Source, "save-context") {
		var store plugins.GameSaveContext
		// A store that can't answer leaves the generic roots: backups still
		// find what they can rather than blocking an uninstall.
		if err := a.plugins.Call(ctx, game.Source, "game.saveContext", map[string]any{"id": game.StoreAppID}, &store); err != nil {
			a.warnf("save context for %s: %v", game.Title, err)
		} else {
			result.Store = &savebackup.StoreContext{Roots: store.Roots, StoreUserID: store.StoreUserID,
				Prefix: store.Prefix, Windows: store.Windows, Proton: store.Proton, WinUser: store.WinUser}
		}
	}
	return result, nil
}

// runnerCodes are the apperr codes a runner plugin may attach to a failure, with
// their params (protocol.md, Runner). The core shows them translated; other
// codes from the plugin are not trusted and fall back to the generic one.
var runnerCodes = map[string]bool{
	"game.noRunner": true, "game.runnerMissing": true,
	"crash.badVerb": true, "crash.noWinetricks": true, "crash.noPrefix": true,
	"crash.umuUnsupported": true, "crash.noProtonWine": true,
}

// runnerFailure is the core's error for a runner plugin failure: the plugin's
// own code when it names a known one, else fallback with params.
func runnerFailure(err error, fallback string, params map[string]string) error {
	var rpc *jsonrpc.Error
	if errors.As(err, &rpc) && len(rpc.Data) > 0 {
		var d struct {
			Code   string            `json:"code"`
			Params map[string]string `json:"params"`
		}
		if json.Unmarshal(rpc.Data, &d) == nil && runnerCodes[d.Code] {
			return apperr.New(d.Code, rpc.Message, d.Params)
		}
	}
	return apperr.Wrap(fallback, err, params)
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

// GetRunnerOptions is the per-game runner choices the game settings show
// (runner.options). Without a runner plugin there are none.
func (a *App) GetRunnerOptions() (types.SettingsSchema, error) {
	present, err := a.runnerPlugin()
	if err != nil || !present {
		return types.SettingsSchema{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), launchTimeout)
	defer cancel()
	var raw json.RawMessage
	if err := a.plugins.Call(ctx, "runner", "runner.options", map[string]any{}, &raw); err != nil {
		return types.SettingsSchema{}, err
	}
	return types.ParseSettingsSchema(raw)
}

// GetRunnerEnv is the env and wrappers a game's runner choices add. Steam starts
// a Steam game itself, so Platypus can't pass them: the game page shows them for
// the Steam launch options instead.
func (a *App) GetRunnerEnv(game types.Game) (types.RunnerEnv, error) {
	present, err := a.runnerPlugin()
	if err != nil || !present {
		return types.RunnerEnv{Env: map[string]string{}, Wrappers: []string{}}, err
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), launchTimeout)
	defer cancel()
	var out types.RunnerEnv
	if err := a.plugins.Call(ctx, "runner", "runner.env", map[string]any{"game": game, "options": gameOptions(game)}, &out); err != nil {
		return types.RunnerEnv{}, err
	}
	return out, nil
}

// gameOptions is the game's saved settings as the runner plugin reads them: the
// modeled settings and every other key the game saved (SettingsExtra). The
// plugin declares the keys it owns in runner.options.
func gameOptions(game types.Game) map[string]any {
	out := map[string]any{}
	if raw, err := json.Marshal(game.Settings); err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	for k, v := range game.SettingsExtra {
		out[k] = v
	}
	return out
}

// migrateRunnerSettings asks the runner plugin to move the settings of an older
// version to the keys it declares (runner.migrateSettings). It is one request
// for every saved game, at startup, while the runner plugin is ready; without
// the plugin nothing changes and the saved values wait for it. Only the games
// the plugin reports as changed are saved. It is idempotent.
func (a *App) migrateRunnerSettings(ctx context.Context) {
	if a.settings == nil {
		return
	}
	if present, err := a.runnerPlugin(); err != nil || !present {
		return
	}
	games := map[string]map[string]any{}
	for id, saved := range a.settings.GameSettings() {
		if len(saved) > 0 {
			games[id] = saved
		}
	}
	if len(games) == 0 {
		return
	}
	callCtx, cancel := context.WithTimeout(ctx, launchTimeout)
	defer cancel()
	var out struct {
		Changed map[string]map[string]any `json:"changed"`
	}
	if err := a.plugins.Call(callCtx, "runner", "runner.migrateSettings", map[string]any{"games": games}, &out); err != nil {
		a.host.Warnf("runner settings not migrated: %v", err)
		return
	}
	for id, settings := range out.Changed {
		if err := a.settings.SetGameSettings(id, settings); err != nil {
			a.host.Warnf("runner settings of %s not saved: %v", id, err)
		}
	}
}
