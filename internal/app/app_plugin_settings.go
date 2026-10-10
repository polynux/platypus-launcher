package app

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// Plugin settings (the `settings` capability, protocol 1.8, docs/plugins/
// protocol.md "Plugin settings"): the plugin describes its options, core shows
// them in the Plugins view (plugin-wide values) and on the game page (per-game
// values), stores what the user picked and sends it to the plugin on every
// change and when the plugin starts. The plugin doesn't need to persist what
// it is sent. Values never carry secrets: those go through auth.

// settingsCall is the wait for one settings request: the store may be slow.
const settingsCall = optionsTimeout

// settingsLocks serializes the settings work of each plugin: saves, and the
// values sent on plugin start. Two saves, or a save and a start, can't overtake
// each other, and each reads the saved values inside its own turn. The zero
// value is ready to use.
type settingsLocks struct {
	mu       sync.Mutex
	byPlugin map[string]*sync.Mutex
}

// of returns the lock of one plugin.
func (l *settingsLocks) of(source string) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byPlugin == nil {
		l.byPlugin = map[string]*sync.Mutex{}
	}
	m, ok := l.byPlugin[source]
	if !ok {
		m = &sync.Mutex{}
		l.byPlugin[source] = m
	}
	return m
}

// fetchSettingsSchema asks the plugin for its schema, normalized for core. The
// call has its own timeout.
func (a *App) fetchSettingsSchema(source string) (types.SettingsSchema, error) {
	ctx, cancel := context.WithTimeout(appCtx(a), settingsCall)
	defer cancel()
	var raw json.RawMessage
	if err := a.plugins.Call(ctx, source, "settings.schema", map[string]any{}, &raw); err != nil {
		return types.SettingsSchema{}, err
	}
	return types.ParseSettingsSchema(raw)
}

// applySettings sends values to the plugin: the plugin-wide ones (gameID "")
// or one game's. Core always sends the complete set for its scope, so the
// plugin can replace what it holds. The call has its own timeout.
func (a *App) applySettings(source, gameID string, values map[string]any) error {
	ctx, cancel := context.WithTimeout(appCtx(a), settingsCall)
	defer cancel()
	params := map[string]any{"values": values}
	if gameID != "" {
		params["gameId"] = gameID
	}
	var res struct{}
	return a.plugins.Call(ctx, source, "settings.apply", params, &res)
}

// savedAnswer is the answer of a save whose values are stored: applied or not.
// A plugin that doesn't take them now keeps them saved, so that is not an error.
func savedAnswer(p plugins.Plugin, err error) types.SettingsSaved {
	if err == nil {
		return types.SettingsSaved{Applied: true}
	}
	return types.SettingsSaved{Message: fmt.Sprintf("saved, but %s didn't take them: %v", p.Manifest.Name, err)}
}

// GetPluginSettings returns the plugin-wide settings of a store plugin, with
// the values the user chose. The Plugins view shows them.
func (a *App) GetPluginSettings(source string) (types.SettingsView, error) {
	if _, err := a.capabilityGate(source, "settings"); err != nil {
		return types.SettingsView{}, err
	}
	schema, err := a.fetchSettingsSchema(source)
	if err != nil {
		return types.SettingsView{}, err
	}
	return schema.ViewFor(types.ScopePlugin, a.settings.PluginValues(source)), nil
}

// SavePluginSettings stores the plugin-wide values the user changed and sends
// the whole set to the plugin. The patch is checked against the plugin's schema
// first: a value of the wrong kind, or out of range, refuses the save and keeps
// nothing. When the plugin doesn't take the values, they stay saved and the
// answer says so (see savedAnswer).
func (a *App) SavePluginSettings(source string, patch map[string]any) (types.SettingsSaved, error) {
	p, err := a.capabilityGate(source, "settings")
	if err != nil {
		return types.SettingsSaved{}, err
	}
	lock := a.settingsLocks.of(source)
	lock.Lock()
	defer lock.Unlock()
	schema, err := a.fetchSettingsSchema(source)
	if err != nil {
		return types.SettingsSaved{}, err
	}
	merged, err := schema.Scoped(types.ScopePlugin).MergeSettingValues(a.settings.PluginValues(source), patch)
	if err != nil {
		return types.SettingsSaved{}, err
	}
	if err := a.settings.SetPluginValues(source, merged); err != nil {
		return types.SettingsSaved{}, err
	}
	return savedAnswer(p, a.applySettings(source, "", merged)), nil
}

