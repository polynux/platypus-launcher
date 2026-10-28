package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/cloudsync"
	"platypus/internal/runner"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// conflictBackend is a cloud backend stuck on one status: Check remembers it
// without any plugin.
type conflictBackend struct{ status types.CloudStatus }

func (b conflictBackend) Offers(context.Context, string) bool { return true }

func (b conflictBackend) Call(_ context.Context, _ string, _ string, _ any, result any) error {
	if st, ok := result.(*types.CloudStatus); ok {
		*st = b.status
	}
	return nil
}

func attentionIntp(i int) *int { return &i }

// Issue #144: the assembled library flags the signals the frontend can't
// derive: save conflicts known this session and damaged Wine prefixes.
func TestLibraryFlagsSaveConflictsAndDamagedPrefixes(t *testing.T) {
	t.Parallel()
	played := int64(1700000000)
	localMin := attentionIntp(30)
	wine := func(id string, mutate func(*types.Game)) types.Game {
		g := types.Game{ID: id, Source: "echo", Store: "echo", StoreAppID: id, Title: id, IsInstalled: true}
		if mutate != nil {
			mutate(&g)
		}
		return g
	}
	src := &fakeSource{id: "echo", games: []types.Game{
		wine("conflict", nil),
		wine("broken", nil),
		wine("healthy", nil),
		wine("missing-played", func(g *types.Game) { g.LastPlayed = played }),
		wine("missing-played-local", func(g *types.Game) { g.PlaytimeLocalMin = localMin }),
		wine("missing-new", nil),
		wine("native", func(g *types.Game) { g.Native = true; g.LastPlayed = played }),
		wine("client", func(g *types.Game) { g.ClientLaunch = true; g.LastPlayed = played }),
		wine("notinstalled", func(g *types.Game) { g.IsInstalled = false }),
		// A sideloaded game forced to native never has a prefix to break.
		func() types.Game {
			g := wine("custom-native", func(g *types.Game) {
				g.Source = "custom"
				g.LastPlayed = played
				g.Settings.RunMode = "native"
			})
			return g
		}(),
	}}
	a := libraryApp(t, src)
	home := t.TempDir()
	a.optHome = home
	prefixOf := func(id string) string {
		for _, g := range src.games {
			if g.ID == id {
				return runner.PrefixPath(g, home, a.effectivePrefixesFolder())
			}
		}
		t.Fatalf("no game %q", id)
		return ""
	}
	mkdirs := func(path string) {
		t.Helper()
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Healthy prefixes: a plain Wine layout and a Proton pfx layout.
	mkdirs(filepath.Join(prefixOf("conflict"), "drive_c"))
	mkdirs(filepath.Join(prefixOf("healthy"), "pfx", "drive_c"))
	// Broken prefixes: created but unfinished, or blocked by a file.
	mkdirs(prefixOf("broken"))
	mkdirs(prefixOf("notinstalled"))
	if err := os.WriteFile(prefixOf("custom-native"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	a.cloud = cloudsync.New(cloudsync.Options{
		Backend: conflictBackend{status: types.CloudStatus{
			State:     types.CloudStateConflict,
			Counts:    types.CloudCounts{Conflict: 1},
			Conflicts: []types.CloudConflict{{Path: "save.dat"}},
		}},
		Settings: a.settings,
	})
	t.Cleanup(func() { a.cloud.Shutdown(0) })
	if _, err := a.cloud.Check(context.Background(),
		cloudsync.Game{ID: "conflict", PluginID: "echo", StoreID: "conflict"},
		types.CloudContext{}); err != nil {
		t.Fatal(err)
	}

	lib, err := a.assembleLibrary([]stores.Store{src}, [][]types.Game{src.games}, []error{nil}, []bool{true})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]types.Game{}
	for _, g := range lib.Games {
		byID[g.ID] = g
	}
	conflict, broken, healthy, missingPlayed, missingPlayedLocal, missingNew, native, client, notinstalled, customNative :=
		byID["conflict"], byID["broken"], byID["healthy"], byID["missing-played"], byID["missing-played-local"],
		byID["missing-new"], byID["native"], byID["client"], byID["notinstalled"], byID["custom-native"]
	if !conflict.HasSaveConflict || conflict.PrefixDamaged {
		t.Fatalf("conflict = %+v, want the save flag only", conflict)
	}
	if broken.HasSaveConflict || !broken.PrefixDamaged {
		t.Fatalf("broken = %+v, want the prefix flag only", broken)
	}
	if healthy.HasSaveConflict || healthy.PrefixDamaged {
		t.Fatalf("healthy = %+v, want no flags", healthy)
	}
	if missingPlayed.HasSaveConflict || !missingPlayed.PrefixDamaged {
		t.Fatalf("missing-played = %+v, want the prefix flag: it ran before", missingPlayed)
	}
	if missingPlayedLocal.HasSaveConflict || !missingPlayedLocal.PrefixDamaged {
		t.Fatalf("missing-played-local = %+v, want the prefix flag: local playtime counts as ran", missingPlayedLocal)
	}
	for name, g := range map[string]types.Game{
		"missing-new": missingNew, "native": native, "client": client,
		"notinstalled": notinstalled, "custom-native": customNative,
	} {
		if g.HasSaveConflict || g.PrefixDamaged {
			t.Fatalf("%s = %+v, want no flags", name, g)
		}
	}
}

// Issue #144: libraries saved before the signals load with both unset.
func TestOldLibrarySnapshotsLoadWithoutAttentionSignals(t *testing.T) {
	t.Parallel()
	var games []types.Game
	if err := json.Unmarshal([]byte(`[{"id":"g","source":"echo","title":"G","is_installed":true}]`), &games); err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].HasSaveConflict || games[0].PrefixDamaged {
		t.Fatalf("old snapshot = %+v, want both signals unset", games)
	}
}
