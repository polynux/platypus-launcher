package amazon_test

// Unit-level checks of hydra-amazon helpers (PLUG-7, PLUG-9), run by loading
// the script as a module so no network or nile is involved.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const pyLoad = `
import importlib.machinery, importlib.util, sys
loader = importlib.machinery.SourceFileLoader("ha", "bin/hydra-amazon")
spec = importlib.util.spec_from_loader("ha", loader)
ha = importlib.util.module_from_spec(spec)
loader.exec_module(ha)
`

func pyRun(t *testing.T, script string, args ...string) string {
	t.Helper()
	out, err := exec.Command("python3", append([]string{"-c", pyLoad + script}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("python: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// The queue shows a speed only when job.progress carries one.
func TestAmazonProgressCarriesSpeed(t *testing.T) {
	got := pyRun(t, `
import io
sent = []
ha.notify = lambda m, p: sent.append((m, p))
class P:
    stdout = io.StringIO("[100.0/400.0MiB (25%, 25.0 MiB/s)] DLing file\n")
    def wait(self): return 0
ha.jobs["j"] = {"proc": None, "canceled": False}
ha.follow_job("j", P())
prog = [p for m, p in sent if m == "job.progress"]
print(prog[0].get("speed"))
`)
	if got != "26214400.0" {
		t.Fatalf("speed = %q, want 25 MiB/s in bytes/s", got)
	}
}

func TestAmazonFindExeSkipsRedistributables(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vcredist_x64.exe"), make([]byte, 5000), 0o755)
	os.WriteFile(filepath.Join(dir, "dotnetfx35.exe"), make([]byte, 6000), 0o755)
	os.WriteFile(filepath.Join(dir, "Game.exe"), make([]byte, 100), 0o755)
	if got := pyRun(t, `print(ha.find_exe(sys.argv[1]))`, dir); got != filepath.Join(dir, "Game.exe") {
		t.Fatalf("find_exe = %q, want Game.exe", got)
	}
}

// setup_env must not rewrite os.environ after startup: it is called from
// every handler thread.
func TestAmazonSetupEnvIsIdempotent(t *testing.T) {
	data := t.TempDir()
	cmd := exec.Command("python3", "-c", pyLoad+`
import os
ha.setup_env()
first = os.environ["NILE_CONFIG_PATH"]
ha.setup_env()
print(first == os.environ["NILE_CONFIG_PATH"] == os.path.join(sys.argv[1], "nile-config"))
`, data)
	cmd.Env = append(os.Environ(), "HYDRA_PLUGIN_DATA="+data)
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "True" {
		t.Fatalf("%v: %s", err, out)
	}
}
