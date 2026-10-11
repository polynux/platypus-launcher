package stores

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/types"
)

// Plugin is a store backed by a store plugin process.
type Plugin struct {
	m *plugins.Manager
	p plugins.Plugin
	// folders returns where the plugin's games may be launched from: its
	// install folder plus folders the user allowed.
	folders func() []string
	// moved says where core moved a game's folder, when it did (see
	// WithRelocations). Nil means core moved nothing.
	moved func(gameID string) (types.Relocation, bool)
}

func NewPlugin(m *plugins.Manager, p plugins.Plugin, folders func() []string) *Plugin {
	return &Plugin{m: m, p: p, folders: folders}
}

// WithRelocations tells the store where core moved its games' folders. A
// plugin that doesn't know about the move still reports the old folder; the
// library shows the new one and launches run from it.
func (s *Plugin) WithRelocations(moved func(gameID string) (types.Relocation, bool)) *Plugin {
	s.moved = moved
	return s
}

func (s *Plugin) relocation(gameID string) (types.Relocation, bool) {
	if s.moved == nil {
		return types.Relocation{}, false
	}
	return s.moved(gameID)
}

func (s *Plugin) ID() string { return s.p.Manifest.ID }

// has reports whether the plugin offers the capability right now: what its
// last initialize said, or the manifest's list before it has initialized.
func (s *Plugin) has(capability string) bool { return s.m.Has(s.ID(), capability) }

