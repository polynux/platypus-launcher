package app

// Offline launches (issue #44): nothing the core does blocks a launch when
// there is no network. The core test launches a sideloaded game and an echo
// game with every outbound request failing, and asserts no outbound request
// happens between LaunchGame and the process start.

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/cloudsync"
	"platypus/internal/jobs"
	"platypus/internal/netstate"
	"platypus/internal/onlineart"
	"platypus/internal/plugins"
	"platypus/internal/session"
	"platypus/internal/settings"
	"platypus/internal/steam"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// offlineApp builds an App with echo installed (fake-installed tiny with a
// real game.sh inside echo's install folder), logged in, and the offline
// tracker marked offline. Every HTTP client fails with a dial error and
// counts its requests in hits.
func offlineApp(t *testing.T, hits *atomic.Int64) (*App, types.Game) {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()
	trust, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	gamesFolder := filepath.Join(root, "Games")
	if _, err := trust.SetGamesFolder(gamesFolder); err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	installEchoApp(t, pluginsDir)
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(echoCloudManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(gamesFolder, "echo", "tiny")
	mustDir(t, installed)
	if err := os.WriteFile(filepath.Join(installed, "game.sh"), []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"loggedIn":true,"installed":{"tiny":` + quote(t, installed) + `}}`
	mustDir(t, filepath.Join(root, "data", "echo"))
	if err := os.WriteFile(filepath.Join(root, "data", "echo", "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	fail := &http.Client{Transport: roundTripFail(hits)}
	a := &App{settings: trust, ctx: context.Background(), optHome: home}
	a.net = netstate.New()
	a.net.SetClientConnected(true)
	a.net.ObserveError(&net.OpError{Op: "dial", Err: errors.New("no route to host")}, "127.0.0.1:9")
	if a.net.Online() {
		t.Fatal("test setup didn't mark offline")
	}
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: trust,
		Home: home, OnNotify: a.pluginNotify,
	})
	t.Cleanup(a.plugins.Shutdown)
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	// Every outbound client fails: the launch must not need any of them.
	a.artwork = &onlineart.Client{HTTP: fail}
	a.sideloadSteam = &steam.StoreClient{HTTPClient: fail}
	q, err := jobs.Open(filepath.Join(root, "jobs.json"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	a.jobs = q
	a.cloud = cloudsync.New(cloudsync.Options{
		Backend: cloudBackend{a}, Settings: trust, StatePath: filepath.Join(root, "cloud.json"),
	})
	t.Cleanup(func() { a.cloud.Shutdown(5 * time.Second) })
	a.sessions = session.NewTracker(func(types.SessionEvent) {})
	a.sessions.SetLogDir(filepath.Join(root, "logs"))
	a.stores = stores.NewRegistry(a.pluginStores)
	g := echoGame(t, a)
	return a, g
}

func quote(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type roundTripFailer struct{ hits *atomic.Int64 }

func roundTripFail(hits *atomic.Int64) http.RoundTripper { return roundTripFailer{hits} }

func (f roundTripFailer) RoundTrip(r *http.Request) (*http.Response, error) {
	f.hits.Add(1)
	return nil, &net.OpError{Op: "dial", Err: errors.New("simulated offline")}
}

func TestCloudSkippedWhileOffline(t *testing.T) {
	t.Parallel()
	a, _ := cloudApp(t, nil, map[string]string{"ECHO_CLOUD": "pending"})
	a.net = netstate.New()
	a.net.ObserveError(&net.OpError{Op: "dial", Err: errors.New("no route")}, "127.0.0.1:9")
	g := echoGame(t, a)
	push, msg, blocked := a.cloudBeforeLaunch(g, &prepared{}, false)
	if blocked || push != nil {
		t.Fatalf("blocked %v, push %v: offline must skip the cloud step", blocked, push)
	}
	if msg != "cloud saves not checked: offline" {
		t.Fatalf("note = %q", msg)
	}
	if calls := echoCalls(t, a); calls["cloud.status"] != 0 {
		t.Fatalf("cloud.status reached the plugin while offline: %v", calls)
	}
}

func TestLaunchEchoGameWhileOffline(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	a, g := offlineApp(t, &hits)
	msg, err := a.LaunchGame(g)
	if err != nil {
		t.Fatalf("echo launch while offline: %v", err)
	}
	t.Cleanup(func() { _ = a.sessions.Stop(g.ID, true) })
	if !strings.Contains(msg, "cloud saves not checked: offline") {
		t.Fatalf("launch note = %q, want the offline note", msg)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("%d outbound HTTP requests during an offline launch", n)
	}
}

func TestLaunchSideloadedGameWhileOffline(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	a, _ := offlineApp(t, &hits)
	exe := filepath.Join(t.TempDir(), "game.sh")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	custom, err := a.settings.AddCustomGame("Offline Native", exe)
	if err != nil {
		t.Fatal(err)
	}
	var game types.Game
	for _, g := range a.customGames() {
		if g.ID == custom.ID {
			game = g
		}
	}
	if game.ID == "" {
		t.Fatal("sideloaded game not listed")
	}
	before := hits.Load()
	if _, err := a.LaunchGame(game); err != nil {
		t.Fatalf("sideloaded launch while offline: %v", err)
	}
	t.Cleanup(func() { _ = a.sessions.Stop(game.ID, true) })
	if n := hits.Load() - before; n != 0 {
		t.Fatalf("%d outbound HTTP requests during a sideloaded offline launch", n)
	}
}

func TestLaunchBoundWhileOnline(t *testing.T) {
	t.Parallel()
	a, _ := cloudApp(t, nil, map[string]string{"ECHO_CLOUD": "pending"})
	a.net = netstate.New() // online: the check runs, bounded by cloudLaunchTimeout
	g := echoGame(t, a)
	start := time.Now()
	_, _, blocked := a.cloudBeforeLaunch(g, &prepared{}, false)
	if blocked {
		t.Fatal("a pending cloud must sync, not block")
	}
	if time.Since(start) > cloudLaunchTimeout+10*time.Second {
		t.Fatal("the pre-launch cloud check ran past its bound")
	}
}
