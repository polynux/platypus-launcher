package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"

	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/types"
)

// Game version selection: the `versions` capability. The plugin lists channels
// and builds and installs the one named by a target; core owns the install
// ledger, the pin and the picker UI (docs/plans/launcher-parity.md §2.2).

const (
	// A listing can go to the store; unlocking checks a password with it.
	versionsTimeout = time.Minute
	unlockTimeout   = 30 * time.Second

	// Bounds on what a plugin may put in front of the UI, and on what the UI
	// may hand to a plugin.
	maxChannels    = 200
	maxBuilds      = 2000
	maxLinks       = 10
	maxTargetField = 256
	maxManualKeys  = 16
	maxManualValue = 4 << 10 // a manifest list, one id per line
	maxPassword    = 512
)

// versionsGate says whether the game's plugin offers versions for it right
// now: the plugin is ready, the capability is on at runtime (which needs a
// protocol that includes it), and the game's own `features` keep it. The
// features are read from the store's library, not from the caller's copy of
// the game. Platypus sends no versions request when this fails.
func (a *App) versionsGate(game types.Game) (plugins.Plugin, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return plugins.Plugin{}, fmt.Errorf("%s doesn't come from a store plugin", game.Title)
	}
	if p.Status != plugins.StatusReady {
		return p, fmt.Errorf("%s is %s", p.Manifest.Name, p.Status)
	}
	if !a.plugins.Has(game.Source, "versions") {
		return p, fmt.Errorf("%s doesn't offer version selection", p.Manifest.Name)
	}
	listed, err := a.libraryCopy(game)
	if err != nil {
		return p, err
	}
	if listed.Features != nil && !slices.Contains(listed.Features, "versions") {
		return p, fmt.Errorf("%s has a single version here", game.Title)
	}
	return p, nil
}

// libraryCopy is the game as its store's library lists it now, found by id.
func (a *App) libraryCopy(game types.Game) (types.Game, error) {
	s, ok := a.stores.Get(game.Source)
	if !ok {
		return types.Game{}, fmt.Errorf("%s isn't in your library", game.Title)
	}
	ctx, cancel := context.WithTimeout(a.ctx, libraryTimeout)
	defer cancel()
	games, err := s.Library(ctx)
	if err != nil {
		return types.Game{}, fmt.Errorf("couldn't check %s's library: %w", game.Title, err)
	}
	for _, g := range games {
		if g.ID == game.ID {
			return g, nil
		}
	}
	return types.Game{}, fmt.Errorf("%s isn't in your library", game.Title)
}

// GetGameVersions asks the game's plugin which channels and builds it can
// install, with Platypus's install history for the game and its pin. Nothing is
// sent to the plugin when it doesn't offer versions for this game.
func (a *App) GetGameVersions(game types.Game) (types.VersionInfo, error) {
	if _, err := a.versionsGate(game); err != nil {
		return types.VersionInfo{}, err
	}
	history := a.ledger.Entries(game.ID)
	params := map[string]any{"id": game.StoreAppID, "ledger": history}
	pin, pinned := a.settings.Pin(game.ID)
	if pinned {
		params["pinned"] = pin
	}
	ctx, cancel := context.WithTimeout(a.ctx, versionsTimeout)
	defer cancel()
	var info types.VersionInfo
	if err := a.plugins.Call(ctx, game.Source, "versions.list", params, &info); err != nil {
		return types.VersionInfo{}, err
	}
	info = normalizeVersions(info)
	info.Ledger = history
	if pinned {
		info.Pinned = &pin
	} else {
		info.Pinned = nil
	}
	return info, nil
}

