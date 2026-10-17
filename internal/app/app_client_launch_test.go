package app

import (
	"context"
	"reflect"
	"testing"
	"time"

	"platypus/internal/stores"
	"platypus/internal/types"
)

// clientStore is a store whose games start through its own client.
type clientStore struct{ command []string }

func (s clientStore) ID() string { return "clientstore" }
func (s clientStore) Library(context.Context) ([]types.Game, error) {
	return nil, nil
}
func (s clientStore) Details(context.Context, types.Game) (*types.GameDetails, error) {
	return nil, nil
}
func (s clientStore) Launch(context.Context, types.Game, bool) (*stores.Launch, error) {
	return &stores.Launch{Client: &stores.ClientLaunch{Client: "Steam", Command: s.command, StartTimeout: time.Second}}, nil
}

// A store-client launch runs the client's command as it is: the runner
// plugin doesn't wrap it, so no gamescope, GameMode or MangoHud goes around
// the client, and the plugin doesn't start for it (#74).
func TestStoreClientLaunchIsNotWrapped(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.ctx = context.Background()
	a.stores = stores.NewRegistry(nil, clientStore{command: []string{"steam", "-applaunch", "620"}})
	g := types.Game{ID: "clientstore-620", Source: "clientstore", StoreAppID: "620", Title: "Portal"}
	p, err := a.prepareLaunch(g, false)
	if err != nil {
		t.Fatalf("prepareLaunch: %v", err)
	}
	if want := []string{"steam", "-applaunch", "620"}; !reflect.DeepEqual(p.args, want) {
		t.Fatalf("args = %q, want the client command %q", p.args, want)
	}
	if p.client == nil {
		t.Fatal("the launch lost its client")
	}
	if len(p.env) != 0 || p.runner.Path != "" {
		t.Fatalf("client launch was wrapped: env=%v runner=%+v", p.env, p.runner)
	}
	if a.plugins.Running("runner") != 0 {
		t.Fatal("the runner plugin ran for a store-client launch")
	}
}
