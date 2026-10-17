package paths

import "path/filepath"

// GameDataDir resolves a game-specific directory override against a shared
// root. It has no knowledge of what the directory contains.
func GameDataDir(gameID, override, root string) string {
	if override != "" {
		return override
	}
	return filepath.Join(root, gameID)
}
