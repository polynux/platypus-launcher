package migrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"platypus/internal/datadir"
)

type logs struct{ info, warn []string }

func (l *logs) opts() Options {
	return Options{
		Info: func(f string, a ...any) { l.info = append(l.info, fmt.Sprintf(f, a...)) },
		Warn: func(f string, a ...any) { l.warn = append(l.warn, fmt.Sprintf(f, a...)) },
	}
}

func write(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func gone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s should not exist (err %v)", path, err)
	}
}

// oldHome builds a home with a Hydra-era data folder holding one of every
// persisted file that carries absolute paths. The strings use the old root.
func oldHome(t *testing.T) (home, oldRoot string) {
	t.Helper()
	datadir.Use("")
	t.Cleanup(func() { datadir.Use("") })
	home = t.TempDir()
	oldRoot = filepath.Join(home, ".local/share/hydra")
	p := func(s string) string { return filepath.Join(oldRoot, s) }

	write(t, p("settings.json"), fmt.Sprintf(`{
  "version": 1,
  "games_folder": "%[2]s/Games",
  "games": {"steam:1": {"wine_prefix": "%[1]s/prefixes/custom", "launch_options": "WINEPREFIX=%[1]s/prefixes/x gamemoderun %%command%%"}},
  "plugins": {"echo": {"hash": "abc", "enabled": true, "install_dir": "%[2]s/Games/echo", "folders": ["%[2]s/Old"]}},
  "relocations": {"echo:tiny": {"from": "%[1]s/stuff", "to": "%[2]s/Games/elsewhere"}},
  "game_folders": ["%[1]s/shared", "/mnt/disk%[1]s/not-ours", "%[1]s-extra/also-not"]
}
`, oldRoot, home), 0o644)
	write(t, p("jobs.json"), fmt.Sprintf(`{"jobs":[{"id":"j1","plugin":"steam-standalone","install_dir":"%s/Games/steam/x","state":"running"}]}`, home), 0o644)
	write(t, p("install-history.json"), fmt.Sprintf(`{"entries":[{"path":"%s/ledger"}]}`, oldRoot), 0o644)
	write(t, p("cloud.json"), fmt.Sprintf(`{"pending":{"echo:tiny":{"paths":["%s/plugin-data/echo/cloud-backups/tiny"]}}}`, oldRoot), 0o644)
	write(t, p("custom-art/customization.json"), `{"version":1,"games":{"a":{"art":{"cover":{"file":"cover.png","v":3}}}}}`, 0o644)
	write(t, p("custom-art/a/cover.png"), "PNG", 0o644)
	write(t, p("compat-cache.json"), `{"version":1}`, 0o644)
	write(t, p("plugin-data/steam-standalone/refresh.token"), "SECRET-TOKEN", 0o600)
	write(t, p("plugin-data/steam-standalone/state.json"), fmt.Sprintf(`{"logged_in":true,"dir":"%s/plugin-data/steam-standalone/jobs"}`, oldRoot), 0o600)
	write(t, p("plugin-data/steam-standalone/manifests/1/2_3.manifest"), "BIN"+oldRoot, 0o644)
	// A Wine prefix is moved as a folder; its files are never edited.
	write(t, p("prefixes/steam:1/pfx/system.reg"), "WINE "+oldRoot, 0o644)
	write(t, p("prefixes/steam:1/pfx/note.json"), fmt.Sprintf(`{"p":"%s"}`, oldRoot), 0o644)
	write(t, p("logs/steam:1.log"), "old log", 0o644)
	// Plugins: one built before the rename, one data plugin, one built since.
	write(t, p("plugins/echo/plugin.json"), `{"id":"echo","exec":"bin/hydra-echo"}`, 0o644)
	write(t, p("plugins/echo/bin/hydra-echo"), "#!/bin/sh\n", 0o755)
	write(t, p("plugins/custom/plugin.json"), `{"id":"custom","exec":"run.py"}`, 0o644)
	write(t, p("plugins/custom/run.py"), "import os\nos.environ.get('HYDRA_PLUGIN_DATA')\n", 0o755)
	write(t, p("plugins/fixes/plugin.json"), `{"id":"fixes","kind":"data","data":["fixes.json"]}`, 0o644)
	write(t, p("plugins/fixes/fixes.json"), `[]`, 0o644)
	write(t, p("plugins/fresh/plugin.json"), `{"id":"fresh","exec":"bin/platypus-fresh"}`, 0o644)
	write(t, p("plugins/fresh/bin/platypus-fresh"), "#!/bin/sh\n", 0o755)
	return home, oldRoot
}

