package savebackup

import (
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/testenv"
	"platypus/internal/types"
)

// fixtureYAML covers every matching axis: a steam id, a gog id, an alias, a
// when filter, globs and a unicode title. Config-tagged entries never back up.
const fixtureYAML = `
# a comment before the first key is dropped by the chunk split
Tést Game ☆:
  aliases:
    - Test Game GOTY
  steam:
    id: 12345
  gog:
    id: 67890
  files:
    <winDocuments>/My Games/TestGame/*.sav:
      tags: [save]
    <home>/.local/share/testgame/saves:
      tags: [save]
      when:
        - os: linux
    <winAppData>/TestGame/config.ini:
      tags: [config]
    <winLocalAppData>/TestGame/*.dat:
      tags: [save]
      when:
        os: windows
Config Only Game:
  steam:
    id: 99999
  files:
    <winAppData>/ConfigOnly/settings.ini:
      tags: [config]
Ambiguous Title:
  steam:
    id: 11111
  files:
    <home>/.ambiguous/save.dat:
      tags: [save]
Ambiguous Title!:
  steam:
    id: 22222
  files:
    <home>/.ambiguous2/save.dat:
      tags: [save]
`

func parseFixture(t *testing.T) []GameEntry {
	t.Helper()
	games, err := ParseManifest([]byte(fixtureYAML))
	if err != nil {
		t.Fatal(err)
	}
	return games
}

func TestChunkSplitKeepsEveryGame(t *testing.T) {
	t.Parallel()
	chunks := SplitManifest([]byte(fixtureYAML))
	if len(chunks) != 4 {
		t.Fatalf("chunks = %d, want 4", len(chunks))
	}
}

func TestFixtureIndexBuild(t *testing.T) {
	t.Parallel()
	games := parseFixture(t)
	if len(games) != 3 {
		t.Fatalf("games = %d, want 3 (config-only entries hold no saves)", len(games))
	}
	var main *GameEntry
	for i := range games {
		if games[i].SteamID == "12345" {
			main = &games[i]
		}
	}
	if main == nil {
		t.Fatal("no entry with steam id 12345")
	}
	if main.GogID != "67890" {
		t.Errorf("gog id = %q, want 67890", main.GogID)
	}
	if len(main.Aliases) != 1 || main.Aliases[0] != "Test Game GOTY" {
		t.Errorf("aliases = %v", main.Aliases)
	}
	if len(main.Files) != 3 {
		t.Errorf("files = %d, want 3 (config entry excluded)", len(main.Files))
	}
}

func TestMatchBySteamGogAliasAndTitle(t *testing.T) {
	t.Parallel()
	idx := &Index{Games: parseFixture(t)}
	if m := Match(idx, "whatever", "12345", ""); m == nil || m.SteamID != "12345" {
		t.Error("no match by steam id")
	}
	if m := Match(idx, "whatever", "", "67890"); m == nil || m.GogID != "67890" {
		t.Error("no match by gog id")
	}
	if m := Match(idx, "test game goty", "", ""); m == nil || m.SteamID != "12345" {
		t.Error("no match by alias")
	}
	if m := Match(idx, "TÉST game ☆", "", ""); m == nil || m.SteamID != "12345" {
		t.Error("no match by normalised unicode title")
	}
	if m := Match(idx, "Ambiguous Title", "", ""); m != nil {
		t.Error("ambiguous title matched, want no match")
	}
	if m := Match(idx, "No Such Game", "", ""); m != nil {
		t.Error("unknown title matched")
	}
}

