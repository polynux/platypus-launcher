package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/datadir"
	"platypus/internal/jobs"
	"platypus/internal/paths"
	"platypus/internal/relocate"
	"platypus/internal/types"
)

// Disk usage grouping (issue #50): which drive each game folder lives on,
// how much of it Platypus games use, and how big every installed game is.
// Only the configured game folders, install locations and prefixes folder
// are ever walked; the rest of each drive is described by the filesystem
// totals, never scanned. Measuring runs in the background and cached (a
// later slice); this file only groups and measures.

// diskMount is one mounted filesystem: where it is mounted, its filesystem
// type, the device behind it, what that device is and the filesystem's label
// (issue #148).
type diskMount struct {
	point  string
	fsType string
	source string
	kind   string
	label  string
}

// readDiskMounts lists the mounts in /proc/self/mountinfo and, for each one,
// what sysfs and udev say about its device (issue #148). It reads those
// small files only, never the drives themselves.
func readDiskMounts() ([]diskMount, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	mounts := parseDiskMounts(data)
	for i := range mounts {
		mounts[i].kind = deviceKind(mounts[i], os.ReadFile)
		mounts[i].label = filesystemLabel(mounts[i].source, "/dev/disk/by-label")
	}
	return mounts, nil
}

// parseDiskMounts picks the mount point, filesystem type and device out of
// mountinfo content: field 5 of every line (the path the filesystem is
// mounted on) and, after the "-" separator, the filesystem type and source.
func parseDiskMounts(data []byte) []diskMount {
	var out []diskMount
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		m := diskMount{point: unescapeMount(f[4])}
		for i := 6; i < len(f); i++ {
			if f[i] == "-" && i+2 < len(f) {
				m.fsType, m.source = f[i+1], unescapeMount(f[i+2])
				break
			}
		}
		out = append(out, m)
	}
	return out
}

// unescapeMount restores the kernel's space escape in a mountinfo path. Only
// spaces occur in practice; anything else rides along byte for byte.
func unescapeMount(p string) string {
	return strings.ReplaceAll(p, `\040`, " ")
}

// networkFsTypes are the filesystem types that live on another machine: the
// drive is not a local device at all.
var networkFsTypes = map[string]bool{
	"9p": true, "afs": true, "ceph": true, "cifs": true, "davfs": true,
	"davfs2": true, "fuse.rclone": true, "fuse.sshfs": true, "glusterfs": true,
	"ncpfs": true, "nfs": true, "nfs4": true, "smb3": true, "smbfs": true,
	"sshfs": true,
}

// deviceKind names what a mount's device is (issue #148): network for a
// remote filesystem, removable for a USB key, nvme for an NVMe drive, else
// ssd or hdd from the kernel's rotational flag. readFile reads sysfs (tests
// inject a fake); an unknown device yields "".
func deviceKind(m diskMount, readFile func(string) ([]byte, error)) string {
	if networkFsTypes[m.fsType] || (!strings.HasPrefix(m.source, "/") && strings.Contains(m.source, ":")) {
		return "network"
	}
	// A loop device holds an image file, not a drive: its rotational flag
	// says nothing about the disk underneath.
	if strings.HasPrefix(filepath.Base(m.source), "loop") {
		return ""
	}
	// The kernel names whole disks in /sys/block, so a partition is read
	// through its disk.
	names := blockNames(m.source)
	for _, n := range names {
		if v, err := readFile("/sys/block/" + n + "/removable"); err == nil && strings.TrimSpace(string(v)) == "1" {
			return "removable"
		}
	}
	for _, n := range names {
		if strings.HasPrefix(n, "nvme") {
			return "nvme"
		}
	}
	for _, n := range names {
		v, err := readFile("/sys/block/" + n + "/queue/rotational")
		if err != nil {
			continue
		}
		switch strings.TrimSpace(string(v)) {
		case "0":
			return "ssd"
		case "1":
			return "hdd"
		}
	}
	return ""
}

// blockNames lists the /sys/block names to try for a mount source: the
// device itself first (a whole disk can be mounted directly), then the disk
// its partition suffix belongs to (sda1 is on sda, nvme0n1p2 on nvme0n1).
func blockNames(source string) []string {
	name := filepath.Base(source)
	base := strings.TrimRight(name, "0123456789")
	if strings.HasSuffix(base, "p") {
		base = strings.TrimSuffix(base, "p")
	}
	if base == "" || base == name {
		return []string{name}
	}
	return []string{name, base}
}

