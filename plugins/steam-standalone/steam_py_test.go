package steam_test

// Unit-level checks of platypus-steam helpers, run by loading the script as a
// module (no Steam, no downloader).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const pyLoad = `
import importlib.machinery, importlib.util, sys
sys.path.insert(0, "bin")
loader = importlib.machinery.SourceFileLoader("hs", "bin/platypus-steam")
spec = importlib.util.spec_from_loader("hs", loader)
hs = importlib.util.module_from_spec(spec)
loader.exec_module(hs)
`

func pyRun(t *testing.T, script string) string {
	t.Helper()
	out, err := exec.Command("python3", "-c", pyLoad+script).CombinedOutput()
	if err != nil {
		t.Fatalf("python: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// STEAM-4: the beta branch and its password reach platypus-steam-dl's stdin
// request (never argv), for the size query and for install/update.
func TestSteamBranchPasswordInRequest(t *testing.T) {
	got := pyRun(t, `
import threading
hs.refresh_token = lambda: "tok"
reqs = []
def fake_run(sub, req, job_id=None, game_dir=None):
    reqs.append((sub, dict(req)))
    return {"os_used": "linux", "disk_bytes": 1, "download_bytes": 1}
hs.dl_run = fake_run
hs.record_install = lambda *a: None
hs.notify = lambda *a: None
hs.install_size({"id": "4321", "branch": "beta", "branchPassword": "s3cret"})
hs.follow_install("j1", "4321", "/tmp/g", "update", {"branch": "beta", "branchPassword": "s3cret"})
hs.follow_install("j2", "4321", "/tmp/g", "install")
print([(s, r["branch"], r.get("branch_password")) for s, r in reqs])
`)
	want := "[('size', 'beta', 's3cret'), ('update', 'beta', 's3cret'), ('install', 'public', None)]"
	if got != want {
		t.Fatalf("requests = %s, want %s", got, want)
	}
}

// The password never reaches the plugin's logs or job table.
func TestSteamBranchPasswordNotRetained(t *testing.T) {
	got := pyRun(t, `
hs.refresh_token = lambda: "tok"
hs.dl_run = lambda *a, **k: {"os_used": "linux"}
hs.record_install = lambda *a: None
sent = []
hs.notify = lambda m, p: sent.append((m, p))
hs.follow_install("j1", "4321", "/tmp/g", "install", {"branch": "beta", "branchPassword": "s3cret"})
print("s3cret" in repr(sent) or "s3cret" in repr(hs.jobs))
`)
	if got != "False" {
		t.Fatalf("branch password leaked into notifications or the job table")
	}
}

// STEAM-7: a token source persisted as JSON (list, mtime maybe an int) still
// matches the live account.config stamp, so the legacy fast path hits.
func TestSteamTokenSourceSurvivesJSONRoundTrip(t *testing.T) {
	got := pyRun(t, `
import json
src = ("/h/account.config", 1700000000.5)
rt = json.loads(json.dumps(src))
print(hs._source_matches(rt, "/h/account.config", 1700000000.5),
      hs._source_matches([ "/h/account.config", 1700000000], "/h/account.config", 1700000000.0),
      hs._source_matches(rt, "/h/other", 1700000000.5),
      hs._source_matches(rt, "/h/account.config", 1700000001.5),
      hs._source_matches(None, "/h/account.config", 1.0))
`)
	if got != "True True False False False" {
		t.Fatalf("_source_matches = %q", got)
	}
}

// STEAM-9: the game's exe often sits in a subfolder of the install.
func TestSteamFindExecutableSearchesSubfolders(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "bin", "x64")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "Game.exe"), make([]byte, 4000), 0o755)
	os.WriteFile(filepath.Join(dir, "setup.exe"), make([]byte, 9000), 0o755)
	os.MkdirAll(filepath.Join(dir, "redist"), 0o755)
	os.WriteFile(filepath.Join(dir, "redist", "Big.exe"), make([]byte, 50000), 0o755)
	cmd := exec.Command("python3", "-c", pyLoad+`print(hs.find_executable(sys.argv[1]))`, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != filepath.Join(sub, "Game.exe") {
		t.Fatalf("find_executable = %q, want bin/x64/Game.exe", got)
	}
}
