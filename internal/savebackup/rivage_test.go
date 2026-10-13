package savebackup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/types"
)

// The Rivage layout (issue #60): a sideloaded game with compat Steam AppID
// 4094660 and a custom wine_prefix where pfx is a symlink to . and
// drive_c/users/<login> is a symlink to steamuser. The manifest entry is
// "<winLocalAppData>/Rivage/Saved/SaveGames/**/*.sav" with when store steam.
//
// The real prefix and data dir are never touched: everything is rebuilt in
// t.TempDir().
const rivageYAML = `
Rivage:
  files:
    <winLocalAppData>/Rivage/Saved/SaveGames/**/*.sav:
      tags: [save]
      when:
        - store: steam
`

// rivagePrefix rebuilds Rivage's prefix layout: pfx → ., a steamuser folder
// holding the saves, and users/<login> → steamuser. It returns the prefix
// (the custom wine_prefix) and the login the link is named after.
func rivagePrefix(t *testing.T, login string) string {
	t.Helper()
	prefix := t.TempDir()
	saves := filepath.Join(prefix, "drive_c", "users", "steamuser", "AppData", "Local", "Rivage", "Saved", "SaveGames")
	write := func(rel, content string) {
		p := filepath.Join(saves, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("savefile.sav", "save1")
	write("settings.sav", "settings")
	write("Fury/campaign.sav", "campaign")
	write("Fury/deep/nested.sav", "nested")
	write("Fury/notes.txt", "not a save")
	if err := os.Symlink(".", filepath.Join(prefix, "pfx")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("steamuser", filepath.Join(prefix, "drive_c", "users", login)); err != nil {
		t.Fatal(err)
	}
	return prefix
}

func rivageGame(prefix string) types.Game {
	game := types.Game{ID: "rivage", Source: types.SourceCustom, Title: "Rivage"}
	game.Settings.WinePrefix = prefix
	return game
}

func rivageIndex(t *testing.T, cacheDir string) {
	t.Helper()
	if _, err := BuildIndex([]byte(rivageYAML), `"rivage"`, cacheDir); err != nil {
		t.Fatal(err)
	}
}

// when: store: steam must match a sideloaded game carrying a compat Steam
// AppID, and must not match a game with no store id at all.
func TestRivageStoreConstraintMatchesCompatAppID(t *testing.T) {
	t.Parallel()
	games, err := ParseManifest([]byte(rivageYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("games = %d, want 1", len(games))
	}
	fe := games[0].Files[0]
	if len(fe.WhenStore) != 1 || fe.WhenStore[0] != "steam" {
		t.Fatalf("whenStore = %v, want [steam]", fe.WhenStore)
	}
	home := t.TempDir()
	game := types.Game{ID: "rivage", Source: types.SourceCustom, Title: "Rivage"}
	wine := NewContextFor(game, home, "", t.TempDir(), "/runners/proton", "")
	if !whenOK(fe, wine, "4094660", "") {
		t.Error("when store steam rejected a sideloaded game with compat AppID 4094660")
	}
	if whenOK(fe, wine, "", "") {
		t.Error("when store steam matched a game with no store id")
	}
}

// The full Rivage case through a Proton runner: the ** entry finds
// savefile.sav, settings.sav and the Fury/ contents, and the backup holds
// exactly those saves.
func TestRivageBackupFindsSaves(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dataDir := filepath.Join(home, "data")
	cacheDir := filepath.Join(home, "cache")
	rivageIndex(t, cacheDir)
	prefix := rivagePrefix(t, "polynux")
	game := rivageGame(prefix)

	settings := newTestSettings(t, home)
	svc := New(Options{Settings: settings, DataDir: dataDir, CacheDir: cacheDir, Home: home})
	b, _, _, _, err := svc.Backup(game, "manual", BackupOptions{RunnerPath: "/runners/proton", SteamID: "4094660"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Files != 4 {
		t.Fatalf("backup files = %d, want 4 (savefile, settings, campaign, nested)", b.Files)
	}
	meta, ok := ReadBackupMeta(svc.backupsRoot(), game.ID, b.ID)
	if !ok {
		t.Fatal("no backup metadata")
	}
	var rels []string
	for _, f := range meta.Files {
		if f.Root != "prefix" {
			t.Errorf("file root = %q, want prefix", f.Root)
		}
		rels = append(rels, filepath.ToSlash(f.Path))
	}
	for _, want := range []string{
		"drive_c/users/steamuser/AppData/Local/Rivage/Saved/SaveGames/savefile.sav",
		"drive_c/users/steamuser/AppData/Local/Rivage/Saved/SaveGames/settings.sav",
		"drive_c/users/steamuser/AppData/Local/Rivage/Saved/SaveGames/Fury/campaign.sav",
		"drive_c/users/steamuser/AppData/Local/Rivage/Saved/SaveGames/Fury/deep/nested.sav",
	} {
		found := false
		for _, r := range rels {
			if r == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("backup holds %v, missing %s", rels, want)
		}
	}
}

// The same layout through plain Wine, where the Windows user is the login
// and users/<login> is a symlink to steamuser: the path through the link
// resolves and the saves are still found.
func TestRivagePlainWineLoginLinkResolves(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	login := currentLogin()
	prefix := rivagePrefix(t, login)
	game := rivageGame(prefix)
	ctx := NewContextFor(game, home, "", filepath.Join(home, "data"), "/usr/bin/wine", "")
	if ctx.WinUser != login {
		t.Fatalf("WinUser = %q, want the login %q", ctx.WinUser, login)
	}
	games, err := ParseManifest([]byte(rivageYAML))
	if err != nil {
		t.Fatal(err)
	}
	roots, patterns, _ := ResolvePaths(game, nil, &games[0], ctx, "4094660", "")
	if len(patterns) != 1 {
		t.Fatalf("patterns = %d, want 1", len(patterns))
	}
	_ = roots
	files, skipped, _, n, _ := Enumerate(patterns, ctx)
	if n != 4 {
		t.Fatalf("files = %v (skipped %d), want 4 saves through the login link", files, skipped)
	}
}

// ResolvePaths lists the SaveGames folder itself so GetSavePaths shows it as
// found automatically and existing.
func TestRivageRootsListedAsFound(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	prefix := rivagePrefix(t, "polynux")
	game := rivageGame(prefix)
	ctx := NewContextFor(game, home, "", filepath.Join(home, "data"), "/runners/proton", "")
	games, err := ParseManifest([]byte(rivageYAML))
	if err != nil {
		t.Fatal(err)
	}
	roots, _, _ := ResolvePaths(game, nil, &games[0], ctx, "4094660", "")
	want := filepath.Join(prefix, "pfx", "drive_c", "users", "steamuser", "AppData", "Local", "Rivage", "Saved", "SaveGames")
	found := false
	for _, r := range roots {
		if r.Path == want {
			found = true
			if r.Origin != "manifest" {
				t.Errorf("origin = %q, want manifest", r.Origin)
			}
			if !r.Exists {
				t.Error("SaveGames listed as missing, want existing")
			}
		}
	}
	if !found {
		t.Fatalf("roots = %v, missing the SaveGames folder", roots)
	}
}

// A symlinked file inside the save root is counted skipped, and symlinked
// subfolders (a loop back into the saves, a link outside the prefix) are
// never followed.
func TestRivageSymlinksNotFollowed(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	prefix := rivagePrefix(t, "polynux")
	saves := filepath.Join(prefix, "drive_c", "users", "steamuser", "AppData", "Local", "Rivage", "Saved", "SaveGames")
	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.sav"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(saves, "savefile.sav"), filepath.Join(saves, "alias.sav")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(saves, "Fury"), filepath.Join(saves, "Loop")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(saves, "Out")); err != nil {
		t.Fatal(err)
	}
	game := rivageGame(prefix)
	ctx := NewContextFor(game, home, "", filepath.Join(home, "data"), "/runners/proton", "")
	games, err := ParseManifest([]byte(rivageYAML))
	if err != nil {
		t.Fatal(err)
	}
	_, patterns, _ := ResolvePaths(game, nil, &games[0], ctx, "4094660", "")
	files, skipped, _, n, _ := Enumerate(patterns, ctx)
	if n != 4 {
		t.Fatalf("files = %v, want the 4 real saves (no loop/outside copies)", files)
	}
	for _, f := range files {
		if strings.Contains(f.Abs, "Loop") || strings.Contains(f.Abs, "Out") || strings.Contains(f.Abs, "secret") {
			t.Errorf("followed a symlinked subfolder: %s", f.Abs)
		}
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1 (the alias.sav link)", skipped)
	}
}

// Two spellings of one file (through users/<login> and users/steamuser, or
// overlapping ** and * patterns) back it up once.
func TestRivageNoDuplicateFiles(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	prefix := rivagePrefix(t, "polynux")
	saves := filepath.Join(prefix, "pfx", "drive_c", "users", "steamuser", "AppData", "Local", "Rivage", "Saved", "SaveGames")
	linked := filepath.Join(prefix, "pfx", "drive_c", "users", "polynux", "AppData", "Local", "Rivage", "Saved", "SaveGames")
	game := rivageGame(prefix)
	ctx := NewContextFor(game, home, "", filepath.Join(home, "data"), "/runners/proton", "")
	patterns := []resolvedPattern{
		{Pattern: filepath.Join(saves, "*.sav"), Root: "prefix", Origin: "manifest"},
		{Pattern: filepath.Join(linked, "*.sav"), Root: "prefix", Origin: "manifest"},
		{Pattern: filepath.Join(saves, "**", "*.sav"), Root: "prefix", Origin: "manifest"},
	}
	_, _, _, n, _ := Enumerate(patterns, ctx)
	if n != 4 {
		t.Fatalf("files = %d, want 4 with no duplicates", n)
	}
}

func TestPrefixForNormalisesOnce(t *testing.T) {
	t.Parallel()
	if p, u := prefixFor("/x/pfx", "polynux", true); p != "/x/pfx" || u != "steamuser" {
		t.Errorf("prefixFor(/x/pfx) = %q, %q; want /x/pfx steamuser", p, u)
	}
	if p, u := prefixFor("/x", "polynux", true); p != "/x/pfx" || u != "steamuser" {
		t.Errorf("prefixFor(/x) = %q, %q; want /x/pfx steamuser", p, u)
	}
	if p, u := prefixFor("/x", "polynux", false); p != "/x" || u != "polynux" {
		t.Errorf("prefixFor plain = %q, %q; want /x polynux", p, u)
	}
}

func TestWinUserDirPicksTheRealFolder(t *testing.T) {
	t.Parallel()
	driveC := filepath.Join(t.TempDir(), "drive_c")
	steam := filepath.Join(driveC, "users", "steamuser")
	if err := os.MkdirAll(steam, 0o755); err != nil {
		t.Fatal(err)
	}
	// Login folder missing: fall back to steamuser (a Proton-made prefix
	// under plain Wine).
	if got := winUserDir(driveC, "polynux"); got != steam {
		t.Errorf("winUserDir without login = %q, want %q", got, steam)
	}
	// Login folder present (even as a link): use it.
	if err := os.Symlink("steamuser", filepath.Join(driveC, "users", "polynux")); err != nil {
		t.Fatal(err)
	}
	if got, want := winUserDir(driveC, "polynux"), filepath.Join(driveC, "users", "polynux"); got != want {
		t.Errorf("winUserDir with login link = %q, want %q", got, want)
	}
	if winUserDir("", "polynux") != "" {
		t.Error("winUserDir without drive_c should be empty")
	}
}

// ** matches zero or more folder levels, * stays within one level, and ? and
// [...] match single characters and classes. Depth and file-count caps keep
// a ** over a huge tree bounded.
func TestDoubleStarEdgeCases(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("saves/top.sav")
	mk("saves/a/mid.sav")
	mk("saves/a/b/deep.sav")
	mk("saves/file1.sav")
	mk("saves/file2.sav")
	mk("saves/file3.bak")
	mk("saves/a/mid.bak")

	glob := func(pattern string) []string {
		m, err := expandGlob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			t.Fatal(err)
		}
		var rel []string
		for _, f := range m {
			r, err := filepath.Rel(root, f)
			if err != nil {
				t.Fatal(err)
			}
			rel = append(rel, filepath.ToSlash(r))
		}
		return rel
	}
	has := func(list []string, want string) bool {
		for _, g := range list {
			if g == want {
				return true
			}
		}
		return false
	}

	// ** spans zero, one and two levels.
	all := glob("saves/**/*.sav")
	for _, want := range []string{"saves/top.sav", "saves/a/mid.sav", "saves/a/b/deep.sav", "saves/file1.sav"} {
		if !has(all, want) {
			t.Errorf("**/*.sav = %v, missing %s", all, want)
		}
	}
	for _, g := range all {
		if strings.HasSuffix(g, ".bak") {
			t.Errorf("**/*.sav matched %s", g)
		}
	}
	// * stays within its own level.
	one := glob("saves/*.sav")
	if !has(one, "saves/top.sav") || has(one, "saves/a/mid.sav") {
		t.Errorf("saves/*.sav = %v, want only the top level", one)
	}
	// ? and [...] work per character.
	if q := glob("saves/file?.sav"); len(q) != 2 {
		t.Errorf("file?.sav = %v, want file1+file2", q)
	}
	if c := glob("saves/file[12].sav"); len(c) != 2 {
		t.Errorf("file[12].sav = %v, want file1+file2", c)
	}
	if c := glob("saves/file[12].bak"); len(c) != 0 {
		t.Errorf("file[12].bak = %v, want nothing", c)
	}
}

func TestDoubleStarDepthCap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	deep := root
	for i := 0; i < globMaxDepth+3; i++ {
		deep = filepath.Join(deep, "d")
		if err := os.MkdirAll(deep, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(deep, "level.sav"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := expandGlob(filepath.Join(root, "**", "*.sav"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("** found nothing in the nested chain")
	}
	bottom := filepath.Join(deep, "level.sav")
	for _, f := range m {
		if f == bottom {
			t.Errorf("depth cap let the walk reach level %d: %s", globMaxDepth+3, f)
		}
	}
	// Shallow levels are still found.
	if _, err := os.Lstat(m[0]); err != nil {
		t.Errorf("first match vanished: %v", err)
	}
}

func TestDoubleStarFileCountCap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	saves := filepath.Join(root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < globMaxFiles+5; i++ {
		p := filepath.Join(saves, strings.Repeat("f", 4)+string(rune('a'+i%26))+string(rune('a'+(i/26)%26))+string(rune('a'+(i/676)%26))+string(rune('a'+(i/17576)%26))+".sav")
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := expandGlob(filepath.Join(saves, "*.sav"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != globMaxFiles {
		t.Fatalf("matches = %d, want the %d cap", len(m), globMaxFiles)
	}
}
