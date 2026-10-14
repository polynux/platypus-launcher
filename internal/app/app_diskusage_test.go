package app

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/relocate"
	"platypus/internal/types"
)

// Disk usage grouping (issue #50). Every test works in t.TempDir() trees
// only: several drives are simulated by injecting fake mount points and
// filesystem totals, never by reading real drives.

// writeFileOf creates dir holding one file of n bytes, so a walk reads n.
func writeFileOf(t *testing.T, dir string, n int) {
	t.Helper()
	mustDir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "data.bin"), make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

// walkLog wraps relocate.TreeSize and records every walked path.
type walkLog struct {
	paths []string
}

func (w *walkLog) measure(dir string) (int64, error) {
	w.paths = append(w.paths, dir)
	return relocate.TreeSize(dir)
}

func (w *walkLog) walked(dir string) bool {
	for _, p := range w.paths {
		if p == dir {
			return true
		}
	}
	return false
}

// evalTemp resolves symlinks in a temp dir (TMPDIR itself may be one), so
// comparisons with resolved roots hold.
func evalTemp(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func driveByID(t *testing.T, usage types.DiskUsage, id string) types.DriveUsage {
	t.Helper()
	for _, d := range usage.Drives {
		if d.ID == id {
			return d
		}
	}
	t.Fatalf("no drive %q in %+v", id, usage.Drives)
	return types.DriveUsage{}
}

func TestDiskUsageGroupsTwoDrives(t *testing.T) {
	t.Parallel()
	driveA, driveB, home := evalTemp(t), evalTemp(t), evalTemp(t)
	prefixes := filepath.Join(driveB, "prefixes")

	alpha := filepath.Join(driveA, "Games", "Alpha")
	writeFileOf(t, alpha, 3<<20)
	gamma := filepath.Join(driveA, "Games", "Gamma")
	writeFileOf(t, gamma, 4<<20)
	beta := filepath.Join(driveB, "Games", "Beta")
	writeFileOf(t, beta, 1<<20)
	writeFileOf(t, filepath.Join(prefixes, "g1"), 2<<20)
	pinned := filepath.Join(driveA, "pinned")
	writeFileOf(t, pinned, 1<<20)

	games := []types.Game{
		{ID: "g1", Title: "Alpha", Source: "custom", Store: types.StoreCustom, InstallDir: alpha, Executable: filepath.Join(alpha, "run.sh"), IsInstalled: true},
		{ID: "g2", Title: "Beta", Source: "echo", Store: types.StoreGOG, InstallDir: beta, Executable: filepath.Join(beta, "start.sh"), IsInstalled: true, SizeBytes: 6 << 20, Settings: types.GameSettings{WinePrefix: pinned}},
		{ID: "g3", Title: "Gamma", Source: "custom", Store: types.StoreCustom, InstallDir: gamma, Executable: filepath.Join(gamma, "run.sh"), IsInstalled: true, Native: true},
		{ID: "g4", Title: "Uninstalled", Source: "custom", Store: types.StoreCustom},
	}
	mounts := []diskMount{{point: driveA}, {point: driveB}}
	var walks walkLog
	spaceOf := func(string) (uint64, uint64, error) { return 100 << 30, 40 << 30, nil }
	// alpha overlaps the configured folder and counts once.
	usage := buildDiskUsage(games, []string{filepath.Join(driveA, "Games"), prefixes, alpha}, home, prefixes, mounts, spaceOf, walks.measure)

	if len(usage.Drives) != 2 {
		t.Fatalf("drives = %+v, want 2", usage.Drives)
	}
	a := driveByID(t, usage, driveA)
	if len(a.Folders) != 2 || a.Folders[0] != filepath.Join(driveA, "Games") || a.Folders[1] != pinned {
		t.Fatalf("driveA folders = %v", a.Folders)
	}
	if a.GamesBytes != 8<<20 {
		t.Fatalf("driveA games = %d, want %d", a.GamesBytes, 8<<20)
	}
	if a.TotalBytes != 100<<30 || a.FreeBytes != 40<<30 || a.OtherBytes != (60<<30-8<<20) {
		t.Fatalf("driveA totals = %+v", a)
	}
	b := driveByID(t, usage, driveB)
	if b.GamesBytes != 8<<20 {
		t.Fatalf("driveB games = %d, want %d", b.GamesBytes, 8<<20)
	}

	if len(usage.Games) != 3 {
		t.Fatalf("games = %+v, want 3 (uninstalled skipped)", usage.Games)
	}
	for i, want := range []string{"g2", "g1", "g3"} {
		if usage.Games[i].GameID != want {
			t.Fatalf("games order = %v, want g2 g1 g3", usage.Games)
		}
	}
	sized := map[string]types.GameSize{}
	for _, g := range usage.Games {
		sized[g.GameID] = g
	}
	if !sized["g2"].FromPlugin || sized["g2"].Measured || sized["g2"].SizeBytes != 6<<20 {
		t.Fatalf("g2 = %+v, want the store's figure", sized["g2"])
	}
	if sized["g2"].PrefixBytes != 1<<20 {
		t.Fatalf("g2 prefix = %d, want the pinned prefix outside the prefixes folder", sized["g2"].PrefixBytes)
	}
	if !sized["g1"].Measured || sized["g1"].FromPlugin || sized["g1"].SizeBytes != 3<<20 || sized["g1"].PrefixBytes != 2<<20 {
		t.Fatalf("g1 = %+v, want measured files and prefix", sized["g1"])
	}
	if sized["g3"].PrefixBytes != 0 {
		t.Fatalf("g3 prefix = %d, want none for a native game", sized["g3"].PrefixBytes)
	}
	if walks.walked(beta) {
		t.Fatalf("walked the store game with a known size: %v", walks.paths)
	}
	if !walks.walked(pinned) || !walks.walked(filepath.Join(prefixes, "g1")) {
		t.Fatalf("prefixes not walked: %v", walks.paths)
	}
	if walks.walked(filepath.Join(prefixes, "g3")) {
		t.Fatalf("walked a native game's prefix: %v", walks.paths)
	}
}

// A game folder reached through a symlinked parent (a games folder on
// another drive) lands on the resolved drive, like removals do.
func TestDiskUsageResolvesSymlinkedFolder(t *testing.T) {
	t.Parallel()
	drive, base, home := evalTemp(t), evalTemp(t), evalTemp(t)
	link := filepath.Join(base, "hdd")
	if err := os.Symlink(drive, link); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(link, "Games", "Gem")
	writeFileOf(t, dir, 1<<20)
	game := types.Game{ID: "gem", Title: "Gem", Source: "custom", Store: types.StoreCustom, InstallDir: dir, Executable: filepath.Join(dir, "gem.sh"), IsInstalled: true, Native: true}
	var walks walkLog
	usage := buildDiskUsage([]types.Game{game}, []string{filepath.Join(link, "Games")}, home, filepath.Join(home, "prefixes"),
		[]diskMount{{point: drive}}, func(string) (uint64, uint64, error) { return 10 << 30, 5 << 30, nil }, walks.measure)
	if len(usage.Drives) != 1 || usage.Drives[0].Mount != drive {
		t.Fatalf("drives = %+v, want one drive at %s", usage.Drives, drive)
	}
	if len(usage.Drives[0].Folders) != 1 || usage.Drives[0].Folders[0] != filepath.Join(drive, "Games") {
		t.Fatalf("folders = %v, want the resolved folder", usage.Drives[0].Folders)
	}
}

func TestDiskMountsParseAndMatch(t *testing.T) {
	t.Parallel()
	data := "26 1 8:1 / / rw,relatime - ext4 /dev/sda1 rw\n" +
		"27 26 8:1 /mnt/games /mnt/games rw,relatime - ext4 /dev/sdb1 rw\n" +
		"28 27 8:1 /with\\040space /mnt/with\\040space rw - ext4 /dev/sdc1 rw\n" +
		"bogus\n"
	mounts := parseDiskMounts([]byte(data))
	if len(mounts) != 3 {
		t.Fatalf("mounts = %+v, want 3", mounts)
	}
	for path, want := range map[string]string{
		"/mnt/games/X":      "/mnt/games",
		"/mnt/games":        "/mnt/games",
		"/mnt/games2":       "/",
		"/other":            "/",
		"/":                 "/",
		"/mnt/with space/Y": "/mnt/with space",
	} {
		if got := driveOf(path, mounts); got != want {
			t.Fatalf("driveOf(%q) = %q, want %q", path, got, want)
		}
	}
}

// A prefix pinned inside the game's own folder is already inside the walked
// files figure, so it isn't measured twice.
func TestDiskUsagePrefixInsideGameDirCountsOnce(t *testing.T) {
	t.Parallel()
	drive, home := evalTemp(t), evalTemp(t)
	dir := filepath.Join(drive, "Games", "Inner")
	writeFileOf(t, dir, 2<<20)
	prefix := filepath.Join(dir, "prefix")
	writeFileOf(t, prefix, 1<<20)
	game := types.Game{ID: "in", Title: "Inner", Source: "custom", Store: types.StoreCustom, InstallDir: dir, Executable: filepath.Join(dir, "run.sh"), IsInstalled: true, Settings: types.GameSettings{WinePrefix: prefix}}
	var walks walkLog
	usage := buildDiskUsage([]types.Game{game}, nil, home, filepath.Join(home, "prefixes"),
		[]diskMount{{point: drive}}, func(string) (uint64, uint64, error) { return 10 << 30, 5 << 30, nil }, walks.measure)
	if len(usage.Games) != 1 {
		t.Fatalf("games = %+v, want 1", usage.Games)
	}
	got := usage.Games[0]
	if got.SizeBytes != 3<<20 || got.PrefixBytes != 0 {
		t.Fatalf("game = %+v, want the walked total with no separate prefix", got)
	}
	if !walks.walked(dir) || walks.walked(prefix) {
		t.Fatalf("walks = %v, want the game folder once", walks.paths)
	}
	if len(usage.Drives) != 1 || usage.Drives[0].GamesBytes != 3<<20 {
		t.Fatalf("drives = %+v, want the total once", usage.Drives)
	}
}

// Two games sharing one Wine prefix (issue #52) count it once on the drive;
// each game's own row still shows its share.
func TestDiskUsageSharedPrefixCountsOnce(t *testing.T) {
	t.Parallel()
	drive, home := evalTemp(t), evalTemp(t)
	prefixes := filepath.Join(drive, "prefixes")
	shared := filepath.Join(prefixes, "shared")
	writeFileOf(t, shared, 2<<20)
	a := filepath.Join(drive, "Games", "A")
	writeFileOf(t, a, 1<<20)
	b := filepath.Join(drive, "Games", "B")
	writeFileOf(t, b, 1<<20)
	mk := func(id, dir string) types.Game {
		return types.Game{ID: id, Title: id, Source: "custom", Store: types.StoreCustom, InstallDir: dir, Executable: filepath.Join(dir, "run.sh"), IsInstalled: true, Settings: types.GameSettings{WinePrefix: shared}}
	}
	var walks walkLog
	usage := buildDiskUsage([]types.Game{mk("a", a), mk("b", b)}, []string{filepath.Join(drive, "Games")}, home, prefixes,
		[]diskMount{{point: drive}}, func(string) (uint64, uint64, error) { return 10 << 30, 5 << 30, nil }, walks.measure)
	if len(usage.Drives) != 1 {
		t.Fatalf("drives = %+v, want 1", usage.Drives)
	}
	// Files 1+1 MB plus the shared prefix once: 4 MB, not 6.
	if usage.Drives[0].GamesBytes != 4<<20 {
		t.Fatalf("games bytes = %d, want %d", usage.Drives[0].GamesBytes, 4<<20)
	}
	for _, g := range usage.Games {
		if g.PrefixBytes != 2<<20 {
			t.Fatalf("game %s prefix = %d, want its own row to keep the share", g.GameID, g.PrefixBytes)
		}
	}
}

// Two library entries pointing at the same install dir count it once, while
// both entries stay listed.
func TestDiskUsageSharedInstallDirCountsOnce(t *testing.T) {
	t.Parallel()
	drive, home := evalTemp(t), evalTemp(t)
	dir := filepath.Join(drive, "Games", "Same")
	writeFileOf(t, dir, 2<<20)
	mk := func(id string) types.Game {
		return types.Game{ID: id, Title: id, Source: "custom", Store: types.StoreCustom, InstallDir: dir, Executable: filepath.Join(dir, "run.sh"), IsInstalled: true, Native: true}
	}
	var walks walkLog
	usage := buildDiskUsage([]types.Game{mk("a"), mk("b")}, []string{filepath.Join(drive, "Games")}, home, filepath.Join(home, "prefixes"),
		[]diskMount{{point: drive}}, func(string) (uint64, uint64, error) { return 10 << 30, 5 << 30, nil }, walks.measure)
	if len(usage.Games) != 2 {
		t.Fatalf("games = %+v, want both entries listed", usage.Games)
	}
	if len(usage.Drives) != 1 || usage.Drives[0].GamesBytes != 2<<20 {
		t.Fatalf("drives = %+v, want the shared folder once", usage.Drives)
	}
}

// A separate mount nested inside a games folder keeps its own drive entry
// with its own totals; its bytes are not lost with the parent's.
func TestDiskUsageNestedMountKeepsItsDrive(t *testing.T) {
	t.Parallel()
	drive, home := evalTemp(t), evalTemp(t)
	nested := filepath.Join(drive, "Games", "big")
	dir := filepath.Join(nested, "Title")
	writeFileOf(t, dir, 3<<20)
	plain := filepath.Join(drive, "Games", "Small")
	writeFileOf(t, plain, 1<<20)
	mk := func(id, d string) types.Game {
		return types.Game{ID: id, Title: id, Source: "custom", Store: types.StoreCustom, InstallDir: d, Executable: filepath.Join(d, "run.sh"), IsInstalled: true, Native: true}
	}
	var walks walkLog
	usage := buildDiskUsage([]types.Game{mk("big", dir), mk("small", plain)}, []string{filepath.Join(drive, "Games")}, home, filepath.Join(home, "prefixes"),
		[]diskMount{{point: drive}, {point: nested}}, func(string) (uint64, uint64, error) { return 10 << 30, 5 << 30, nil }, walks.measure)
	if len(usage.Drives) != 2 {
		t.Fatalf("drives = %+v, want the nested mount kept", usage.Drives)
	}
	if got := driveByID(t, usage, nested); got.GamesBytes != 3<<20 {
		t.Fatalf("nested drive games = %d, want %d", got.GamesBytes, 3<<20)
	}
	if got := driveByID(t, usage, drive); got.GamesBytes != 1<<20 {
		t.Fatalf("parent drive games = %d, want %d", got.GamesBytes, 1<<20)
	}
}
