package migrate

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/plugins"
	"platypus/internal/settings"
)

// The user approved plugins before the rename. After it they must find their
// login where it was, see the old build gone from the list, and approve the
// reinstalled build exactly once.
func TestApprovedPluginNeedsOneApprovalAfterReinstall(t *testing.T) {
	home, oldRoot := oldHome(t)
	// Replace the fixture's plugins with a real, valid, approved one.
	os.RemoveAll(filepath.Join(oldRoot, "plugins"))
	manifest := func(exec string) string {
		return `{"id":"echo","name":"Echo","version":"1","manifestVersion":1,"protocol":{"versions":["1.0"],"transport":"jsonrpc-stdio"},
"store":"echo","exec":"` + exec + `","capabilities":["library"],"install_dir":"echo","permissions":{"network":[],"read":[],"write":[]}}`
	}
	write(t, filepath.Join(oldRoot, "plugins/echo/plugin.json"), manifest("bin/hydra-echo"), 0o644)
	write(t, filepath.Join(oldRoot, "plugins/echo/bin/hydra-echo"), "#!/bin/sh\nexit 0\n", 0o755)
	write(t, filepath.Join(oldRoot, "plugin-data/echo/state.json"), "{}", 0o600)

	manager := func(root string) *plugins.Manager {
		st, err := settings.Open(filepath.Join(root, "settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		return plugins.NewManager(plugins.Options{Dir: filepath.Join(root, "plugins"), DataDir: filepath.Join(root, "plugin-data"), Settings: st})
	}

	before := manager(oldRoot)
	if _, err := before.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := before.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	if p, _ := before.Plugin("echo"); p.Status != plugins.StatusReady {
		t.Fatalf("approved plugin = %s", p.Status)
	}

	rep := Run(home, Options{})
	newRoot := rep.Root
	after := manager(newRoot)
	if _, err := after.Discover(); err != nil {
		t.Fatal(err)
	}
	if _, listed := after.Plugin("echo"); listed {
		t.Fatal("the pre-rename build still runs from plugins/")
	}
	read(t, filepath.Join(newRoot, "plugin-data/echo/state.json")) // the plugin's data is where it was

	// plugins/install.sh puts the new build in place.
	write(t, filepath.Join(newRoot, "plugins/echo/plugin.json"), manifest("bin/platypus-echo"), 0o644)
	write(t, filepath.Join(newRoot, "plugins/echo/bin/platypus-echo"), "#!/bin/sh\nexit 0\n", 0o755)
	if _, err := after.Discover(); err != nil {
		t.Fatal(err)
	}
	if p, _ := after.Plugin("echo"); p.Status != plugins.StatusChanged {
		t.Fatalf("reinstalled plugin = %s, want %s (approve once)", p.Status, plugins.StatusChanged)
	}
	if err := after.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	if p, _ := after.Plugin("echo"); p.Status != plugins.StatusReady {
		t.Fatalf("after approval = %s", p.Status)
	}
	// And it stays approved across a restart.
	again := manager(newRoot)
	again.Discover()
	if p, _ := again.Plugin("echo"); p.Status != plugins.StatusReady {
		t.Fatalf("after restart = %s", p.Status)
	}
}
