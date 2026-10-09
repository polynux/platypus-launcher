package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
)

// Family (the `family` capability, protocol 1.8, docs/plans/launcher-parity.md
// §2.5): the store's family group, the games members lend (LibraryGame.sharedBy,
// marked by the store adapter), and the warning a plugin raises while a
// borrowed game is in use. Hydra shows members and warnings; it never enforces
// exclusions or parental limits, those are the store's rules.

// familyTimeout bounds family.members. The plugin may be waiting on the store.
const familyTimeout = 30 * time.Second

// familyWarningMax bounds the text of a family.warning shown to the user.
const familyWarningMax = 300

// appCtx is the context plugin calls run under: the app's, or a background one
// before the window exists (tests).
func appCtx(a *App) context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// capabilityGate starts the plugin if needed and says whether it is ready and
// offers capability right now. Both are asked after initialize, so the answer
// is the plugin's own and not the manifest's guess (see optionsAllowed).
func (a *App) capabilityGate(source, capability string) (plugins.Plugin, error) {
	p, ok := a.plugins.Plugin(source)
	if !ok {
		return plugins.Plugin{}, fmt.Errorf("%s doesn't come from a store plugin", source)
	}
	if p.Status != plugins.StatusReady {
		return p, fmt.Errorf("%s is %s", p.Manifest.Name, p.Status)
	}
	ctx, cancel := context.WithTimeout(appCtx(a), optionsTimeout)
	defer cancel()
	if _, err := a.plugins.Initialize(ctx, source); err != nil {
		return p, err
	}
	if !a.plugins.Has(source, capability) {
		return p, fmt.Errorf("%s doesn't offer %s", p.Manifest.Name, capability)
	}
	return p, nil
}

// GetFamilyMembers lists the family group of a store plugin, for its Families
// section. Members are shown as the store describes them.
func (a *App) GetFamilyMembers(source string) ([]types.FamilyMember, error) {
	p, err := a.capabilityGate(source, "family")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(appCtx(a), familyTimeout)
	defer cancel()
	return stores.NewPlugin(a.plugins, p, nil).FamilyMembers(ctx)
}

// familyOffered says whether a plugin, as its initialize settled it, offers
// family. Before that the manifest is only a guess, so a notification from a
// plugin that has not initialized is not taken.
func familyOffered(rt plugins.Runtime) bool {
	return rt.Initialized && slices.Contains(rt.Capabilities, "family")
}

// familyWarningParams is the params of a family.warning notification.
type familyWarningParams struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	SecondsLeft int    `json:"secondsLeft"`
}

// familyWarning passes a plugin's family.warning on to the UI. It is only
// taken from a plugin that offers family right now, and only with a message
// for the user; the id is the store's game id, Hydra's game id is derived from
// it the same way the library does.
func (a *App) familyWarning(source string, params json.RawMessage) {
	var w familyWarningParams
	if json.Unmarshal(params, &w) != nil || w.ID == "" {
		return
	}
	message := strings.TrimSpace(w.Message)
	if message == "" || !familyOffered(a.plugins.Runtime(source)) || a.familyEmit == nil {
		return
	}
	if r := []rune(message); len(r) > familyWarningMax {
		message = string(r[:familyWarningMax])
	}
	a.familyEmit(types.FamilyWarning{
		Source:      source,
		GameID:      stores.GameID(source, w.ID),
		Message:     message,
		SecondsLeft: max(w.SecondsLeft, 0),
	})
}
