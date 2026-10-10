package app

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/settings"
	"platypus/internal/types"
)

// Install location around the echo test plugin: the folder the user picks
// becomes the job's installDir, after the same guards as a move.

// installEnv is a versions environment whose home is a temporary folder, so
// the home and data-folder guards act on folders the test owns.
func installEnv(t *testing.T, protocols, caps string) *versionsEnv {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	return versionsApp(t, protocols, caps)
}

// bareInstallApp is an App with settings only, for the folder guards, which
// need no plugin. Its home is a temporary folder.
func bareInstallApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	st, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &App{settings: st}
}

func TestCheckInstallFolderGuards(t *testing.T) {
	e := struct{ a *App }{bareInstallApp(t)}
	home, _ := os.UserHomeDir()
	ok := filepath.Join(t.TempDir(), "ssd", "Games") // doesn't exist yet: created on the first install
	got, err := e.a.CheckInstallFolder(ok)
	if err != nil {
		t.Fatalf("a new folder under a writable parent was refused: %v", err)
	}
	if got.Folder != ok || got.Exists || got.FreeBytes <= 0 {
		t.Fatalf("check = %+v: want the clean path, not existing, with the room of its parent", got)
	}
	if again, err := e.a.CheckInstallFolder(ok + "/./"); err != nil || again.Folder != ok {
		t.Fatalf("an unclean path = %+v, %v; want it cleaned", again, err)
	}

	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "sneaky")
	if err := os.Symlink("/usr/lib", link); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"relative":             "Games/here",
		"empty":                "",
		"root":                 "/",
		"system folder":        "/usr/share/games",
		"home itself":          home,
		"a file":               file,
		"inside a file":        filepath.Join(file, "sub"),
		"symlink to /usr":      link,
		"below a symlink":      filepath.Join(link, "x"),
		"platypus data folder": filepath.Join(home, ".local/share/platypus/plugins"),
	}
	for name, folder := range cases {
		if _, err := e.a.CheckInstallFolder(folder); err == nil {
			t.Errorf("%s (%q): accepted", name, folder)
		}
	}
}

func TestCheckInstallFolderRefusesAFolderTheUserCantWriteTo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	e := struct{ a *App }{bareInstallApp(t)}
	ro := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	for _, folder := range []string{ro, filepath.Join(ro, "Games")} {
		if _, err := e.a.CheckInstallFolder(folder); err == nil {
			t.Errorf("%s: accepted", folder)
		}
	}
}

func TestInstallGameToUsesTheChosenFolderAndRemembersIt(t *testing.T) {
	e := installEnv(t, allVersions, echoCaps)
	chosen := filepath.Join(t.TempDir(), "ssd")
	j, err := e.a.InstallGameTo(e.game, chosen)
	if err != nil {
		t.Fatal(err)
	}
	if j.InstallDir != chosen {
		t.Fatalf("job installDir = %q, want %q", j.InstallDir, chosen)
	}
	e.waitJob(t, j.ID, jobs.StateDone)

	if got := e.a.settings.RecentInstallFolders(); !slices.Equal(got, []string{chosen}) {
		t.Fatalf("recent folders = %v", got)
	}
	// The plugin may launch from it: the game would read as "outside" otherwise.
	if folders := e.a.plugins.Folders("echo", e.a.settings.GamesFolder()); !slices.Contains(folders, chosen) {
		t.Fatalf("allowed folders = %v, want %s among them", folders, chosen)
	}
	// The default location is not "a folder picked by hand".
	def, _ := e.a.plugins.InstallFolder("echo", e.a.settings.GamesFolder())
	e.game.IsInstalled = false
	j2, err := e.a.InstallGameTo(e.game, def)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJob(t, j2.ID, jobs.StateDone)
	if got := e.a.settings.RecentInstallFolders(); !slices.Equal(got, []string{chosen}) {
		t.Fatalf("installing into the default changed the recent folders: %v", got)
	}
}

