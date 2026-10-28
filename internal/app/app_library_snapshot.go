package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"platypus/internal/stores"
	"platypus/internal/types"
)

// Last good listings (issue #44): a persistent snapshot per source,
// <data>/library-snapshots/<source>.json, written after each success. When a
// source fails, its games come from the in-memory lastGood or this snapshot,
// with a stale SourceWarning, so the library survives an offline restart.

// librarySnapshotDir names the snapshots folder under the data dir.
func librarySnapshotDir(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "library-snapshots")
}

// snapshotPath names one source's snapshot file. Sources with unsafe names
// (separators) get none: snapshots are best-effort.
func snapshotPath(snapDir, source string) string {
	if snapDir == "" || source == "" || strings.ContainsAny(source, `/\`) || source == "." || source == ".." {
		return ""
	}
	return filepath.Join(snapDir, source+".json")
}

// saveLibrarySnapshot writes the source's games after a successful listing.
// Failures are ignored: snapshots are best-effort.
func saveLibrarySnapshot(snapDir, source string, games []types.Game) {
	path := snapshotPath(snapDir, source)
	if path == "" {
		return
	}
	data, err := json.Marshal(games)
	if err != nil {
		return
	}
	_ = os.MkdirAll(snapDir, 0o755)
	tmp, err := os.CreateTemp(snapDir, ".snapshot-*.json")
	if err != nil {
		return
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return
	}
	_ = tmp.Close()
	_ = os.Rename(tmp.Name(), path)
}

// loadLibrarySnapshot reads one source's snapshot. Corrupt or missing files
// read as no snapshot.
func loadLibrarySnapshot(snapDir, source string) []types.Game {
	path := snapshotPath(snapDir, source)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var games []types.Game
	if err := json.Unmarshal(data, &games); err != nil || len(games) == 0 {
		return nil
	}
	return games
}

// storeMissingGames serves installed rows from snapshots whose store is gone
// (issue #161): an uninstalled store's games stay in the library as "store
// missing" instead of dropping out. Rows that were never installed are
// omitted: with no store they can't be installed. Removal (the uninstall
// option) deletes the snapshot, so its entries go with it.
func (a *App) storeMissingGames(all []stores.Store) []types.Game {
	if a.library.snapDir == "" {
		return nil
	}
	known := map[string]bool{}
	for _, s := range all {
		known[s.ID()] = true
	}
	entries, err := os.ReadDir(a.library.snapDir)
	if err != nil {
		return nil
	}
	var out []types.Game
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
			continue
		}
		source := strings.TrimSuffix(name, ".json")
		if source == "" || known[source] || !filepath.IsLocal(source) {
			continue
		}
		for _, g := range loadLibrarySnapshot(a.library.snapDir, source) {
			if !g.IsInstalled {
				continue
			}
			g.StoreMissing = true
			a.applySavedSettings(&g)
			out = append(out, g)
		}
	}
	return out
}
