package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/cloudsync"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/session"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
)

const echoCloudManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.2"]},"store":"echo","exec":"bin/hydra-echo-app",
	"requires":[{"bin":"sleep"}],
	"capabilities":["library","install","launch","auth","cloud-saves"]}`

type cloudEvents struct {
	mu   sync.Mutex
	list []types.CloudEvent
}

func (c *cloudEvents) emit(e types.CloudEvent) {
	c.mu.Lock()
	c.list = append(c.list, e)
	c.mu.Unlock()
}

func (c *cloudEvents) find(kind string) *types.CloudEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.list) - 1; i >= 0; i-- {
		if c.list[i].Kind == kind {
			e := c.list[i]
			return &e
		}
	}
	return nil
}

// cloudApp is an App wired to the real echo plugin and a real cloud orchestrator.
func cloudApp(t *testing.T, protocols []plugins.Version, env map[string]string) (*App, *cloudEvents) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	trust, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	installEchoApp(t, pluginsDir)
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(echoCloudManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{settings: trust, ctx: context.Background()}
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: trust,
		Protocols: protocols, OnNotify: a.pluginNotify,
	})
	t.Cleanup(a.plugins.Shutdown)
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	ev := &cloudEvents{}
	a.cloud = cloudsync.New(cloudsync.Options{
		Backend: cloudBackend{a}, Settings: trust, StatePath: filepath.Join(root, "cloud.json"),
		Emit: ev.emit, StableDelay: time.Millisecond,
	})
	t.Cleanup(func() { a.cloud.Shutdown(5 * time.Second) })
	a.sessions = session.NewTracker(func(types.SessionEvent) {})
	a.stores = stores.NewRegistry(a.pluginStores)
	return a, ev
}

func echoGame(t *testing.T) types.Game {
	return types.Game{ID: "p.echo.tiny", Source: "echo", Store: "echo", StoreAppID: "tiny", Title: "Echo Tiny",
		InstallDir: t.TempDir(), IsInstalled: true, Native: true}
}

func echoCalls(t *testing.T, a *App) map[string]int {
	t.Helper()
	var calls map[string]int
	if err := a.plugins.Call(context.Background(), "echo", "calls", nil, &calls); err != nil {
		t.Fatal(err)
	}
	return calls
}

func TestCloudGameNeedsThePluginTheCapabilityAndTheGameFeature(t *testing.T) {
	a, _ := cloudApp(t, nil, nil)
	g := echoGame(t)
	if _, err := a.plugins.Initialize(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.cloudGame(g); !ok {
		t.Fatal("an echo game should have cloud saves")
	}
	g.Features = []string{"install"}
	if _, ok := a.cloudGame(g); ok {
		t.Fatal("the game's features leave out cloud-saves")
	}
	for _, src := range []string{"", "steam", "custom", "no-such-plugin"} {
		other := echoGame(t)
		other.Source = src
		if _, ok := a.cloudGame(other); ok {
			t.Fatalf("source %q has no cloud saves", src)
		}
	}
	if info := a.GetCloudInfo(g); info.Mode != types.CloudOff {
		t.Fatalf("a game without cloud saves reports mode %q", info.Mode)
	}
}

func TestAProtocol11PluginGetsNoCloudCalls(t *testing.T) {
	a, _ := cloudApp(t, []plugins.Version{{Major: 1, Minor: 0}, {Major: 1, Minor: 1}}, nil) // an older Hydra: speaks up to 1.1
	g := echoGame(t)
	if _, err := a.plugins.Initialize(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if rt := a.plugins.Runtime("echo"); rt.Protocol != "1.1" || a.plugins.Has("echo", "cloud-saves") {
		t.Fatalf("runtime %+v", rt)
	}
	if _, ok := a.cloudGame(g); ok {
		t.Fatal("cloudGame accepted a plugin that settled on 1.1")
	}
	if _, err := a.CheckCloudSaves(g); err == nil {
		t.Fatal("Check worked for a 1.1 plugin")
	}
	p := &prepared{}
	if _, _, blocked := a.cloudBeforeLaunch(g, p); blocked {
		t.Fatal("a plugin without cloud saves blocked the launch")
	}
	if n := echoCalls(t, a); n["cloud.status"]+n["cloud.pull"]+n["cloud.push"]+n["cloud.resolve"] != 0 {
		t.Fatalf("cloud.* reached a 1.1 plugin: %v", n)
	}
}

func TestPreLaunchPullsThenTheExitPushes(t *testing.T) {
	a, ev := cloudApp(t, nil, map[string]string{"ECHO_CLOUD": "pending"})
	g := echoGame(t)
	push, msg, blocked := a.cloudBeforeLaunch(g, &prepared{})
	if blocked || push == nil {
		t.Fatalf("blocked %v (%s), push %v", blocked, msg, push)
	}
	info := a.GetCloudInfo(g)
	if info.Status == nil || info.Status.Counts.Download != 0 || info.Status.Counts.Upload != 1 || info.Status.State != types.CloudStatePending {
		t.Fatalf("after the pull the download should be done and the upload still owed: %+v", info.Status)
	}
	if calls := echoCalls(t, a); calls["cloud.pull"] != 1 {
		t.Fatalf("calls %v", calls)
	}

	// The game runs and exits; the exit callback pushes what is left.
	opts := push.options()
	if _, err := a.sessions.StartDirect(g.ID, exec.Command("true"), "true", nil, opts...); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for ev.find(types.CloudEvPushed) == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	e := ev.find(types.CloudEvPushed)
	if e == nil || e.Status == nil || e.Status.State != types.CloudStateSynced {
		t.Fatalf("no pushed event")
	}
	if got := a.cloud.PendingPushes(); len(got) != 0 {
		t.Fatalf("still owed: %+v", got)
	}
	if calls := echoCalls(t, a); calls["cloud.push"] != 1 {
		t.Fatalf("calls %v", calls)
	}
}

func TestPreLaunchConflictBlocksUntilResolved(t *testing.T) {
	a, ev := cloudApp(t, nil, map[string]string{"ECHO_CLOUD": "conflict2"})
	g := echoGame(t)
	push, msg, blocked := a.cloudBeforeLaunch(g, &prepared{})
	if !blocked || push != nil || msg == "" {
		t.Fatalf("a conflict must block the launch: blocked %v, %q", blocked, msg)
	}
	e := ev.find(types.CloudEvPrompt)
	if e == nil || e.Prompt != types.CloudPromptConflict || e.Status == nil || len(e.Status.Conflicts) != 2 {
		t.Fatalf("prompt %+v", e)
	}
	first := e.Status.Conflicts[0]
	if first.Local.Size == 0 || first.Remote.Machine == "" {
		t.Fatalf("the dialog needs both sides' sizes, dates and the other machine: %+v", first)
	}

	// Settle one file; the other stays a conflict and still blocks.
	info, err := a.ResolveCloudSaves(g, map[string]string{first.Path: "remote"})
	if err != nil || info.Status == nil || info.Status.Counts.Conflict != 1 {
		t.Fatalf("%+v, %v", info.Status, err)
	}
	if _, _, blocked := a.cloudBeforeLaunch(g, &prepared{}); !blocked {
		t.Fatal("one conflict left, the launch went through")
	}
	rest := info.Status.Conflicts[0].Path
	info, err = a.ResolveCloudSaves(g, map[string]string{rest: "local"})
	if err != nil || info.Status.State != types.CloudStatePending || info.Status.Counts.Upload != 1 {
		t.Fatalf("%+v, %v", info.Status, err)
	}
	if _, _, blocked := a.cloudBeforeLaunch(g, &prepared{}); blocked {
		t.Fatal("still blocked after every file was settled")
	}
	if info.Status.BackupDir == "" {
		t.Fatal("a cloud copy replaced a local file, but the backup folder isn't reported")
	}
}

func TestOfflineAsksAndLaunchAnywayLaunches(t *testing.T) {
	a, ev := cloudApp(t, nil, map[string]string{"ECHO_CLOUD": "offline"})
	g := echoGame(t)
	if _, msg, blocked := a.cloudBeforeLaunch(g, &prepared{}); !blocked || msg == "" {
		t.Fatalf("blocked %v %q", blocked, msg)
	}
	if e := ev.find(types.CloudEvPrompt); e == nil || e.Prompt != types.CloudPromptOffline {
		t.Fatalf("%+v", e)
	}
	// "Launch anyway" is the same call LaunchGame makes next.
	a.cloud.Allow(g.ID, true)
	if _, _, blocked := a.cloudBeforeLaunch(g, &prepared{}); blocked {
		t.Fatal("launch anyway didn't launch")
	}
}

func TestTheStoreClientRunningMeansNoSync(t *testing.T) {
	a, ev := cloudApp(t, nil, map[string]string{"ECHO_CLOUD": "pending", "ECHO_CLOUD_CLIENT": "1"})
	g := echoGame(t)
	_, _, blocked := a.cloudBeforeLaunch(g, &prepared{})
	e := ev.find(types.CloudEvPrompt)
	if !blocked || e == nil || e.Prompt != types.CloudPromptClient || e.Client != "Echo client" {
		t.Fatalf("blocked %v, %+v", blocked, e)
	}
	if calls := echoCalls(t, a); calls["cloud.pull"]+calls["cloud.push"] != 0 {
		t.Fatalf("synced while the client runs: %v", calls)
	}
	// The plugin can start the game through its client.
	s, _ := a.stores.Get("echo")
	launch, err := s.(stores.ClientLauncher).LaunchViaClient(context.Background(), g)
	if err != nil || launch.Client != "Echo client" || len(launch.Command) == 0 {
		t.Fatalf("%+v, %v", launch, err)
	}

	a2, ev2 := cloudApp(t, nil, map[string]string{"ECHO_CLOUD": "pending", "ECHO_CLOUD_CLIENT": "nolaunch"})
	if _, _, blocked := a2.cloudBeforeLaunch(g, &prepared{}); blocked {
		t.Fatal("a client that can't start the game should not block it")
	}
	if e := ev2.find(types.CloudEvSkipped); e == nil || e.Client == "" || e.Message == "" {
		t.Fatalf("the banner needs a reason: %+v", e)
	}
}

func TestCancelStopsAPullInEcho(t *testing.T) {
	a, ev := cloudApp(t, nil, map[string]string{"ECHO_CLOUD": "pending", "ECHO_CLOUD_DELAY_MS": "300"})
	g := echoGame(t)
	done := make(chan bool, 1)
	go func() {
		_, _, blocked := a.cloudBeforeLaunch(g, &prepared{})
		done <- blocked
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !a.CancelCloudSync(g.ID) {
		if time.Now().After(deadline) {
			t.Fatal("the pull never became cancelable")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case blocked := <-done:
		if !blocked {
			t.Fatal("a canceled pull still launched the game")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancel didn't end the launch step")
	}
	if ev.find(types.CloudEvCanceled) == nil {
		t.Fatal("no canceled event")
	}
	// Echo stopped: nothing was downloaded.
	info, err := a.CheckCloudSaves(g)
	if err != nil || info.Status.Counts.Download != 1 {
		t.Fatalf("%+v %v", info.Status, err)
	}
}

func TestProgressReachesTheEventsWhileEchoWorks(t *testing.T) {
	a, ev := cloudApp(t, nil, map[string]string{"ECHO_CLOUD": "pending", "ECHO_CLOUD_DELAY_MS": "130"})
	g := echoGame(t)
	if _, _, blocked := a.cloudBeforeLaunch(g, &prepared{}); blocked {
		t.Fatal("blocked")
	}
	e := ev.find(types.CloudEvProgress)
	if e == nil || e.GameID != g.ID || e.Phase == "" || e.Total == 0 {
		t.Fatalf("%+v", e)
	}
}

func TestCloudModesPerPluginAndPerGame(t *testing.T) {
	a, _ := cloudApp(t, nil, nil)
	g := echoGame(t)
	if _, err := a.plugins.Initialize(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if info := a.GetCloudInfo(g); info.Mode != types.CloudAsk || info.PluginMode != types.CloudAsk || info.GameMode != "" {
		t.Fatalf("defaults: %+v", info)
	}
	if err := a.SetCloudPluginMode("echo", types.CloudAuto); err != nil {
		t.Fatal(err)
	}
	if info := a.GetCloudInfo(g); info.Mode != types.CloudAuto {
		t.Fatalf("%+v", info)
	}
	if err := a.SetCloudGameMode(g.ID, types.CloudOff); err != nil {
		t.Fatal(err)
	}
	if info := a.GetCloudInfo(g); info.Mode != types.CloudOff || info.PluginMode != types.CloudAuto || info.GameMode != types.CloudOff {
		t.Fatalf("%+v", info)
	}
	if s := a.GetCloudSettings(); s.Plugins["echo"] != "auto" || s.Games[g.ID] != "off" {
		t.Fatalf("%+v", s)
	}
	// Off: no plugin call from the launch step.
	if _, _, blocked := a.cloudBeforeLaunch(g, &prepared{}); blocked {
		t.Fatal("blocked")
	}
	if calls := echoCalls(t, a); calls["cloud.status"] != 0 {
		t.Fatalf("off mode called the plugin: %v", calls)
	}
	if err := a.SetCloudGameMode(g.ID, "sometimes"); err == nil {
		t.Fatal("accepted a nonsense mode")
	}
}
