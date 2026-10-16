package main

import (
	"strings"
	"testing"
)

// saveEchoGlobals snapshots the library globals loadFixtures mutates, so the
// test can restore them.
func saveEchoGlobals(t *testing.T) (map[string]game, []string, map[string]fixtureSpec) {
	t.Helper()
	gamesCopy := make(map[string]game, len(games))
	for k, v := range games {
		gamesCopy[k] = v
	}
	orderCopy := append([]string{}, order...)
	specsCopy := make(map[string]fixtureSpec, len(fixtureSpecs))
	for k, v := range fixtureSpecs {
		specsCopy[k] = v
	}
	return gamesCopy, orderCopy, specsCopy
}

func restoreEchoGlobals(t *testing.T, g map[string]game, o []string, s map[string]fixtureSpec) {
	t.Helper()
	for k := range games {
		delete(games, k)
	}
	for k, v := range g {
		games[k] = v
	}
	order = o
	for k := range fixtureSpecs {
		delete(fixtureSpecs, k)
	}
	for k, v := range s {
		fixtureSpecs[k] = v
	}
}

func TestFixturesJoinTheLibrary(t *testing.T) {
	g, o, s := saveEchoGlobals(t)
	defer restoreEchoGlobals(t, g, o, s)

	t.Setenv("ECHO_FIXTURES", `{"crash-vcrun":{"mode":"crash","log":"vcruntime"},`+
		`"quick-ok":{"mode":"exit0"},"flaky-fail":{"mode":"fail","code":3,"afterMs":200},`+
		`"steady":{"mode":"run"},"bad mode":{"mode":"nope"},"tiny":{"mode":"exit0"},`+
		`"bulk-0001":{"mode":"exit0"}}`)
	loadFixtures()

	for _, id := range []string{"crash-vcrun", "quick-ok", "flaky-fail", "steady"} {
		if _, ok := games[id]; !ok {
			t.Fatalf("%s not listed as a game", id)
		}
		if _, ok := fixtureSpecs[id]; !ok {
			t.Fatalf("%s has no behaviour spec", id)
		}
	}
	// Bad ids, clashes with echo's own games and the bulk prefix are skipped.
	for _, id := range []string{"bad mode", "bulk-0001"} {
		if _, ok := fixtureSpecs[id]; ok {
			t.Fatalf("%s should have been skipped", id)
		}
	}
	if spec := fixtureSpecs["quick-ok"]; spec.Mode != "exit0" {
		t.Fatalf("quick-ok mode = %q", spec.Mode)
	}
	if _, taken := games["tiny"]; !taken {
		t.Fatal("echo's own tiny game went missing")
	}
	if spec, ok := fixtureSpecs["tiny"]; ok {
		t.Fatalf("tiny must not become a fixture: %+v", spec)
	}
}

func TestFixtureScriptsDoOneThing(t *testing.T) {
	g, o, s := saveEchoGlobals(t)
	defer restoreEchoGlobals(t, g, o, s)

	t.Setenv("ECHO_FIXTURES", `{"crash-vcrun":{"mode":"crash","log":"vcruntime"},`+
		`"quick-ok":{"mode":"exit0"},"flaky-fail":{"mode":"fail","code":3,"afterMs":200},`+
		`"steady":{"mode":"run"}}`)
	loadFixtures()

	if script := fixtureScript("crash-vcrun", "b1"); !strings.Contains(script, "vcruntime140_1.dll") ||
		!strings.Contains(script, "MSVCP140.dll") || !strings.Contains(script, "exit 1") {
		t.Fatalf("crash script misses the Wine stderr or the exit:\n%s", script)
	}
	if script := fixtureScript("quick-ok", "b1"); !strings.Contains(script, "exit 0") {
		t.Fatalf("exit0 script:\n%s", script)
	}
	if script := fixtureScript("flaky-fail", "b1"); !strings.Contains(script, "exit 3") ||
		!strings.Contains(script, "sleep") {
		t.Fatalf("fail script misses the delay or the code:\n%s", script)
	}
	if script := fixtureScript("steady", "b1"); !strings.Contains(script, "ECHO_GAME_SECONDS") {
		t.Fatalf("run script should sleep until stopped:\n%s", script)
	}
	if script := fixtureScript("tiny", "b1"); script != "" {
		t.Fatalf("echo's own games keep the default script, got:\n%s", script)
	}
}

// Every crash log name writes lines its rule matches: the core's hints read
// them back (the full shape is pinned in internal/crashhints).
func TestFixtureCrashLogsNameTheirRule(t *testing.T) {
	cases := map[string]string{
		"vcruntime": "vcruntime140_1.dll",
		"directx":   "d3dx9_43.dll",
		"media":     "mfplat",
		"dotnet":    "mscoree",
		"vulkan":    "Vulkan",
		"anticheat": "EasyAntiCheat",
		"generic":   "import_dll",
	}
	for name, want := range cases {
		lines, ok := fixtureLogs[name]
		if !ok || len(lines) == 0 {
			t.Fatalf("no lines for %s", name)
		}
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, want) {
			t.Fatalf("%s lines miss %q:\n%s", name, want, joined)
		}
	}
}
