package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/types"
)

// limitApp is an App around an echo plugin whose manifest declares the given
// protocol versions: enough to build job params (OffersProtocol never starts
// the plugin, so no login or binary behaviour is needed).
func limitApp(t *testing.T, versions string) *App {
	t.Helper()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	installEchoApp(t, pluginsDir)
	manifest := fmt.Sprintf(`{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,`+
		`"protocol":{"versions":%s},"store":"echo","exec":"bin/platypus-echo-app"}`, versions)
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, optHome: root}
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st, Home: root,
	})
	t.Cleanup(a.plugins.Shutdown)
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	return a
}

// attachDownloadLimit (issue #207, protocol 1.21): the cap rides on installs,
// updates and repair verifies to a plugin that declares 1.21, and nowhere
// else.
func TestAttachDownloadLimit(t *testing.T) {
	const limit = int64(1 << 20)
	speaks := limitApp(t, `["1.0","1.19","1.21"]`)
	below := limitApp(t, `["1.0","1.19"]`)

	job := func(kind string, repair bool) types.Job {
		return types.Job{ID: "j1", Kind: kind, Source: "echo", GameID: "g",
			StoreGameID: "tiny", Title: "Tiny", InstallDir: "/g/tiny", Repair: repair}
	}
	cases := []struct {
		name   string
		a      *App
		kind   string
		repair bool
		limit  int64
		want   bool
	}{
		{"install", speaks, jobs.KindInstall, false, limit, true},
		{"update", speaks, jobs.KindUpdate, false, limit, true},
		{"repair verify", speaks, jobs.KindVerify, true, limit, true},
		{"verify check", speaks, jobs.KindVerify, false, limit, false},
		{"unlimited", speaks, jobs.KindInstall, false, 0, false},
		{"below 1.21", below, jobs.KindInstall, false, limit, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.a.settings.SetDownloadLimit(c.limit); err != nil {
				t.Fatal(err)
			}
			params := map[string]any{}
			c.a.attachDownloadLimit(job(c.kind, c.repair), params)
			got, ok := params["downloadLimit"]
			if ok != c.want {
				t.Fatalf("downloadLimit present = %v, want %v (params %v)", ok, c.want, params)
			}
			if c.want && got != c.limit {
				t.Fatalf("downloadLimit = %v, want %d", got, c.limit)
			}
		})
	}
}
