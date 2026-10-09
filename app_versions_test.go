package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/ledger"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
)

// A whole App around the echo test plugin: settings, install queue, ledger and
// plugin manager wired the way startup wires them. Echo's "tiny" game installs
// in milliseconds when ECHO_STEP_MS is small.

type versionsEnv struct {
	a    *App
	m    *plugins.Manager
	root string
	game types.Game
}

const versionsManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":%s},"store":"echo","exec":"bin/hydra-echo-app",
	"capabilities":%s}`

func versionsApp(t *testing.T, protocols, caps string) *versionsEnv {
	t.Helper()
	t.Setenv("ECHO_STEP_MS", "2")
	root := t.TempDir()
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetGamesFolder(filepath.Join(root, "Games")); err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, ctx: context.Background()}
	if a.ledger, err = ledger.Open(filepath.Join(root, "install-history.json")); err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st,
		OnNotify: a.pluginNotify,
	})
	t.Cleanup(a.plugins.Shutdown)
	installEchoApp(t, pluginsDir)
	manifest := fmt.Sprintf(versionsManifest, protocols, caps)
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	a.jobs, err = jobs.Open(filepath.Join(root, "jobs.json"), jobBackend{a}, nil)
	if err != nil {
		t.Fatal(err)
	}
	a.jobs.SetOnInstalled(a.recordInstall)
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.jobs.Run(runCtx) }()
	t.Cleanup(func() { cancel(); <-done })
	if _, err := a.plugins.Initialize(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	// The version gate reads the game from the store's library, which needs a login.
	a.stores = stores.NewRegistry(a.pluginStores)
	if err := a.plugins.Call(context.Background(), "echo", "auth.complete", map[string]any{"fields": map[string]string{"code": "letmein"}}, nil); err != nil {
		t.Fatal(err)
	}
	return &versionsEnv{a: a, m: a.plugins, root: root, game: types.Game{
		ID: stores.GameID("echo", "tiny"), Source: "echo", Store: "echo", StoreAppID: "tiny", Title: "Echo Tiny", Installable: true,
	}}
}

const allVersions = `["1.0","1.1","1.3"]`
const echoCaps = `["library","install","auth","discard","versions"]`

func (e *versionsEnv) waitJob(t *testing.T, id, state string) types.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range e.a.jobs.List() {
			if j.ID == id && (j.State == state || j.State == jobs.StateFailed) {
				if j.State != state {
					t.Fatalf("job %s ended %s: %s", id, j.State, j.Error)
				}
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never reached %s: %+v", id, state, e.a.jobs.List())
	return types.Job{}
}

func (e *versionsEnv) install(t *testing.T, target *types.VersionTarget) types.Job {
	t.Helper()
	var j types.Job
	var err error
	if target == nil {
		j, err = e.a.InstallGame(e.game)
	} else {
		j, err = e.a.InstallGameVersion(e.game, *target)
	}
	if err != nil {
		t.Fatal(err)
	}
	e.game.IsInstalled = true
	return e.waitJob(t, j.ID, jobs.StateDone)
}

func (e *versionsEnv) switchTo(t *testing.T, target types.VersionTarget) types.Job {
	t.Helper()
	j, err := e.a.UpdateGameVersion(e.game, target)
	if err != nil {
		t.Fatal(err)
	}
	return e.waitJob(t, j.ID, jobs.StateDone)
}

func (e *versionsEnv) calls(t *testing.T) map[string]int {
	t.Helper()
	var calls map[string]int
	if err := e.m.Call(context.Background(), "echo", "calls", nil, &calls); err != nil {
		t.Fatal(err)
	}
	return calls
}

func (e *versionsEnv) echoLibrary(t *testing.T) types.Game {
	t.Helper()
	// echo wants a login before it lists its library
	if err := e.m.Call(context.Background(), "echo", "auth.complete", map[string]any{"fields": map[string]string{"code": "letmein"}}, nil); err != nil {
		t.Fatal(err)
	}
	p, _ := e.m.Plugin("echo")
	s := stores.NewPlugin(e.m, p, func() []string { return e.m.Folders("echo", e.a.settings.GamesFolder()) })
	lib, err := s.Library(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range lib {
		if g.StoreAppID == "tiny" {
			return g
		}
	}
	t.Fatal("echo has no tiny game")
	return types.Game{}
}

func TestInstallingATargetFillsTheLedgerAndTheVersionList(t *testing.T) {
	e := versionsApp(t, allVersions, echoCaps)
	job := e.install(t, &types.VersionTarget{Build: "b200"})
	if job.Target == nil || job.Target.Build != "b200" {
		t.Fatalf("job target = %+v", job.Target)
	}
	e.switchTo(t, types.VersionTarget{Manual: map[string]string{"build": "b777"}})

	history := e.a.ledger.Entries(e.game.ID)
	if len(history) != 2 || history[0].Build != "b200" || history[0].VersionName != "2.0.0" || history[1].Build != "b777" {
		t.Fatalf("ledger = %+v", history)
	}
	if history[0].Store != "echo" || history[0].Game != e.game.ID || history[0].Date == "" || len(history[0].ArchiveRef) == 0 {
		t.Fatalf("ledger entry misses fields: %+v", history[0])
	}

	info, err := e.a.GetGameVersions(e.game)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Channels) != 3 || len(info.Ledger) != 2 || info.Pinned != nil {
		t.Fatalf("version list = %+v", info)
	}
	sources := map[string]string{}
	var current string
	for _, b := range info.Builds {
		sources[b.ID] = b.Source
		if b.Current {
			current = b.ID
		}
		if b.Available == nil {
			t.Errorf("build %s: availability not resolved", b.ID)
		}
	}
	if sources["b300"] != "store" || sources["b777"] != "archive" {
		t.Fatalf("build sources = %v: a typed build comes back from the history as an archive build", sources)
	}
	if current != "b777" {
		t.Fatalf("current build = %q, want b777", current)
	}
	if info.Manual == nil || info.Manual.Label == "" || len(info.Links) != 1 {
		t.Fatalf("manual %+v links %+v", info.Manual, info.Links)
	}
	if !info.UpdateAvailable {
		t.Fatal("b777 is not the latest build")
	}
}

func TestPinBecomesTheTargetOfAnUpdate(t *testing.T) {
	e := versionsApp(t, allVersions, echoCaps)
	e.install(t, nil) // the store's default: b300
	if err := e.a.PinGameVersion(e.game, types.VersionPin{Kind: types.PinBuild, ID: "b200"}); err != nil {
		t.Fatal(err)
	}
	j, err := e.a.UpdateGame(e.game)
	if err != nil {
		t.Fatal(err)
	}
	if j.Target == nil || j.Target.Build != "b200" || j.Target.Channel != "" {
		t.Fatalf("the update of a pinned game should target its pin, got %+v", j.Target)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if h := e.a.ledger.Entries(e.game.ID); h[len(h)-1].Build != "b200" {
		t.Fatalf("the pinned update installed %+v", h)
	}

	// A channel pin targets the channel.
	if err := e.a.PinGameVersion(e.game, types.VersionPin{Kind: types.PinChannel, ID: "legacy"}); err != nil {
		t.Fatal(err)
	}
	j, err = e.a.UpdateGame(e.game)
	if err != nil {
		t.Fatal(err)
	}
	if j.Target == nil || j.Target.Channel != "legacy" || j.Target.Build != "" {
		t.Fatalf("channel pin target = %+v", j.Target)
	}
	e.waitJob(t, j.ID, jobs.StateDone)

	// Unpinned, an update asks for the store's default again.
	if err := e.a.UnpinGameVersion(e.game.ID); err != nil {
		t.Fatal(err)
	}
	j, err = e.a.UpdateGame(e.game)
	if err != nil {
		t.Fatal(err)
	}
	if j.Target != nil {
		t.Fatalf("an unpinned update carries no target: %+v", j.Target)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if h := e.a.ledger.Entries(e.game.ID); h[len(h)-1].Build != "b300" {
		t.Fatalf("return to latest installed %+v", h)
	}
}

func TestReinstallingAPinnedGameInstallsThePin(t *testing.T) {
	e := versionsApp(t, allVersions, echoCaps)
	if err := e.a.PinGameVersion(e.game, types.VersionPin{Kind: types.PinBuild, ID: "b290"}); err != nil {
		t.Fatal(err)
	}
	j := e.install(t, nil)
	if j.Target == nil || j.Target.Build != "b290" {
		t.Fatalf("install of a pinned game: target %+v", j.Target)
	}
}

func TestUpdatePromptIsSuppressedWhilePinned(t *testing.T) {
	e := versionsApp(t, allVersions, echoCaps)
	e.install(t, &types.VersionTarget{Build: "b200"})
	g := e.echoLibrary(t)
	if !g.UpdateAvailable {
		t.Fatalf("echo says an older build has an update: %+v", g)
	}
	lib := types.Library{Games: []types.Game{g, {ID: "other", UpdateAvailable: true}}}

	e.a.applyPins(&lib)
	if !lib.Games[0].UpdateAvailable || lib.Games[0].Pinned != nil {
		t.Fatalf("an unpinned game keeps its prompt: %+v", lib.Games[0])
	}

	if err := e.a.PinGameVersion(g, types.VersionPin{Kind: types.PinBuild, ID: "b200"}); err != nil {
		t.Fatal(err)
	}
	lib = types.Library{Games: []types.Game{g, {ID: "other", UpdateAvailable: true}}}
	e.a.applyPins(&lib)
	if lib.Games[0].UpdateAvailable {
		t.Fatal("update prompt still on for a pinned game")
	}
	if p := lib.Games[0].Pinned; p == nil || p.ID != "b200" || p.Kind != types.PinBuild {
		t.Fatalf("pinned game isn't marked: %+v", lib.Games[0].Pinned)
	}
	if !lib.Games[1].UpdateAvailable || lib.Games[1].Pinned != nil {
		t.Fatal("another game was touched by the pin")
	}

	// The plugin's own verdict, relative to the pin, says nothing to update.
	info, err := e.a.GetGameVersions(g)
	if err != nil {
		t.Fatal(err)
	}
	if info.UpdateAvailable || info.Pinned == nil || info.Pinned.ID != "b200" {
		t.Fatalf("versions relative to the pin: update %v pinned %+v", info.UpdateAvailable, info.Pinned)
	}
}

func TestPinSurvivesARestart(t *testing.T) {
	e := versionsApp(t, allVersions, echoCaps)
	if err := e.a.PinGameVersion(e.game, types.VersionPin{Kind: types.PinChannel, ID: "legacy"}); err != nil {
		t.Fatal(err)
	}
	again, err := settings.Open(filepath.Join(e.root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := again.Pin(e.game.ID); !ok || p.ID != "legacy" {
		t.Fatalf("pin after reopening settings: %+v %v", p, ok)
	}
}

func TestPasswordsNeverReachCoreFiles(t *testing.T) {
	const wrong, right = "not-the-password-hunter2", "hydra-beta" // echo's fixed test password
	e := versionsApp(t, allVersions, echoCaps)

	// Locked until unlocked: the install is refused with a message.
	j, err := e.a.InstallGameVersion(e.game, types.VersionTarget{Channel: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	failed := e.waitJobEnd(t, j.ID)
	if failed.State != jobs.StateFailed || !strings.Contains(failed.Error, "locked") {
		t.Fatalf("a locked channel should fail the install with its reason: %+v", failed)
	}
	if err := e.a.jobs.Discard(context.Background(), j.ID); err == nil {
		t.Log("(failed jobs aren't discardable; fine)")
	}
	e.a.ClearFinishedJobs()

	ok, err := e.a.UnlockVersionChannel(e.game, "beta", wrong)
	if err != nil || ok {
		t.Fatalf("wrong password: unlocked %v, err %v", ok, err)
	}
	if ok, err := e.a.UnlockVersionChannel(e.game, "beta", right); err != nil || !ok {
		t.Fatalf("right password: unlocked %v, err %v", ok, err)
	}
	info, err := e.a.GetGameVersions(e.game)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range info.Channels {
		if c.ID == "beta" && (!c.Unlocked || c.Requires != "password") {
			t.Fatalf("beta after unlock: %+v", c)
		}
	}
	job := e.install(t, &types.VersionTarget{Channel: "beta"})
	if job.Target == nil || job.Target.Channel != "beta" {
		t.Fatalf("target = %+v", job.Target)
	}
	if err := e.a.PinGameVersion(e.game, types.VersionPin{Kind: types.PinChannel, ID: "beta"}); err != nil {
		t.Fatal(err)
	}

	// Nothing Hydra wrote holds a password: not the queue, the settings, the
	// install history, nor its logs.
	var files []string
	filepath.WalkDir(e.root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.Contains(path, string(filepath.Separator)+"data"+string(filepath.Separator)) && !strings.Contains(path, string(filepath.Separator)+"plugins"+string(filepath.Separator)) {
			files = append(files, path)
		}
		return nil
	})
	seen := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		seen++
		for _, secret := range []string{wrong, right} {
			if strings.Contains(string(raw), secret) {
				t.Errorf("%s contains a password", f)
			}
		}
	}
	if seen < 3 {
		t.Fatalf("expected settings, jobs and history to be checked, saw %v", files)
	}
}

func (e *versionsEnv) waitJobEnd(t *testing.T, id string) types.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range e.a.jobs.List() {
			if j.ID == id && (j.State == jobs.StateFailed || j.State == jobs.StateDone) {
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never ended", id)
	return types.Job{}
}

func TestNoVersionsCallsWithoutTheCapability(t *testing.T) {
	e := versionsApp(t, allVersions, `["library","install","auth","discard"]`)
	if _, err := e.a.GetGameVersions(e.game); err == nil {
		t.Fatal("versions listed for a plugin that doesn't declare them")
	}
	if _, err := e.a.UnlockVersionChannel(e.game, "beta", "x"); err == nil {
		t.Fatal("unlock sent to a plugin without versions")
	}
	if _, err := e.a.InstallGameVersion(e.game, types.VersionTarget{Build: "b200"}); err == nil {
		t.Fatal("a version install was queued for a plugin without versions")
	}
	if err := e.a.PinGameVersion(e.game, types.VersionPin{Kind: "build", ID: "b1"}); err == nil {
		t.Fatal("pinned a game whose plugin has no versions")
	}
	if len(e.a.jobs.List()) != 0 {
		t.Fatalf("jobs queued: %+v", e.a.jobs.List())
	}
	// The plain install still works and reports an `installed` echo ignores.
	e.install(t, nil)
	if h := e.a.ledger.Entries(e.game.ID); len(h) != 0 {
		t.Fatalf("a plugin without versions fed the ledger: %+v", h)
	}
	for _, m := range []string{"versions.list", "versions.unlock"} {
		if n := e.calls(t)[m]; n != 0 {
			t.Errorf("%s reached the plugin %d times", m, n)
		}
	}
}

func TestNoVersionsCallsOnAnOldProtocol(t *testing.T) {
	// The manifest declares versions, but the plugin only agreed to 1.1: the
	// capability is newer than that, so it is off and nothing is sent.
	e := versionsApp(t, `["1.0","1.1"]`, echoCaps)
	if e.m.Has("echo", "versions") {
		t.Fatalf("versions on over protocol %s", e.m.Runtime("echo").Protocol)
	}
	if _, err := e.a.GetGameVersions(e.game); err == nil {
		t.Fatal("versions listed over protocol 1.1")
	}
	if _, err := e.a.InstallGameVersion(e.game, types.VersionTarget{Build: "b200"}); err == nil {
		t.Fatal("version install queued over protocol 1.1")
	}
	if n := e.calls(t)["versions.list"]; n != 0 {
		t.Fatalf("versions.list reached the plugin %d times", n)
	}
	// The pin can still be removed.
	if err := e.a.settings.SetPin(e.game.ID, types.VersionPin{Kind: "build", ID: "b1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.UpdateGame(types.Game{ID: e.game.ID, Source: "echo", StoreAppID: "tiny", Title: "Echo Tiny", IsInstalled: true}); err == nil {
		t.Fatal("a pinned game was updated to the latest because versions went away")
	}
	if err := e.a.UnpinGameVersion(e.game.ID); err != nil {
		t.Fatal(err)
	}
}

func TestVersionsHiddenWhenTheGameTurnsThemOff(t *testing.T) {
	t.Setenv("ECHO_GAME_FEATURES", `{"tiny":["install"]}`)
	e := versionsApp(t, allVersions, echoCaps)
	g := e.echoLibrary(t)
	if len(g.Features) != 1 || g.Features[0] != "install" {
		t.Fatalf("echo's per-game features = %v", g.Features)
	}
	if _, err := e.a.GetGameVersions(g); err == nil {
		t.Fatal("versions listed for a game that turned them off")
	}
	if _, err := e.a.InstallGameVersion(g, types.VersionTarget{Build: "b200"}); err == nil {
		t.Fatal("version install queued for a game that turned them off")
	}
	if n := e.calls(t)["versions.list"]; n != 0 {
		t.Fatalf("versions.list reached the plugin %d times", n)
	}
}

func TestBadTargetsAreRefusedBeforeTheQueue(t *testing.T) {
	e := versionsApp(t, allVersions, echoCaps)
	long := strings.Repeat("x", maxTargetField+1)
	bad := map[string]types.VersionTarget{
		"empty":         {},
		"control chars": {Build: "b1\nb2"},
		"long channel":  {Channel: long},
		"empty value":   {Manual: map[string]string{"build": ""}},
		"long value":    {Manual: map[string]string{"build": strings.Repeat("x", maxManualValue+1)}},
		"blank value":   {Manual: map[string]string{"input": " \n\t "}},
	}
	many := map[string]string{}
	for i := 0; i < maxManualKeys+1; i++ {
		many[string(rune('a'+i))] = "v"
	}
	bad["too many"] = types.VersionTarget{Manual: many}
	for name, target := range bad {
		if _, err := e.a.InstallGameVersion(e.game, target); err == nil {
			t.Errorf("%s: queued", name)
		}
	}
	if len(e.a.jobs.List()) != 0 {
		t.Fatalf("jobs queued for bad targets: %+v", e.a.jobs.List())
	}
	if _, err := e.a.UpdateGameVersion(e.game, types.VersionTarget{Build: "b1"}); err == nil {
		t.Fatal("updated a game that isn't installed")
	}
}

func TestNormalizeVersionsCleansWhatThePluginSent(t *testing.T) {
	no := false
	in := types.VersionInfo{
		Channels: []types.VersionChannel{{ID: "a", Requires: "carrier pigeon"}, {ID: ""}},
		Builds: []types.VersionBuild{
			{ID: "b1", Source: "rumour"}, {ID: "b2", Source: "archive", Available: &no}, {ID: ""},
		},
		Links: []types.VersionLink{
			{Label: "ok", URL: "https://steamdb.info/app/480/depots/"},
			{URL: "javascript:alert(1)"}, {URL: "file:///etc/passwd"}, {URL: "/relative"},
			{URL: "http://example.com/x"},
		},
		Manual: &types.VersionManual{},
	}
	out := normalizeVersions(in)
	if len(out.Channels) != 1 || out.Channels[0].Requires != "none" || out.Channels[0].Name != "a" {
		t.Errorf("channels = %+v", out.Channels)
	}
	if len(out.Builds) != 2 || out.Builds[0].Source != "store" || !*out.Builds[0].Available || *out.Builds[1].Available {
		t.Errorf("builds = %+v", out.Builds)
	}
	if len(out.Links) != 2 || out.Links[0].URL != "https://steamdb.info/app/480/depots/" || out.Links[1].Label != "http://example.com/x" {
		t.Errorf("links = %+v", out.Links)
	}
	if out.Manual != nil {
		t.Errorf("a manual hint without a label is dropped: %+v", out.Manual)
	}
	empty := normalizeVersions(types.VersionInfo{})
	if empty.Channels == nil || empty.Builds == nil || empty.Links == nil {
		t.Errorf("empty lists must not be null for the UI: %+v", empty)
	}
}

func TestInstallParamsCarryTheTarget(t *testing.T) {
	j := types.Job{ID: "j", Kind: jobs.KindUpdate, StoreGameID: "tiny", InstallDir: "/g", Target: &types.VersionTarget{Build: "b2"}}
	params := installParams(j)
	got, ok := params["target"].(*types.VersionTarget)
	if !ok || got.Build != "b2" {
		t.Fatalf("target missing from game.update params: %v", params)
	}
	delete(params, "target")
	j.Target = nil
	if _, has := installParams(j)["target"]; has {
		t.Fatal("a job without a target sent one")
	}
	j.Kind, j.Target = jobs.KindUninstall, &types.VersionTarget{Build: "b2"}
	if _, has := installParams(j)["target"]; has {
		t.Fatal("an uninstall carries no target")
	}
}

func TestTypedManifestIdsMayRunOverLines(t *testing.T) {
	e := versionsApp(t, allVersions, echoCaps)
	j, err := e.a.InstallGameVersion(e.game, types.VersionTarget{Manual: map[string]string{"input": "  481: 7845\n482: 9921\r\n"}})
	if err != nil {
		t.Fatalf("one id per line should be accepted: %v", err)
	}
	if j.Target == nil || j.Target.Manual["input"] != "481: 7845\n482: 9921" {
		t.Fatalf("the typed value should be sent trimmed: %+v", j.Target)
	}
	big := strings.Repeat("1\n", maxManualValue)
	if _, err := e.a.InstallGameVersion(e.game, types.VersionTarget{Manual: map[string]string{"input": big}}); err == nil {
		t.Fatal("a typed value over the cap was queued")
	}
}

func TestAnArchiveBuildSendsTheLedgerReference(t *testing.T) {
	e := versionsApp(t, allVersions, echoCaps)
	ref := json.RawMessage(`{"echo":true,"build":"b1"}`)
	if err := e.a.ledger.Append(types.LedgerEntry{Store: "echo", Game: e.game.ID, Build: "b1", ArchiveRef: ref}); err != nil {
		t.Fatal(err)
	}
	e.game.IsInstalled = true
	j, err := e.a.UpdateGameVersion(e.game, types.VersionTarget{Build: "b1"})
	if err != nil {
		t.Fatal(err)
	}
	if j.Target == nil || string(j.Target.ArchiveRef) != string(ref) {
		t.Fatalf("the archive build must carry its reference to game.update: %+v", j.Target)
	}
	if got := installParams(j)["target"].(*types.VersionTarget); string(got.ArchiveRef) != string(ref) {
		t.Fatalf("the reference did not reach the params: %+v", got)
	}
}

func TestAJobWithAVersionNeedsTheVersionsGateWhenItRuns(t *testing.T) {
	// The store no longer offers versions for this plugin (the capability is gone).
	e := versionsApp(t, allVersions, `["library","install","auth","discard"]`)
	j := types.Job{ID: "j", Kind: jobs.KindUpdate, Source: "echo", GameID: e.game.ID, StoreGameID: "tiny",
		Title: e.game.Title, InstallDir: "/g", Target: &types.VersionTarget{Build: "b2"}}
	err := jobBackend{e.a}.Start(context.Background(), j)
	if err == nil || !strings.Contains(err.Error(), "chosen version") {
		t.Fatalf("a resumed job with a version must not install the latest: %v", err)
	}
	if e.calls(t)["game.update"] != 0 {
		t.Fatal("game.update reached the plugin without the capability")
	}
}

func TestTheGateTrustsTheLibraryNotTheCaller(t *testing.T) {
	// The library says this game has a single version; the caller's copy says nothing.
	t.Setenv("ECHO_GAME_FEATURES", `{"tiny":["install"]}`)
	e := versionsApp(t, allVersions, echoCaps)
	if _, err := e.a.GetGameVersions(e.game); err == nil {
		t.Fatal("the versions list was fetched for a game the library gives a single version")
	}
	forged := e.game
	forged.ID = "p.echo.nothere"
	if _, err := e.a.GetGameVersions(forged); err == nil {
		t.Fatal("a game the library doesn't list was accepted")
	}
	if e.calls(t)["versions.list"] != 0 {
		t.Fatal("versions.list reached the plugin")
	}
}
