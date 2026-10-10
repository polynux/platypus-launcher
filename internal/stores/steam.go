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
