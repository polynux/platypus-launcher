package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

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