func TestInstallGameToWithoutAFolderIsTheNormalInstall(t *testing.T) {
	e := installEnv(t, allVersions, echoCaps)
	j, err := e.a.InstallGameTo(e.game, "")
	if err != nil {
		t.Fatal(err)
	}
	def, _ := e.a.plugins.InstallFolder("echo", e.a.settings.GamesFolder())
	if j.InstallDir != def {
		t.Fatalf("job installDir = %q, want the plugin's %q", j.InstallDir, def)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if len(e.a.settings.RecentInstallFolders()) != 0 {
		t.Fatal("the default install was remembered as a picked folder")
	}
}

func TestInstallGameToRefusesWithoutQueueingAnything(t *testing.T) {
	e := installEnv(t, allVersions, echoCaps)
	for _, folder := range []string{"relative/dir", "/usr/games", "/"} {
		if _, err := e.a.InstallGameTo(e.game, folder); err == nil {
			t.Errorf("%q: accepted", folder)
		}
	}
	if n := len(e.a.jobs.List()); n != 0 {
		t.Fatalf("%d jobs queued by refused installs", n)
	}
	if len(e.a.settings.RecentInstallFolders()) != 0 || len(e.a.plugins.Folders("echo", e.a.settings.GamesFolder())) != 1 {
		t.Fatal("a refused install left a recent or an allowed folder")
	}

	// An installed game moves with Move; it is not installed again elsewhere.
	e.game.IsInstalled = true
	if _, err := e.a.InstallGameTo(e.game, t.TempDir()); err == nil {
		t.Error("an installed game was installed again")
	}
	e.game.IsInstalled, e.game.Installable = false, false
	if _, err := e.a.InstallGameTo(e.game, t.TempDir()); err == nil {
		t.Error("a game Platypus can't install was queued")
	}
}

func TestInstallGameWithOptionsToUsesTheChosenFolder(t *testing.T) {
	e := optionsEnv(t)
	t.Setenv("HOME", t.TempDir())
	chosen := filepath.Join(t.TempDir(), "big-disk", "Games")
	j, err := e.a.InstallGameWithOptionsTo(e.game, map[string]any{"language": "fr"}, chosen)
	if err != nil {
		t.Fatal(err)
	}
	if j.InstallDir != chosen || j.Options["language"] != "fr" {
		t.Fatalf("job = installDir %q options %v", j.InstallDir, j.Options)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if saved, ok := e.a.settings.Options(e.game.ID); !ok || saved["language"] != "fr" {
		t.Fatalf("options not remembered: %v", saved)
	}

	// A refused folder queues nothing and remembers no choices.
	fresh := optionsEnv(t)
	if _, err := fresh.a.InstallGameWithOptionsTo(fresh.game, map[string]any{"language": "fr"}, "/etc"); err == nil {
		t.Fatal("/etc accepted")
	}
	if len(fresh.a.jobs.List()) != 0 {
		t.Fatal("a refused install queued a job")
	}
	if _, ok := fresh.a.settings.Options(fresh.game.ID); ok {
		t.Fatal("a refused install saved its options")
	}
}

func TestInstallLocationsListDefaultLibrariesAndRecents(t *testing.T) {
	e := installEnv(t, allVersions, echoCaps)
	def, _ := e.a.plugins.InstallFolder("echo", e.a.settings.GamesFolder())
	ssd := filepath.Join(t.TempDir(), "ssd")
	if _, err := e.a.AddGameFolder(ssd); err != nil {
		t.Fatal(err)
	}
	picked := filepath.Join(t.TempDir(), "picked")
	for _, f := range []string{picked, def, filepath.Join(ssd, "echo")} { // the last two repeat a listed place
		if err := e.a.settings.AddRecentInstallFolder(f); err != nil {
			t.Fatal(err)
		}
	}
	locs, err := e.a.GetInstallLocations(e.game)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range locs {
		got = append(got, l.Kind+" "+l.Folder)
		if l.FreeBytes <= 0 {
			t.Errorf("%s: free = %d", l.Folder, l.FreeBytes)
		}
	}
	// The old default is a library now, after the default and before the added folder.
	libs := e.a.settings.GameFolders()
	want := []string{"default " + def, "library " + filepath.Join(libs[0], "echo"), "library " + filepath.Join(ssd, "echo"), "recent " + picked}
	if !slices.Equal(got, want) {
		t.Fatalf("locations =\n%v\nwant\n%v", got, want)
	}
	if locs[2].Root != ssd {
		t.Errorf("library root = %q", locs[2].Root)
	}
	if _, err := e.a.GetInstallLocations(types.Game{Source: "steam", Title: "x"}); err == nil {
		t.Error("a game that is not from a plugin got install locations")
	}
}

func TestGameFoldersAreChecked(t *testing.T) {
	e := struct{ a *App }{bareInstallApp(t)}
	list := e.a.GetGameFolders()
	if len(list) != 1 || !list[0].IsDefault || list[0].Path != e.a.settings.GamesFolder() {
		t.Fatalf("folders = %+v: want the Games folder alone, marked default", list)
	}
	if _, err := e.a.AddGameFolder("/usr/local/games"); err == nil {
		t.Error("a system folder was added")
	}
	ssd := filepath.Join(t.TempDir(), "ssd")
	list, err := e.a.AddGameFolder(ssd)
	if err != nil || len(list) != 2 || list[1].Path != ssd || list[1].IsDefault || list[1].Exists {
		t.Fatalf("AddGameFolder = %+v, %v", list, err)
	}
	if list, err = e.a.RemoveGameFolder(ssd); err != nil || len(list) != 1 {
		t.Fatalf("RemoveGameFolder = %+v, %v", list, err)
	}
	if _, err := e.a.RemoveGameFolder(e.a.settings.GamesFolder()); err == nil {
		t.Error("the default folder was removed")
	}
}

func TestInstallSizeIsCachedAndUnknownWhenTheStoreCantSay(t *testing.T) {
	e := installEnv(t, allVersions, echoCaps)
	first, err := e.a.GetInstallSize(e.game)
	if err != nil || first <= 0 {
		t.Fatalf("size = %d, %v", first, err)
	}
	if again, err := e.a.GetInstallSize(e.game); err != nil || again != first {
		t.Fatalf("second size = %d, %v", again, err)
	}
	if n := e.calls(t)["game.installSize"]; n != 1 {
		t.Fatalf("game.installSize reached the plugin %d times, want 1 (cached)", n)
	}

	t.Setenv("ECHO_MISSING", "game.installSize")
	blind := installEnv(t, allVersions, echoCaps)
	if _, err := blind.a.GetInstallSize(blind.game); err == nil {
		t.Fatal("a store that can't tell gave a size")
	}
}

func TestSizeCacheExpiresAndAnswersConcurrentAsksWithOneCall(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	var c sizeCache
	c.now = func() time.Time { return now }
	var calls atomic.Int32
	size := int64(5)
	fetch := func() (int64, error) { calls.Add(1); return size, nil }

	if v, _ := c.get("g", nil, fetch); v != 5 {
		t.Fatalf("size = %d", v)
	}
	size = 9
	now = now.Add(sizeTTL - time.Second)
	if v, _ := c.get("g", nil, fetch); v != 5 || calls.Load() != 1 {
		t.Fatalf("within the ttl: size %d, %d calls", v, calls.Load())
	}
	now = now.Add(2 * time.Second)
	if v, _ := c.get("g", nil, fetch); v != 9 || calls.Load() != 2 {
		t.Fatalf("after the ttl: size %d, %d calls", v, calls.Load())
	}
	// Another version of the game is another entry.
	if v, _ := c.get("g", &types.VersionTarget{Build: "b1"}, fetch); v != 9 || calls.Load() != 3 {
		t.Fatalf("a pinned version: size %d, %d calls", v, calls.Load())
	}

	// A failure is remembered, but for a shorter time.
	fail := func() (int64, error) { calls.Add(1); return 0, os.ErrInvalid }
	before := calls.Load()
	if _, err := c.get("bad", nil, fail); err == nil {
		t.Fatal("no error")
	}
	if _, err := c.get("bad", nil, fail); err == nil || calls.Load() != before+1 {
		t.Fatalf("a failure was not remembered (%d calls)", calls.Load()-before)
	}
	now = now.Add(sizeFailTTL + time.Second)
	_, _ = c.get("bad", nil, fail)
	if calls.Load() != before+2 {
		t.Fatalf("a failure outlived its ttl (%d calls)", calls.Load()-before)
	}

	// Concurrent asks share one call.
	release := make(chan struct{})
	var slow atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, _ := c.get("slow", nil, func() (int64, error) { slow.Add(1); <-release; return 7, nil })
			if v != 7 {
				t.Errorf("size = %d", v)
			}
		}()
	}
	for slow.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // let the others queue behind the first call
	close(release)
	wg.Wait()
	if slow.Load() != 1 {
		t.Fatalf("%d concurrent calls, want 1", slow.Load())
	}
}