func checkMigrated(t *testing.T, home, newRoot string, rep Report) {
	t.Helper()
	p := func(s string) string { return filepath.Join(newRoot, s) }
	if rep.Root != newRoot || rep.Err != nil {
		t.Fatalf("report = %+v", rep)
	}

	var s struct {
		GamesFolder string                    `json:"games_folder"`
		Games       map[string]map[string]any `json:"games"`
		Plugins     map[string]struct {
			Hash       string   `json:"hash"`
			Enabled    bool     `json:"enabled"`
			InstallDir string   `json:"install_dir"`
			Folders    []string `json:"folders"`
		} `json:"plugins"`
		Relocations map[string]map[string]string `json:"relocations"`
		GameFolders []string                     `json:"game_folders"`
	}
	if err := json.Unmarshal([]byte(read(t, p("settings.json"))), &s); err != nil {
		t.Fatal(err)
	}
	if s.GamesFolder != home+"/Games" {
		t.Errorf("games_folder changed to %q (it is outside the data folder)", s.GamesFolder)
	}
	if got := s.Games["steam:1"]["wine_prefix"]; got != newRoot+"/prefixes/custom" {
		t.Errorf("wine_prefix = %v", got)
	}
	if got := s.Games["steam:1"]["launch_options"]; got != "WINEPREFIX="+newRoot+"/prefixes/x gamemoderun %command%" {
		t.Errorf("an old path inside a longer string = %v", got)
	}
	if st := s.Plugins["echo"]; st.Hash != "abc" || !st.Enabled || st.InstallDir != home+"/Games/echo" || st.Folders[0] != home+"/Old" {
		t.Errorf("plugin approval = %+v (the hash is kept; it simply stops matching)", st)
	}
	if r := s.Relocations["echo:tiny"]; r["from"] != newRoot+"/stuff" || r["to"] != home+"/Games/elsewhere" {
		t.Errorf("relocation = %v", r)
	}
	oldRoot := filepath.Join(home, ".local/share/hydra")
	if s.GameFolders[0] != newRoot+"/shared" || s.GameFolders[1] != "/mnt/disk"+oldRoot+"/not-ours" || s.GameFolders[2] != oldRoot+"-extra/also-not" {
		t.Errorf("game_folders = %v (only whole path prefixes are rewritten)", s.GameFolders)
	}
	for file, want := range map[string]string{
		"jobs.json":            home + "/Games/steam/x",
		"install-history.json": newRoot + "/ledger",
		"cloud.json":           newRoot + "/plugin-data/echo/cloud-backups/tiny",
		"plugin-data/steam-standalone/state.json": newRoot + "/plugin-data/steam-standalone/jobs",
	} {
		if got := read(t, p(file)); !strings.Contains(got, want) || strings.Contains(got, oldRoot) {
			t.Errorf("%s = %s, want %s and no old root", file, got, want)
		}
	}
	if got := read(t, p("custom-art/customization.json")); !strings.Contains(got, `"file":"cover.png"`) {
		t.Errorf("customization index damaged: %s", got)
	}
	// Secrets move untouched and keep their permissions.
	if read(t, p("plugin-data/steam-standalone/refresh.token")) != "SECRET-TOKEN" {
		t.Error("refresh token changed")
	}
	for _, f := range []string{"plugin-data/steam-standalone/refresh.token", "plugin-data/steam-standalone/state.json"} {
		if fi, _ := os.Stat(p(f)); fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v", f, fi.Mode().Perm())
		}
	}
	// Prefixes, logs and binary state are never edited.
	if !strings.Contains(read(t, p("prefixes/steam:1/pfx/system.reg")), oldRoot) || !strings.Contains(read(t, p("prefixes/steam:1/pfx/note.json")), oldRoot) {
		t.Error("a Wine prefix was edited")
	}
	if !strings.Contains(read(t, p("plugin-data/steam-standalone/manifests/1/2_3.manifest")), oldRoot) {
		t.Error("a non-JSON state file was edited")
	}
	read(t, p("logs/steam:1.log"))
	// Plugins built before the rename are set aside; the others stay.
	for _, id := range []string{"echo", "custom"} {
		gone(t, p("plugins/"+id))
		read(t, p("plugins-before-rename/"+id+"/plugin.json"))
	}
	read(t, p("plugins-before-rename/README.txt"))
	read(t, p("plugins/fixes/plugin.json"))
	read(t, p("plugins/fresh/plugin.json"))
	if len(rep.SetAside) != 2 {
		t.Errorf("SetAside = %v", rep.SetAside)
	}
	gone(t, p(pendingMarker))
	// No temp files left by the rewrite.
	if m, _ := filepath.Glob(p(".rewrite-*")); len(m) > 0 {
		t.Errorf("leftover %v", m)
	}
}

