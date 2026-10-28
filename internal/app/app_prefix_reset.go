package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/runner"
	"platypus/internal/types"
)

// ResetPrefix moves a damaged Wine prefix aside and recreates it fresh (issue
// #144): the resolve action behind the Needs attention row. Only the game's
// own prefix path is ever touched: a user-chosen folder outside the prefixes
// root is refused, as are native builds, store-client launches and running
// games. The old prefix is kept next to the fresh one; nothing is deleted.
// The fresh prefix carries a marker so it reads as fresh, not damaged, until
// the game runs in it. Afterwards a library event refreshes the flag.
func (a *App) ResetPrefix(gameID string) (types.PrefixResetResult, error) {
	game, err := a.presetGame(gameID)
	if err != nil {
		return types.PrefixResetResult{}, err
	}
	if game.Native {
		return types.PrefixResetResult{}, apperr.New("prefix.reset.native",
			fmt.Sprintf("%s runs natively, without a Wine prefix", game.Title),
			map[string]string{"title": game.Title})
	}
	if game.ClientLaunch {
		return types.PrefixResetResult{}, apperr.New("prefix.reset.clientLaunch",
			fmt.Sprintf("%s launches through its store client, which owns its prefix", game.Title),
			map[string]string{"title": game.Title})
	}
	if a.isRunning(gameID) {
		return types.PrefixResetResult{}, apperr.New("game.running",
			fmt.Sprintf("%s is running; reset its prefix after it exits", game.Title),
			map[string]string{"title": game.Title})
	}
	root := a.effectivePrefixesFolder()
	prefix := runner.PrefixPath(game, a.homeDir(), root)
	rel, err := filepath.Rel(root, prefix)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return types.PrefixResetResult{}, apperr.New("prefix.reset.outsideRoot",
			fmt.Sprintf("%s uses a custom prefix outside the prefixes folder; reset it by hand", game.Title),
			map[string]string{"title": game.Title})
	}
	res := types.PrefixResetResult{Prefix: prefix}
	if st, err := os.Stat(prefix); err == nil {
		if !st.IsDir() {
			return types.PrefixResetResult{}, apperr.New("prefix.reset.notDir",
				fmt.Sprintf("%s has a file where its Wine prefix should be; remove it by hand", game.Title),
				map[string]string{"title": game.Title})
		}
		backup := fmt.Sprintf("%s.bak-%d", prefix, time.Now().Unix())
		for i := 1; ; i++ {
			if _, err := os.Stat(backup); os.IsNotExist(err) {
				break
			}
			backup = fmt.Sprintf("%s.bak-%d-%d", prefix, time.Now().Unix(), i)
		}
		if err := os.Rename(prefix, backup); err != nil {
			return types.PrefixResetResult{}, apperr.Wrap("prefix.reset.moveFailed", err,
				map[string]string{"title": game.Title})
		}
		res.Backup = backup
	} else if !os.IsNotExist(err) {
		return types.PrefixResetResult{}, apperr.Wrap("prefix.reset.moveFailed", err,
			map[string]string{"title": game.Title})
	}
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return types.PrefixResetResult{}, apperr.Wrap("prefix.reset.moveFailed", err,
			map[string]string{"title": game.Title})
	}
	if err := os.WriteFile(filepath.Join(prefix, runner.FreshMarker), []byte(time.Now().UTC().Format(time.RFC3339)), 0o644); err != nil {
		return types.PrefixResetResult{}, apperr.Wrap("prefix.reset.moveFailed", err,
			map[string]string{"title": game.Title})
	}
	if a.host != nil {
		a.host.Emit("library", gameID)
	}
	return res, nil
}
