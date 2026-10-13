package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/stores"
	"platypus/internal/types"
	"platypus/internal/apperr"
)

// Terms (protocol 1.11, the `terms` capability, docs/plugins/protocol.md
// "Terms"): the agreements a store lists for a game. Platypus shows them before an
// install, and before an update that comes with a new term version, with a
// link to each one. The user accepts them; Platypus never does. Acceptance is kept
// per (term id, version) in settings.json, and a store that keeps it too gets
// game.acceptTerms when the user accepts.

const termsTimeout = 30 * time.Second

// termsVersion is the first protocol revision with terms.
var termsVersion = plugins.Version{Major: 1, Minor: 11}

// termsOffered says whether the plugin offers terms right now: it has
// initialized, speaks 1.11 or later, the capability is on, and the game's
// `features`, when it lists some, keep it.
func (a *App) termsOffered(ctx context.Context, game types.Game) bool {
	ctx, cancel := context.WithTimeout(ctx, termsTimeout)
	defer cancel()
	if _, err := a.plugins.Initialize(ctx, game.Source); err != nil {
		return false
	}
	rt := a.plugins.Runtime(game.Source)
	v, err := plugins.ParseVersion(rt.Protocol)
	if err != nil || v.Less(termsVersion) || !slices.Contains(rt.Capabilities, "terms") {
		return false
	}
	return game.Features == nil || slices.Contains(game.Features, "terms")
}

// termsAdapter is the store adapter for a game's plugin, once the plugin is
// known to be ready.
func (a *App) termsAdapter(game types.Game) (*stores.Plugin, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return nil, apperr.New("game.notFromStore", fmt.Sprintf("%s doesn't come from a store plugin", game.Title), map[string]string{"title": game.Title})
	}
	if p.Status != plugins.StatusReady {
		return nil, apperr.New("plugin.notReady", fmt.Sprintf("%s is %s", p.Manifest.Name, p.Status), map[string]string{"name": p.Manifest.Name, "status": string(p.Status)})
	}
	return stores.NewPlugin(a.plugins, p, nil), nil
}

// gameTerms lists the store's terms for a game, each marked with the user's
// acceptance of its current version. A game with no terms, or a plugin that
// doesn't offer them, has none.
func (a *App) gameTerms(ctx context.Context, game types.Game) ([]types.GameTerm, error) {
	if !a.termsOffered(ctx, game) {
		return nil, nil
	}
	sp, err := a.termsAdapter(game)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, termsTimeout)
	defer cancel()
	terms, err := sp.Terms(ctx, game.StoreAppID)
	if err != nil {
		return nil, err
	}
	return types.MarkAccepted(terms, a.settings.AcceptedTerms(game.ID)), nil
}

// GetGameTerms lists the terms of a game and whether the user has accepted
// each version. The install dialog asks for it before it queues anything.
func (a *App) GetGameTerms(game types.Game) ([]types.GameTerm, error) {
	return a.gameTerms(appCtx(a), game)
}

// AcceptGameTerms records the user's acceptance of the given term versions.
// Only versions the store lists now are accepted, so a dialog that is out of
// date can't accept a version that has changed since. If the store keeps
// acceptance too, it is told first (game.acceptTerms, optional); the record
// is kept in Platypus either way.
func (a *App) AcceptGameTerms(game types.Game, refs []types.TermRef) error {
	ctx := appCtx(a)
	terms, err := a.gameTerms(ctx, game)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	var accepted []types.TermAcceptance
	for _, ref := range refs {
		i := slices.IndexFunc(terms, func(t types.GameTerm) bool { return t.ID == ref.ID && t.Version == ref.Version })
		if i < 0 {
			return apperr.New("terms.changed", fmt.Sprintf("the terms of %s changed; look at them again", game.Title), map[string]string{"title": game.Title})
		}
		accepted = append(accepted, types.TermAcceptance{ID: ref.ID, Version: ref.Version})
	}
	sp, err := a.termsAdapter(game)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, termsTimeout)
	defer cancel()
	if err := sp.AcceptTerms(ctx, game.StoreAppID, refs); err != nil && !stores.IsNotOffered(err) {
		return fmt.Errorf("%s didn't take the acceptance: %w", game.Title, err)
	}
	now := time.Now().Unix()
	for i := range accepted {
		accepted[i].At = now
	}
	return a.settings.AcceptTerms(game.ID, accepted)
}

// termsGate refuses an install or update while the user hasn't accepted the
// current version of a term. The install dialog accepts the terms first (see
// AcceptGameTerms); Platypus never accepts on the user's behalf. A job that was
// queued after its acceptance is not asked again when it starts.
func (a *App) termsGate(game types.Game) error {
	terms, err := a.gameTerms(appCtx(a), game)
	if err != nil {
		return apperr.Wrap("terms.unreadable", fmt.Errorf("couldn't read the terms of %s from the store; retry: %w", game.Title, err), map[string]string{"title": game.Title})
	}
	pending := types.Pending(terms)
	if len(pending) == 0 {
		return nil
	}
	titles := make([]string, len(pending))
	for i, t := range pending {
		titles[i] = t.Title
	}
	return apperr.New("terms.acceptFirst", fmt.Sprintf("accept the terms of %s first: %s", game.Title, strings.Join(titles, ", ")), map[string]string{"title": game.Title, "terms": strings.Join(titles, ", ")})
}

// termsKind says whether a queued kind is gated by terms: a fresh install and
// an update. Verify, move and uninstall don't ask for an agreement.
func termsKind(kind string) bool {
	return kind == jobs.KindInstall || kind == jobs.KindUpdate
}
