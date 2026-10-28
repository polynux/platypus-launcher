package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/relocate"
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
	// Containment is on resolved paths: a symlinked subfolder (or root)
	// must not smuggle the reset outside. EvalSymlinks fails for paths that
	// don't exist yet, so the deepest existing folder is resolved and the
	// missing part appended as written: a missing folder under a symlink
	// would otherwise read as inside the root. A path that can't be resolved
	// at all is refused below. The prefix itself is checked with Lstat, so a
	// symlinked prefix is refused, never followed.
	rroot, rrootErr := resolveExistingPath(root)
	rparent, rparentErr := resolveExistingPath(filepath.Dir(prefix))
	if rrootErr != nil || rparentErr != nil {
		return types.PrefixResetResult{}, apperr.New("prefix.reset.outsideRoot",
			fmt.Sprintf("%s uses a custom prefix outside the prefixes folder; reset it by hand", game.Title),
			map[string]string{"title": game.Title})
	}
	rel, err := filepath.Rel(rroot, filepath.Join(rparent, filepath.Base(prefix)))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return types.PrefixResetResult{}, apperr.New("prefix.reset.outsideRoot",
			fmt.Sprintf("%s uses a custom prefix outside the prefixes folder; reset it by hand", game.Title),
			map[string]string{"title": game.Title})
	}
	res := types.PrefixResetResult{Prefix: prefix}
	st, err := os.Lstat(prefix)
	if err == nil && st.Mode()&os.ModeSymlink != 0 {
		return types.PrefixResetResult{}, apperr.New("prefix.reset.symlink",
			fmt.Sprintf("refusing to reset %s: its prefix is a symlink; reset it by hand", game.Title),
			map[string]string{"title": game.Title})
	}
	if err == nil && !st.IsDir() {
		return types.PrefixResetResult{}, apperr.New("prefix.reset.notDir",
			fmt.Sprintf("%s has a file where its Wine prefix should be; remove it by hand", game.Title),
			map[string]string{"title": game.Title})
	}
	if err == nil {
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

// resolveExistingPath resolves the deepest existing folder of p and appends
// the part that doesn't exist yet, unresolved. EvalSymlinks alone fails on a
// missing path, which would hide a symlinked parent: <root>/link/new (link
// pointing outside) must resolve to the outside folder, not read as lexical.
// Only a missing component is skipped: a component that exists but can't be
// resolved (a loop, no permission, a dangling link) is an error, so callers
// refuse instead of trusting the lexical path.
func resolveExistingPath(p string) (string, error) {
	r, err := filepath.EvalSymlinks(p)
	if err == nil {
		return r, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if st, lerr := os.Lstat(p); lerr == nil && st.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is a symlink that can't be resolved", p)
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p, nil
	}
	rp, err := resolveExistingPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(rp, filepath.Base(p)), nil
}

// prefixBackupName matches the backups ResetPrefix leaves: <prefix>.bak-<unix>,
// and the -<n> repeats of the same second.
var prefixBackupName = regexp.MustCompile(`\.bak-(\d+)(?:-\d+)?$`)

// prefixOwners maps every game's current Wine prefix to that game: its saved
// choice, or the default under the prefixes folder. A backup a user pointed a
// game back at is a live prefix, never a reclaimable folder. An unlistable
// library leaves the map empty (nothing is known to use the folder).
func (a *App) prefixOwners() map[string]types.Game {
	owners := map[string]types.Game{}
	games, err := a.diskLibrary()
	if err != nil {
		return owners
	}
	root, home := a.effectivePrefixesFolder(), a.homeDir()
	for _, g := range games {
		if g.Native || g.ClientLaunch {
			continue
		}
		owners[filepath.Clean(runner.PrefixPath(g, home, root))] = g
	}
	return owners
}

// PrefixBackups lists the prefix backups kept in the prefixes folder, stalest
// first: each reset of a damaged prefix leaves one behind, and nothing else
// ever tells the user they take space. Only the folder's own entries are
// looked at (a game's prefix is <root>/<game id>): a backup of a custom prefix
// nested deeper in the folder isn't listed. A link is never listed, and nor is
// a backup a game uses as its prefix again.
func (a *App) PrefixBackups() []types.PrefixBackup {
	out := []types.PrefixBackup{}
	root := a.effectivePrefixesFolder()
	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	owners := a.prefixOwners()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m := prefixBackupName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		at, _ := strconv.ParseInt(m[1], 10, 64)
		path := filepath.Join(root, e.Name())
		if _, used := owners[filepath.Clean(path)]; used {
			continue
		}
		size, _ := relocate.TreeSize(path) // an unreadable folder still lists, at 0
		out = append(out, types.PrefixBackup{Name: e.Name(), Path: path, Bytes: size, At: at})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

// DeletePrefixBackup deletes one prefix backup and returns the bytes it freed.
// The path comes from the UI, so it is checked again here: a .bak-* folder
// directly inside the prefixes folder, never a link and never anything else.
// A game using it as its prefix is refused, and so is deleting under a running
// game: the backup is the folder a wineserver would have open.
func (a *App) DeletePrefixBackup(path string) (int64, error) {
	root := filepath.Clean(a.effectivePrefixesFolder())
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) || filepath.Dir(clean) != root || !prefixBackupName.MatchString(filepath.Base(clean)) {
		return 0, apperr.New("prefix.backup.outsideRoot",
			"only a prefix backup in the prefixes folder can be deleted",
			map[string]string{"path": path})
	}
	st, err := os.Lstat(clean)
	if err != nil {
		return 0, apperr.Wrap("prefix.backup.missing", err, map[string]string{"path": clean})
	}
	if !st.IsDir() {
		return 0, apperr.New("prefix.backup.notDir",
			fmt.Sprintf("%s is not a prefix backup folder", clean),
			map[string]string{"path": clean})
	}
	if game, used := a.prefixOwners()[clean]; used {
		if a.isRunning(game.ID) {
			return 0, apperr.New("game.running",
				fmt.Sprintf("%s is running; delete its old prefix after it exits", game.Title),
				map[string]string{"title": game.Title})
		}
		return 0, apperr.New("prefix.backup.inUse",
			fmt.Sprintf("%s uses this folder as its Wine prefix", game.Title),
			map[string]string{"title": game.Title, "path": clean})
	}
	size, _ := relocate.TreeSize(clean)
	if err := os.RemoveAll(clean); err != nil {
		return 0, apperr.Wrap("prefix.backup.deleteFailed", err, map[string]string{"path": clean})
	}
	return size, nil
}