// gameSettingsGate says whether a game's own settings apply: its plugin offers
// settings, and the game's features include it.
func (a *App) gameSettingsGate(game types.Game) (plugins.Plugin, error) {
	p, err := a.capabilityGate(game.Source, "settings")
	if err != nil {
		return p, err
	}
	if !slices.Contains(game.Features, "settings") {
		return p, fmt.Errorf("%s has no settings for %s", p.Manifest.Name, game.Title)
	}
	return p, nil
}

// GetGameSettingsFields returns the per-game settings of a game, with the
// values chosen for it. The game page shows them.
func (a *App) GetGameSettingsFields(game types.Game) (types.SettingsView, error) {
	if _, err := a.gameSettingsGate(game); err != nil {
		return types.SettingsView{}, err
	}
	schema, err := a.fetchSettingsSchema(game.Source)
	if err != nil {
		return types.SettingsView{}, err
	}
	saved, _ := a.settings.PluginGameSettings(game.ID)
	return schema.ViewFor(types.ScopeGame, saved.Values), nil
}

// SaveGameSettingsFields stores the per-game values of a game the user changed
// and sends them to its plugin, the same way SavePluginSettings does for the
// plugin-wide ones.
func (a *App) SaveGameSettingsFields(game types.Game, patch map[string]any) (types.SettingsSaved, error) {
	p, err := a.gameSettingsGate(game)
	if err != nil {
		return types.SettingsSaved{}, err
	}
	lock := a.settingsLocks.of(game.Source)
	lock.Lock()
	defer lock.Unlock()
	schema, err := a.fetchSettingsSchema(game.Source)
	if err != nil {
		return types.SettingsSaved{}, err
	}
	saved, _ := a.settings.PluginGameSettings(game.ID)
	merged, err := schema.Scoped(types.ScopeGame).MergeSettingValues(saved.Values, patch)
	if err != nil {
		return types.SettingsSaved{}, err
	}
	record := settings.PluginGameValues{Source: game.Source, StoreID: game.StoreAppID, Values: merged}
	if err := a.settings.SetPluginGameSettings(game.ID, record); err != nil {
		return types.SettingsSaved{}, err
	}
	return savedAnswer(p, a.applySettings(game.Source, game.StoreAppID, merged)), nil
}

// pluginReady sends a plugin its saved settings when it has initialized (the
// manager calls it on its own goroutine, once per run). It takes the plugin's
// settings lock, reads the current schema and sends only the saved values the
// schema still takes. A plugin that doesn't offer settings, or has nothing
// saved, gets nothing. A failure is logged: the values stay saved and the next
// start tries again.
func (a *App) pluginReady(source string) {
	if !a.plugins.Has(source, "settings") {
		return
	}
	lock := a.settingsLocks.of(source)
	lock.Lock()
	defer lock.Unlock()
	schema, err := a.fetchSettingsSchema(source)
	if err != nil {
		a.warnf("%s: settings not read on start: %v", source, err)
		return
	}
	if values := schema.Scoped(types.ScopePlugin).KeepValid(a.settings.PluginValues(source)); len(values) > 0 {
		if err := a.applySettings(source, "", values); err != nil {
			a.warnf("%s: plugin settings not applied on start: %v", source, err)
		}
	}
	perGame := schema.Scoped(types.ScopeGame)
	for _, g := range a.settings.PluginGameSettingsFor(source) {
		values := perGame.KeepValid(g.Values)
		if err := a.applySettings(source, g.StoreID, values); err != nil {
			a.warnf("%s: settings of game %s not applied on start: %v", source, g.StoreID, err)
		}
	}
}

// forgetSettingsOfRemoved drops the settings of plugins the scan no longer
// finds: a removed plugin takes its values with it. Call it after a scan that
// succeeded; a failed one says nothing about what is gone.
func (a *App) forgetSettingsOfRemoved(found []plugins.Plugin) {
	present := map[string]bool{}
	for _, p := range found {
		present[p.Manifest.ID] = true
	}
	if err := a.settings.PrunePlugins(func(id string) bool { return present[id] }); err != nil {
		a.warnf("plugin settings of removed plugins not forgotten: %v", err)
	}
}

// forgetRemovedGameSettings drops the per-game settings of games a store no
// longer lists. A store whose listing failed keeps its values: the games may
// still be there.
func (a *App) forgetRemovedGameSettings(all []stores.Store, results [][]types.Game, errs []error) {
	for i, s := range all {
		if errs[i] != nil {
			continue
		}
		keep := map[string]bool{}
		for _, g := range results[i] {
			keep[g.ID] = true
		}
		if err := a.settings.PruneGames(s.ID(), keep); err != nil {
			a.warnf("%s: settings of removed games not forgotten: %v", s.ID(), err)
		}
	}
}
