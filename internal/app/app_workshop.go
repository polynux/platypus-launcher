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

// Workshop subscriptions: the `workshop` capability (protocol 1.22,
// docs/plugins/protocol.md "Workshop"). Listing is read-only; a sync is a
// normal queued job the plugin runs, so progress and cancel ride the queue.

const (
	workshopTimeout = 45 * time.Second
	// maxWorkshopItems bounds one listing, like the other store lists.
	maxWorkshopItems = 500
)

// GetGameWorkshop returns the game's subscribed Workshop items. A nil, nil
// answer means the plugin doesn't offer them for this game: the page then
// shows no Workshop section (not an error).
func (a *App) GetGameWorkshop(game types.Game) ([]types.WorkshopItem, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok || p.Status != plugins.StatusReady || !a.plugins.Has(game.Source, "workshop") {
		return nil, nil
	}
	if game.Features != nil && !slices.Contains(game.Features, "workshop") {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, workshopTimeout)
	defer cancel()
	var reply []plugins.WorkshopItem
	if err := a.plugins.Call(ctx, game.Source, "workshop.list", map[string]any{"id": game.StoreAppID}, &reply); err != nil {
		return nil, err
	}
	return normalizeWorkshop(reply), nil
}

// normalizeWorkshop drops entries core can't show (no id) and anything the
// protocol forbids: preview URLs must be http(s) with a host.
func normalizeWorkshop(items []plugins.WorkshopItem) []types.WorkshopItem {
	out := make([]types.WorkshopItem, 0, len(items))
	for _, it := range items {
		if it.ID == "" || len(out) == maxWorkshopItems {
			continue
		}
		out = append(out, types.WorkshopItem{
			ID: it.ID, Title: it.Title, PreviewURL: httpURL(it.PreviewURL),
			SizeBytes: it.SizeBytes, UpdatedAt: it.UpdatedAt,
			Installed: it.Installed, NeedsUpdate: it.NeedsUpdate,
		})
	}
	return out
}

// SyncWorkshop queues a job that downloads or updates the game's subscribed
// Workshop items. The plugin decides where inside its game folder the
// content goes; Platypus only names the installed game's folder.
func (a *App) SyncWorkshop(game types.Game) (types.Job, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return types.Job{}, apperr.New("game.notFromStore", fmt.Sprintf("%s doesn't come from a store plugin", game.Title), map[string]string{"title": game.Title})
	}
	if p.Status != plugins.StatusReady {
		return types.Job{}, apperr.New("plugin.notReady", fmt.Sprintf("%s is %s", p.Manifest.Name, p.Status), map[string]string{"name": p.Manifest.Name, "status": string(p.Status)})
	}
	if !a.plugins.Has(game.Source, "workshop") {
		return types.Job{}, apperr.New("workshop.unsupported", fmt.Sprintf("%s doesn't offer Workshop items", p.Manifest.Name), map[string]string{"name": p.Manifest.Name})
	}
	if !game.IsInstalled {
		return types.Job{}, apperr.New("game.notInstalled", fmt.Sprintf("%s is not installed", game.Title), map[string]string{"title": game.Title})
	}
	folder, err := a.currentFolder(game)
	if err != nil {
		return types.Job{}, err
	}
	return a.jobs.AddWorkshop(game.Source, game.ID, game.StoreAppID, game.OriginalTitle(), folder)
}

// startWorkshopSync sends one workshop.sync job to its plugin.
func (a *App) startWorkshopSync(ctx context.Context, j types.Job) error {
	return a.plugins.Call(ctx, j.Source, "workshop.sync", map[string]any{
		"id": j.StoreGameID, "installDir": j.InstallDir, "jobId": j.ID,
	}, nil)
}
