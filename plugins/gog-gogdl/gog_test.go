// Tests the GOG plugin through Platypus's real plugin path (manager, JSON-RPC,
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

	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
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
		if j.Target != nil {
			p["target"] = j.Target
		}
		if len(j.Options) > 0 {
			p["options"] = j.Options
		}
	}
	if j.Kind == jobs.KindVerify {
		p["repair"] = j.Repair
	}
	return b.m.Call(ctx, j.Source, "game."+j.Kind, p, nil)
}

func (b backend) Cancel(ctx context.Context, j types.Job) error {
	return b.m.Call(ctx, j.Source, "job.cancel", map[string]any{"jobId": j.ID}, nil)
}

func setup(t *testing.T, extra ...map[string]string) (*plugins.Manager, *jobs.Queue, *stores.Plugin, string) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	root := t.TempDir()
	plug := filepath.Join(root, "plugins", id)
	copyFile(t, "plugin.json", filepath.Join(plug, "plugin.json"), 0o644)
	copyFile(t, "bin/platypus-gog", filepath.Join(plug, "bin/platypus-gog"), 0o755)
	bin := filepath.Join(root, "fakebin")
	copyFile(t, "testdata/gogdl", filepath.Join(bin, "gogdl"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	state := filepath.Join(root, "gogdl-state.json")
	t.Setenv("FAKE_GOGDL_STATE", state)
	// The plugin's GOG API calls answer from this file instead of the network:
	// the account library listing, and the product API under _test.
	userdata := filepath.Join(root, "userdata.json")
	os.WriteFile(userdata, []byte(`{"username": "GogUser", "totalPages": 1, "products": [{"id": 1234, "title": "Super Game", "isGame": true, "image": "//img.gog/box"}], "_test": {"id": 1234, "title": "Super Game", "description": {"lead": "A super game."}, "genres": [{"name": "Action"}], "release_date": "2019-09-20T00:00:00Z", "downloads": {"installers": [{"os": "linux", "language": "en-US"}]}, "screenshots": [{"formatter_template_url": "//images.gog.com/x_{formatter}.png", "formatted_images": [{"formatter_name": "ggvgl", "image_url": "https://images.gog.com/x_ggvgl.jpg"}]}]}}`), 0o644)
	t.Setenv("PLATYPUS_GOG_API_FILE", userdata)
	for _, env := range extra {
		for k, v := range env {
			t.Setenv(k, v)
		}
	}

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

// forceLinuxPlatform makes the fixture's game a Linux installer (the plugin
// then installs through gogdl's Linux manager).
func forceLinuxPlatform(t *testing.T) {
	t.Helper()
	path := os.Getenv("PLATYPUS_GOG_API_FILE")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["downloads"] = map[string]any{"installers": []any{map[string]any{"os": "linux"}}}
	products := doc["products"].([]any)
	products[0].(map[string]any)["worksOn"] = map[string]any{"Windows": false, "Mac": false, "Linux": true}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// installFixture installs the setup's only game and returns its library entry.
func installFixture(t *testing.T, q *jobs.Queue, store *stores.Plugin, folder string) types.Game {
	t.Helper()
	return installWith(t, q, store, folder, nil)
}

// installWith installs the setup's only game with the given install options.
func installWith(t *testing.T, q *jobs.Queue, store *stores.Plugin, folder string, options types.InstallOptions) types.Game {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) == 0 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	game := lib[0]
	j, err := q.AddWithOptions(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitState(t, q, j.ID, jobs.StateDone, jobs.StateFailed); done.State != jobs.StateDone {
		t.Fatalf("install: %+v", done)
	}
	lib, err = store.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range lib {
		if g.ID == game.ID {
			if !g.IsInstalled || g.InstallDir == "" {
				t.Fatalf("installed game: %+v", g)
			}
			return g
		}
	}
	t.Fatalf("install didn't show up: %+v", lib)
	return types.Game{}
}

// runVerify queues a check (or repair) and waits for its report.
func runVerify(t *testing.T, q *jobs.Queue, game types.Game, dir string, repair bool) types.Job {
	t.Helper()
	j, err := q.AddVerify(id, game.ID, game.StoreAppID, game.Title, dir, repair)
	if err != nil {
		t.Fatal(err)
	}
	done := waitState(t, q, j.ID, jobs.StateDone, jobs.StateFailed)
	if done.State != jobs.StateDone || done.Verify == nil {
		t.Fatalf("verify (repair %v): %+v", repair, done)
	}
	return done
}

func TestGogVerifyLinuxInstall(t *testing.T) {
	m, q, store, folder := setup(t)
	forceLinuxPlatform(t)
	login(t, m)
	game := installFixture(t, q, store, folder)
	dir := game.InstallDir

	check := func(repair bool) types.Job {
		t.Helper()
		return runVerify(t, q, game, dir, repair)
	}

	done := check(false)
	if done.Verify.Damaged != 0 || done.Verify.Missing != 0 || done.Verify.Repaired != 0 || done.Verify.Checked < 3 {
		t.Fatalf("clean check: %+v", done.Verify)
	}
	files := done.Verify.Checked

	// A corrupted game file: the check reports it, the repair restores it.
	if err := os.WriteFile(filepath.Join(dir, "data.bin"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := check(false).Verify; v.Damaged != 1 || v.Missing != 0 || v.Repaired != 0 || v.Checked != files {
		t.Fatalf("damaged check: %+v", v)
	}
	if v := check(true).Verify; v.Repaired != 1 || v.Damaged != 0 || v.Missing != 0 {
		t.Fatalf("repair: %+v", v)
	}
	readCalls(t).check(t, "repair 1234 --path "+dir+" --platform linux")

	// A deleted game file is missing, and the repair brings it back.
	if err := os.Remove(filepath.Join(dir, "start.sh")); err != nil {
		t.Fatal(err)
	}
	if v := check(false).Verify; v.Missing != 1 || v.Damaged != 0 {
		t.Fatalf("missing check: %+v", v)
	}
	if v := check(true).Verify; v.Repaired != 1 || v.Missing != 0 {
		t.Fatalf("repair of a missing file: %+v", v)
	}
	if v := check(false).Verify; v.Damaged != 0 || v.Missing != 0 {
		t.Fatalf("check after repair: %+v", v)
	}
}

func TestGogVerifyWindowsRepair(t *testing.T) {
	// The Windows install's stored manifest points at a depot list on the
	// fake CDN, which verify hashes file by file.
	m, q, store, folder := setup(t, map[string]string{"PLATYPUS_GOG_API_BASE": fakeDepot(t)})
	login(t, m)
	game := installFixture(t, q, store, folder)
	dir := game.InstallDir

	if v := runVerify(t, q, game, dir, false).Verify; v.Damaged != 0 || v.Missing != 0 || v.Checked != 1 {
		t.Fatalf("clean check: %+v", v)
	}
	// One byte of the game's only depot file changes.
	f, err := os.OpenFile(filepath.Join(dir, "data.bin"), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte{1}, 0); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	if v := runVerify(t, q, game, dir, false).Verify; v.Damaged != 1 || v.Missing != 0 {
		t.Fatalf("damaged check: %+v", v)
	}
	if v := runVerify(t, q, game, dir, true).Verify; v.Repaired != 1 || v.Damaged != 0 || v.Missing != 0 {
		t.Fatalf("repair: %+v", v)
	}
	if v := runVerify(t, q, game, dir, false).Verify; v.Damaged != 0 || v.Missing != 0 {
		t.Fatalf("check after repair: %+v", v)
	}
	readCalls(t).check(t, "repair 1234 --path "+dir+" --platform windows")
}

func TestGogLinuxRepairKeepsLanguageAndDLC(t *testing.T) {
	m, q, store, folder := setup(t)
	forceLinuxPlatform(t)
	login(t, m)
	options := types.InstallOptions{"language": "de-DE", "dlc": []any{"111"}}
	game := installWith(t, q, store, folder, options)
	dir := game.InstallDir

	if b, err := os.ReadFile(filepath.Join(dir, "language.txt")); err != nil || string(b) != "de-DE" {
		t.Fatalf("installed language: %q, %v", b, err)
	}
	// A Linux install has no gogdl manifest; the record keeps the choice.
	raw, err := os.ReadFile(filepath.Join(m.DataDirOf(id), "installed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	if rec["1234"]["language"] != "de-DE" || len(rec["1234"]["dlc"].([]any)) != 1 {
		t.Fatalf("install record: %+v", rec["1234"])
	}
	var opts struct {
		Groups []struct {
			ID      string `json:"id"`
			Default string `json:"default"`
			Values  []struct {
				ID        string `json:"id"`
				Installed bool   `json:"installed"`
			} `json:"values"`
		} `json:"groups"`
	}
	if err := call(t, m, "game.options", map[string]any{"id": "1234"}, &opts); err != nil {
		t.Fatal(err)
	}
	if len(opts.Groups) != 2 || opts.Groups[0].Default != "de-DE" {
		t.Fatalf("game.options: %+v", opts.Groups)
	}
	dlc := opts.Groups[1]
	if dlc.ID != "dlc" || len(dlc.Values) == 0 || dlc.Values[0].ID != "111" || !dlc.Values[0].Installed {
		t.Fatalf("dlc group: %+v", dlc)
	}

	// The DLC file goes missing: the repair must rebuild it with the same
	// language and add-ons, or it would drop what the install chose.
	if err := os.Remove(filepath.Join(dir, "dlc-111.bin")); err != nil {
		t.Fatal(err)
	}
	if v := runVerify(t, q, game, dir, false).Verify; v.Missing != 1 || v.Damaged != 0 {
		t.Fatalf("missing check: %+v", v)
	}
	if v := runVerify(t, q, game, dir, true).Verify; v.Repaired != 1 || v.Missing != 0 {
		t.Fatalf("repair: %+v", v)
	}
	if _, err := os.Stat(filepath.Join(dir, "dlc-111.bin")); err != nil {
		t.Fatalf("repair didn't restore the DLC file: %v", err)
	}
	readCalls(t).check(t, "repair 1234 --path "+dir+" --platform linux --lang de-DE --with-dlcs --dlcs 111")
}

func TestGogVerifyRefusedWithoutManifest(t *testing.T) {
	m, q, store, folder := setup(t)
	forceLinuxPlatform(t)
	login(t, m)
	game := installFixture(t, q, store, folder)
	if err := os.Remove(filepath.Join(game.InstallDir, ".gogdl-linux-manifest")); err != nil {
		t.Fatal(err)
	}
	var junk any
	err := call(t, m, "game.verify", map[string]any{"id": "1234", "installDir": game.InstallDir,
		"jobId": "verify-refused", "repair": false}, &junk)
	if err == nil || !strings.Contains(err.Error(), "no stored game manifest") {
		t.Fatalf("verify without a manifest: %v", err)
	}
}

func TestGogInstallSendsOptions(t *testing.T) {
	m, q, store, folder := setup(t)
	login(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lib, err := store.Library(ctx)
	if err != nil || len(lib) == 0 {
		t.Fatalf("library: %+v, %v", lib, err)
	}
	game := lib[0]
	options := types.InstallOptions{"language": "de-DE", "dlc": []any{"111", "222"}}
	j, err := q.AddWithOptions(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, folder, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitState(t, q, j.ID, jobs.StateDone, jobs.StateFailed); done.State != jobs.StateDone {
		t.Fatalf("install: %+v", done)
	}
	readCalls(t).check(t, "--lang de-DE --with-dlcs --dlcs 111,222")
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
	readCalls(t).check(t, "info 1234 --platform windows")

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
	readCalls(t).check(t, "download 1234 --path "+folder+" --platform windows --skip-dlcs")

	lib, err = store.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	game = lib[0]
	if !game.IsInstalled || game.OutsideFolder {
		t.Fatalf("after install: %+v", game)
	}

	l, err := store.Launch(ctx, game, false)
	if err != nil || l.Direct == nil {
		t.Fatalf("launch: %+v, %v", l, err)
	}
	if !l.Direct.NeedsWine || !strings.HasSuffix(l.Direct.Exe, "start.sh") || l.Direct.Dir != filepath.Dir(l.Direct.Exe) {
		t.Fatalf("direct: %+v", l.Direct)
	}
	// The game itself runs: the fake's start.sh echoes.
	out, err := exec.Command(l.Direct.Exe, l.Direct.Args...).Output()
	if err != nil || !strings.Contains(string(out), "gog game") {
		t.Fatalf("game script: %s, %v", out, err)
	}

	d, err := store.Details(ctx, game)
	if err != nil || d.Summary != "A super game." || len(d.Genres) != 1 || len(d.Screenshots) != 1 || !strings.HasSuffix(d.Screenshots[0], "_ggvgl.jpg") {
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