// normalizeVersions makes a plugin's answer safe and complete for the UI:
// lists are never null and bounded, entries without ids are dropped, unknown
// vocabulary falls back to the plain meaning, links must be http(s).
func normalizeVersions(in types.VersionInfo) types.VersionInfo {
	out := types.VersionInfo{
		Channels:        []types.VersionChannel{},
		Builds:          []types.VersionBuild{},
		Links:           []types.VersionLink{},
		Manual:          in.Manual,
		UpdateAvailable: in.UpdateAvailable,
	}
	for _, c := range in.Channels {
		if c.ID == "" || len(out.Channels) >= maxChannels {
			continue
		}
		if c.Requires != types.ChannelPassword {
			c.Requires = types.ChannelOpen
		}
		if c.Name == "" {
			c.Name = c.ID
		}
		out.Channels = append(out.Channels, c)
	}
	for _, b := range in.Builds {
		if b.ID == "" || len(out.Builds) >= maxBuilds {
			continue
		}
		switch b.Source {
		case types.BuildStore, types.BuildArchive, types.BuildManual:
		default:
			b.Source = types.BuildStore
		}
		if b.Available == nil {
			yes := true
			b.Available = &yes
		}
		out.Builds = append(out.Builds, b)
	}
	for _, l := range in.Links {
		if u := httpURL(l.URL); u != "" && len(out.Links) < maxLinks {
			label := l.Label
			if label == "" {
				label = u
			}
			out.Links = append(out.Links, types.VersionLink{Label: label, URL: u})
		}
	}
	if out.Manual != nil && out.Manual.Label == "" {
		out.Manual = nil
	}
	return out
}

// httpURL keeps http(s) URLs with a host, "" for anything else. The UI opens
// these in the user's browser.
func httpURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.String()
}

// UnlockVersionChannel hands a channel's password to the plugin, which checks
// it and keeps it. Platypus stores it nowhere and does not log it; a target only
// ever names the channel.
func (a *App) UnlockVersionChannel(game types.Game, channel, password string) (bool, error) {
	if _, err := a.versionsGate(game); err != nil {
		return false, err
	}
	if channel == "" || len(channel) > maxTargetField {
		return false, errors.New("pick a channel to unlock")
	}
	if password == "" || len(password) > maxPassword {
		return false, errors.New("type the channel's password")
	}
	ctx, cancel := context.WithTimeout(a.ctx, unlockTimeout)
	defer cancel()
	var res struct {
		Unlocked bool `json:"unlocked"`
	}
	params := map[string]any{"id": game.StoreAppID, "channel": channel, "password": password}
	if err := a.plugins.Call(ctx, game.Source, "versions.unlock", params, &res); err != nil {
		var re *jsonrpc.Error
		if errors.As(err, &re) && re.Code == jsonrpc.CodeMethodNotFound {
			return false, errors.New("this store has no channel passwords")
		}
		return false, err
	}
	return res.Unlocked, nil
}

// PinGameVersion holds the game at a build or channel: updates install that
// version and the update prompt stays off.
func (a *App) PinGameVersion(game types.Game, pin types.VersionPin) error {
	if _, err := a.versionsGate(game); err != nil {
		return err
	}
	return a.settings.SetPin(game.ID, pin)
}

// UnpinGameVersion returns the game to following the latest version. It works
// even when the plugin no longer offers versions.
func (a *App) UnpinGameVersion(gameID string) error {
	return a.settings.ClearPin(gameID)
}

// InstallGameVersion installs a game at a specific version.
func (a *App) InstallGameVersion(game types.Game, target types.VersionTarget) (types.Job, error) {
	if game.IsInstalled {
		return types.Job{}, fmt.Errorf("%s is already installed; switch its version instead", game.Title)
	}
	if !game.Installable {
		return types.Job{}, fmt.Errorf("%s can't be installed by Platypus", game.Title)
	}
	return a.queueVersionJob("install", game, target)
}

// UpdateGameVersion switches an installed game to another version (an older
// build counts: that is a downgrade, and the UI has warned about it).
func (a *App) UpdateGameVersion(game types.Game, target types.VersionTarget) (types.Job, error) {
	if !game.IsInstalled {
		return types.Job{}, fmt.Errorf("%s is not installed", game.Title)
	}
	return a.queueVersionJob("update", game, target)
}

func (a *App) queueVersionJob(kind string, game types.Game, target types.VersionTarget) (types.Job, error) {
	target, err := validateTarget(target)
	if err != nil {
		return types.Job{}, err
	}
	if _, err := a.versionsGate(game); err != nil {
		return types.Job{}, err
	}
	target = a.withArchiveRef(game.ID, target)
	return a.queueJobWith(kind, game, &target, a.savedOptions(game))
}

