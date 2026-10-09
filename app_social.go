package main

import (
	"context"
	"fmt"
	"time"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/social"
	"hydra-launcher/internal/types"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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
		Backend: socialBackend{a},
		Enabled: a.settings.SocialEnabled,
		Emit:    func(v social.View) { wruntime.EventsEmit(ctx, "social", v) },
		Logf:    func(f string, args ...any) { wruntime.LogInfof(ctx, "social: "+f, args...) },
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

// socialSession tells the friends orchestrator when a launched game starts
// and stops; it reports only the games of plugins with a friends list.
func (a *App) socialSession(e types.SessionEvent) {
	if a.social == nil {
		return
	}
	switch e.State {
	case "running":
		a.social.GameStarted(e.GameID)
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
		return fmt.Errorf("%s has no friends list", pluginID)
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

// SetSocialStatus changes the user's own status on a store, one of online,
// away, busy, snooze or invisible.
func (a *App) SetSocialStatus(pluginID, state string) error {
	if a.social == nil {
		return fmt.Errorf("friends are not running")
	}
	ctx, cancel := context.WithTimeout(a.ctx, socialStatusTimeout)
	defer cancel()
	return a.social.SetStatus(ctx, pluginID, state)
}
