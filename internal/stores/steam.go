package stores

import (
	"hydra-launcher/internal/session"
	"hydra-launcher/internal/steam"
	"hydra-launcher/internal/types"
)

// Steam is the built-in Steam store: local library manifests, the Store API
// for metadata, and launches through the Steam client.
type Steam struct {
	meta *steam.StoreClient
}

func NewSteam() *Steam {
	return &Steam{meta: steam.NewStoreClient()}
}

func (s *Steam) ID() types.StoreType { return types.StoreSteam }

func (s *Steam) Library() ([]types.Game, error) {
	return steam.ScanSteamLibrary()
}

func (s *Steam) Details(game types.Game) (*types.GameDetails, error) {
	if game.StoreAppID == "" {
		return &types.GameDetails{}, nil
	}
	return s.meta.Details(steam.Root(), game.StoreAppID)
}

// ClientLaunch hands the game to the Steam client unless the user set an
// executable override, in which case Hydra runs that executable itself.
func (s *Steam) ClientLaunch(game types.Game) *ClientLaunch {
	if !steam.IsClientLaunch(game) {
		return nil
	}
	appID := game.StoreAppID
	return &ClientLaunch{
		Client:  "Steam",
		Command: steam.Command(game),
		Find:    func() int { return session.FindSteamGame(appID) },
	}
}
