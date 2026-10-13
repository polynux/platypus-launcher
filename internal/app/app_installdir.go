package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"platypus/internal/jobs"
	"platypus/internal/settings"
	"platypus/internal/types"
	"platypus/internal/apperr"
)

// Install location: where a game installs. Core owns installDir (plugins never
// choose paths), so a different place is a different installDir in the job.
// By default that is the plugin's install folder (<Games folder>/<store>).
// The user can pick one of their game folders ("libraries": each one holds a
// folder per store, like the Games folder does), a folder they installed into
// before, or any folder through a picker; the last is used as the install
// folder as it is. Every choice passes the same guards as a move
// (checkFolder), and a folder picked this way is allowed for the plugin's
// launches, because the game would otherwise read as installed "outside" it.

// folderState reads a folder's disk: whether it exists and the room there
// (-1 when unreadable). A folder that doesn't exist yet reports the room of
// its nearest existing parent, where it would be created.
func folderState(dir string) (exists bool, free int64) {
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		exists = true
	}
	f, err := jobs.FreeSpace(dir)
	if err != nil {
		return exists, -1
	}
	return exists, int64(f)
}

// existingAncestor is the nearest folder at or above dir that exists.
func existingAncestor(dir string) (string, error) {
	for p := dir; ; p = filepath.Dir(p) {
		st, err := os.Stat(p)
		if err == nil {
			if !st.IsDir() {
				return "", apperr.New("install.folder.isFile", fmt.Sprintf("%s is a file, not a folder", p), map[string]string{"path": p})
			}
			return p, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if filepath.Dir(p) == p {
			return "", apperr.New("install.folder.noParent", fmt.Sprintf("no folder exists above %s", dir), map[string]string{"dir": dir})
		}
	}
}

// checkInstallFolder validates a folder to install into and returns it clean.
// The folder doesn't have to exist (the plugin creates it), but its nearest
// existing parent must be a folder the user can write to; the folder, as
// symlinks resolve, must pass checkFolder; and it must not be inside Platypus's
// own data folder.
func (a *App) checkInstallFolder(folder string) (string, error) {
	home := a.homeDir()
	clean, err := settings.CleanFolderFor(folder, home)
	if err != nil {
		return "", err
	}
	if err := checkFolderFor(clean, "", "install into", home); err != nil {
		return "", err
	}
	anc, err := existingAncestor(clean)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(anc)
	if err != nil {
		return "", err
	}
	// The same folder, as the disk names it: a symlink into /usr or the
	// home folder itself is refused like the path would be.
	resolved := filepath.Join(real, strings.TrimPrefix(clean, anc))
	if err := checkFolderFor(resolved, "", "install into", home); err != nil {
		return "", err
	}
	if data := a.resolvedDataDir(); data != "" && (resolved == data || strings.HasPrefix(resolved, data+string(filepath.Separator))) {
		return "", apperr.New("install.folder.ownData", fmt.Sprintf("refusing to install into Platypus's own data folder (%s)", data), map[string]string{"path": data})
	}
	if err := syscall.Access(anc, 2); err != nil { // W_OK
		return "", apperr.New("install.folder.notWritable", fmt.Sprintf("you can't write to %s", anc), map[string]string{"path": anc})
	}
	return clean, nil
}

// CheckInstallFolder says what Platypus knows about a folder to install into: the
// clean path and the room there. Refusals come back as the error.
func (a *App) CheckInstallFolder(folder string) (types.InstallFolderCheck, error) {
	clean, err := a.checkInstallFolder(folder)
	if err != nil {
		return types.InstallFolderCheck{}, err
	}
	exists, free := folderState(clean)
	return types.InstallFolderCheck{Folder: clean, Exists: exists, FreeBytes: free}, nil
}

// GetInstallLocations lists where the install dialog can put a game: the
// store's install folder first, then each of the user's game folders (as the
// store's folder inside it), then folders picked before. Folders that repeat
// are listed once.
func (a *App) GetInstallLocations(game types.Game) ([]types.InstallLocation, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return nil, apperr.New("game.notFromStore", fmt.Sprintf("%s doesn't come from a store plugin", game.Title), map[string]string{"title": game.Title})
	}
	def, _ := a.plugins.InstallFolder(game.Source, a.settings.GamesFolder())
	var out []types.InstallLocation
	add := func(kind, folder, root string) {
		if slices.ContainsFunc(out, func(l types.InstallLocation) bool { return l.Folder == folder }) {
			return
		}
		exists, free := folderState(folder)
		out = append(out, types.InstallLocation{Folder: folder, Kind: kind, Root: root, FreeBytes: free, Exists: exists})
	}
	add("default", def, "")
	for _, root := range a.settings.GameFolders() {
		add("library", p.Manifest.InstallPath(root), root)
	}
	for _, recent := range a.settings.RecentInstallFolders() {
		add("recent", recent, "")
	}
	return out, nil
}

// PickInstallFolder asks where to install a game. Returns "" when the user cancels.
func (a *App) PickInstallFolder(start string) (string, error) {
	return a.chooseFolder("Install the game into", start)
}