func TestFreshInstallCreatesNothing(t *testing.T) {
	datadir.Use("")
	home := t.TempDir()
	var l logs
	rep := Run(home, l.opts())
	want := filepath.Join(home, ".local/share/platypus")
	if rep.Root != want || rep.Moved || rep.Copied || rep.BothExist || rep.Err != nil {
		t.Fatalf("report = %+v", rep)
	}
	gone(t, filepath.Join(home, ".local"))
	if len(l.warn) > 0 {
		t.Fatalf("warnings: %v", l.warn)
	}
}

func TestOldFolderIsRenamedAndPathsFollow(t *testing.T) {
	home, oldRoot := oldHome(t)
	var l logs
	rep := Run(home, l.opts())
	newRoot := filepath.Join(home, ".local/share/platypus")
	if !rep.Moved || rep.Copied {
		t.Fatalf("report = %+v", rep)
	}
	gone(t, oldRoot)
	checkMigrated(t, home, newRoot, rep)
	if len(rep.Rewritten) != 4 { // jobs.json only names places outside the data folder
		t.Errorf("Rewritten = %v", rep.Rewritten)
	}
	// Plugins set aside are announced.
	warned := strings.Join(l.warn, "\n")
	if !strings.Contains(warned, "echo") || !strings.Contains(warned, "install.sh") {
		t.Errorf("warnings = %q", warned)
	}
}

func TestSecondStartDoesNothing(t *testing.T) {
	home, _ := oldHome(t)
	Run(home, Options{})
	before := read(t, filepath.Join(home, ".local/share/platypus/settings.json"))
	var l logs
	rep := Run(home, l.opts())
	if rep.Moved || rep.Copied || rep.BothExist || len(rep.Rewritten) > 0 || len(l.warn) > 0 {
		t.Fatalf("second run did something: %+v %v", rep, l.warn)
	}
	if read(t, filepath.Join(home, ".local/share/platypus/settings.json")) != before {
		t.Fatal("settings changed on the second run")
	}
}

func TestBothFoldersExistUsesNewAndMergesNothing(t *testing.T) {
	home, oldRoot := oldHome(t)
	newRoot := filepath.Join(home, ".local/share/platypus")
	write(t, filepath.Join(newRoot, "settings.json"), `{"version":1,"games_folder":"/fresh"}`, 0o644)
	var l logs
	rep := Run(home, l.opts())
	if rep.Root != newRoot || !rep.BothExist || rep.Moved || rep.Copied {
		t.Fatalf("report = %+v", rep)
	}
	if read(t, filepath.Join(newRoot, "settings.json")) != `{"version":1,"games_folder":"/fresh"}` {
		t.Error("the new settings were touched")
	}
	gone(t, filepath.Join(newRoot, "jobs.json"))
	if !strings.Contains(read(t, filepath.Join(oldRoot, "settings.json")), oldRoot) {
		t.Error("the old folder was edited")
	}
	read(t, filepath.Join(oldRoot, "plugin-data/steam-standalone/refresh.token"))
	if len(l.warn) != 1 || !strings.Contains(l.warn[0], "both") {
		t.Errorf("warnings = %v", l.warn)
	}
}

