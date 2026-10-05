package stores

import (
	"testing"

	"hydra-launcher/internal/types"
)

func TestRegistryGet(t *testing.T) {
	r := NewRegistry(NewSteam())
	if s, ok := r.Get(types.StoreSteam); !ok || s.ID() != types.StoreSteam {
		t.Fatalf("Get(steam) = %v, %v", s, ok)
	}
	if _, ok := r.Get(types.StoreCustom); ok {
		t.Fatal("custom games must have no store")
	}
}

func TestSteamClientLaunch(t *testing.T) {
	s := NewSteam()
	game := types.Game{Store: types.StoreSteam, StoreAppID: "620"}
	cl := s.ClientLaunch(game)
	if cl == nil || cl.Client != "Steam" || len(cl.Command) == 0 || cl.Find == nil {
		t.Fatalf("installed Steam app should launch through the client, got %+v", cl)
	}

	// An executable override means Hydra runs the game itself.
	game.Executable = "/games/portal2/portal2.exe"
	if cl := s.ClientLaunch(game); cl != nil {
		t.Fatalf("override should bypass the client, got %+v", cl)
	}
}
