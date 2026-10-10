package gog_test

// Unit-level checks of platypus-gog helpers (PLUG-9), run by loading the script
// as a module so no network or gogdl is involved.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/jobs"
)

const pyLoad = `
import importlib.machinery, importlib.util, sys
loader = importlib.machinery.SourceFileLoader("hg", "bin/platypus-gog")
spec = importlib.util.spec_from_loader("hg", loader)
hg = importlib.util.module_from_spec(spec)
loader.exec_module(hg)
`

func pyRun(t *testing.T, script string, args ...string) string {
	t.Helper()
	out, err := exec.Command("python3", append([]string{"-c", pyLoad + script}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("python: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestGogFindExecutableWithoutIsPrimary(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Game.exe"), []byte("MZ"), 0o755)
	os.WriteFile(filepath.Join(dir, "goggame-1.info"),
		[]byte(`{"playTasks": [{"path": "Game.exe", "category": "game"}]}`), 0o644)
	if got := pyRun(t, `print(hg.find_executable(sys.argv[1]))`, dir); got != filepath.Join(dir, "Game.exe") {
		t.Fatalf("find_executable = %q, want the playTasks path", got)
	}
}

func TestGogFindExecutablePrefersPrimary(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Config.exe"), []byte("MZ"), 0o755)
	os.WriteFile(filepath.Join(dir, "Game.exe"), []byte("MZ"), 0o755)
	os.WriteFile(filepath.Join(dir, "goggame-1.info"), []byte(`{"playTasks": [
	  {"path": "Config.exe"}, {"path": "Game.exe", "isPrimary": true}]}`), 0o644)
	if got := pyRun(t, `print(hg.find_executable(sys.argv[1]))`, dir); got != filepath.Join(dir, "Game.exe") {
		t.Fatalf("find_executable = %q, want the primary task", got)
	}
}

func TestGogAnonymousAPICallSendsNoBearer(t *testing.T) {
	got := pyRun(t, `
import urllib.request, io
reqs = []
class R(io.BytesIO):
    def __enter__(self): return self
    def __exit__(self, *a): pass
def fake(req, timeout=None):
    reqs.append({k.lower(): v for k, v in req.header_items()})
    return R(b"{}")
urllib.request.urlopen = fake
hg.api_get("https://api.gog.com/products/1", "")
hg.api_get("https://embed.gog.com/userData.json", "tok")
print("authorization" in reqs[0], reqs[1].get("authorization"))
`)
	if got != "False Bearer tok" {
		t.Fatalf("anonymous call sent an Authorization header, or the token call lost it: %q", got)
	}
}

// A game installed through Platypus whose account entry later vanishes (removed
// from the library, or listed under another account) keeps the title it had.
func TestGogOrphanKeepsTitle(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	game := lib[0]
	j, err := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		done := false
		for _, x := range q.List() {
			if x.ID == j.ID && x.State == jobs.StateDone {
				done = true
			}
		}
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("install never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// The account no longer lists the game.
	api := os.Getenv("PLATYPUS_GOG_API_FILE")
	os.WriteFile(api, []byte(`{"totalPages": 1, "products": []}`), 0o644)
	lib, err = store.Library(ctx)
	if err != nil || len(lib) != 1 {
		t.Fatalf("library after: %+v, %v", lib, err)
	}
	if lib[0].Title != "Super Game" {
		t.Fatalf("orphan title = %q, want %q", lib[0].Title, "Super Game")
	}
}

// Two installs finishing at once must both land in installed.json: the index
// is a read-modify-write shared by the job threads.
func TestGogConcurrentRecordInstallKeepsBoth(t *testing.T) {
	data := t.TempDir()
	cmd := exec.Command("python3", "-c", pyLoad+`
import threading, time, json, os
hg.platform_of = lambda app: "windows"
orig = hg.find_executable
def slow(d):
    time.sleep(0.3)
    return orig(d)
hg.find_executable = slow
ts = [threading.Thread(target=hg.record_install, args=({"id": a, "path": "/tmp/x", "title": a}, "install")) for a in ("1", "2")]
[t.start() for t in ts]; [t.join() for t in ts]
print(sorted(hg.installed_games()))
`)
	cmd.Env = append(os.Environ(), "PLATYPUS_PLUGIN_DATA="+data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "['1', '2']" {
		t.Fatalf("installed after concurrent record_install = %s", got)
	}
}

func TestGogDetailsUnknownIDIsAnError(t *testing.T) {
	got := pyRun(t, `
import urllib.request, urllib.error
def fake(req, timeout=None):
    raise urllib.error.HTTPError(req.full_url, 404, "Not Found", {}, None)
urllib.request.urlopen = fake
try:
    print(hg.game_details({"id": "typo"}))
except hg.RPCError as e:
    print("error:", e)
`)
	if !strings.HasPrefix(got, "error:") {
		t.Fatalf("unknown id details = %q, want an RPC error", got)
	}
}

// installSize must not turn an unrelated stderr that merely contains "auth"
// into a login prompt.
func TestGogInfoFailureMentioningAuthIsNotALoginPrompt(t *testing.T) {
	got := pyRun(t, `
hg.credentials = lambda: {"access_token": "t"}
hg.platform_of = lambda a: "windows"
hg.gogdl = lambda *a, **k: (1, "", "authoritative server error")
try:
    hg.install_size({"id": "1"})
except hg.RPCError as e:
    print(e.code, e)
`)
	if strings.Contains(got, "log in") {
		t.Fatalf("misreported as logged out: %q", got)
	}
}
