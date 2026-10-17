package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/paths"
	"platypus/internal/plugins"
	"platypus/internal/relocate"
	"platypus/internal/settings"
	"platypus/internal/types"
)

// Removal of sideloaded games (issue #43): forget the library entry (as
// before), optionally deleting the Wine prefix and/or the game files first.
// The request carries booleans only; core recomputes the paths, so a stale
// UI can never aim a deletion elsewhere.

// removeKinds are the folders a removal deletes.
const (
	removeKindPrefix = "prefix"
	removeKindFiles  = "files"
)

// removeTarget is one folder a removal deletes, recomputed by core.
type removeTarget struct {
	kind string
	path string
}

// customGameByID returns the sideloaded game as the library holds it (with
// its saved settings, so the pinned prefix resolves). Anything else is an
// unknown game: removals with deletion are for sideloaded games only.
func (a *App) customGameByID(id string) (types.Game, error) {
	for _, g := range a.customGames() {
		if g.ID == id {
			return g, nil
		}
	}
	return types.Game{}, apperr.New("sideload.unknown", fmt.Sprintf("unknown game %q", id), map[string]string{"id": id})
}

// effectivePrefixesFolder is where Wine prefixes live: the user's choice, or
// the default under the data folder in use.
func (a *App) effectivePrefixesFolder() string {
	if p := a.prefixesFolder(); p != "" {
		return p
	}
	return settings.DefaultPrefixesFolderFor(a.resolvedDataDir())
}

// allGameFolders is every configured game folder: the default first, then
// the library folders.
func (a *App) allGameFolders() []string {
	return append([]string{a.settings.GamesFolder()}, a.settings.GameFolders()...)
}

// removeTargets recomputes what a removal deletes: the game data folder comes
// from the game's own override when set, otherwise the configured root; files are the folder of
// its executable. A native game has no prefix (issue #47): the prefix target
// is never offered for one, even when asked for.
func (a *App) removeTargets(game types.Game, opts types.RemoveOptions) ([]removeTarget, error) {
	var out []removeTarget
	if opts.DeletePrefix && !game.Native {
		out = append(out, removeTarget{kind: removeKindPrefix, path: paths.GameDataDir(game.ID, game.Settings.WinePrefix, a.effectivePrefixesFolder())})
	}
	if opts.DeleteFiles {
		if strings.TrimSpace(game.Executable) == "" {
			return nil, apperr.New("remove.noExecutable",
				fmt.Sprintf("%s has no executable, so Platypus doesn't know which folder holds its files", game.Title),
				map[string]string{"title": game.Title})
		}
		out = append(out, removeTarget{kind: removeKindFiles, path: filepath.Dir(game.Executable)})
	}
	return out, nil
}

// pathOverlaps says whether a and b name the same folder or one holds the
// other. Both are clean absolute paths.
func pathOverlaps(a, b string) bool {
	return a == b ||
		strings.HasPrefix(a, b+string(filepath.Separator)) ||
		strings.HasPrefix(b, a+string(filepath.Separator))
}

// dirContains says whether dir is root or sits above it.
func dirContains(dir, root string) bool {
	return dir == root || strings.HasPrefix(root, dir+string(filepath.Separator))
}

// isSystemDir says whether dir is a system folder no deletion may touch.
// Shared with checkFolderFor.
func isSystemDir(dir string) bool {
	for _, sys := range []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib32", "/lib64", "/proc", "/run", "/sbin", "/sys", "/usr", "/var"} {
		if dir == sys || strings.HasPrefix(dir, sys+"/") {
			return true
		}
	}
	return false
}

// isHomeOrAbove says whether dir is home or a folder above it. Shared with
// checkFolderFor.
func isHomeOrAbove(dir, home string) bool {
	if home == "" {
		return false
	}
	home = filepath.Clean(home)
	return dir == home || strings.HasPrefix(home, dir+"/")
}

// xdgUserDirKeys maps user-dirs.dirs keys to their default folder names.
var xdgUserDirKeys = []struct{ key, def string }{
	{"XDG_DESKTOP_DIR", "Desktop"},
	{"XDG_DOCUMENTS_DIR", "Documents"},
	{"XDG_DOWNLOAD_DIR", "Downloads"},
	{"XDG_MUSIC_DIR", "Music"},
	{"XDG_PICTURES_DIR", "Pictures"},
	{"XDG_VIDEOS_DIR", "Videos"},
	{"XDG_PUBLICSHARE_DIR", "Public"},
}