// installFolderFor is the install folder a job of kind gets for a game: the
// folder the user picked, checked, or the plugin's own. Only a fresh install
// can be placed; an installed game updates where it is.
func (a *App) installFolderFor(game types.Game, folder string) (string, bool, error) {
	if folder == "" {
		return "", false, nil
	}
	if game.IsInstalled {
		return "", false, apperr.New("install.installedUseMove", fmt.Sprintf("%s is installed; use Move to put it elsewhere", game.Title), map[string]string{"title": game.Title})
	}
	clean, err := a.checkInstallFolder(folder)
	if err != nil {
		return "", false, err
	}
	def, _ := a.plugins.InstallFolder(game.Source, a.settings.GamesFolder())
	return clean, clean != def, nil
}

// rememberInstallFolder makes a folder picked for an install count: the plugin
// may launch games from it, and it comes first in the recent list.
func (a *App) rememberInstallFolder(source, folder string) {
	inside := slices.ContainsFunc(a.plugins.Folders(source, a.settings.GamesFolder()), func(f string) bool {
		return folder == f || strings.HasPrefix(folder, f+string(filepath.Separator))
	})
	if !inside {
		if err := a.plugins.AllowFolder(source, folder); err != nil {
			a.warnf("install: couldn't allow %s for %s: %v", folder, source, err)
		}
	}
	if err := a.settings.AddRecentInstallFolder(folder); err != nil {
		a.warnf("install: couldn't remember %s: %v", folder, err)
	}
}

// InstallGameTo queues an install into a folder the user picked ("" is the
// store's own folder, like InstallGame).
func (a *App) InstallGameTo(game types.Game, folder string) (types.Job, error) {
	if game.IsInstalled {
		return types.Job{}, apperr.New("install.alreadyInstalled", fmt.Sprintf("%s is already installed", game.Title), map[string]string{"title": game.Title})
	}
	if !game.Installable {
		return types.Job{}, apperr.New("install.cannotInstall", fmt.Sprintf("%s can't be installed by Platypus", game.Title), map[string]string{"title": game.Title})
	}
	target, err := a.pinnedTarget(game)
	if err != nil {
		return types.Job{}, err
	}
	return a.queueJobIn(jobs.KindInstall, game, target, a.savedOptions(game), folder)
}

// InstallGameWithOptionsTo is InstallGameWithOptions into a folder the user
// picked ("" is the store's own folder).
func (a *App) InstallGameWithOptionsTo(game types.Game, options map[string]any, folder string) (types.Job, error) {
	if game.IsInstalled {
		return types.Job{}, apperr.New("install.alreadyInstalled", fmt.Sprintf("%s is already installed", game.Title), map[string]string{"title": game.Title})
	}
	if !game.Installable {
		return types.Job{}, apperr.New("install.cannotInstall", fmt.Sprintf("%s can't be installed by Platypus", game.Title), map[string]string{"title": game.Title})
	}
	return a.queueWithOptionsIn(jobs.KindInstall, game, options, folder)
}

// gameFolderList is the Settings list: the default folder first, then the
// library folders.
func (a *App) gameFolderList() []types.GameFolder {
	paths := append([]string{a.settings.GamesFolder()}, a.settings.GameFolders()...)
	out := make([]types.GameFolder, 0, len(paths))
	for i, p := range paths {
		exists, free := folderState(p)
		out = append(out, types.GameFolder{Path: p, IsDefault: i == 0, Exists: exists, FreeBytes: free})
	}
	return out
}

// GetGameFolders lists the user's game folders, the default one first.
func (a *App) GetGameFolders() []types.GameFolder { return a.gameFolderList() }

// MakeDefaultGameFolder makes a game folder the default one, where installs go.
// A folder not in the list yet is added, and it is checked like an install
// folder. The old default stays in the list.
func (a *App) MakeDefaultGameFolder(path string) ([]types.GameFolder, error) {
	clean, err := a.checkInstallFolder(path)
	if err != nil {
		return a.gameFolderList(), err
	}
	if _, err := a.settings.SetGamesFolder(clean); err != nil {
		return a.gameFolderList(), err
	}
	return a.gameFolderList(), nil
}

// AddGameFolder adds a library folder to install into. It is checked like an
// install folder; it need not exist yet.
func (a *App) AddGameFolder(path string) ([]types.GameFolder, error) {
	clean, err := a.checkInstallFolder(path)
	if err != nil {
		return a.gameFolderList(), err
	}
	if _, err := a.settings.AddGameFolder(clean); err != nil {
		return a.gameFolderList(), err
	}
	return a.gameFolderList(), nil
}

// RemoveGameFolder takes a library folder off the list. The default folder
// stays until another one is made the default. Games already in the removed
// folder keep working.
func (a *App) RemoveGameFolder(path string) ([]types.GameFolder, error) {
	if _, err := a.settings.RemoveGameFolder(path); err != nil {
		return a.gameFolderList(), err
	}
	return a.gameFolderList(), nil
}

// ChooseGameFolder opens a folder picker to add a game folder. Returns "" if
// the user cancels.
func (a *App) ChooseGameFolder() (string, error) {
	return a.chooseFolder("Add a game folder", a.settings.GamesFolder())
}
