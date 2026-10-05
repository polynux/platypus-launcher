package stores

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// Plugin is a store backed by a store plugin process.
type Plugin struct {
	m *plugins.Manager
	p plugins.Plugin
	// folders returns where the plugin's games may be launched from: its
	// install folder plus folders the user allowed.
	folders func() []string
}

func NewPlugin(m *plugins.Manager, p plugins.Plugin, folders func() []string) *Plugin {
	return &Plugin{m: m, p: p, folders: folders}
}

func (s *Plugin) ID() string { return s.p.Manifest.ID }

func (s *Plugin) has(capability string) bool {
	return slices.Contains(s.p.Manifest.Capabilities, capability)
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
		Settings:        types.GameSettings{UseGameMode: true},
	}
	if lg.Installed && lg.InstallPath != "" {
		g.InstallDir = filepath.Clean(lg.InstallPath)
		real := g.InstallDir
		if r, err := filepath.EvalSymlinks(real); err == nil {
			real = r
		}
		g.OutsideFolder = !insideAny(real, folders)
	}
	return g
}

// Details: the protocol has no metadata method yet; the library entry's art
// and title are all a plugin game shows.
func (s *Plugin) Details(context.Context, types.Game) (*types.GameDetails, error) {
	return &types.GameDetails{}, nil
}

func (s *Plugin) Launch(ctx context.Context, game types.Game, preview bool) (*Launch, error) {
	var spec plugins.LaunchSpec
	params := map[string]any{"id": game.StoreAppID, "preview": preview}
	if err := s.m.Call(ctx, s.ID(), "game.launchSpec", params, &spec); err != nil {
		return nil, err
	}
	return ValidateSpec(spec, s.p.Manifest, s.folders())
}

var unsafeIDChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// GameID is Hydra's id for a plugin game. It's used in file names (wine
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
