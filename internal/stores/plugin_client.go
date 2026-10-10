package stores

import (
	"context"
	"errors"

	"platypus/internal/plugins"
	"platypus/internal/types"
)

// ClientLauncher is a store that can start a game through its own client on
// request, which the user asks for when the client is running and Platypus won't
// sync cloud saves behind its back (game.launchSpec with "via": "client").
type ClientLauncher interface {
	LaunchViaClient(ctx context.Context, game types.Game) (*ClientLaunch, error)
}

// LaunchViaClient asks the plugin for a store-client launch of the game.
func (s *Plugin) LaunchViaClient(ctx context.Context, game types.Game) (*ClientLaunch, error) {
	var spec plugins.LaunchSpec
	params := map[string]any{"id": game.StoreAppID, "preview": false, "via": "client"}
	if err := s.m.Call(ctx, s.ID(), "game.launchSpec", params, &spec); err != nil {
		return nil, err
	}
	l, err := ValidateSpec(spec, s.p.Manifest, s.folders())
	if err != nil {
		return nil, err
	}
	if l.Client == nil {
		return nil, errors.New(s.p.Manifest.Name + " didn't return a launch through its client")
	}
	return l.Client, nil
}
