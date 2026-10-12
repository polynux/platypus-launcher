package app

import (
	"context"
	"strings"
	"testing"

	"platypus/internal/stores"
	"platypus/internal/types"
)

// A game the store lists as installed, with no folder Platypus can name (a
// store client, a borrowed copy the plugin keeps elsewhere), is installed for
// every feature that reads its state: the folder comes from the library, and
// when there is none the refusal says so instead of "not installed".

// libraryGame lists echo's library the way the app does (relocations applied)
// and returns the game with the given store id.
func (e *versionsEnv) libraryGame(t *testing.T, storeID string) types.Game {
	t.Helper()
	if err := e.m.Call(context.Background(), "echo", "auth.complete", map[string]any{"fields": map[string]string{"code": "letmein"}}, nil); err != nil {
		t.Fatal(err)
	}
	p, _ := e.m.Plugin("echo")
	s := stores.NewPlugin(e.m, p, func() []string { return e.m.Folders("echo", e.a.settings.GamesFolder()) }).
		WithRelocations(e.a.settings.Relocation)
	lib, err := s.Library(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range lib {
		if g.StoreAppID == storeID {
			return g
		}
	}
	t.Fatalf("echo has no %s game", storeID)
	return types.Game{}
}

// noFolderGame is an echo game the library lists as installed with no folder.
func (e *versionsEnv) noFolderGame(t *testing.T) types.Game {
	t.Helper()
	g := e.libraryGame(t, "client")
	if !g.IsInstalled || g.InstallDir != "" {
		t.Fatalf("precondition: the library lists %s installed=%v in %q", g.Title, g.IsInstalled, g.InstallDir)
	}
	return g
}

func TestVerifyMoveAndUninstallOfAGameWithNoKnownFolderSayItIsInstalled(t *testing.T) {
	t.Parallel()
	e := maintenanceEnv(t, protocol14, maintenanceCaps)
	g := e.noFolderGame(t)

	if _, err := e.a.VerifyGame(g); err == nil {
		t.Fatal("a check ran in no folder")
	} else if strings.Contains(err.Error(), "not installed") {
		t.Fatalf("verify says %q, but the library lists the game installed", err)
	}
	if _, err := e.a.CheckMove(g, t.TempDir()); err == nil {
		t.Fatal("a move ran from no folder")
	} else if strings.Contains(err.Error(), "not installed") {
		t.Fatalf("move says %q, but the library lists the game installed", err)
	}
	if _, err := e.a.UninstallGame(g); err == nil {
		t.Fatal("an uninstall ran with no folder")
	} else if strings.Contains(err.Error(), "not installed") {
		t.Fatalf("uninstall says %q, but the library lists the game installed", err)
	}
}

func TestCloudSavesOfAGameWithNoKnownFolderAreRefusedWithoutAPluginCall(t *testing.T) {
	t.Parallel()
	a, _ := cloudApp(t, nil, nil)
	if err := a.plugins.Call(context.Background(), "echo", "auth.complete", map[string]any{"fields": map[string]string{"code": "letmein"}}, nil); err != nil {
		t.Fatal(err)
	}
	game := types.Game{ID: "p.echo.ana", Source: "echo", Store: "echo", StoreAppID: "ana", Title: "Echo Borrowed Quest",
		IsInstalled: true, Native: true}

	_, err := a.CheckCloudSaves(game)
	if err == nil {
		t.Fatal("cloud saves were checked in no folder")
	}
	if strings.Contains(err.Error(), "invalid params") || strings.Contains(err.Error(), "ctx") {
		t.Fatalf("cloud saves refused with the plugin's wording: %v", err)
	}
	if calls := echoCalls(t, a); calls["cloud.status"] != 0 {
		t.Fatalf("calls %v: the plugin was asked about saves of a game with no folder", calls)
	}
}
