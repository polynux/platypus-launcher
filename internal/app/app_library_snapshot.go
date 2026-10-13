package app

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"platypus/internal/steam"
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
	mapSteamArtURLs(games)
	return games
}

// mapSteamArtURLs rewrites the built-in Steam source's art URLs
// (/platypus-art/steam/<rel>, issue #54) to the bundled steam plugin's
// local-art form (/platypus-art/steam/<base64url(abs path)>), so snapshots
// written before the move keep serving the same files. Customization never
// stored those URLs (it copies the images), and fresh listings already carry
// the new form; without a Steam install the old URLs stay as they are (they
// 404, as before when Steam was missing).
func mapSteamArtURLs(games []types.Game) {
	root := steam.Root()
	if root == "" {
		return
	}
	for i := range games {
		g := &games[i]
		g.CoverURL = mapSteamArtURL(root, g.CoverURL)
		g.HeroURL = mapSteamArtURL(root, g.HeroURL)
		g.BannerURL = mapSteamArtURL(root, g.BannerURL)
	}
}

func mapSteamArtURL(root, u string) string {
	rel, ok := strings.CutPrefix(u, steam.ArtURLPrefix)
	if !ok || !strings.Contains(rel, "/") {
		return u // not an old built-in URL (new form has no slash)
	}
	rel = filepath.FromSlash(rel)
	if !filepath.IsLocal(rel) {
		return u
	}
	abs := filepath.Join(root, "appcache/librarycache", rel)
	if !filepath.IsAbs(abs) {
		return u
	}
	return stores.GameArtURLPrefix + "steam/" + base64.RawURLEncoding.EncodeToString([]byte(abs))
}
