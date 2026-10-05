// Package stores is the one code path the core uses for every game source.
// Steam is built in; store plugins get an adapter behind the same interface in
// a later milestone (see docs/plans/launcher-plugins.md).
package stores

import "hydra-launcher/internal/types"

// Store is a game source: it lists the library, provides game page metadata
// and says how a game is launched.
type Store interface {
	ID() types.StoreType
	// Library returns the store's games, installed ones only for now.
	Library() ([]types.Game, error)
	// Details returns summary, screenshots and achievements for the game page.
	// A store without a metadata source returns empty details.
	Details(game types.Game) (*types.GameDetails, error)
	// ClientLaunch returns how the store's own client starts the game, or nil
	// when Hydra starts it itself through the runner pipeline.
	ClientLaunch(game types.Game) *ClientLaunch
}

// ClientLaunch is a launch handed over to a store client.
type ClientLaunch struct {
	// Client names the store client in messages ("Steam").
	Client string
	// Command asks the client to start the game. It usually forwards the
	// request to the running client and exits, so its PID says nothing about
	// the game.
	Command []string
	// Find returns the PID of the running game, or 0. Sessions are followed
	// through it.
	Find func() int
}

// Registry holds the enabled stores in scan order.
type Registry struct {
	stores []Store
}

func NewRegistry(stores ...Store) *Registry {
	return &Registry{stores: stores}
}

func (r *Registry) All() []Store {
	return r.stores
}

// Get returns the store a game belongs to. Custom (sideloaded) games have none.
func (r *Registry) Get(id types.StoreType) (Store, bool) {
	for _, s := range r.stores {
		if s.ID() == id {
			return s, true
		}
	}
	return nil, false
}
