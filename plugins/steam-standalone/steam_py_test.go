package steam_test

// Unit-level checks of hydra-steam helpers, run by loading the script as a
// module (no Steam, no downloader).

import (
	"os/exec"
	"strings"
	"testing"
)

const pyLoad = `
import importlib.machinery, importlib.util, sys
sys.path.insert(0, "bin")
loader = importlib.machinery.SourceFileLoader("hs", "bin/hydra-steam")
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

// STEAM-4: the beta branch and its password reach hydra-steam-dl's stdin
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