// gameFeatures resolves a library entry's `features` against the runtime
// capabilities. A plugin that says nothing offers every capability for the
// game; one that lists some offers those it really has, once each, in the
// registry's vocabulary of the plugin's own manifest.
func (s *Plugin) gameFeatures(listed []string) []string {
	runtime := s.m.Runtime(s.ID()).Capabilities
	if listed == nil {
		return slices.Clone(runtime)
	}
	out := []string{}
	for _, f := range listed {
		if slices.Contains(runtime, f) && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

func (s *Plugin) Library(ctx context.Context) ([]types.Game, error) {
	var res plugins.LibraryList
	if err := s.m.Call(ctx, s.ID(), "library.list", nil, &res); err != nil {
		return nil, err
	}
	folders := s.folders()
	games := make([]types.Game, 0, len(res.Games))
	seen := map[string]bool{}
	for _, lg := range res.Games {
		if lg.ID == "" || seen[lg.ID] {
			continue
		}
		seen[lg.ID] = true
		games = append(games, s.toGame(lg, folders))
	}
	return games, nil
}

func (s *Plugin) toGame(lg plugins.LibraryGame, folders []string) types.Game {
	title := lg.Title
	if title == "" {
		title = lg.ID
	}
	g := types.Game{
		ID:              GameID(s.ID(), lg.ID),
		Source:          s.ID(),
		Store:           types.StoreType(s.p.Manifest.Store),
		StoreAppID:      lg.ID,
		Title:           title,
		CoverURL:        webURL(lg.CoverURL),
		HeroURL:         webURL(lg.HeroURL),
		BannerURL:       webURL(lg.BannerURL),
		IsInstalled:     lg.Installed,
		SizeBytes:       lg.SizeBytes,
		Version:         lg.Version,
		UpdateAvailable: lg.Installed && lg.UpdateAvailable,
		Installable:     lg.Installable && s.has("install"),
		Note:            lg.Note,
		Native:          lg.Native,
		Features:        s.gameFeatures(lg.Features),
		Settings:        types.GameSettings{UseGameMode: true},
	}
	// A borrowed game is only marked while the game offers `family`: the
	// plugin's own list (or the runtime set) decides, as for every capability.
	if lg.SharedBy != nil && lg.SharedBy.ID != "" && slices.Contains(g.Features, "family") {
		g.SharedBy = &types.SharedBy{ID: lg.SharedBy.ID, Name: lg.SharedBy.Name}
		// Copies only mean something for a borrowed game: an own game has none.
		g.Copies, g.PreferredCopy = normalizeCopies(lg.Copies, lg.PreferredCopy)
		g.CopyCompare = compareCopies(g.Copies, g.PreferredCopy)
	}
	if lg.Installed && lg.InstallPath != "" {
		g.InstallDir = filepath.Clean(lg.InstallPath)
		if rel, ok := s.relocation(g.ID); ok && g.InstallDir == rel.From {
			g.InstallDir = rel.To
			folders = append(slices.Clip(folders), rel.To)
		}
		real := g.InstallDir
		if r, err := filepath.EvalSymlinks(real); err == nil {
			real = r
		}
		g.OutsideFolder = !insideAny(real, folders)
	}
	return g
}

// Details asks the plugin for the game page's metadata. Plugins that answer
// game.details (stores with a metadata source) fill the same fields Steam
// games show; others leave the details empty.
func (s *Plugin) Details(ctx context.Context, game types.Game) (*types.GameDetails, error) {
	var d plugins.GameDetails
	params := map[string]any{"id": game.StoreAppID}
	if err := s.m.Call(ctx, s.ID(), "game.details", params, &d); err != nil {
		var rpcErr *jsonrpc.Error
		// An older plugin without game.details shows the library entry only.
		if errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeMethodNotFound {
			return &types.GameDetails{}, nil
		}
		return nil, err
	}
	out := &types.GameDetails{
		Summary:     d.Summary,
		Genres:      d.Genres,
		Developers:  d.Developers,
		Publishers:  d.Publishers,
		ReleaseDate: d.ReleaseDate,
		StoreURL:    storeURL(d.StoreURL),
	}
	// The compat fields are protocol 1.10: an older plugin's answer has none.
	if s.m.Runtime(s.ID()).Speaks(plugins.CompatVersion) {
		out.SteamAppID = d.SteamAppID
		out.Platforms = d.Platforms
		if d.DRM != nil {
			out.DRM = &types.DRM{
				DRMFree: d.DRM.DRMFree, OnlineRequired: d.DRM.OnlineRequired,
				Launcher: d.DRM.Launcher, DenuvoLikely: d.DRM.DenuvoLikely, Notes: d.DRM.Notes,
			}
		}
	}
	for _, u := range d.Screenshots {
		if u = webURL(u); u != "" {
			out.Screenshots = append(out.Screenshots, u)
		}
	}
	if d.HeroURL = webURL(d.HeroURL); d.HeroURL != "" {
		out.HeroURL = d.HeroURL
	}
	if d.Achievements != nil && d.Achievements.Total > 0 {
		list := make([]types.Achievement, 0, len(d.Achievements.List))
		for _, a := range d.Achievements.List {
			list = append(list, types.Achievement{
				ID: a.ID, Name: a.Name, Description: a.Description,
				Icon: webURL(a.Icon), GlobalPercent: a.GlobalPercent,
			})
		}
		// Progress is store-level data: unlocked stays false and UnlockedAt 0,
		// so the game page says "highlighted by the store".
		out.Achievements = &types.Achievements{Total: d.Achievements.Total, Unlocked: -1, List: list}
	}
	return out, nil
}

// familyRoles are the member roles the Families section names; any other
// role the plugin sends is shown as none.
var familyRoles = []string{"adult", "child"}

// FamilyMembers asks the plugin for the family group (protocol 1.8). Members
// without an id are dropped; roles and avatars are normalized like the rest
// of the plugin's data. Platypus shows them and enforces nothing.
func (s *Plugin) FamilyMembers(ctx context.Context) ([]types.FamilyMember, error) {
	var wire []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Role      string `json:"role"`
		AvatarURL string `json:"avatarUrl"`
		State     string `json:"state"`
	}
	if err := s.m.Call(ctx, s.ID(), "family.members", map[string]any{}, &wire); err != nil {
		return nil, err
	}
	out := []types.FamilyMember{}
	seen := map[string]bool{}
	for _, w := range wire {
		if w.ID == "" || seen[w.ID] {
			continue
		}
		seen[w.ID] = true
		m := types.FamilyMember{ID: w.ID, Name: w.Name, AvatarURL: webURL(w.AvatarURL), State: w.State}
		if m.Name == "" {
			m.Name = w.ID
		}
		if slices.Contains(familyRoles, w.Role) {
			m.Role = w.Role
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *Plugin) Launch(ctx context.Context, game types.Game, preview bool) (*Launch, error) {
	var spec plugins.LaunchSpec
	params := map[string]any{"id": game.StoreAppID, "preview": preview}
	if err := s.m.Call(ctx, s.ID(), "game.launchSpec", params, &spec); err != nil {
		return nil, err
	}
	folders := s.folders()
	if rel, ok := s.relocation(game.ID); ok {
		// The plugin may still name the folder core moved the game from.
		spec = rebaseSpec(spec, rel.From, rel.To)
		folders = append(slices.Clip(folders), rel.To)
	}
	return ValidateSpec(spec, s.p.Manifest, folders)
}

// rebaseSpec points the paths of a launch spec that are inside from at to.
// Arguments are left alone: they are the game's own.
func rebaseSpec(spec plugins.LaunchSpec, from, to string) plugins.LaunchSpec {
	spec.Exe = rebasePath(spec.Exe, from, to)
	spec.Cwd = rebasePath(spec.Cwd, from, to)
	if spec.Command != nil {
		command := slices.Clone(spec.Command)
		for i, c := range command {
			command[i] = rebasePath(c, from, to)
		}
		spec.Command = command
	}
	return spec
}

// rebasePath returns p with the from folder replaced by to, when p is inside
// from; any other path comes back as it is.
func rebasePath(p, from, to string) string {
	if p == from {
		return to
	}
	if rest, ok := strings.CutPrefix(p, from+string(filepath.Separator)); ok {
		return filepath.Join(to, rest)
	}
	return p
}

var unsafeIDChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// GameID is Platypus's id for a plugin game. It's used in file names (wine
// prefix, launch log), so the plugin's id is reduced to safe characters, with
// a hash suffix when that changed it so ids stay unique.
func GameID(source, id string) string {
	safe := unsafeIDChars.ReplaceAllString(id, "_")
	out := "p." + source + "." + safe
	if safe != id || safe == "" {
		sum := sha256.Sum256([]byte(id))
		out += "-" + hex.EncodeToString(sum[:4])
	}
	return out
}

// webURL keeps http(s) art URLs and drops anything else (file:, javascript:).
func webURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.String()
}

// isLoopback reports whether host is a loopback address for store-URL
// purposes: 127.0.0.0/8, ::1, or localhost (the test plugin's local page).
func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// storeURL keeps a store page link (issue #17): https, or an http loopback
// page for the test plugin. Anything else is dropped.
func storeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme == "https" {
		return u.String()
	}
	if u.Scheme == "http" && isLoopback(u.Hostname()) {
		return u.String()
	}
	return ""
}
