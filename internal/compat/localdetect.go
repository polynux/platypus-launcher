package compat

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// LocalHit is a well-known anti-cheat or service file found in an install.
type LocalHit struct {
	Kind  string `json:"kind"` // "anticheat" or "eos"
	Label string `json:"label"`
	Name  string `json:"name"` // the file or folder name that matched
}

// Scan limits: a game folder can hold hundreds of thousands of files, and the
// check is a hint, so it stops early.
const (
	scanMaxDepth   = 8
	scanMaxEntries = 50000
)

// matchLocal says whether a file or folder name is a marker Platypus knows. Only
// names that identify a system are matched: not steam_api.dll (every Steam
// game has one), and nothing guessed for Denuvo.
func matchLocal(name string) (LocalHit, bool) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "easyanticheat"):
		return LocalHit{Kind: "anticheat", Label: "Easy Anti-Cheat", Name: name}, true
	case strings.HasPrefix(lower, "beclient"):
		return LocalHit{Kind: "anticheat", Label: "BattlEye", Name: name}, true
	case strings.Contains(lower, "epiconlineservicesinstaller"), strings.Contains(lower, "eosbootstrapper"):
		return LocalHit{Kind: "eos", Label: "Epic Online Services", Name: name}, true
	}
	return LocalHit{}, false
}

// ScanInstall looks for markers under root. It returns the distinct hits and
// whether the walk stopped at a limit (then "not found" is not a fact).
func ScanInstall(root string) (hits []LocalHit, truncated bool) {
	seen := map[string]bool{}
	entries := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable folders are skipped, not fatal
		}
		if entries++; entries > scanMaxEntries {
			truncated = true
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(root, path)
		if rel != "." && strings.Count(rel, string(os.PathSeparator)) >= scanMaxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		if hit, ok := matchLocal(d.Name()); ok && !seen[hit.Label] {
			seen[hit.Label] = true
			hits = append(hits, hit)
		}
		return nil
	})
	return hits, truncated
}
