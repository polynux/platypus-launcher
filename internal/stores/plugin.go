package stores

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"

	"hydra-launcher/internal/jsonrpc"
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
		Native:          lg.Native,
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
