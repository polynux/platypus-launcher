// Tests the GOG plugin through Hydra's real plugin path (manager, JSON-RPC,
// job queue, launch-spec validation) against testdata/gogdl, a fake CLI that
// prints the real gogdl's output formats. It doesn't reach GOG.
package gog_test

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

const id = "gog-gogdl"

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

func setup(t *testing.T) (*plugins.Manager, *jobs.Queue, *stores.Plugin, string) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	root := t.TempDir()
	plug := filepath.Join(root, "plugins", id)
	copyFile(t, "plugin.json", filepath.Join(plug, "plugin.json"), 0o644)
	copyFile(t, "bin/hydra-gog", filepath.Join(plug, "bin/hydra-gog"), 0o755)
	bin := filepath.Join(root, "fakebin")
	copyFile(t, "testdata/gogdl", filepath.Join(bin, "gogdl"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	state := filepath.Join(root, "gogdl-state.json")
	t.Setenv("FAKE_GOGDL_STATE", state)
	// The plugin's GOG API calls answer from this file instead of the network:
	// a userData.json body, and the store search under _test.
	userdata := filepath.Join(root, "userdata.json")
	os.WriteFile(userdata, []byte(`{"username": "GogUser", "ownedGames": [{"id": 1234, "title": "Super Game", "image": "/img/box"}], "_test": {"products": [{"id": 1234, "description": "A super game.", "genres": ["Action"], "galleryImages": [{"formattedUrl": "//images.gog.com{formattedType}"}]}]}}`), 0o644)
	t.Setenv("HYDRA_GOG_API_FILE", userdata)

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

func call(t *testing.T, m *plugins.Manager, method string, params, result any) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return m.Call(ctx, id, method, params, result)
}

func login(t *testing.T, m *plugins.Manager) {
	t.Helper()
	var st types.AuthStatus
	if err := call(t, m, "auth.complete", map[string]any{"fields": map[string]string{"code": "https://embed.gog.com/on_login_success?origin=client&code=" + "Xa1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("login: %+v, %v", st, err)
	}
}

func (c calls) check(t *testing.T, want string) {
	t.Helper()
	if !strings.Contains(c.body, want) {
		t.Fatalf("calls %q don't contain %q", c.body, want)
	}
}

type calls struct{ body string }

func readCalls(t *testing.T) calls {
	t.Helper()
	state := os.Getenv("FAKE_GOGDL_STATE")
	data, _ := os.ReadFile(state + ".calls")
	return calls{body: string(data)}
}

func TestGogAuth(t *testing.T) {
	m, _, _, _ := setup(t)
	var st types.AuthStatus
	if err := call(t, m, "auth.status", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("status: %+v, %v", st, err)
	}
	var pr types.AuthPrompt
	if err := call(t, m, "auth.begin", nil, &pr); err != nil || !strings.HasPrefix(pr.URL, "https://auth.gog.com/auth") || len(pr.Fields) != 1 {
		t.Fatalf("begin: %+v, %v", pr, err)
	}
	bad := map[string]any{"fields": map[string]string{"code": "wrong code"}}
	if err := call(t, m, "auth.complete", bad, &st); err == nil {
		t.Fatal("bad code accepted")
	}
	login(t, m)
	if err := call(t, m, "auth.logout", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("logout: %+v, %v", st, err)
	}
}

func TestGogInstallLaunchUninstall(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib) != 1 || lib[0].StoreAppID != "1234" || lib[0].Title != "Super Game" || lib[0].IsInstalled {
		t.Fatalf("library: %+v", lib)
	}
	game := lib[0]

	var size plugins.InstallSize
	if err := call(t, m, "game.installSize", map[string]any{"id": "1234"}, &size); err != nil || size.DiskBytes != 12<<20 {
		t.Fatalf("size: %+v, %v", size, err)
	}
	readCalls(t).check(t, "info 1234 --platform linux")

	j, err := q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	var done types.Job
	for time.Now().Before(deadline) {
		for _, x := range q.List() {
			if x.ID == j.ID && (x.State == jobs.StateDone || x.State == jobs.StateFailed) {
				done = x
			}
		}
		if done.ID != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if done.State != jobs.StateDone || done.Total != 8<<20 || done.Bytes != 8<<20 {
		t.Fatalf("install: %+v", done)
	}
	readCalls(t).check(t, "download 1234 --path "+folder+" --platform linux --skip-dlcs")

	lib, err = store.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	game = lib[0]
	if !game.IsInstalled || game.OutsideFolder || !game.Native {
		t.Fatalf("after install: %+v", game)
	}

	l, err := store.Launch(ctx, game, false)
	if err != nil || l.Direct == nil {
		t.Fatalf("launch: %+v, %v", l, err)
	}
	if l.Direct.NeedsWine || !strings.HasSuffix(l.Direct.Exe, "start.sh") || l.Direct.Dir != filepath.Dir(l.Direct.Exe) {
		t.Fatalf("direct: %+v", l.Direct)
	}
	// The game itself runs: the fake's start.sh echoes.
	out, err := exec.Command(l.Direct.Exe, l.Direct.Args...).Output()
	if err != nil || !strings.Contains(string(out), "gog game") {
		t.Fatalf("game script: %s, %v", out, err)
	}

	d, err := store.Details(ctx, game)
	if err != nil || d.Summary != "A super game." || len(d.Genres) != 1 || len(d.Screenshots) != 1 || !strings.HasSuffix(d.Screenshots[0], ".1920x1080.jpg") {
		t.Fatalf("details: %+v, %v", d, err)
	}

	u, err := q.Add(jobs.KindUninstall, id, game.ID, game.StoreAppID, game.Title, folder)
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(20 * time.Second)
	for {
		list := q.List()
		for _, x := range list {
			if x.ID == u.ID && x.State == jobs.StateDone {
				lib, _ := store.Library(ctx)
				if lib[0].IsInstalled {
					t.Fatal("still installed")
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("uninstall never finished: %+v", list)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