// pinnedTarget is the target an install or update of the game must use: its
// pin, or nil. A pin the plugin can no longer honour (versions switched off,
// plugin gone) stops the job instead of quietly installing the latest.
func (a *App) pinnedTarget(game types.Game) (*types.VersionTarget, error) {
	pin, ok := a.settings.Pin(game.ID)
	if !ok {
		return nil, nil
	}
	if _, err := a.versionsGate(game); err != nil {
		return nil, fmt.Errorf("%s is pinned to %s, which this store can't install right now (%v); unpin it to continue", game.Title, pin.ID, err)
	}
	t := a.withArchiveRef(game.ID, pin.Target())
	return &t, nil
}

// withArchiveRef gives a build from the install history the reference the
// history kept for it, so the plugin can fetch it again. The UI never sends
// the reference; Platypus looks it up here, for the latest install of the build.
func (a *App) withArchiveRef(gameID string, t types.VersionTarget) types.VersionTarget {
	if t.Build == "" {
		return t
	}
	for _, e := range a.ledger.Entries(gameID) {
		if e.Build == t.Build && len(e.ArchiveRef) > 0 {
			t.ArchiveRef = e.ArchiveRef
		}
	}
	return t
}

// validateTarget bounds what the UI may ask a plugin to install and returns the
// target as it is to be sent: typed values are trimmed. A target is a channel,
// a build or manual values; it has no room for a password or an archive
// reference, which Platypus adds itself.
func validateTarget(t types.VersionTarget) (types.VersionTarget, error) {
	if t.Channel == "" && t.Build == "" && len(t.Manual) == 0 {
		return t, errors.New("pick a channel, a build, or type what the store asks for")
	}
	bad := func(s string) bool {
		return len(s) > maxTargetField || strings.IndexFunc(s, unicode.IsControl) >= 0
	}
	if bad(t.Channel) || bad(t.Build) {
		return t, errors.New("that version id is not valid")
	}
	if len(t.ArchiveRef) > 0 {
		return t, errors.New("that version id is not valid")
	}
	if len(t.Manual) > maxManualKeys {
		return t, fmt.Errorf("at most %d values may be typed", maxManualKeys)
	}
	if len(t.Manual) == 0 {
		return t, nil
	}
	// Typed values may run over lines (Steam: one manifest id per depot).
	typedBad := func(s string) bool {
		return len(s) > maxManualValue || strings.IndexFunc(s, func(r rune) bool {
			return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t'
		}) >= 0
	}
	manual := make(map[string]string, len(t.Manual))
	for k, v := range t.Manual {
		v = strings.TrimSpace(v)
		if k == "" || bad(k) || v == "" || typedBad(v) {
			return t, errors.New("a typed value is empty or not valid")
		}
		manual[k] = v
	}
	t.Manual = manual
	return t, nil
}

// applyPins marks pinned games and switches their update prompt off: while a
// game is held at a version, "update available" is not the plugin's to say.
func (a *App) applyPins(lib *types.Library) {
	pins := a.settings.Pins()
	if len(pins) == 0 {
		return
	}
	for i := range lib.Games {
		if pin, ok := pins[lib.Games[i].ID]; ok {
			pin := pin
			lib.Games[i].Pinned = &pin
			lib.Games[i].UpdateAvailable = false
		}
	}
}

// recordInstall appends an install the plugin reported (job.done's
// `installed`) to the game's ledger. A plugin that doesn't offer versions has
// no history to keep.
func (a *App) recordInstall(j types.Job, in types.Installed) {
	if a.plugins == nil || !a.plugins.Has(j.Source, "versions") {
		return
	}
	err := a.ledger.Append(types.LedgerEntry{
		Store: j.Source, Game: j.GameID, Build: in.Build, Channel: in.Channel,
		VersionName: in.VersionName, ArchiveRef: in.ArchiveRef,
		Date: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		a.warnf("install history: %v", err)
	}
}

// warnf logs a warning to the host's log, and to stderr when there is no host.
func (a *App) warnf(format string, args ...any) {
	if a.host != nil && a.ctx != nil {
		a.host.Warnf(format, args...)
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
