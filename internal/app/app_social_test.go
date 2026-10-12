package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/social"
	"platypus/internal/types"
)

// socialEnv is an App around the echo plugin with its real manifest, which
// declares the social capability: the friends orchestrator runs against a
// real plugin process.
func socialEnv(t *testing.T) (*App, *settings.Store) {
	t.Helper()
	return socialEnvWith(t, false)
}

// socialEnvWith is socialEnv, and with echo-b (the second store, the same
// program under its own manifest) installed and approved too when withB is set.
func socialEnvWith(t *testing.T, withB bool) (*App, *settings.Store) {
	t.Helper()
	root := t.TempDir()
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, ctx: context.Background()}
	pluginsDir := filepath.Join(root, "plugins")
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st,
		OnNotify: a.pluginNotify,
	})
	t.Cleanup(a.plugins.Shutdown)
	installEchoApp(t, pluginsDir)
	writeEchoManifest(t, filepath.Join(pluginsDir, "echo", "plugin.json"))
	if withB {
		installEchoB(t, pluginsDir)
	}
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	if withB {
		if err := a.plugins.Approve("echo-b"); err != nil {
			t.Fatal(err)
		}
	}
	a.social = social.NewManager(social.Options{
		Backend: socialBackend{a},
		Enabled: st.SocialEnabled,
		Emit:    func(social.View) {},
	})
	t.Cleanup(a.social.StopAll)
	return a, st
}

// writeEchoManifest copies the repository's echo manifest, pointing its exec at
// the binary installEchoApp built.
func writeEchoManifest(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(repoPath("plugins/echo/plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	m["exec"] = "bin/platypus-echo-app"
	out, _ := json.Marshal(m)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// waitSocial polls cond until it holds or the test times out.
func waitSocial(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// echoRecorded is what echo's `recorded` method reports.
func echoRecorded(t *testing.T, a *App) (statuses, playing []any) {
	t.Helper()
	var rec struct {
		SetStatus  []any `json:"setStatus"`
		SetPlaying []any `json:"setPlaying"`
	}
	if err := a.plugins.Call(context.Background(), "echo", "recorded", nil, &rec); err != nil {
		t.Fatalf("recorded: %v", err)
	}
	return rec.SetStatus, rec.SetPlaying
}

func TestSocialWithTheEchoPlugin(t *testing.T) {
	a, _ := socialEnv(t)
	a.syncSocial()

	waitSocial(t, "a connected friends list", func() bool {
		v := a.social.View("echo")
		return v.Connection.State == "online" && len(v.Friends) == 12
	})
	views := a.SocialViews()
	if len(views) != 1 || views[0].Plugin != "echo" || !views[0].Enabled || views[0].Self == nil {
		t.Fatalf("SocialViews: %+v", views)
	}

	// The friends reconnect on their own when the plugin process hiccups
	// (under load the session can flap between the wait above and this
	// call), so a transient "not connected" waits for the reconnect instead
	// of failing. Anything else — a rejected status, a missing method —
	// still fails at once.
	setStatus := func(state string) error {
		deadline := time.Now().Add(15 * time.Second)
		for {
			err := a.SetSocialStatus("echo", state)
			if err == nil || err.Error() != "not connected to the store" || time.Now().After(deadline) {
				return err
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if err := setStatus("busy"); err != nil {
		t.Fatalf("SetSocialStatus: %v", err)
	}
	if err := a.SetSocialStatus("echo", "lurking"); err == nil {
		t.Fatal("an unknown status reached the store")
	}
	if statuses, _ := echoRecorded(t, a); len(statuses) != 1 || statuses[0] != "busy" {
		t.Fatalf("setStatus calls: %v", statuses)
	}

	// Only the plugin's own games are reported as played: launchGame notes the
	// game with its source, and a session event for a game not noted is ignored.
	a.noteSocialGame(types.Game{ID: "p.echo.tiny", Source: "echo", StoreAppID: "tiny"})
	a.socialSession(types.SessionEvent{GameID: "p.echo.tiny", State: "running"})
	waitSocial(t, "tiny reported as played", func() bool {
		_, playing := echoRecorded(t, a)
		return len(playing) == 1 && playing[0] == "tiny"
	})
	a.socialSession(types.SessionEvent{GameID: "steam-10", State: "running"})
	a.socialSession(types.SessionEvent{GameID: "p.echo.tiny", State: "stopped"})
	waitSocial(t, "tiny cleared", func() bool {
		_, playing := echoRecorded(t, a)
		return len(playing) == 2 && playing[1] == nil
	})

	// Switching the friends off disconnects; switching them on connects again.
	if err := a.SetSocialEnabled("echo", false); err != nil {
		t.Fatal(err)
	}
	if a.social.Running("echo") || a.SocialViews()[0].Enabled {
		t.Fatal("friends still on after switching off")
	}
	if err := a.SetSocialEnabled("echo", true); err != nil {
		t.Fatal(err)
	}
	waitSocial(t, "reconnected", func() bool { return a.social.View("echo").Connection.State == "online" })
}

func TestSocialStaysOffWhenTheUserSwitchesItOff(t *testing.T) {
	a, st := socialEnv(t)
	if err := st.SetSocialEnabled("echo", false); err != nil {
		t.Fatal(err)
	}
	a.syncSocial()
	if a.social.Running("echo") {
		t.Fatal("a plugin switched off in settings is subscribed at launch")
	}
	if err := a.SetSocialStatus("echo", "online"); err == nil {
		t.Fatal("status changed on a store with the friends off")
	}
}

// A plugin's start runs two things at once: the settings are sent (pluginReady,
// after initialize) and the friends list starts. Both reach the plugin, the
// friends connect and the saved value is applied once, with no race.
func TestPluginStartSettingsAndFriendsStartTogether(t *testing.T) {
	a, st := socialEnv(t)
	if err := st.SetPluginValues("echo", map[string]any{"eos-overlay": true}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		a.pluginReady("echo")
		close(done)
	}()
	a.syncSocial()
	<-done

	waitSocial(t, "a connected friends list", func() bool {
		return a.social.View("echo").Connection.State == "online"
	})
	var calls map[string]int
	if err := a.plugins.Call(context.Background(), "echo", "calls", nil, &calls); err != nil {
		t.Fatal(err)
	}
	if calls["settings.apply"] != 1 || calls["social.start"] != 1 {
		t.Fatalf("calls %v: want the saved settings applied once and the friends started once", calls)
	}
}