func TestCrossFilesystemMoveCopiesVerifiesAndKeepsTheOldFolder(t *testing.T) {
	home, oldRoot := oldHome(t)
	var l logs
	o := l.opts()
	o.rename = func(a, b string) error { return &os.LinkError{Op: "rename", Old: a, New: b, Err: syscall.EXDEV} }
	// Something a plain copy would get wrong: a symlink and a read-only dir.
	if err := os.Symlink("../settings.json", filepath.Join(oldRoot, "plugin-data/link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(oldRoot, "custom-art/a"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(oldRoot, "custom-art/a"), 0o755) })
	oldSettings := read(t, filepath.Join(oldRoot, "settings.json"))

	rep := Run(home, o)
	newRoot := filepath.Join(home, ".local/share/platypus")
	if !rep.Copied || rep.Moved {
		t.Fatalf("report = %+v", rep)
	}
	checkMigrated(t, home, newRoot, rep)
	if got, _ := os.Readlink(filepath.Join(newRoot, "plugin-data/link")); got != "../settings.json" {
		t.Errorf("symlink = %q", got)
	}
	if fi, _ := os.Stat(filepath.Join(newRoot, "custom-art/a")); fi.Mode().Perm() != 0o555 {
		t.Errorf("directory mode = %v", fi.Mode().Perm())
	}
	os.Chmod(filepath.Join(newRoot, "custom-art/a"), 0o755)
	gone(t, newRoot+".migrating")
	// The old folder is kept as it was (plus a note), including secrets and plugins.
	if read(t, filepath.Join(oldRoot, "settings.json")) != oldSettings {
		t.Error("the old settings were edited")
	}
	read(t, filepath.Join(oldRoot, "plugin-data/steam-standalone/refresh.token"))
	read(t, filepath.Join(oldRoot, "plugins/echo/bin/hydra-echo"))
	read(t, filepath.Join(oldRoot, keptNote))
	gone(t, filepath.Join(oldRoot, pendingMarker))

	// Next start: two folders, no scary warning, nothing moves.
	var l2 logs
	rep2 := Run(home, l2.opts())
	if !rep2.BothExist || len(l2.warn) != 0 || rep2.Root != newRoot {
		t.Fatalf("second start: %+v %v", rep2, l2.warn)
	}
}

func TestFailedCopyKeepsTheOldFolderAndUsesIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	home, oldRoot := oldHome(t)
	unreadable := filepath.Join(oldRoot, "plugin-data/steam-standalone/refresh.token")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o600) })
	var l logs
	o := l.opts()
	o.rename = func(a, b string) error { return syscall.EXDEV }
	rep := Run(home, o)
	newRoot := filepath.Join(home, ".local/share/platypus")
	if rep.Err == nil || rep.Root != oldRoot || rep.Moved || rep.Copied {
		t.Fatalf("report = %+v", rep)
	}
	gone(t, newRoot)
	gone(t, newRoot+".migrating")
	gone(t, filepath.Join(oldRoot, pendingMarker))
	if got := datadir.Path(home); got != oldRoot {
		t.Errorf("datadir = %s, want the old folder while the move is not done", got)
	}
	if !strings.Contains(read(t, filepath.Join(oldRoot, "settings.json")), oldRoot) {
		t.Error("the old folder was edited")
	}
	// Once the problem is fixed the next start migrates.
	datadir.Use("")
	os.Chmod(unreadable, 0o600)
	rep = Run(home, Options{})
	if !rep.Copied && !rep.Moved {
		t.Fatalf("retry = %+v", rep)
	}
	_ = rep
}

func TestVerifyCatchesADifferentCopy(t *testing.T) {
	src := t.TempDir()
	write(t, filepath.Join(src, "a/b.txt"), "hello", 0o644)
	write(t, filepath.Join(src, "c.txt"), "x", 0o600)

	clone := func() string {
		dst := filepath.Join(t.TempDir(), "d")
		if err := copyTree(src, dst); err != nil {
			t.Fatal(err)
		}
		return dst
	}
	if err := verifyTree(src, clone()); err != nil {
		t.Fatalf("a faithful copy fails: %v", err)
	}
	cases := map[string]func(dst string){
		"content":  func(d string) { write(t, filepath.Join(d, "a/b.txt"), "hellp", 0o644) },
		"missing":  func(d string) { os.Remove(filepath.Join(d, "c.txt")) },
		"extra":    func(d string) { write(t, filepath.Join(d, "extra"), "", 0o644) },
		"mode":     func(d string) { os.Chmod(filepath.Join(d, "c.txt"), 0o644) },
		"kind":     func(d string) { os.Remove(filepath.Join(d, "c.txt")); os.Mkdir(filepath.Join(d, "c.txt"), 0o755) },
		"truncate": func(d string) { os.Truncate(filepath.Join(d, "a/b.txt"), 2) },
	}
	for name, mutate := range cases {
		dst := clone()
		mutate(dst)
		if err := verifyTree(src, dst); err == nil {
			t.Errorf("%s: a different copy passed verification", name)
		}
	}
}