// xdgUserDirs returns home's XDG user folders (Desktop, Documents, ...),
// read from user-dirs.dirs with the English defaults for whatever is unset.
// A folder directly holding a game's executable (say ~/Downloads) must never
// be deleted, nor may anything holding or inside one.
func xdgUserDirs(home string) []string {
	defs := make([]string, 0, len(xdgUserDirKeys))
	for _, k := range xdgUserDirKeys {
		defs = append(defs, filepath.Join(home, k.def))
	}
	if home == "" {
		return defs
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "user-dirs.dirs"))
	if err != nil {
		return defs
	}
	vals := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		// Values are shell-quoted by convention; take bare ones too.
		if len(val) >= 2 {
			if (strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) ||
				(strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`)) {
				val = val[1 : len(val)-1]
			}
		}
		if key == "" || val == "" {
			continue
		}
		vals[key] = val
	}
	out := make([]string, 0, len(xdgUserDirKeys))
	seen := map[string]bool{}
	for _, k := range xdgUserDirKeys {
		raw, ok := vals[k.key]
		if !ok {
			raw = filepath.Join(home, k.def)
		} else if strings.HasPrefix(raw, "$HOME") {
			raw = filepath.Join(home, strings.TrimPrefix(raw, "$HOME"))
		} else if !filepath.IsAbs(raw) {
			raw = filepath.Join(home, raw)
		}
		clean := filepath.Clean(raw)
		if clean == "" || seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out
}

// otherGameDir is a folder another game owns: its title names it in refusals.
type otherGameDir struct {
	title string
	path  string
}

// otherGameDirs lists the folders other games own: sideloaded games' prefixes
// and executable folders, every wine_prefix in the game settings, and cached
// store games' install folders, executable folders and prefixes. The game
// being removed (selfID) is left out, so its own folders stay deletable.
func (a *App) otherGameDirs(selfID string) []otherGameDir {
	prefixes := a.effectivePrefixesFolder()
	var out []otherGameDir
	titles := map[string]string{}
	for _, g := range a.customGames() {
		titles[g.ID] = g.Title
		if g.ID == selfID {
			continue
		}
		if p := paths.GameDataDir(g.ID, g.Settings.WinePrefix, prefixes); p != "" {
			out = append(out, otherGameDir{title: g.Title, path: p})
		}
		if g.Executable != "" {
			out = append(out, otherGameDir{title: g.Title, path: filepath.Dir(g.Executable)})
		}
	}
	for id, m := range a.settings.GameSettings() {
		if id == selfID {
			continue
		}
		v, ok := m["wine_prefix"]
		if !ok {
			continue
		}
		s, _ := v.(string)
		if strings.TrimSpace(s) == "" {
			continue
		}
		title := titles[id]
		if title == "" {
			title = id
		}
		out = append(out, otherGameDir{title: title, path: s})
	}
	a.library.mu.Lock()
	entries := make([]libraryCacheEntry, 0, len(a.library.entries))
	for _, e := range a.library.entries {
		entries = append(entries, e)
	}
	a.library.mu.Unlock()
	for _, e := range entries {
		for _, g := range e.games {
			if g.ID == selfID {
				continue
			}
			titles[g.ID] = g.Title
			if g.InstallDir != "" {
				out = append(out, otherGameDir{title: g.Title, path: g.InstallDir})
			}
			if g.Executable != "" {
				out = append(out, otherGameDir{title: g.Title, path: filepath.Dir(g.Executable)})
			}
			if p := paths.GameDataDir(g.ID, g.Settings.WinePrefix, prefixes); p != "" && g.Settings.WinePrefix != "" {
				out = append(out, otherGameDir{title: g.Title, path: p})
			}
		}
	}
	return out
}

// checkDeleteDirFor says whether dir may be deleted for gameID's removal and
// whether it exists. A missing folder is not a refusal (a retry counts it as
// done); everything else that must not go is a coded error. kind is "prefix"
// or "files": prefixes additionally need a Wine marker.
func (a *App) checkDeleteDirFor(gameID, dir, kind string) (bool, error) {
	home := a.homeDir()
	refuse := func(code, msg string, params map[string]string) (bool, error) {
		return false, apperr.New(code, msg, params)
	}
	if dir == "" || !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return refuse("remove.notAbsolute",
			fmt.Sprintf("refusing to delete %q: not a clean absolute path", dir),
			map[string]string{"path": dir})
	}
	if dir == "/" {
		return refuse("remove.root", "refusing to delete the filesystem root", nil)
	}
	if isSystemDir(dir) {
		return refuse("remove.system",
			fmt.Sprintf("refusing to delete system folder %s", dir),
			map[string]string{"path": dir})
	}
	if isHomeOrAbove(dir, home) {
		return refuse("remove.home",
			fmt.Sprintf("refusing to delete your home folder or one above it (%s)", dir),
			map[string]string{"path": dir})
	}
	// Folders Platypus manages: the target must neither be one nor hold one.
	protected := []struct {
		code  string
		roots []string
	}{
		{"remove.gamesFolder", a.allGameFolders()},
		{"remove.prefixesRoot", []string{a.effectivePrefixesFolder()}},
		{"remove.ownData", []string{a.resolvedDataDir(), paths.ConfigDirFor(home), a.resolvedCacheDir()}},
	}
	st, statErr := os.Lstat(dir)
	switch {
	case statErr == nil:
	case os.IsNotExist(statErr):
		// Missing counts as done; only the path-shape guards above apply,
		// plus the managed-folder and user-folder overlaps below.
	default:
		return refuse("remove.unreadable",
			fmt.Sprintf("refusing to delete %s: it can't be read (%v)", dir, statErr),
			map[string]string{"path": dir})
	}
	exists := statErr == nil
	if exists {
		if st.Mode()&os.ModeSymlink != 0 {
			return refuse("remove.symlink",
				fmt.Sprintf("refusing to delete %s: it is a symlink", dir),
				map[string]string{"path": dir})
		}
		// A parent may be a symlink (a games folder on another drive, say):
		// what gets deleted is the resolved folder, so it must pass every
		// check too. The resolved path resolves to itself, so this recurses once.
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return refuse("remove.escaped",
				fmt.Sprintf("refusing to delete %s: it can't be resolved (%v)", dir, err),
				map[string]string{"path": dir})
		}
		if resolved != dir {
			if _, err := a.checkDeleteDirFor(gameID, resolved, kind); err != nil {
				return false, err
			}
		}
	}
	for _, p := range protected {
		for _, root := range p.roots {
			if root == "" {
				continue
			}
			if dirContains(dir, filepath.Clean(root)) || dirContains(dir, evalOrSelf(root)) {
				return refuse(p.code,
					fmt.Sprintf("refusing to delete %s: it holds Platypus's %s", dir, root),
					map[string]string{"path": dir, "folder": root})
			}
		}
	}
	for _, u := range xdgUserDirs(home) {
		if pathOverlaps(dir, u) || pathOverlaps(dir, evalOrSelf(u)) {
			return refuse("remove.userDir",
				fmt.Sprintf("refusing to delete %s: it overlaps your %s folder", dir, u),
				map[string]string{"path": dir, "folder": u})
		}
	}
	if exists && kind == removeKindPrefix {
		if _, err := os.Stat(filepath.Join(dir, "system.reg")); err != nil {
			if _, err := os.Stat(filepath.Join(dir, "pfx", "system.reg")); err != nil {
				return refuse("remove.noMarker",
					fmt.Sprintf("refusing to delete %s: it has no Wine prefix in it (no system.reg)", dir),
					map[string]string{"path": dir})
			}
		}
	}
	for _, o := range a.otherGameDirs(gameID) {
		if o.path == "" || !filepath.IsAbs(o.path) {
			continue
		}
		if pathOverlaps(dir, filepath.Clean(o.path)) || pathOverlaps(dir, evalOrSelf(o.path)) {
			return refuse("remove.shared",
				fmt.Sprintf("refusing to delete %s: %s still uses it", dir, o.title),
				map[string]string{"path": dir, "title": o.title})
		}
	}
	return exists, nil
}

// evalOrSelf is p with its symlinks resolved, or p cleaned when it can't be
// resolved (it may not exist).
func evalOrSelf(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// checkedTarget is a removal target with its size, for the check and the job.
type checkedTarget struct {
	kind   string
	path   string
	exists bool
	bytes  int64
}

// checkRemoveTargets validates every target of gameID's removal and measures
// what is there.
func (a *App) checkRemoveTargets(gameID string, targets []removeTarget) ([]checkedTarget, error) {
	out := make([]checkedTarget, 0, len(targets))
	for _, t := range targets {
		exists, err := a.checkDeleteDirFor(gameID, t.path, t.kind)
		if err != nil {
			return nil, err
		}
		ct := checkedTarget{kind: t.kind, path: t.path, exists: exists}
		if exists {
			size, err := relocate.TreeSize(t.path)
			if err != nil {
				return nil, apperr.Wrap("remove.unreadable", fmt.Errorf("refusing to delete %s: it can't be measured: %w", t.path, err), map[string]string{"path": t.path})
			}
			ct.bytes = size
		}
		out = append(out, ct)
	}
	return out, nil
}

// reasonOf splits a refusal into display text and its translatable code.
func reasonOf(err error) (reason, code string, params map[string]string) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Error(), ae.Code, ae.Params
	}
	return err.Error(), "", nil
}

// CheckRemove says what removing a sideloaded game would delete, before the
// user confirms. Each requested target carries its path and size, or is
// disallowed with its reason (refused, not found). Saves previews the save
// gate (issue #42): backups off, no known save paths, or the saves that
// will be backed up first.
func (a *App) CheckRemove(gameID string, opts types.RemoveOptions) (types.RemoveCheck, error) {
	check := types.RemoveCheck{}
	game, err := a.customGameByID(gameID)
	if err != nil {
		return check, err
	}
	if a.isRunning(game.ID) {
		return check, apperr.New("game.running",
			fmt.Sprintf("%s is running", game.Title), map[string]string{"title": game.Title})
	}
	if j, ok := a.jobs.Holding(game.ID, jobs.KindMove, jobs.KindVerify, jobs.KindRemove); ok {
		return check, apperr.New("remove.busy",
			fmt.Sprintf("%s already has a %s queued", game.Title, j.Kind),
			map[string]string{"title": game.Title, "kind": j.Kind})
	}
	targets, err := a.removeTargets(game, opts)
	if err != nil {
		return check, err
	}
	for _, t := range targets {
		exists, rerr := a.checkDeleteDirFor(game.ID, t.path, t.kind)
		rt := types.RemoveTarget{Kind: t.kind, Path: t.path, Exists: exists}
		switch {
		case rerr != nil:
			rt.Reason, rt.ReasonCode, rt.ReasonParams = reasonOf(rerr)
		case !exists:
			reason, code, params := reasonOf(apperr.New("remove.notFound",
				fmt.Sprintf("%s is not on disk", t.path), map[string]string{"path": t.path}))
			rt.Reason, rt.ReasonCode, rt.ReasonParams = reason, code, params
		default:
			size, serr := relocate.TreeSize(t.path)
			if serr != nil {
				return check, apperr.Wrap("remove.unreadable",
					fmt.Errorf("refusing to delete %s: it can't be measured: %w", t.path, serr),
					map[string]string{"path": t.path})
			}
			rt.Bytes, rt.Allowed = size, true
		}
		check.Targets = append(check.Targets, rt)
	}
	saves, err := a.savesPreview(game)
	if err != nil {
		return check, err
	}
	check.Saves = saves
	return check, nil
}

// RemoveGame queues deleting a sideloaded game's Wine prefix and/or game
// files, then forgetting it. At least one delete flag is required;
// library-only stays RemoveCustomGame (synchronous, no job).
func (a *App) RemoveGame(gameID string, opts types.RemoveOptions) (types.Job, error) {
	if !opts.DeletePrefix && !opts.DeleteFiles {
		return types.Job{}, apperr.New("remove.nothingToDelete",
			"choose what to delete: the Wine prefix, the game files, or both", nil)
	}
	game, err := a.customGameByID(gameID)
	if err != nil {
		return types.Job{}, err
	}
	if a.isRunning(game.ID) {
		return types.Job{}, apperr.New("game.running",
			fmt.Sprintf("%s is running", game.Title), map[string]string{"title": game.Title})
	}
	if j, ok := a.jobs.Holding(game.ID, jobs.KindMove, jobs.KindVerify, jobs.KindRemove); ok {
		return types.Job{}, apperr.New("remove.busy",
			fmt.Sprintf("%s already has a %s queued", game.Title, j.Kind),
			map[string]string{"title": game.Title, "kind": j.Kind})
	}
	targets, err := a.removeTargets(game, opts)
	if err != nil {
		return types.Job{}, err
	}
	if _, err := a.checkRemoveTargets(game.ID, targets); err != nil {
		return types.Job{}, err
	}
	return a.jobs.AddRemove(game.Source, game.ID, game.StoreAppID, game.Title,
		opts.DeletePrefix, opts.DeleteFiles, opts.AllowNoSaves)
}

// startCoreRemove runs a removal core does itself. It returns at once; the
// deletion runs in its own goroutine and reports through the queue like a
// plugin job. A removal canceled before its run starts never deletes.
func (a *App) startCoreRemove(j types.Job) error {
	ctx, cancel := context.WithCancel(a.ctx)
	run, err := a.removes.begin(j.ID, cancel)
	if err != nil {
		cancel()
		return err
	}
	go func() {
		defer a.removes.end(j.ID, run)
		defer cancel()
		a.runCoreRemove(ctx, j)
	}()
	return nil
}

// runCoreRemove deletes the prefix, then the files, then forgets the library
// entry, last and only on full success. The save gate runs before any
// deletion: on its error nothing is deleted. Missing targets count as done,
// so a retry finishes a partial removal. A partial failure fails the job and
// lists what's left in Leftover.
func (a *App) runCoreRemove(ctx context.Context, j types.Job) {
	fail := func(format string, args ...any) {
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: fmt.Sprintf(format, args...)})
	}
	opts := types.RemoveOptions{DeletePrefix: j.DeletePrefix, DeleteFiles: j.DeleteFiles, AllowNoSaves: j.AllowNoSaves}
	game, err := a.customGameByID(j.GameID)
	if err != nil {
		fail("removing %s: %v", j.Title, err)
		return
	}
	targets, err := a.removeTargets(game, opts)
	if err != nil {
		fail("removing %s: %v", j.Title, err)
		return
	}
	checked, err := a.checkRemoveTargets(game.ID, targets)
	if err != nil {
		fail("removing %s: %v", j.Title, err)
		return
	}
	// remaining names what is still on disk, for cancels and failures.
	remaining := func(done map[string]bool) []string {
		var left []string
		for _, t := range checked {
			if done[t.path] || !t.exists {
				continue
			}
			left = append(left, t.path)
		}
		return left
	}
	if ctx.Err() != nil {
		a.jobs.SetLeftover(j.ID, fmt.Sprintf("still on disk: %s", strings.Join(remaining(nil), ", ")))
		return
	}
	// The gate runs before any deletion: a failed backup deletes nothing.
	if _, err := a.backupBeforeDelete(ctx, game, "remove", opts.AllowNoSaves); err != nil {
		fail("backing up the saves of %s first: %v", j.Title, err)
		return
	}
	if ctx.Err() != nil {
		a.jobs.SetLeftover(j.ID, fmt.Sprintf("still on disk: %s", strings.Join(remaining(nil), ", ")))
		return
	}
	done := map[string]bool{}
	for _, t := range checked {
		if !t.exists {
			continue // a retry counts missing targets as done
		}
		if ctx.Err() != nil {
			a.jobs.SetLeftover(j.ID, fmt.Sprintf("still on disk: %s", strings.Join(remaining(done), ", ")))
			return
		}
		// os.RemoveAll removes nested symlinks themselves without following
		// them: what a link points at stays on disk (tested).
		if err := os.RemoveAll(t.path); err != nil {
			done[t.path] = false
			left := remaining(done)
			a.jobs.SetLeftover(j.ID, fmt.Sprintf("still on disk: %s", strings.Join(left, ", ")))
			fail("deleting %s: %v", t.path, err)
			return
		}
		done[t.path] = true
	}
	// Last and only on full success: forget the library entry (with its
	// per-game rows, see settings.RemoveCustomGame).
	if err := a.settings.RemoveCustomGame(game.ID); err != nil {
		a.jobs.SetLeftover(j.ID, fmt.Sprintf("%s is still in the library: %v", j.Title, err))
		fail("forgetting %s: %v", j.Title, err)
		return
	}
	a.jobs.Notify(j.Source, "job.done", plugins.JobUpdate{JobID: j.ID})
}
