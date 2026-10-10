package plugtest_test

// plugins/install.sh against a throwaway plugin tree (PLUG-5): reinstalls
// must not leave files upstream deleted, a data plugin must install, the
// manifest is parsed as JSON, and a helper built from cmd/ survives a
// reinstall.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type installEnv struct {
	t     *testing.T
	tree  string // stands in for the repo: tree/go.mod, tree/plugins/...
	dest  string // PLATYPUS_PLUGINS
	build string // tree/plugins
}

func newInstallEnv(t *testing.T) *installEnv {
	t.Helper()
	for _, bin := range []string{"python3", "go"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	script, err := os.ReadFile("../../plugins/install.sh")
	if err != nil {
		t.Fatal(err)
	}
	tree := t.TempDir()
	e := &installEnv{t: t, tree: tree, dest: filepath.Join(t.TempDir(), "installed"), build: filepath.Join(tree, "plugins")}
	e.write("go.mod", "module fixture\n\ngo 1.21\n")
	if err := os.WriteFile(e.path("plugins/install.sh"), script, 0o755); err != nil {
		os.MkdirAll(e.path("plugins"), 0o755)
		if err := os.WriteFile(e.path("plugins/install.sh"), script, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func (e *installEnv) path(rel string) string { return filepath.Join(e.tree, filepath.FromSlash(rel)) }

func (e *installEnv) write(rel, body string) {
	e.t.Helper()
	p := e.path(rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *installEnv) install(name string) (string, error) {
	e.t.Helper()
	cmd := exec.Command("sh", e.path("plugins/install.sh"), name)
	cmd.Env = append(os.Environ(), "PLATYPUS_PLUGINS="+e.dest)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// installed reports whether the plugin's installed folder holds the file.
func (e *installEnv) installed(name, rel string) bool {
	_, err := os.Stat(filepath.Join(e.dest, name, filepath.FromSlash(rel)))
	return err == nil
}

func TestInstallScriptRemovesFilesDeletedUpstream(t *testing.T) {
	e := newInstallEnv(t)
	e.write("plugins/p/plugin.json", `{"id": "p", "exec": "bin/platypus-p"}`)
	e.write("plugins/p/bin/platypus-p", "#!/bin/sh\n")
	e.write("plugins/p/bin/old_helper.py", "x = 1\n")
	if out, err := e.install("p"); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if !e.installed("p", "bin/old_helper.py") {
		t.Fatal("helper wasn't copied")
	}
	os.Remove(e.path("plugins/p/bin/old_helper.py"))
	if out, err := e.install("p"); err != nil {
		t.Fatalf("reinstall: %v\n%s", err, out)
	}
	if e.installed("p", "bin/old_helper.py") {
		t.Fatal("a file deleted upstream stayed in the installed plugin")
	}
	if !e.installed("p", "bin/platypus-p") {
		t.Fatal("the exec was removed")
	}
}

func TestInstallScriptParsesManifestAsJSON(t *testing.T) {
	e := newInstallEnv(t)
	e.write("plugins/p/plugin.json", `{"id": "p", "description": "see \"exec\": \"bin/evil\" in the docs", "exec": "bin/platypus-p"}`)
	e.write("plugins/p/bin/platypus-p", "#!/bin/sh\n")
	e.write("plugins/p/bin/evil", "#!/bin/sh\n")
	if out, err := e.install("p"); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if !e.installed("p", "bin/platypus-p") {
		t.Fatal("the real exec wasn't installed")
	}
}

func TestInstallScriptInstallsDataPlugins(t *testing.T) {
	e := newInstallEnv(t)
	e.write("plugins/pack/plugin.json", `{"id": "pack", "kind": "data", "data": ["fixes.json"]}`)
	e.write("plugins/pack/fixes.json", `[]`)
	if out, err := e.install("pack"); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if !e.installed("pack", "plugin.json") || !e.installed("pack", "fixes.json") {
		t.Fatal("data plugin files missing")
	}
}

// A helper built from cmd/ lives in bin/ but not in the source bin/; the
// cleanup of upstream-deleted files must leave it alone.
func TestInstallScriptKeepsBuiltHelpersOnReinstall(t *testing.T) {
	e := newInstallEnv(t)
	e.write("plugins/p/plugin.json", `{"id": "p", "exec": "bin/platypus-p"}`)
	e.write("plugins/p/bin/platypus-p", "#!/bin/sh\n")
	e.write("plugins/p/cmd/helper/main.go", "package main\n\nfunc main() {}\n")
	for i := 0; i < 2; i++ {
		if out, err := e.install("p"); err != nil {
			t.Fatalf("install %d: %v\n%s", i, err, out)
		}
		if !e.installed("p", "bin/helper") {
			t.Fatalf("install %d: the helper built from cmd/ is missing", i)
		}
	}
}

func TestInstallScriptRejectsEscapingExec(t *testing.T) {
	e := newInstallEnv(t)
	e.write("plugins/p/plugin.json", `{"id": "p", "exec": "../x"}`)
	out, err := e.install("p")
	if err == nil || !strings.Contains(out, "escapes") {
		t.Fatalf("expected a refusal, got %v: %s", err, out)
	}
}
