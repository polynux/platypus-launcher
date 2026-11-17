package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/social"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// inviteEnv is socialEnv with the echo plugin speaking 1.24: its manifest
// and its ECHO_PROTOCOLS both gain the revision, so the gate negotiates it.
func inviteEnv(t *testing.T) *App {
	t.Helper()
	t.Setenv("ECHO_PROTOCOLS", "1.0,1.1,1.2,1.3,1.4,1.5,1.6,1.7,1.8,1.9,1.10,1.11,1.12,1.13,1.14,1.15,1.16,1.17,1.18,1.19,1.20,1.21,1.22,1.24")
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, ctx: context.Background(), optHome: root}
	pluginsDir := filepath.Join(root, "plugins")
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st,
		Home: root, OnNotify: a.pluginNotify,
	})
	t.Cleanup(a.plugins.Shutdown)
	installEchoApp(t, pluginsDir)
	data, err := os.ReadFile(repoPath("plugins/echo/plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	m["exec"] = "bin/platypus-echo-app"
	proto := m["protocol"].(map[string]any)
	proto["versions"] = append(proto["versions"].([]any), "1.24")
	out, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	a.social = social.NewManager(social.Options{
		Backend: socialBackend{a},
		Enabled: st.SocialEnabled,
		Emit:    func(social.View) {},
	})
	t.Cleanup(a.social.StopAll)
	return a
}

func inviteParams(friend, name, storeID, kind, frag string) json.RawMessage {
	iv := map[string]any{
		"from": map[string]any{"id": friend, "name": name},
		"game": map[string]any{"storeId": storeID, "title": "Game " + storeID},
		"kind": kind, "at": 1791400000,
	}
	if kind == "lobby" {
		iv["lobbyId"] = frag
	} else {
		iv["connect"] = frag
	}
	raw, _ := json.Marshal(iv)
	return raw
}

// A 1.22 plugin's invite never reaches the view; a 1.24 plugin's does, but
// only for a friend on its own list.
func TestInviteGateNeeds124AndAFriend(t *testing.T) {
	// No t.Parallel: inviteEnv sets ECHO_PROTOCOLS for the plugin process.
	old, _ := socialEnv(t)
	old.syncSocial()
	waitSocial(t, "echo friends", func() bool { return len(old.social.View("echo").Friends) > 0 })
	old.pluginNotify("echo", social.NoteInvite,
		inviteParams("7656119800000001", "Ana Ruiz", "1", "connect", "+connect 1.2.3.4:27015"))
	if v := old.social.View("echo"); len(v.Invites) != 0 {
		t.Fatalf("1.22 invite reached the view: %+v", v.Invites)
	}

	a := inviteEnv(t)
	a.syncSocial()
	waitSocial(t, "echo friends", func() bool { return len(a.social.View("echo").Friends) > 0 })
	a.pluginNotify("echo", social.NoteInvite,
		inviteParams("7656119800000001", "Ana Ruiz", "1", "connect", "+connect 1.2.3.4:27015"))
	if v := a.social.View("echo"); len(v.Invites) != 1 || v.Invites[0].Kind != "connect" {
		t.Fatalf("1.24 invite missing: %+v", v.Invites)
	}
	a.pluginNotify("echo", social.NoteInvite,
		inviteParams("7656119900000001", "Zed", "1", "connect", "+connect 1.2.3.4:27015"))
	if v := a.social.View("echo"); len(v.Invites) != 1 {
		t.Fatalf("stranger invite reached the view: %+v", v.Invites)
	}
}

// directStore answers a direct launch spec without validation, like
// clientStore does for client launches.
type directStore struct {
	exe  string
	args []string
}

func (s directStore) ID() string { return "directstore" }
func (s directStore) Library(context.Context) ([]types.Game, error) {
	return nil, nil
}
func (s directStore) Details(context.Context, types.Game) (*types.GameDetails, error) {
	return nil, nil
}
func (s directStore) Launch(context.Context, types.Game, bool) (*stores.Launch, error) {
	dir := filepath.Dir(s.exe)
	return &stores.Launch{Direct: &stores.DirectLaunch{Exe: s.exe, Args: s.args, Dir: dir}}, nil
}

// Extra arguments ride the launch spec through the runner wrap: argv for
// direct launches, after the app id for store-client launches.
func TestLaunchWithArgsThreadsThroughWrap(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.ctx = context.Background()
	exe := writeExe(t, filepath.Join(home, "game", "game"), elfBytes, true)
	a.stores = stores.NewRegistry(nil, directStore{exe: exe, args: []string{"-windowed"}})
	g := types.Game{ID: "directstore-1", Source: "directstore", StoreAppID: "1", Title: "Direct", Native: true, Executable: exe}
	extra := []string{"+connect", "1.2.3.4:27015"}
	p, err := a.prepareLaunchArgs(g, true, extra)
	if err != nil {
		t.Fatalf("prepareLaunchArgs: %v", err)
	}
	if tail := p.args[len(p.args)-len(extra):]; !reflect.DeepEqual(tail, extra) {
		t.Fatalf("args = %q, want the tail %q", p.args, extra)
	}
	if got := p.shown(); !strings.Contains(got, "+connect 1.2.3.4:27015") {
		t.Fatalf("preview = %q, want the join fragment", got)
	}

	a.stores = stores.NewRegistry(nil, clientStore{command: []string{"steam", "-applaunch", "620"}})
	cg := types.Game{ID: "clientstore-620", Source: "clientstore", StoreAppID: "620", Title: "Portal"}
	cp, err := a.prepareLaunchArgs(cg, true, []string{"+connect_lobby", "109775241000000001"})
	if err != nil {
		t.Fatalf("client prepareLaunchArgs: %v", err)
	}
	if want := []string{"steam", "-applaunch", "620", "+connect_lobby", "109775241000000001"}; !reflect.DeepEqual(cp.args, want) {
		t.Fatalf("client args = %q, want %q", cp.args, want)
	}
}

// Join arguments are validated like the spec's own: bounded in count and
// length, with no control characters, and never shelled.
func TestLaunchWithArgsValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args []string
		code string
	}{
		{"ok", []string{"+connect", "1.2.3.4:27015"}, ""},
		{"quoted", []string{`+connect "1.2.3.4:27015"`}, ""},
		{"tooMany", make([]string, 17), "game.tooManyJoinArgs"},
		{"tooLong", []string{strings.Repeat("c", 257)}, "game.badJoinArg"},
		{"newline", []string{"+connect", "a\nb"}, "game.badJoinArg"},
		{"nul", []string{"a\x00b"}, "game.badJoinArg"},
	} {
		if err := validateExtraArgs("G", tc.args); (err == nil) != (tc.code == "") {
			t.Fatalf("%s: err = %v, want code %q", tc.name, err, tc.code)
		}
	}
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.ctx = context.Background()
	g := types.Game{ID: "c-1", Source: "custom", Title: "G"}
	_, err := a.LaunchWithArgs(g, make([]string, 17))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "game.tooManyJoinArgs" {
		t.Fatalf("LaunchWithArgs with 17 args: %v", err)
	}
}