func TestInterruptedMoveIsFinishedAtTheNextStart(t *testing.T) {
	// The folder was renamed (marker inside) but the process died before the
	// paths were rewritten.
	home, oldRoot := oldHome(t)
	write(t, filepath.Join(oldRoot, pendingMarker), oldRoot+"\n", 0o600)
	newRoot := filepath.Join(home, ".local/share/platypus")
	if err := os.Rename(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}
	var l logs
	rep := Run(home, l.opts())
	checkMigrated(t, home, newRoot, rep)
}

func TestRewriteFailureKeepsTheMarkerForARetry(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes anywhere")
	}
	home, _ := oldHome(t)
	newRoot := filepath.Join(home, ".local/share/platypus")
	var l logs
	rep := Run(home, l.opts()) // a clean move first
	if rep.Err != nil {
		t.Fatal(rep.Err)
	}
	// Make a file that needs rewriting live in a read-only folder.
	dir := filepath.Join(newRoot, "plugin-data/ro")
	write(t, filepath.Join(dir, "s.json"), `{"p":"`+filepath.Join(home, ".local/share/hydra/x")+`"}`, 0o644)
	write(t, filepath.Join(newRoot, pendingMarker), "x\n", 0o600)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	rep = Run(home, l.opts())
	if rep.Err == nil {
		t.Fatal("a file that couldn't be written went unnoticed")
	}
	read(t, filepath.Join(newRoot, pendingMarker))
	os.Chmod(dir, 0o755)
	rep = Run(home, l.opts())
	if rep.Err != nil || !strings.Contains(read(t, filepath.Join(dir, "s.json")), newRoot+"/x") {
		t.Fatalf("retry = %+v", rep)
	}
	gone(t, filepath.Join(newRoot, pendingMarker))
}

