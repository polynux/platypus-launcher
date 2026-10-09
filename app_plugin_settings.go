package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/types"
)

// Plugin settings (the `settings` capability, protocol 1.8, docs/plugins/
// protocol.md "Plugin settings"): the plugin describes its options, core shows
// them in the Plugins view (plugin-wide values) and on the game page (per-game
// values), stores what the user picked and sends it to the plugin on every
// change and when the plugin starts. The plugin doesn't need to persist what
// it is sent. Values never carry secrets: those go through auth.

// settingsCall is the wait for one settings request: the store may be slow.
const settingsCall = optionsTimeout

// fetchSettingsSchema asks the plugin for its schema, normalized for core.
func (a *App) fetchSettingsSchema(ctx context.Context, source string) (types.SettingsSchema, error) {
	var raw json.RawMessage
	if err := a.plugins.Call(ctx, source, "settings.schema", map[string]any{}, &raw); err != nil {
		return types.SettingsSchema{}, err
	}
	return types.ParseSettingsSchema(raw)
}

// applySettings sends values to the plugin: the plugin-wide ones (gameID "")
// or one game's. Core always sends the complete set for its scope, so the
// plugin can replace what it holds.
func (a *App) applySettings(ctx context.Context, source, gameID string, values map[string]any) error {
	params := map[string]any{"values": values}
	if gameID != "" {
		params["gameId"] = gameID
	}
	var res struct{}
	return a.plugins.Call(ctx, source, "settings.apply", params, &res)
}

// GetPluginSettings returns the plugin-wide settings of a store plugin, with
// the values the user chose. The Plugins view shows them.
func (a *App) GetPluginSettings(source string) (types.SettingsView, error) {
	if _, err := a.capabilityGate(source, "settings"); err != nil {
		return types.SettingsView{}, err
	}
	ctx, cancel := context.WithTimeout(appCtx(a), settingsCall)
	defer cancel()
	schema, err := a.fetchSettingsSchema(ctx, source)
	if err != nil {
		return types.SettingsView{}, err
	}
	return schema.ViewFor(types.ScopePlugin, a.settings.PluginValues(source)), nil
}

// SavePluginSettings stores the plugin-wide values the user chose and sends
// them to the plugin. Values are checked against the plugin's schema first:
// a value of the wrong kind, or out of range, refuses the save. The values are
// kept even when the plugin can't take them now; it gets them again when it
// starts, and the error says so.
func (a *App) SavePluginSettings(source string, values map[string]any) error {
	p, err := a.capabilityGate(source, "settings")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(appCtx(a), settingsCall)
	defer cancel()
	schema, err := a.fetchSettingsSchema(ctx, source)
	if err != nil {
		return err
	}
	clean, err := schema.Scoped(types.ScopePlugin).CleanSettingValues(values)
	if err != nil {
		return err
	}
	if err := a.settings.SetPluginValues(source, clean); err != nil {
		return err
	}
	if err := a.applySettings(ctx, source, "", clean); err != nil {
		return fmt.Errorf("saved, but %s didn't take them: %w", p.Manifest.Name, err)
	}
	return nil
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
	ctx, cancel := context.WithTimeout(appCtx(a), settingsCall)
	defer cancel()
	schema, err := a.fetchSettingsSchema(ctx, game.Source)
	if err != nil {
		return types.SettingsView{}, err
	}
	saved, _ := a.settings.PluginGameSettings(game.ID)
	return schema.ViewFor(types.ScopeGame, saved.Values), nil
}

// SaveGameSettingsFields stores the per-game values of a game and sends them
// to its plugin, the same way SavePluginSettings does for the plugin-wide ones.
func (a *App) SaveGameSettingsFields(game types.Game, values map[string]any) error {
	p, err := a.gameSettingsGate(game)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(appCtx(a), settingsCall)
	defer cancel()
	schema, err := a.fetchSettingsSchema(ctx, game.Source)
	if err != nil {
		return err
	}
	clean, err := schema.Scoped(types.ScopeGame).CleanSettingValues(values)
	if err != nil {
		return err
	}
	record := settings.PluginGameValues{Source: game.Source, StoreID: game.StoreAppID, Values: clean}
	if err := a.settings.SetPluginGameSettings(game.ID, record); err != nil {
		return err
	}
	if err := a.applySettings(ctx, game.Source, game.StoreAppID, clean); err != nil {
		return fmt.Errorf("saved, but %s didn't take them: %w", p.Manifest.Name, err)
	}
	return nil
}

// pluginReady sends a plugin its saved settings when it has initialized (the
// manager calls it on its own goroutine, once per run). A plugin that doesn't
// offer settings, or has nothing saved, gets nothing. A failure is logged: the
// values stay saved and the next start tries again.
func (a *App) pluginReady(source string) {
	if !a.plugins.Has(source, "settings") {
		return
	}
	ctx, cancel := context.WithTimeout(appCtx(a), settingsCall)
	defer cancel()
	if values := a.settings.PluginValues(source); len(values) > 0 {
		if err := a.applySettings(ctx, source, "", values); err != nil {
			a.warnf("%s: plugin settings not applied on start: %v", source, err)
		}
	}
	for _, g := range a.settings.PluginGameSettingsFor(source) {
		if err := a.applySettings(ctx, source, g.StoreID, g.Values); err != nil {
			a.warnf("%s: settings of game %s not applied on start: %v", source, g.StoreID, err)
		}
	}
}
