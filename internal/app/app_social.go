package app

import (
	"context"
	"fmt"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/plugins"
	"platypus/internal/social"
	"platypus/internal/types"
)

// socialStatusTimeout bounds a status change sent to a store.
const socialStatusTimeout = 15 * time.Second

// socialBackend reaches plugins for the social orchestrator.
type socialBackend struct{ a *App }

// Offers: the plugin runs, has initialized, and offers social right now. A
// plugin that settled on a protocol older than 1.6 never does.
func (b socialBackend) Offers(ctx context.Context, pluginID string) bool {
	if _, err := b.a.plugins.Initialize(ctx, pluginID); err != nil {
		return false
	}
	return b.a.plugins.Has(pluginID, "social")
}

func (b socialBackend) Call(ctx context.Context, pluginID, method string, params, result any) error {
	return b.a.plugins.Call(ctx, pluginID, method, params, result)
}

// startSocial creates the friends orchestrator. Subscriptions start with
// syncSocial, once the plugins are known.
func (a *App) startSocial(ctx context.Context) {
	a.social = social.NewManager(social.Options{
		Backend:     socialBackend{a},
		Enabled:     a.settings.SocialEnabled,
		ShowPlaying: a.settings.SocialPlaying,
		Emit:        func(v social.View) { a.host.Emit("social", v) },
		Logf:        func(f string, args ...any) { a.host.Infof("social: "+f, args...) },
	})
}

// socialWanted says whether a plugin's friends list should run: the plugin is
// ready, offers social, and the user keeps the list on (on by default).
func (a *App) socialWanted(p plugins.Plugin) bool {
	id := p.Manifest.ID
	return p.Status == plugins.StatusReady && a.plugins.Has(id, "social") && a.settings.SocialEnabled(id)
}

// syncSocial starts the friends lists that should run and stops the others,
// including those of plugins that were removed. Starting is cheap: each
// subscription does its work in the background.
func (a *App) syncSocial() {
	if a.social == nil || a.plugins == nil {
		return
	}
	wanted := map[string]bool{}
	for _, p := range a.plugins.Plugins() {
		id := p.Manifest.ID
		if a.socialWanted(p) {
			wanted[id] = true
			a.social.Start(id)
		}
	}
	for _, id := range a.social.Plugins() {
		if !wanted[id] {
			a.social.Stop(id)
		}
	}
}

// socialGame is a launched game of a store plugin with a friends list: the
// plugin and its id for the game.
type socialGame struct{ plugin, storeID string }

// noteSocialGame remembers a launch of a game from a plugin with a friends
// list, so that the session event for it can say which store it belongs to.
func (a *App) noteSocialGame(game types.Game) {
	if a.plugins == nil || game.StoreAppID == "" || !a.plugins.Has(game.Source, "social") {
		return
	}
	a.socialMu.Lock()
	defer a.socialMu.Unlock()
	if a.socialGames == nil {
		a.socialGames = map[string]socialGame{}
	}
	a.socialGames[game.ID] = socialGame{plugin: game.Source, storeID: game.StoreAppID}
}

// socialSession tells the friends orchestrator when a launched game of a
// plugin with a friends list starts and stops.
func (a *App) socialSession(e types.SessionEvent) {
	if a.social == nil {
		return
	}
	a.socialMu.Lock()
	g, ok := a.socialGames[e.GameID]
	if e.State == "stopped" || e.State == "failed" {
		delete(a.socialGames, e.GameID)
	}
	a.socialMu.Unlock()
	switch e.State {
	case "running":
		if ok {
			a.social.GameStarted(g.plugin, e.GameID, g.storeID)
		}
	case "stopped", "failed":
		a.social.GameStopped(e.GameID)
	}
}

// SocialViews is every ready plugin that offers social, with its friends as
// the last change left them.
func (a *App) SocialViews() []social.View {
	out := []social.View{}
	if a.social == nil || a.plugins == nil {
		return out
	}
	for _, p := range a.plugins.Plugins() {
		if p.Status != plugins.StatusReady || !a.plugins.Has(p.Manifest.ID, "social") {
			continue
		}
		out = append(out, a.social.View(p.Manifest.ID))
	}
	return out
}

// SetSocialEnabled turns a plugin's friends list on or off. Off disconnects
// from the store; on connects at once.
func (a *App) SetSocialEnabled(pluginID string, enabled bool) error {
	p, ok := a.plugins.Plugin(pluginID)
	if !ok || !a.plugins.Has(pluginID, "social") {
		return apperr.New("social.none", fmt.Sprintf("%s has no friends list", pluginID), map[string]string{"plugin": pluginID})
	}
	if err := a.settings.SetSocialEnabled(pluginID, enabled); err != nil {
		return err
	}
	if enabled && a.socialWanted(p) {
		a.social.Start(pluginID)
	} else {
		a.social.Stop(pluginID)
	}
	return nil
}

// SetSocialPlaying turns on or off whether friends see the game the user
// plays on a plugin's store.
func (a *App) SetSocialPlaying(pluginID string, show bool) error {
	if !a.plugins.Has(pluginID, "social") {
		return apperr.New("social.none", fmt.Sprintf("%s has no friends list", pluginID), map[string]string{"plugin": pluginID})
	}
	if err := a.settings.SetSocialPlaying(pluginID, show); err != nil {
		return err
	}
	if a.social != nil {
		a.social.Refresh(pluginID)
	}
	return nil
}

// RestartSocial reconnects a plugin's friends: off and on again. The banner
// offers it after the store replaced the session or the user logged out.
func (a *App) RestartSocial(pluginID string) error {
	p, ok := a.plugins.Plugin(pluginID)
	if !ok || !a.plugins.Has(pluginID, "social") {
		return apperr.New("social.none", fmt.Sprintf("%s has no friends list", pluginID), map[string]string{"plugin": pluginID})
	}
	a.social.Stop(pluginID)
	if a.socialWanted(p) {
		a.social.Start(pluginID)
	}
	return nil
}

// socialLoggedIn brings a plugin's friends back after the user logged in: a
// session that was logged out is reconnected at once.
func (a *App) socialLoggedIn(pluginID string) {
	if a.social == nil {
		return
	}
	if a.social.Running(pluginID) {
		_ = a.RestartSocial(pluginID)
		return
	}
	a.syncSocial()
}

// SetSocialStatus changes the user's own status on a store, one of online,
// away, busy, snooze or invisible.
func (a *App) SetSocialStatus(pluginID, state string) error {
	if a.social == nil {
		return apperr.New("social.off", "friends are not running", nil)
	}
	ctx, cancel := context.WithTimeout(a.ctx, socialStatusTimeout)
	defer cancel()
	return a.social.SetStatus(ctx, pluginID, state)
}
