package stores

import (
	"context"

	"platypus/internal/session"
	"platypus/internal/steam"
	"platypus/internal/types"
)

// Steam is the built-in Steam store: local library manifests, the Store API
// for metadata, and launches through the Steam client.
type Steam struct {
	meta *steam.StoreClient
}

func NewSteam() *Steam {
	return &Steam{meta: steam.NewStoreClient()}
}

// SetLocale sets the BCP 47 tag for store metadata lookups.
func (s *Steam) SetLocale(locale string) {
	if s != nil && s.meta != nil {
		s.meta.Locale = locale
	}
}

func (s *Steam) ID() string { return types.SourceSteam }

func (s *Steam) Library(context.Context) ([]types.Game, error) {
	return steam.ScanSteamLibrary()
}

func (s *Steam) Details(_ context.Context, game types.Game) (*types.GameDetails, error) {
	if game.StoreAppID == "" {
		return &types.GameDetails{}, nil
	}
	return s.meta.Details(steam.Root(), game.StoreAppID)
}

// HasSteamCloudCached reports whether the cached store details for appID
// include Steam Cloud, without a new request. ok is false when nothing is
// cached (unknown means false for callers).
func (s *Steam) HasSteamCloudCached(appID string) (has, ok bool) {
	if s == nil || s.meta == nil {
		return false, false
	}
	return s.meta.CachedHasSteamCloud(appID)
}

// Launch hands the game to the Steam client unless the user set an
// executable override, in which case Platypus runs that executable itself.
func (s *Steam) Launch(_ context.Context, game types.Game, _ bool) (*Launch, error) {
	if !steam.IsClientLaunch(game) {
		return nil, nil
	}
	appID := game.StoreAppID
	return &Launch{Client: &ClientLaunch{
		Client:  "Steam",
		Command: steam.Command(game),
		Find:    func() int { return session.FindSteamGame(appID) },
	}}, nil
}
