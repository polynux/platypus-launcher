// Package stores is the one code path the core uses for every game source:
// the built-in Steam store, and an adapter around each approved store plugin.
package stores

import (
	"context"
	"time"

	"platypus/internal/types"
)

// Store is a game source: it lists the library, provides game page metadata
// and says how a game is launched.
type Store interface {
	// ID is the source id games carry in Game.Source.
	ID() string
	// Library returns the source's games, installed or only owned.
	Library(ctx context.Context) ([]types.Game, error)
	// Details returns summary, screenshots and achievements for the game page.
	// A store without a metadata source returns empty details.
	Details(ctx context.Context, game types.Game) (*types.GameDetails, error)
	// Launch says how to start the game. nil means Platypus runs the game's own
	// Executable through the runner pipeline (Steam executable overrides).
	// preview asks for the command only: the store must not fetch launch
	// tokens or check for updates.
	Launch(ctx context.Context, game types.Game, preview bool) (*Launch, error)
}

// Launch is exactly one of Client or Direct.
type Launch struct {
	Client *ClientLaunch
	Direct *DirectLaunch
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
	// StartTimeout is how long the client gets to start the game; 0 is the
	// session default.
	StartTimeout time.Duration
}

// DirectLaunch is a validated plugin launch spec Platypus runs itself.
type DirectLaunch struct {
	Exe       string
	Args      []string
	Dir       string
	Env       map[string]string
	NeedsWine bool
	// Redact lists argument prefixes whose values are hidden in previews and logs.
	Redact []string
}

// Registry holds the built-in stores plus the plugin stores available right
// now (approved, enabled, with the library capability).
type Registry struct {
	builtin []Store
	dynamic func() []Store
}

func NewRegistry(dynamic func() []Store, builtin ...Store) *Registry {
	return &Registry{builtin: builtin, dynamic: dynamic}
}

// All returns every store, built-in ones first.
func (r *Registry) All() []Store {
	out := append([]Store{}, r.builtin...)
	if r.dynamic != nil {
		out = append(out, r.dynamic()...)
	}
	return out
}

// Get returns the store a game comes from (Game.Source). Sideloaded games
// have none.
func (r *Registry) Get(source string) (Store, bool) {
	for _, s := range r.All() {
		if s.ID() == source {
			return s, true
		}
	}
	return nil, false
}

// SetLocale forwards the BCP 47 tag to the built-in stores that fetch
// localized metadata.
func (r *Registry) SetLocale(locale string) {
	if r == nil {
		return
	}
	for _, s := range r.builtin {
		if st, ok := s.(*Steam); ok {
			st.SetLocale(locale)
		}
	}
}
