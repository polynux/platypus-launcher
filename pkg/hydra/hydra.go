// Package hydra re-exports the backend's scan and lookup APIs. Everything it
// exposes is what the Wails app itself calls (see app.go), for frontends that
// live in another module — such as UI previews — that can't import
// internal/… packages directly.
package hydra

import (
	"hydra-launcher/internal/fixes"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/runner"
	"hydra-launcher/internal/steam"
	"hydra-launcher/internal/types"
)

func NewSteamStore() *steam.StoreClient { return steam.NewStoreClient() }

func SteamRoot() string { return steam.Root() }

// ScanSteamGames lists every installed game across the Steam libraries, with
// playtime, last-played and local art paths merged in.
func ScanSteamGames() []types.Game { g, _ := steam.ScanSteamLibrary(); return g }

// ScanRunners lists the Proton, GE-Proton and Wine builds found on the system.
func ScanRunners() []types.ProtonVersion { return runner.ScanProtonVersions() }

// PickRunner is what "auto" resolves to.
func PickRunner() (types.ProtonVersion, bool) { return runner.PickAuto(ScanRunners()) }

// ArtPath finds a cached library image for appID in Steam's librarycache: it
// returns a path relative to the steam root, or "".
func ArtPath(root, appID string, names ...string) string { return steam.ArtPath(root, appID, names...) }

// Details merges the local Steam library cache with the Store API: summary,
// screenshots, achievements, genres. Screenshots are full-size URLs.
func Details(client *steam.StoreClient, appID string) (*types.GameDetails, error) {
	return client.Details(steam.Root(), appID)
}

// Fixes resolves compatibility data for a title: the curated recipe first,
// then ProtonDB's live rating (needs the network; nil when neither has data).
func Fixes(s *fixes.Service, appID, title string) *types.GameFixInfo {
	if fix := s.GetCuratedFix(title); fix != nil {
		return fix
	}
	fix, err := s.FetchProtonDBFix(appID)
	if err != nil {
		return nil
	}
	return fix
}

// Plugins lists the registered store connectors and fix sources.
func Plugins() []types.PluginInfo { return plugins.NewManager().ListPlugins() }

// Tweaks are the recommended per-machine tweaks, exactly as the app offers them.
func Tweaks() []types.Tweak { return fixes.SuggestTweaks(fixes.DetectSystem()) }

type (
	StoreClient = steam.StoreClient
	FixService  = fixes.Service
	GameSettings = types.GameSettings
)

func NewFixService() *fixes.Service { return fixes.NewService() }

type (
	Game          = types.Game
	ProtonVersion = types.ProtonVersion
	FixInfo       = types.GameFixInfo
	GameDetails   = types.GameDetails
	Achievements  = types.Achievements
	PluginInfo    = types.PluginInfo
	Tweak         = types.Tweak
)
