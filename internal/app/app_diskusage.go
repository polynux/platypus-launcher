package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"platypus/internal/runner"
	"platypus/internal/types"
)

// Disk usage grouping (issue #50): which drive each game folder lives on,
// how much of it Platypus games use, and how big every installed game is.
// Only the configured game folders, install locations and prefixes folder
// are ever walked; the rest of each drive is described by the filesystem
// totals, never scanned. Measuring runs in the background and cached (a
// later slice); this file only groups and measures.

// diskMount is one mounted filesystem, by mount point.
type diskMount struct {
	point string
}

// readDiskMounts lists the mount points in /proc/self/mountinfo. It reads
// that one file only, never the drives themselves.
func readDiskMounts() ([]diskMount, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	return parseDiskMounts(data), nil
}

// parseDiskMounts picks the mount points out of mountinfo content: field 5
// of every line (the path the filesystem is mounted on).
func parseDiskMounts(data []byte) []diskMount {
	var out []diskMount
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		out = append(out, diskMount{point: unescapeMount(f[4])})
	}
	return out
}

// unescapeMount restores the kernel's space escape in a mountinfo path. Only
// spaces occur in practice; anything else rides along byte for byte.
func unescapeMount(p string) string {
	return strings.ReplaceAll(p, `\040`, " ")
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
		p := runner.PrefixPath(game, home, prefixes)
		if !walked || !pathOverlaps(filepath.Clean(resolveDiskPath(p)), filepath.Clean(resolveDiskPath(dir))) {
			if n, err := measure(p); err == nil {
				out.PrefixBytes = n
			}
		}
	}
	return out
}

// buildDiskUsage groups installed games by drive and sums per-drive bytes.
// roots are the folders to attribute (game folders, install dirs, the
// prefixes folder); overlapping roots count once. spaceOf and measure are
// the filesystem probes (jobs.SpaceStats and relocate.TreeSize in
// production); tests inject fakes to simulate several drives. Games come
// out biggest first.
func buildDiskUsage(games []types.Game, roots []string, home, prefixes string, mounts []diskMount, spaceOf func(string) (uint64, uint64, error), measure func(string) (int64, error)) types.DiskUsage {
	out := types.DiskUsage{GeneratedAt: time.Now().Unix()}
	seen := map[string]bool{}
	byDrive := map[string][]string{}
	addRoot := func(root string) {
		root = filepath.Clean(resolveDiskPath(root))
		if !filepath.IsAbs(root) {
			return
		}
		for s := range seen {
			if pathOverlaps(root, s) {
				return
			}
		}
		seen[root] = true
		drive := driveOf(root, mounts)
		byDrive[drive] = append(byDrive[drive], root)
	}
	for _, r := range roots {
		if strings.TrimSpace(r) == "" {
			continue
		}
		addRoot(r)
	}
	driveGames := map[string]int64{}
	for _, g := range games {
		if !g.IsInstalled {
			continue
		}
		sized := measureGame(g, home, prefixes, measure)
		out.Games = append(out.Games, sized)
		if dir := sized.InstallDir; dir != "" {
			addRoot(dir)
			driveGames[driveOf(filepath.Clean(resolveDiskPath(dir)), mounts)] += sized.SizeBytes
		}
		if !g.Native {
			p := runner.PrefixPath(g, home, prefixes)
			addRoot(p)
			driveGames[driveOf(filepath.Clean(resolveDiskPath(p)), mounts)] += sized.PrefixBytes
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
