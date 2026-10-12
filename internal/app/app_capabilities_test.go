package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/types"
)

func echoApp(t *testing.T, manifest string) (*App, *plugins.Manager) {
	t.Helper()
	root := t.TempDir()
	trust, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	m := plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: trust,
		Home: root,
	})
	t.Cleanup(m.Shutdown)
	installEchoApp(t, pluginsDir)
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Discover(); err != nil {
		t.Fatal(err)
	}
	return &App{plugins: m, settings: trust, optHome: root}, m
}

func offer(p types.StorePlugin, name string) (types.PluginCapability, bool) {
	for _, c := range p.Offers {
		if c.Name == name {
			return c, true
		}
	}
	return types.PluginCapability{}, false
}

func TestStorePluginListsCapabilitiesWithRuntimeAndFeatures(t *testing.T) {
	t.Setenv("ECHO_OFF", "discard")
	t.Setenv("ECHO_FEATURES", `{"versions":{"channels":true,"password":false}}`)
	a, m := echoApp(t, `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
		"protocol":{"versions":["1.0","1.1","1.3"]},"store":"echo","exec":"bin/platypus-echo-app",
		"capabilities":["library","install","discard","versions","hologram"]}`)
	if err := m.Approve("echo"); err != nil {
		t.Fatal(err)
	}

	before := a.ListStorePlugins()[0]
	if before.Initialized {
		t.Fatal("initialized before the plugin ever ran")
	}
	if c, _ := offer(before, "discard"); !c.On {
		t.Fatal("before the first initialize the declared capabilities are shown as on")
	}

	if _, err := m.Initialize(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	p := a.ListStorePlugins()[0]
	if !p.Initialized || p.Protocol != "1.3" {
		t.Fatalf("initialized %v, protocol %q", p.Initialized, p.Protocol)
	}
	if got := p.Declared; len(got) != 5 {
		t.Fatalf("declared = %v", got)
	}
	for _, c := range p.Capabilities {
		if c == "discard" {
			t.Fatalf("capabilities lists a switched-off capability: %v", p.Capabilities)
		}
	}
	if c, ok := offer(p, "discard"); !ok || c.On || !c.Known {
		t.Fatalf("discard: %+v", c)
	}
	if c, _ := offer(p, "versions"); !c.On || !c.Known || !c.Orchestrated || !c.Features["channels"] || c.Features["password"] {
		t.Fatalf("versions: %+v", c)
	}
	if c, ok := offer(p, "hologram"); !ok || c.Known || c.Orchestrated {
		t.Fatalf("an unknown capability is listed as known: %+v", c)
	}
	if len(p.Warnings) != 1 {
		t.Fatalf("warnings = %q", p.Warnings)
	}
}

func TestStorePluginReportsAnIncompatibleProtocol(t *testing.T) {
	t.Parallel()
	a, _ := echoApp(t, `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
		"protocol":{"versions":["2.0"]},"store":"echo","exec":"bin/platypus-echo-app"}`)
	p := a.ListStorePlugins()[0]
	if p.Status != "needs-newer-platypus" || p.Problem == "" {
		t.Fatalf("%+v", p)
	}
}

func TestLibraryWarningMarksUnavailableApartFromLogin(t *testing.T) {
	t.Parallel()
	auth := &jsonrpc.Error{Code: plugins.CodeAuthRequired, Message: "log in"}
	busy := &jsonrpc.Error{Code: plugins.CodeUnavailable, Message: "Family member Sam is playing this game"}
	w := sourceWarning("echo", auth)
	if !w.NeedsAuth || w.Unavailable {
		t.Fatalf("login: %+v", w)
	}
	w = sourceWarning("echo", busy)
	if w.NeedsAuth || !w.Unavailable || w.Message != busy.Message {
		t.Fatalf("unavailable: %+v", w)
	}
	w = sourceWarning("echo", errors.New("boom"))
	if w.NeedsAuth || w.Unavailable {
		t.Fatalf("other: %+v", w)
	}
}
