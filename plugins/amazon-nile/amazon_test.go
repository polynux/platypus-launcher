// Tests the Amazon plugin through Hydra's real plugin path (manager, JSON-RPC,
// job queue, launch-spec validation) against testdata/nile, a fake CLI that
// prints the real nile's output formats. It doesn't reach Amazon.
package amazon_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
)

const id = "amazon-nile"

const MIB = 1024 * 1024

type backend struct{ m *plugins.Manager }

func (b backend) Size(ctx context.Context, j types.Job) (plugins.InstallSize, error) {
	var s plugins.InstallSize
	err := b.m.Call(ctx, j.Source, "game.installSize", map[string]any{"id": j.StoreGameID}, &s)
	return s, err
}

func (b backend) Start(ctx context.Context, j types.Job) error {
	p := map[string]any{"id": j.StoreGameID, "jobId": j.ID}
	if j.Kind != jobs.KindUninstall {
		p["installDir"] = j.InstallDir
	}
	return b.m.Call(ctx, j.Source, "game."+j.Kind, p, nil)
}

func (b backend) Cancel(ctx context.Context, j types.Job) error {
	return b.m.Call(ctx, j.Source, "job.cancel", map[string]any{"jobId": j.ID}, nil)
}

func copyFile(t *testing.T, src, dst string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(dst), 0o755)
	if err := os.WriteFile(dst, data, mode); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) (*plugins.Manager, *jobs.Queue, *stores.Plugin, string) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	root := t.TempDir()
	plug := filepath.Join(root, "plugins", id)
	copyFile(t, "plugin.json", filepath.Join(plug, "plugin.json"), 0o644)
	copyFile(t, "bin/hydra-amazon", filepath.Join(plug, "bin/hydra-amazon"), 0o755)
	bin := filepath.Join(root, "fakebin")
	copyFile(t, "testdata/nile", filepath.Join(bin, "nile"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	t.Setenv("FAKE_NILE_STATE", filepath.Join(root, "nile-state.json"))

	sets, _ := settings.Open(filepath.Join(root, "settings.json"))
	var q *jobs.Queue
	m := plugins.NewManager(plugins.Options{
		Dir: filepath.Join(root, "plugins"), LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: sets,
		OnNotify: func(pid, method string, params json.RawMessage) {
			var u plugins.JobUpdate
			if json.Unmarshal(params, &u) == nil {
				q.Notify(pid, method, u)
			}
		},
		OnExit: func(pid string, err error) { q.PluginExited(pid, err) },
	})
	q, _ = jobs.Open(filepath.Join(root, "jobs.json"), backend{m}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go q.Run(ctx)
	t.Cleanup(func() { cancel(); m.Shutdown() })

	if _, err := m.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := m.Approve(id); err != nil {
		t.Fatal(err)
	}
	p, _ := m.Plugin(id)
	if p.Status != plugins.StatusReady {
		t.Fatalf("not ready: %+v", p)
	}
	folder, _ := m.InstallFolder(id, filepath.Join(root, "Games"))
	return m, q, stores.NewPlugin(m, p, func() []string { return m.Folders(id, filepath.Join(root, "Games")) }), folder
}

func call(t *testing.T, m *plugins.Manager, method string, params, result any) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return m.Call(ctx, id, method, params, result)
}

func login(t *testing.T, m *plugins.Manager) {
	t.Helper()
	var st types.AuthStatus
	if err := call(t, m, "auth.status", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("status: %+v, %v", st, err)
	}
	// begin generates the device registration; complete registers with it.
	var pr types.AuthPrompt
	if err := call(t, m, "auth.begin", nil, &pr); err != nil || !strings.HasPrefix(pr.URL, "https://amazon.com/ap/signin") {
		t.Fatalf("begin: %+v, %v", pr, err)
	}
	redirect := "https://www.amazon.com/?openid.oa2.authorization_code=AQCODE123&openid.oa2.token_type=bearer"
	if err := call(t, m, "auth.complete", map[string]any{"fields": map[string]string{"code": redirect}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("complete: %+v, %v", st, err)
	}
	if !strings.Contains(st.User, "AmazonUser") {
		t.Fatalf("user: %+v", st)
	}
}

func waitJob(t *testing.T, q *jobs.Queue, jobID string) types.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, x := range q.List() {
			if x.ID == jobID && (x.State == jobs.StateDone || x.State == jobs.StateFailed) {
				return x
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job never finished: %+v", q.List())
	return types.Job{}
}

func TestAmazonAuth(t *testing.T) {
	m, _, _, _ := setup(t)
	var st types.AuthStatus
	if err := call(t, m, "auth.status", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("status: %+v, %v", st, err)
	}
	var pr types.AuthPrompt
	if err := call(t, m, "auth.begin", nil, &pr); err != nil || !strings.HasPrefix(pr.URL, "https://amazon.com/ap/signin") || len(pr.Fields) != 1 {
		t.Fatalf("begin: %+v, %v", pr, err)
	}
	if err := call(t, m, "auth.complete", map[string]any{"fields": map[string]string{"code": "no code here"}}, &st); err == nil {
		t.Fatal("address without a code accepted")
	}
	login(t, m)
	if err := call(t, m, "auth.logout", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("logout: %+v, %v", st, err)
	}
}

func TestAmazonLibraryInstallLaunch(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib) != 2 || lib[0].StoreAppID != "amzn1.tt.1" || lib[0].Title != "Amazon Game" || lib[0].IsInstalled {
		t.Fatalf("library: %+v", lib)
	}
	game := lib[0]

	var size plugins.InstallSize
	if err := call(t, m, "game.installSize", map[string]any{"id": "amzn1.tt.1"}, &size); err != nil || size.DiskBytes != 600*MIB {
		t.Fatalf("size: %+v, %v", size, err)
	}

	j, err := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, q, j.ID)
	if done.State != jobs.StateDone || done.Total != 400*MIB || done.Percent != 100 {
		t.Fatalf("install: %+v", done)
	}

	lib, _ = store.Library(ctx)
	game = lib[0]
	if !game.IsInstalled || game.OutsideFolder || game.Native {
		t.Fatalf("after install: %+v", game)
	}

	l, err := store.Launch(ctx, game, true)
	if err != nil || l.Direct == nil {
		t.Fatalf("launch: %+v, %v", l, err)
	}
	if !l.Direct.NeedsWine || !strings.HasSuffix(l.Direct.Exe, "game.exe") {
		t.Fatalf("direct: %+v", l.Direct)
	}

	u, err := q.Add(jobs.KindUninstall, id, game.ID, game.StoreAppID, game.Title, folder)
	if err != nil {
		t.Fatal(err)
	}
	if got := waitJob(t, q, u.ID); got.State != jobs.StateDone {
		t.Fatalf("uninstall: %+v", got)
	}
	lib, _ = store.Library(ctx)
	if lib[0].IsInstalled {
		t.Fatal("still installed")
	}
}