// filesystemLabel is the label udev recorded for a device in dir
// (/dev/disk/by-label), "" when it recorded none: most filesystems carry no
// label, and a machine without udev has no dir at all.
func filesystemLabel(source, dir string) string {
	if !strings.HasPrefix(source, "/dev/") {
		return ""
	}
	device := filepath.Base(source)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join(dir, e.Name()))
		if err != nil || filepath.Base(filepath.Clean(target)) != device {
			continue
		}
		// udev escapes what can't sit in a file name: blanks as \x20.
		return strings.ReplaceAll(strings.ReplaceAll(e.Name(), `\x20`, " "), `\x5c`, `\`)
	}
	return ""
}

// resolveDiskPath resolves symlinks in path, so a games folder on another
// drive (a symlink, see TestRemoveThroughSymlinkedParent) is attributed to
// that drive. A folder that doesn't exist yet resolves through its nearest
// existing parent, like freeSpace does.
func resolveDiskPath(path string) string {
	for p := path; ; {
		if _, err := os.Stat(p); err == nil {
			if r, err := filepath.EvalSymlinks(p); err == nil {
				return r
			}
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return filepath.Clean(path)
		}
		p = parent
	}
}

// driveOf names the drive holding path: the longest mount point at or above
// it. A path under no known mount falls back to the filesystem root.
func driveOf(path string, mounts []diskMount) string {
	best := ""
	for _, m := range mounts {
		if m.point == "" {
			continue
		}
		if m.point == "/" || path == m.point || strings.HasPrefix(path, m.point+string(filepath.Separator)) {
			if len(m.point) > len(best) {
				best = m.point
			}
		}
	}
	if best == "" {
		return "/"
	}
	return best
}

// gameFilesDir is the folder holding the game's files: its install dir, or
// the folder of its executable when the plugin named none.
func gameFilesDir(game types.Game) string {
	if strings.TrimSpace(game.InstallDir) != "" {
		return game.InstallDir
	}
	if strings.TrimSpace(game.Executable) != "" {
		return filepath.Dir(game.Executable)
	}
	return ""
}

func gameDataDir(game types.Game, home, root string) string {
	if root == "" {
		root = filepath.Join(datadir.Path(home), "prefixes")
	}
	return paths.GameDataDir(game.ID, game.Settings.WinePrefix, root)
}

// measureGame sizes one installed game. The store's figure wins when it has
// one (FromPlugin, no walk); otherwise the game files are walked. The Wine
// prefix is always measured separately: plugins never know it, and native
// games have none (issue #47). A prefix pinned inside the game's own folder
// is already inside the walked files figure, so it isn't measured twice.
func measureGame(game types.Game, home, prefixes string, measure func(string) (int64, error)) types.GameSize {
	out := types.GameSize{
		GameID: game.ID, Title: game.Title, Source: game.Source, Store: game.Store,
		InstallDir: gameFilesDir(game),
	}
	dir := gameFilesDir(game)
	walked := false
	if game.SizeBytes > 0 {
		out.SizeBytes, out.FromPlugin = game.SizeBytes, true
	} else if dir != "" {
		if n, err := measure(dir); err == nil {
			out.SizeBytes, out.Measured, walked = n, true, true
		}
	}
	if !game.Native {
		p := gameDataDir(game, home, prefixes)
		if !walked || !pathOverlaps(filepath.Clean(resolveDiskPath(p)), filepath.Clean(resolveDiskPath(dir))) {
			if n, err := measure(p); err == nil {
				out.PrefixBytes = n
			}
		}
	}
	return out
}

// buildDiskUsage sizes every game with measureGame: the store's figure when
// it has one, else a walk. A scan pass uses buildDiskUsageWith to reuse the
// previous snapshot and force walks instead.
func buildDiskUsage(games []types.Game, roots []string, home, prefixes string, mounts []diskMount, spaceOf func(string) (uint64, uint64, error), measure func(string) (int64, error)) types.DiskUsage {
	return buildDiskUsageWith(games, roots, home, prefixes, mounts, spaceOf, func(game types.Game) types.GameSize {
		return measureGame(game, home, prefixes, measure)
	})
}

// buildDiskUsageWith groups installed games by drive and sums per-drive
// bytes, sizing each game with sizer. home and prefixes resolve the Wine
// prefixes for attribution. roots are the folders to attribute
// (game folders, install dirs, the prefixes folder); overlapping roots on
// one drive count once, and a folder shared by several games counts once.
// spaceOf is the filesystem probe (jobs.SpaceStats in production); tests
// inject fakes to simulate several drives. Games come out biggest first.
func buildDiskUsageWith(games []types.Game, roots []string, home, prefixes string, mounts []diskMount, spaceOf func(string) (uint64, uint64, error), sizer func(types.Game) types.GameSize) types.DiskUsage {
	out := types.DiskUsage{GeneratedAt: time.Now().Unix()}
	seen := map[string]string{} // resolved root -> drive
	byDrive := map[string][]string{}
	// addRoot attributes a folder to its drive. Overlapping folders share
	// one entry when they sit on the same drive; a separate mount nested
	// inside a games folder is its own drive with its own totals.
	addRoot := func(root string) {
		root = filepath.Clean(resolveDiskPath(root))
		if !filepath.IsAbs(root) {
			return
		}
		drive := driveOf(root, mounts)
		for s, d := range seen {
			if d == drive && pathOverlaps(root, s) {
				return
			}
		}
		seen[root] = drive
		byDrive[drive] = append(byDrive[drive], root)
	}
	for _, r := range roots {
		if strings.TrimSpace(r) == "" {
			continue
		}
		addRoot(r)
	}
	// counted remembers which resolved folders already contributed their
	// bytes to a drive: two games sharing one prefix (issue #52) or one
	// install dir count it once, first wins. Every game is still listed.
	counted := map[string]bool{}
	driveGames := map[string]int64{}
	for _, g := range games {
		if !g.IsInstalled {
			continue
		}
		sized := sizer(g)
		out.Games = append(out.Games, sized)
		if dir := sized.InstallDir; dir != "" {
			addRoot(dir)
			rdir := filepath.Clean(resolveDiskPath(dir))
			if !counted[rdir] {
				counted[rdir] = true
				driveGames[driveOf(rdir, mounts)] += sized.SizeBytes
			}
		}
		if !g.Native {
			p := gameDataDir(g, home, prefixes)
			addRoot(p)
			rp := filepath.Clean(resolveDiskPath(p))
			if !counted[rp] {
				counted[rp] = true
				driveGames[driveOf(rp, mounts)] += sized.PrefixBytes
			}
		}
	}
	sort.SliceStable(out.Games, func(i, j int) bool {
		si := out.Games[i].SizeBytes + out.Games[i].PrefixBytes
		sj := out.Games[j].SizeBytes + out.Games[j].PrefixBytes
		return si > sj
	})
	drives := make([]string, 0, len(byDrive))
	for d := range byDrive {
		drives = append(drives, d)
	}
	sort.Strings(drives)
	for _, d := range drives {
		folders := byDrive[d]
		sort.Strings(folders)
		du := types.DriveUsage{ID: d, Mount: d, Folders: folders, GamesBytes: driveGames[d]}
		for _, m := range mounts {
			if m.point != d {
				continue
			}
			// What the drive is (issue #148): the filesystem type, the
			// device kind from sysfs and the filesystem's label.
			du.FsType, du.DeviceKind, du.Label = m.fsType, m.kind, m.label
			break
		}
		if total, free, err := spaceOf(folders[0]); err == nil {
			du.TotalBytes, du.FreeBytes = int64(total), int64(free)
			// Other is derived, never scanned: whatever the totals say is
			// used minus what the games above account for.
			du.OtherBytes = du.TotalBytes - du.FreeBytes - du.GamesBytes
			if du.OtherBytes < 0 {
				du.OtherBytes = 0
			}
		} else {
			du.TotalBytes, du.FreeBytes = -1, -1
		}
		out.Drives = append(out.Drives, du)
	}
	return out
}

// How long a disk usage snapshot is served before a new pass starts. A
// store's figure changes with an update, not by the minute.
const diskUsageTTL = 5 * time.Minute

// diskUsageCache holds the last disk usage snapshot and the forced scans
// waiting for a pass. The zero value is ready to use. Forced scans and
// measured figures stay in memory only: a restart re-trusts the stores.
type diskUsageCache struct {
	mu   sync.Mutex
	now  func() time.Time // tests replace it
	snap types.DiskUsage
	at   time.Time
	// seen says a snapshot was stored this session (the view was opened):
	// only then does invalidation start a pass, so idle sessions never walk.
	seen       bool
	scanning   bool
	forceAll   bool
	forceGames map[string]bool
}

func (c *diskUsageCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// fresh says the snapshot is worth serving: stored and younger than the TTL.
func (c *diskUsageCache) fresh() bool {
	return !c.at.IsZero() && c.clock().Sub(c.at) < diskUsageTTL
}

// GetDiskUsage returns the last disk usage snapshot, starting a background
// measuring pass when it is missing or stale. It never walks: a fresh pass
// arrives as a "diskusage" event.
func (a *App) GetDiskUsage() types.DiskUsage { return a.diskSnapshot() }

// RefreshDiskUsage measures every game on disk, even when the store reported
// a size: the walk outranks the store until something invalidates it. It
// returns the current snapshot; the measured figures arrive as a "diskusage"
// event.
func (a *App) RefreshDiskUsage() types.DiskUsage {
	a.diskUsage.mu.Lock()
	a.diskUsage.forceAll = true
	a.diskUsage.mu.Unlock()
	return a.diskSnapshot()
}

// ScanGameSize measures one installed game and its prefix on disk, like a
// refresh but for that game only. Measuring never deletes, so the passed
// game is used as is; a game that isn't installed has nothing to measure.
func (a *App) ScanGameSize(game types.Game) (types.DiskUsage, error) {
	if !game.IsInstalled {
		return types.DiskUsage{}, apperr.New("storage.notInstalled",
			fmt.Sprintf("%s is not installed", game.Title),
			map[string]string{"title": game.Title})
	}
	a.diskUsage.mu.Lock()
	if a.diskUsage.forceGames == nil {
		a.diskUsage.forceGames = map[string]bool{}
	}
	a.diskUsage.forceGames[game.ID] = true
	a.diskUsage.mu.Unlock()
	return a.diskSnapshot(), nil
}

// diskSnapshot returns the snapshot, starting a background pass when it is
// stale or a forced scan waits.
func (a *App) diskSnapshot() types.DiskUsage {
	a.diskUsage.mu.Lock()
	defer a.diskUsage.mu.Unlock()
	if !a.diskUsage.fresh() || a.diskUsage.forceAll || len(a.diskUsage.forceGames) > 0 {
		a.startDiskScanLocked()
		snap := a.diskUsage.snap
		snap.Scanning = true
		return snap
	}
	return a.diskUsage.snap
}

// startDiskScanLocked starts the background pass unless one runs. Callers
// hold a.diskUsage.mu.
func (a *App) startDiskScanLocked() {
	if a.diskUsage.scanning {
		return
	}
	a.diskUsage.scanning = true
	go a.runDiskScan()
}

// invalidateDiskUsage drops every reused figure: the game-folder settings
// call it, as no figure can be trusted once the folders changed. The next
// pass re-sizes all games with measureGame.
func (a *App) invalidateDiskUsage() {
	a.diskUsage.mu.Lock()
	defer a.diskUsage.mu.Unlock()
	a.diskUsage.snap.Games = nil
	a.staleDiskUsageLocked()
}

// invalidateDiskGame drops one game's reused figures: a finished job calls
// it for its game, as only that game's bytes changed. The next pass
// re-sizes it with measureGame (the store's figure, else a walk), which
// also drops its forced "Measured" override.
func (a *App) invalidateDiskGame(gameID string) {
	a.diskUsage.mu.Lock()
	defer a.diskUsage.mu.Unlock()
	// A fresh slice: GetDiskUsage handed the old one to callers, so
	// filtering in place would rewrite their snapshot too.
	old := a.diskUsage.snap.Games
	kept := make([]types.GameSize, 0, len(old))
	for _, g := range old {
		if g.GameID != gameID {
			kept = append(kept, g)
		}
	}
	a.diskUsage.snap.Games = kept
	a.staleDiskUsageLocked()
}

// staleDiskUsageLocked marks the snapshot stale and starts a pass, but only
// when the view was opened this session: idle sessions never walk, and the
// next GetDiskUsage starts their pass. Callers hold a.diskUsage.mu.
func (a *App) staleDiskUsageLocked() {
	a.diskUsage.at = time.Time{}
	if a.diskUsage.seen {
		a.startDiskScanLocked()
	}
}

// runDiskScan measures until no forced scan is left. A pass that finds new
// markers runs again instead of leaving them for the next visit.
func (a *App) runDiskScan() {
	for a.scanDiskPass() {
	}
}

// scanDiskPass runs one measuring pass over the library with the production
// probes. A mount table that can't be read still yields grouped sizes:
// grouping falls back to the filesystem root, never to an error.
func (a *App) scanDiskPass() bool {
	games, err := a.diskLibrary()
	if err != nil {
		// The stores are down: keep the old snapshot and try again on the
		// next visit.
		a.diskUsage.mu.Lock()
		a.diskUsage.scanning = false
		a.diskUsage.mu.Unlock()
		return false
	}
	mounts, _ := readDiskMounts()
	return a.scanDiskWith(games, mounts, jobs.SpaceStats, relocate.TreeSize)
}

// diskLibrary lists the games a scan measures: the whole library, or the
// tests' replacement so no plugin is needed. A partially built App (no
// registry or stores yet) can't list anything: like an unreachable store,
// the old snapshot is kept.
func (a *App) diskLibrary() ([]types.Game, error) {
	if a.diskGames != nil {
		return a.diskGames()
	}
	if a.data == nil || a.stores == nil {
		return nil, errors.New("library not ready")
	}
	lib, err := a.GetLibrary()
	if err != nil {
		return nil, err
	}
	return lib.Games, nil
}

// scanDiskWith runs one measuring pass over games with the given probes and
// stores the snapshot, telling the UI through a "diskusage" event. A forced
// game is always walked; every other game keeps its previous figures, so a
// pass touches nothing already known. It reports whether new forced scans
// arrived meanwhile and another pass is due.
func (a *App) scanDiskWith(games []types.Game, mounts []diskMount, spaceOf func(string) (uint64, uint64, error), measure func(string) (int64, error)) bool {
	a.diskUsage.mu.Lock()
	forceAll := a.diskUsage.forceAll
	force := a.diskUsage.forceGames
	a.diskUsage.forceAll = false
	a.diskUsage.forceGames = nil
	prev := map[string]types.GameSize{}
	for _, g := range a.diskUsage.snap.Games {
		prev[g.GameID] = g
	}
	a.diskUsage.mu.Unlock()

	home, prefixes := a.homeDir(), a.effectivePrefixesFolder()
	roots := append([]string{prefixes}, a.allGameFolders()...)
	sizer := func(game types.Game) types.GameSize {
		if !forceAll && !force[game.ID] {
			if p, ok := prev[game.ID]; ok {
				p.Title, p.Source, p.Store = game.Title, game.Source, game.Store
				p.InstallDir = gameFilesDir(game)
				return p
			}
		}
		g := game
		if forceAll || force[game.ID] {
			g.SizeBytes = 0 // a forced scan walks; the store doesn't outrank it
		}
		return measureGame(g, home, prefixes, measure)
	}
	snap := buildDiskUsageWith(games, roots, home, prefixes, mounts, spaceOf, sizer)

	a.diskUsage.mu.Lock()
	a.diskUsage.snap = snap
	a.diskUsage.at = a.diskUsage.clock()
	a.diskUsage.seen = true
	a.diskUsage.mu.Unlock()
	if a.host != nil {
		a.host.Emit("diskusage", snap)
	}
	// Markers set while measuring (or emitting) ask for another pass; the
	// starters that set them skipped their own, the running one covers them.
	a.diskUsage.mu.Lock()
	defer a.diskUsage.mu.Unlock()
	if a.diskUsage.forceAll || len(a.diskUsage.forceGames) > 0 {
		return true
	}
	a.diskUsage.scanning = false
	return false
}

// OpenGameFolder opens the folder holding the game's files in the file
// manager, like OpenPluginsFolder does for the plugins folder.
func (a *App) OpenGameFolder(game types.Game) error {
	dir := gameFilesDir(game)
	if dir == "" {
		return apperr.New("storage.openFailed",
			fmt.Sprintf("%s has no folder Platypus can open", game.Title),
			map[string]string{"title": game.Title})
	}
	cmd := exec.Command("xdg-open", dir)
	if err := cmd.Start(); err != nil {
		return apperr.Wrap("storage.openFailed", err, map[string]string{"title": game.Title})
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