// fakePrefix builds a Wine prefix (plain or Proton's pfx layout) with save
// files under every placeholder root.
func fakePrefix(t *testing.T, proton bool) (prefix, user string) {
	t.Helper()
	root := t.TempDir()
	user = "testuser"
	base := root
	if proton {
		base = filepath.Join(root, "pfx")
		user = "steamuser"
	}
	write := func(rel, content string) {
		p := filepath.Join(base, "drive_c", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join("users", user, "Documents", "My Games", "TestGame", "slot1.sav"), "save1")
	write(filepath.Join("users", user, "AppData", "Roaming", "TestGame", "roam.sav"), "save2")
	write(filepath.Join("users", user, "AppData", "Local", "TestGame", "loc.sav"), "save3")
	write(filepath.Join("users", user, "AppData", "LocalLow", "TestGame", "low.sav"), "save4")
	write(filepath.Join("users", "Public", "TestGame", "pub.sav"), "save5")
	write(filepath.Join("ProgramData", "TestGame", "prog.sav"), "save6")
	write(filepath.Join("windows", "TestGame", "win.sav"), "save7")
	return root, user
}

func TestPlaceholdersInFakePrefixes(t *testing.T) {
	t.Parallel()
	for _, proton := range []bool{false, true} {
		prefix, user := fakePrefix(t, proton)
		facts := Prefix{Dir: t.TempDir()}
		if proton {
			facts.User = "steamuser"
		}
		game := types.Game{ID: "g", Title: "T", InstallDir: t.TempDir()}
		ctx := NewContextFor(game, t.TempDir(), "", facts)
		// Point the prefix at the fake one.
		ctx.Roots["prefix"] = prefix
		if proton {
			ctx.Roots["prefix"] = filepath.Join(prefix, "pfx")
		}
		ctx.WinUser = user
		cases := map[string]string{
			"<winAppData>/TestGame/roam.sav":             "save2",
			"<winLocalAppData>/TestGame/loc.sav":         "save3",
			"<winLocalAppDataLow>/TestGame/low.sav":      "save4",
			"<winDocuments>/My Games/TestGame/slot1.sav": "save1",
			"<winPublic>/TestGame/pub.sav":               "save5",
			"<winProgramData>/TestGame/prog.sav":         "save6",
			"<winDir>/TestGame/win.sav":                  "save7",
		}
		for pattern, want := range cases {
			expanded, un := expandPattern(pattern, ctx, game)
			if un != 0 || len(expanded) != 1 {
				t.Errorf("proton=%v %s: expanded=%v unresolved=%d", proton, pattern, expanded, un)
				continue
			}
			data, err := os.ReadFile(expanded[0])
			if err != nil || string(data) != want {
				t.Errorf("proton=%v %s: read %q, %v", proton, pattern, data, err)
			}
		}
		// <storeUserId> is unresolvable: the path is skipped and counted.
		if _, un := expandPattern("<storeUserId>/saves/x.sav", ctx, game); un != 1 {
			t.Errorf("proton=%v: storeUserId unresolved = %d, want 1", proton, un)
		}
	}
}

func TestHomeXdgBaseGameRootPlaceholders(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	install := filepath.Join(home, "Games", "T")
	for _, rel := range []string{".local/share/t/s.sav", ".config/t/c.sav", "Games/T/g.sav"} {
		p := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	game := types.Game{ID: "g", Title: "T", InstallDir: install}
	ctx := NewContextFor(game, home, "", Prefix{})
	for pattern := range map[string]bool{
		"<home>/.local/share/t/s.sav": true,
		"<xdgData>/t/s.sav":           true,
		"<xdgConfig>/t/c.sav":         true,
		"<base>/g.sav":                true,
		"<root>/<game>/g.sav":         true,
		"<root>/T/g.sav":              true,
	} {
		expanded, un := expandPattern(pattern, ctx, game)
		if un != 0 || len(expanded) != 1 {
			t.Errorf("%s: expanded=%v unresolved=%d", pattern, expanded, un)
		}
	}
	for _, pattern := range []string{"<base>/g.sav", "<root>/<game>/g.sav"} {
		if expanded, _ := expandPattern(pattern, ctx, game); len(expanded) != 1 || expanded[0] != filepath.Join(install, "g.sav") {
			t.Errorf("%s expanded to %v, want the install folder's g.sav", pattern, expanded)
		}
	}
	if _, un := expandPattern("<osUserName>/x", ctx, game); un != 0 {
		t.Errorf("osUserName unresolved = %d", un)
	}
}

func TestWhenFilter(t *testing.T) {
	t.Parallel()
	games := parseFixture(t)
	var main GameEntry
	for _, g := range games {
		if g.SteamID == "12345" {
			main = g
		}
	}
	home := t.TempDir()
	game := types.Game{ID: "g", Title: "Tést Game ☆", InstallDir: home}
	// Native game: only the linux entry applies.
	native := NewContextFor(game, home, "", Prefix{})
	var linux, windows int
	for _, fe := range main.Files {
		if whenOK(fe, native, "12345", "") {
			linux++
		}
	}
	// Windows game: the unfiltered + windows entries apply.
	wine := NewContextFor(game, home, "", Prefix{Dir: t.TempDir()})
	for _, fe := range main.Files {
		if whenOK(fe, wine, "12345", "") {
			windows++
		}
	}
	if linux != 2 {
		t.Errorf("native matches = %d, want 2 (unfiltered + linux)", linux)
	}
	if windows != 2 {
		t.Errorf("wine matches = %d, want 2 (unfiltered + windows)", windows)
	}
}

func TestSymlinksAndEscapesSkipped(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	saves := filepath.Join(home, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "ok.sav"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsideDir, err := os.MkdirTemp("", "platypus-outside")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(outsideDir) })
	outside := filepath.Join(outsideDir, "outside.sav")
	if err := os.WriteFile(outside, []byte("evil"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(saves, "link.sav")); err != nil {
		t.Fatal(err)
	}
	game := types.Game{ID: "g", Title: "T", InstallDir: home}
	ctx := NewContextFor(game, home, "", Prefix{})
	patterns := []resolvedPattern{
		{Pattern: filepath.Join(saves, "*.sav"), Root: "home", Origin: "user"},
		{Pattern: outside, Root: "", Origin: "user"},
	}
	files, skipped, _, n, _ := Enumerate(patterns, ctx)
	if n != 1 || files[0].Abs != filepath.Join(saves, "ok.sav") {
		t.Errorf("files = %v, want only ok.sav", files)
	}
	if skipped != 2 {
		t.Errorf("skipped = %d, want 2 (symlink + escape)", skipped)
	}
}

func TestRoundTripDedupeRetention(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dataDir := filepath.Join(home, "data")
	saves := filepath.Join(home, "game-saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "slot.sav"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := newTestSettings(t, home)
	svc := New(Options{Settings: settings, DataDir: dataDir, CacheDir: filepath.Join(home, "cache"), Home: home})
	game := types.Game{ID: "g1", Source: "custom", Title: "G1", InstallDir: home}
	o := BackupOptions{UserPaths: []string{saves}}
	b1, _, _, _, err := svc.Backup(game, "manual", o)
	if err != nil {
		t.Fatal(err)
	}
	if b1.Files != 1 || b1.Bytes != 2 {
		t.Errorf("backup = %+v, want 1 file 2 bytes", b1)
	}
	// Dedupe: unchanged saves return the newest backup with Unchanged set.
	b2, _, _, _, err := svc.Backup(game, "manual", o)
	if err != nil {
		t.Fatal(err)
	}
	if !b2.Unchanged || b2.ID != b1.ID {
		t.Errorf("dedupe = %+v, want unchanged reuse of %s", b2, b1.ID)
	}
	if got := len(ListBackups(svc.backupsRoot(), "g1")); got != 1 {
		t.Errorf("backups = %d, want 1 after dedupe", got)
	}
	// Mutate, back up, restore over a clobber: byte-identical.
	if err := os.WriteFile(filepath.Join(saves, "slot.sav"), []byte("v2-longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	b3, _, _, _, err := svc.Backup(game, "manual", o)
	if err != nil {
		t.Fatal(err)
	}
	if b3.Unchanged || b3.ID == b1.ID {
		t.Errorf("changed saves reused the backup: %+v", b3)
	}
	if err := os.WriteFile(filepath.Join(saves, "slot.sav"), []byte("clobbered!!"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := NewContextFor(game, home, "", Prefix{})
	if _, _, err := svc.Restore(game, b3.ID, ctx, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(saves, "slot.sav"))
	if string(data) != "v2-longer" {
		t.Errorf("restored = %q, want v2-longer", data)
	}
	// Retention: keep 2, newest never pruned.
	if err := settings.SetKeepBackups(2); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if err := os.WriteFile(filepath.Join(saves, "slot.sav"), []byte(strings.Repeat("x", i+3)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, _, _, err := svc.Backup(game, "manual", o); err != nil {
			t.Fatal(err)
		}
	}
	list := ListBackups(svc.backupsRoot(), "g1")
	if len(list) != 2 {
		t.Fatalf("backups = %d, want 2 after retention", len(list))
	}
	if list[0].ID == b1.ID {
		t.Error("newest backup was pruned")
	}
}

func TestWriteFailureLeavesNoVersion(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	// A file where the backups folder should be: every write fails.
	blocker := filepath.Join(home, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := newTestSettings(t, home)
	svc := New(Options{Settings: settings, DataDir: blocker, CacheDir: filepath.Join(home, "cache"), Home: home})
	game := types.Game{ID: "g", Source: "custom", Title: "G", InstallDir: home}
	saves := filepath.Join(home, "s")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := svc.Backup(game, "manual", BackupOptions{UserPaths: []string{saves}}); err == nil {
		t.Fatal("backup succeeded into a file, want an error")
	}
	if got := ListBackups(svc.backupsRoot(), "g"); len(got) != 0 {
		t.Errorf("versions left after failure: %v", got)
	}
}

func TestZipSlipRefused(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dataDir := filepath.Join(home, "data")
	settings := newTestSettings(t, home)
	svc := New(Options{Settings: settings, DataDir: dataDir, CacheDir: filepath.Join(home, "cache"), Home: home})
	game := types.Game{ID: "g", Source: "custom", Title: "G", InstallDir: home}
	saves := filepath.Join(home, "s")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _, _, _, err := svc.Backup(game, "manual", BackupOptions{UserPaths: []string{saves}})
	if err != nil {
		t.Fatal(err)
	}
	// A backup whose embedded metadata names an escaping path refuses the
	// restore outright: rewrite the zip's metadata in place.
	poison(t, filepath.Join(svc.backupsRoot(), "g", b.ID+".zip"))
	ctx := NewContextFor(game, home, "", Prefix{})
	if _, _, err := Restore(svc.backupsRoot(), "g", b.ID, ctx); err == nil {
		t.Fatal("restore of a zip-slip backup succeeded, want refusal")
	}
	if _, err := os.Stat(filepath.Join(home, "evil.sav")); !os.IsNotExist(err) {
		t.Error("zip-slip restore wrote outside the roots")
	}
}

func TestBeforeRestoreBackupBlocks(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dataDir := filepath.Join(home, "data")
	settings := newTestSettings(t, home)
	svc := New(Options{Settings: settings, DataDir: dataDir, CacheDir: filepath.Join(home, "cache"), Home: home})
	game := types.Game{ID: "g", Source: "custom", Title: "G", InstallDir: home}
	saves := filepath.Join(home, "s")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _, _, _, err := svc.Backup(game, "manual", BackupOptions{UserPaths: []string{saves}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewContextFor(game, home, "", Prefix{})
	before, _ := os.ReadFile(filepath.Join(saves, "a.sav"))
	if _, _, err := svc.Restore(game, b.ID, ctx, func() error {
		return errTestSafety
	}); err == nil || !strings.Contains(err.Error(), "before-restore") {
		t.Fatalf("restore err = %v, want the before-restore block", err)
	}
	after, _ := os.ReadFile(filepath.Join(saves, "a.sav"))
	if string(before) != string(after) {
		t.Error("blocked restore still wrote files")
	}
	// With a working safety backup the restore proceeds and leaves a
	// before-restore version behind (mutate first so dedupe writes anew).
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := func() int { return len(ListBackups(svc.backupsRoot(), "g")) }
	n0 := names()
	if _, _, err := svc.Restore(game, b.ID, ctx, func() error {
		_, _, _, _, err := svc.Backup(game, "before-restore", BackupOptions{UserPaths: []string{saves}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if names() != n0+1 {
		t.Error("no before-restore backup was written")
	}
}

func TestManifestETag304And200(t *testing.T) {
	t.Parallel()
	var lastIfNone string
	etag := `"v1"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastIfNone = r.Header.Get("If-None-Match")
		if lastIfNone == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte(fixtureYAML))
	}))
	defer srv.Close()
	cache := filepath.Join(t.TempDir(), "cache")
	changed, newEtag, data, err := FetchManifest(cache, srv.URL, "", nil)
	if err != nil || !changed || newEtag != etag {
		t.Fatalf("first fetch = %v %q %v", changed, newEtag, err)
	}
	idx, err := StoreManifest(cache, data, newEtag)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Games) != 3 {
		t.Fatalf("index games = %d, want 3", len(idx.Games))
	}
	// Second fetch sends If-None-Match and keeps the cache on 304.
	changed, _, _, err = FetchManifest(cache, srv.URL, StoredEtag(cache), nil)
	if err != nil || changed {
		t.Fatalf("second fetch = %v %v, want unchanged", changed, err)
	}
	if lastIfNone != etag {
		t.Errorf("If-None-Match = %q, want %q", lastIfNone, etag)
	}
	if st := ManifestState(cache); st != "ready" {
		t.Errorf("state = %q, want ready", st)
	}
	// A changed manifest rebuilds the index.
	etag = `"v2"`
	changed, newEtag, data, err = FetchManifest(cache, srv.URL, StoredEtag(cache), nil)
	if err != nil || !changed {
		t.Fatalf("third fetch = %v %v, want changed", changed, err)
	}
	if _, err := StoreManifest(cache, data, newEtag); err != nil {
		t.Fatal(err)
	}
	if got := StoredEtag(cache); got != `"v2"` {
		t.Errorf("stored etag = %q", got)
	}
}

func TestIndexRoundTripThroughGzip(t *testing.T) {
	t.Parallel()
	cache := filepath.Join(t.TempDir(), "cache")
	idx, err := BuildIndex([]byte(fixtureYAML), `"e"`, cache)
	if err != nil {
		t.Fatal(err)
	}
	loaded := LoadIndex(cache)
	if loaded == nil || len(loaded.Games) != len(idx.Games) {
		t.Fatalf("loaded = %+v", loaded)
	}
	f, err := os.Open(indexPath(cache))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	var back Index
	if err := json.NewDecoder(gz).Decode(&back); err != nil {
		t.Fatal(err)
	}
	if back.Etag != `"e"` || back.Version != IndexVersion {
		t.Errorf("meta = %+v", back)
	}
}

var errTestSafety = errTestSafetySentinel()

func errTestSafetySentinel() error { return &testSafetyError{} }

type testSafetyError struct{}

func (e *testSafetyError) Error() string { return "safety net failed" }

// poison rewrites a backup zip's embedded metadata to name an escaping path,
// simulating a hostile archive for the zip-slip test.
func poison(t *testing.T, zipPath string) {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		name string
		data []byte
	}
	var entries []entry
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			zr.Close()
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			zr.Close()
			t.Fatal(err)
		}
		entries = append(entries, entry{f.Name, data})
	}
	zr.Close()
	var meta types.BackupMeta
	for _, e := range entries {
		if e.name == "platypus-backup.json" {
			if err := json.Unmarshal(e.data, &meta); err != nil {
				t.Fatal(err)
			}
		}
	}
	meta.Files = append(meta.Files, types.SaveFile{Root: "home", Path: "../evil.sav", Size: 4})
	mb, _ := json.Marshal(meta)
	out, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	for _, e := range entries {
		data := e.data
		if e.name == "platypus-backup.json" {
			data = mb
		}
		w, err := zw.Create(e.name)
		if err != nil {
			out.Close()
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			out.Close()
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

var _ = testenv.Slow

// A game moved since its backup restores into its current folder: root
// labels resolve against the game as it is now, not where it was.
func TestRestoreFollowsAMovedInstallFolder(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dataDir := filepath.Join(home, "data")
	oldDir := filepath.Join(home, "Games", "Old")
	if err := os.MkdirAll(filepath.Join(oldDir, "saves"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "saves", "slot.sav"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := New(Options{Settings: newTestSettings(t, home), DataDir: dataDir, CacheDir: filepath.Join(home, "cache"), Home: home})
	game := types.Game{ID: "g1", Source: "custom", Title: "G1", InstallDir: oldDir}
	b, _, _, _, err := svc.Backup(game, "manual", BackupOptions{UserPaths: []string{filepath.Join(oldDir, "saves")}})
	if err != nil || b.Files != 1 {
		t.Fatalf("backup = %+v, %v", b, err)
	}
	newDir := filepath.Join(home, "Games", "New")
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(newDir, "saves", "slot.sav")); err != nil {
		t.Fatal(err)
	}
	game.InstallDir = newDir
	ctx := NewContextFor(game, home, "", Prefix{})
	if _, added, err := svc.Restore(game, b.ID, ctx, nil); err != nil || added != 1 {
		t.Fatalf("restore added %d, %v; want 1 file", added, err)
	}
	if data, err := os.ReadFile(filepath.Join(newDir, "saves", "slot.sav")); err != nil || string(data) != "v1" {
		t.Fatalf("restored into the moved folder = %q, %v", data, err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("restore recreated the old folder: %v", err)
	}
}

// Game and backup ids can't climb out of the backups folder.
func TestBackupIDsStayInsideTheBackupsFolder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, id := range []string{"..", ".", "", "a/../../x", `a\b`} {
		if dir := gameBackupDir(root, id); filepath.Dir(dir) != root {
			t.Errorf("game id %q maps to %s, outside %s", id, dir, root)
		}
	}
	for _, id := range []string{"", ".", "..", "../x", "a/b", `a\b`} {
		if p, ok := backupZip(root, "g", id); ok {
			t.Errorf("backup id %q accepted as %s", id, p)
		}
	}
	svc := New(Options{Settings: newTestSettings(t, root), DataDir: root, CacheDir: filepath.Join(root, "cache"), Home: root})
	if err := svc.Delete("g", "../../escape"); err == nil {
		t.Error("deleting a climbing backup id was accepted")
	}
}

// Nothing on disk: no empty version is written.
func TestBackupWithNothingFoundWritesNothing(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	svc := New(Options{Settings: newTestSettings(t, home), DataDir: filepath.Join(home, "data"), CacheDir: filepath.Join(home, "cache"), Home: home})
	game := types.Game{ID: "g1", Source: "custom", Title: "G1", InstallDir: home}
	_, _, _, _, err := svc.Backup(game, "manual", BackupOptions{UserPaths: []string{filepath.Join(home, "missing")}})
	if !errors.Is(err, ErrNothingFound) {
		t.Fatalf("err = %v, want ErrNothingFound", err)
	}
	if got := svc.List("g1"); len(got) != 0 {
		t.Fatalf("an empty backup was written: %+v", got)
	}
}

// An index from an older format is rebuilt from the cached manifest, with no
// download (issue #60: version 2 carries when: store constraints).
func TestOldIndexIsRebuiltFromTheCachedManifest(t *testing.T) {
	t.Parallel()
	cache := filepath.Join(t.TempDir(), "cache")
	if err := os.MkdirAll(filepath.Dir(manifestPath(cache)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath(cache), []byte(fixtureYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(indexPath(cache))
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	if err := json.NewEncoder(gz).Encode(Index{Version: IndexVersion - 1}); err != nil {
		t.Fatal(err)
	}
	gz.Close()
	f.Close()
	idx := LoadIndex(cache)
	if idx == nil || idx.Version != IndexVersion || len(idx.Games) == 0 {
		t.Fatalf("old index not rebuilt: %+v", idx)
	}
	if again := LoadIndex(cache); again == nil || again.Version != IndexVersion {
		t.Fatalf("rebuilt index not written: %+v", again)
	}
}
