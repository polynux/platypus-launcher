package stores

import (
	"errors"
	"strings"
	"testing"

	"platypus/internal/jobs"
	"platypus/internal/plugins"
)

// Protocol 1.15 (issue #44): offline states, cloudSync, local-art, gameIdPrefix,
// the executable param and optional requires.

func TestOfflineMapping(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	lib, err := e.store.Library(e.ctx(t))
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	cloud := map[string]bool{}
	for _, g := range lib {
		byID[g.StoreAppID] = g.Offline
		cloud[g.StoreAppID] = g.CloudSync
	}
	if byID["tiny"] != "ok" {
		t.Errorf("tiny offline = %q, want ok", byID["tiny"])
	}
	if byID["huge"] != "network" || byID["client"] != "network" {
		t.Errorf("huge/client offline = %q/%q, want network", byID["huge"], byID["client"])
	}
	if byID["slow"] != "unknown" {
		t.Errorf("slow offline = %q, want unknown (absent means unknown)", byID["slow"])
	}
	if !cloud["licensed"] || !cloud["client"] {
		t.Errorf("licensed/client cloudSync = %v/%v, want true", cloud["licensed"], cloud["client"])
	}
	if cloud["tiny"] {
		t.Error("tiny cloudSync = true, want false")
	}
}

func TestOfflineInvalidIsUnknown(t *testing.T) {
	for _, raw := range []string{"yes", "OFFLINE", "ok ", "never"} {
		if got := normalizeOffline(raw, true); got != "unknown" {
			t.Errorf("normalizeOffline(%q) = %q, want unknown", raw, got)
		}
	}
	if got := normalizeOffline("ok", false); got != "unknown" {
		t.Errorf("pre-1.15 offline = %q, want unknown", got)
	}
}

func TestGameIDPrefix(t *testing.T) {
	if got := GameIDWithPrefix("", "echo", "tiny"); got != "p.echo.tiny" {
		t.Errorf("default prefix: %q", got)
	}
	if got := GameIDWithPrefix("steam_", "steam", "440"); got != "steam_440" {
		t.Errorf("steam prefix: %q", got)
	}
	if GameIDWithPrefix("steam_", "steam", "440") == GameID("steam", "440") {
		t.Error("custom prefix must differ from the default form")
	}
	// Unsafe ids stay unique and path-safe under a custom prefix too.
	a, b := GameIDWithPrefix("steam_", "steam", "a/b"), GameIDWithPrefix("steam_", "steam", "a_b")
	if a == b || strings.ContainsAny(a, "/") {
		t.Errorf("unsafe ids must stay unique and path-safe: %s %s", a, b)
	}
}

func TestGameIDPrefixValidation(t *testing.T) {
	base := plugins.Manifest{ID: "echo", Name: "n", Version: "v", Store: "echo", Exec: "bin/x", Protocol: &plugins.ProtocolSpec{Versions: plugins.VersionSet{List: []plugins.Version{{Major: 1, Minor: 15}}}}}
	bad := base
	bad.GameIDPrefix = "steam_"
	if err := bad.ValidateForTest(); err == nil {
		t.Error("third-party gameIdPrefix must be rejected")
	}
	// The allowlist names the bundled plugin; test with a temporary entry
	// since "steam" itself is reserved for the built-in source until phase 2.
	plugins.BundledPluginIDs["prefix-fixture"] = true
	defer delete(plugins.BundledPluginIDs, "prefix-fixture")
	good := base
	good.ID = "prefix-fixture"
	good.Store = "prefix-fixture"
	good.GameIDPrefix = "steam_"
	if err := good.ValidateForTest(); err != nil {
		t.Errorf("bundled gameIdPrefix rejected: %v", err)
	}
	for _, prefix := range []string{"p.echo.", "way-too-long-prefix-over-32-chars!!", "has space"} {
		m := good
		m.GameIDPrefix = prefix
		if err := m.ValidateForTest(); err == nil {
			t.Errorf("prefix %q accepted", prefix)
		}
	}
}

func TestLocalArtRewritten(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	g := e.game(t, "tiny")
	if !strings.HasPrefix(g.CoverURL, "/platypus-art/echo/") {
		t.Fatalf("tiny cover = %q, want /platypus-art/echo/<b64>", g.CoverURL)
	}
}

func TestLocalArtRefusals(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	s := e.store
	for _, raw := range []string{
		"file://relative/path.png",
		"file:///tmp/evil.exe",
		"file:///tmp/platypus-echo-art/cover.bmp",
		"file:///etc/passwd",
		"javascript:alert(1)",
	} {
		if got := s.artURL(raw); got != "" {
			t.Errorf("artURL(%q) = %q, want dropped", raw, got)
		}
	}
}

func TestLaunchSpecExecutableParam(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	j := e.install(t, "install", "tiny")
	if got := waitJob(t, e.q, j.ID, jobs.StateDone, jobs.StateFailed); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	g := e.game(t, "tiny")
	if g.InstallDir == "" {
		t.Fatal("tiny not installed")
	}
	// The override file exists inside the install folder, so validation passes.
	g.Executable = g.InstallDir + "/game.sh"
	l, err := e.store.Launch(e.ctx(t), g, true)
	if err != nil || l == nil || l.Direct == nil {
		t.Fatalf("launch with executable override: %+v, %v", l, err)
	}
}

func TestOptionalRequiresNotMissing(t *testing.T) {
	m := plugins.Manifest{
		ID: "x", Name: "n", Version: "v", Store: "x", Exec: "bin/x",
		Requires: []plugins.Requirement{
			{Bin: "definitely-not-on-path-platypus", Hint: "h"},
			{Bin: "also-not-on-path-platypus", Hint: "h", Optional: true},
		},
	}
	missing := m.MissingRequirements(func(s string) (string, error) {
		return "", errors.New("not found")
	})
	if len(missing) != 1 || missing[0].Bin != "definitely-not-on-path-platypus" {
		t.Fatalf("optional requires reported as missing: %+v", missing)
	}
}