func TestDataFolderBehindASymlinkMovesAsALink(t *testing.T) {
	datadir.Use("")
	home := t.TempDir()
	disk := t.TempDir()
	write(t, filepath.Join(disk, "settings.json"), fmt.Sprintf(`{"games":{"a":{"wine_prefix":"%s/.local/share/hydra/prefixes/a"}}}`, home), 0o644)
	if err := os.MkdirAll(filepath.Join(home, ".local/share"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(disk, filepath.Join(home, ".local/share/hydra")); err != nil {
		t.Fatal(err)
	}
	rep := Run(home, Options{})
	newRoot := filepath.Join(home, ".local/share/platypus")
	if !rep.Moved || rep.Err != nil {
		t.Fatalf("report = %+v", rep)
	}
	if got := read(t, filepath.Join(disk, "settings.json")); !strings.Contains(got, newRoot+"/prefixes/a") {
		t.Errorf("settings = %s", got)
	}
}

func TestLinkedFolderCopyFollowsTheLink(t *testing.T) {
	datadir.Use("")
	home := t.TempDir()
	disk := t.TempDir()
	write(t, filepath.Join(disk, "jobs.json"), `{}`, 0o644)
	os.MkdirAll(filepath.Join(home, ".local/share"), 0o755)
	os.Symlink(disk, filepath.Join(home, ".local/share/hydra"))
	o := Options{rename: func(a, b string) error { return syscall.EXDEV }}
	rep := Run(home, o)
	if !rep.Copied || rep.Err != nil {
		t.Fatalf("report = %+v", rep)
	}
	fi, err := os.Lstat(filepath.Join(home, ".local/share/platypus"))
	if err != nil || !fi.IsDir() {
		t.Fatalf("new root = %v %v", fi, err)
	}
	read(t, filepath.Join(home, ".local/share/platypus/jobs.json"))
	read(t, filepath.Join(disk, "jobs.json"))
}

func TestRewriteStringsOnlyChangeWholePathPrefixes(t *testing.T) {
	r := newRewriter("/home/u/.local/share/hydra", "/home/u/.local/share/platypus")
	for in, want := range map[string]string{
		"/home/u/.local/share/hydra":                                    "/home/u/.local/share/platypus",
		"/home/u/.local/share/hydra/prefixes/x":                         "/home/u/.local/share/platypus/prefixes/x",
		"~/.local/share/hydra/prefixes/x":                               "/home/u/.local/share/platypus/prefixes/x",
		"$HOME/.local/share/hydra/prefixes/x":                           "/home/u/.local/share/platypus/prefixes/x",
		"${HOME}/.local/share/hydra":                                    "/home/u/.local/share/platypus",
		"A=/home/u/.local/share/hydra/a B=/home/u/.local/share/hydra/b": "A=/home/u/.local/share/platypus/a B=/home/u/.local/share/platypus/b",
		"/mnt/home/u/.local/share/hydra/x":                              "/mnt/home/u/.local/share/hydra/x",
		"/home/u/.local/share/hydra-extra/x":                            "/home/u/.local/share/hydra-extra/x",
		"/home/u/.local/share/hydrax":                                   "/home/u/.local/share/hydrax",
		"/home/u/.local/share/hydr":                                     "/home/u/.local/share/hydr",
		"nothing":                                                       "nothing",
		"":                                                              "",
		"/home/v/.local/share/hydra/x":                                  "/home/v/.local/share/hydra/x",
	} {
		if got, _ := r.str(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	// Keys too, nested; numbers survive exactly.
	dir := t.TempDir()
	f := filepath.Join(dir, "x.json")
	write(t, f, `{"/home/u/.local/share/hydra/k":{"n":12345678901234567890,"l":["/home/u/.local/share/hydra/a",3,null,true,1.50]}}`, 0o640)
	changed, err := r.file(f)
	if err != nil || !changed {
		t.Fatalf("file: %v %v", changed, err)
	}
	got := read(t, f)
	want := `{"/home/u/.local/share/platypus/k":{"l":["/home/u/.local/share/platypus/a",3,null,true,1.50],"n":12345678901234567890}}`
	if got != want {
		t.Errorf("file = %s\nwant   %s", got, want)
	}
	if fi, _ := os.Stat(f); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v", fi.Mode().Perm())
	}
	if changed, _ := r.file(f); changed {
		t.Error("a second rewrite changed the file again")
	}
	// A file that isn't JSON is left alone and reported.
	write(t, filepath.Join(dir, "bad.json"), `{"a": "/home/u/.local/share/hydra/x"`, 0o644)
	if _, err := r.file(filepath.Join(dir, "bad.json")); err == nil {
		t.Error("broken JSON was accepted")
	}
}

func TestPluginNeedingTheOldEnvironmentIsSetAsideWhateverItIsCalled(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "plugins/a/plugin.json"), `{"id":"a","exec":"bin/a"}`, 0o644)
	write(t, filepath.Join(root, "plugins/a/bin/a"), "data := os.Getenv(\"HYDRA_PLUGIN_DATA\")", 0o755)
	write(t, filepath.Join(root, "plugins/b/plugin.json"), `{"id":"b","exec":"bin/b"}`, 0o644)
	write(t, filepath.Join(root, "plugins/b/bin/b"), "os.Getenv(\"PLATYPUS_PLUGIN_DATA\")", 0o755)
	write(t, filepath.Join(root, "plugins/c/plugin.json"), `{"id":"c","exec":"../../etc/passwd"}`, 0o644)
	write(t, filepath.Join(root, "plugins-before-rename/a/old"), "an earlier one", 0o644)
	moved, err := SetAsideLegacyPlugins(root)
	if err != nil || len(moved) != 1 || moved[0] != "a" {
		t.Fatalf("moved = %v, %v", moved, err)
	}
	read(t, filepath.Join(root, "plugins-before-rename/a/old"))
	read(t, filepath.Join(root, "plugins-before-rename/a-2/bin/a"))
	read(t, filepath.Join(root, "plugins/b/bin/b"))
	read(t, filepath.Join(root, "plugins/c/plugin.json"))
	if m, err := SetAsideLegacyPlugins(filepath.Join(root, "nope")); err != nil || m != nil {
		t.Errorf("no plugins folder: %v %v", m, err)
	}
}

func TestMarkerCannotBeWrittenStillMoves(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes anywhere")
	}
	home, oldRoot := oldHome(t)
	os.Chmod(oldRoot, 0o555) // the marker can't go in; the rename needs only the parent's permission
	t.Cleanup(func() { os.Chmod(filepath.Join(home, ".local/share/platypus"), 0o755); os.Chmod(oldRoot, 0o755) })
	rep := Run(home, Options{})
	if !rep.Moved {
		t.Fatalf("report = %+v", rep)
	}
	gone(t, oldRoot)
}
