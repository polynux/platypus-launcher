package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// Discard asks the job's plugin to delete the partial data of a canceled
// download. Plugins own their staging folders; Hydra only checks the install
// folder it names and which job it is about.
func (b jobBackend) Discard(ctx context.Context, j types.Job) error {
	p, ok := b.a.plugins.Plugin(j.Source)
	if !ok || !slices.Contains(p.Manifest.Capabilities, "discard") {
		return jobs.ErrDiscardUnsupported
	}
	params, err := discardParams(j, b.a.settings.GamesFolder())
	if err != nil {
		return err
	}
	err = b.a.plugins.Call(ctx, j.Source, "job.discard", params, nil)
	if errors.Is(err, plugins.ErrExited) {
		return fmt.Errorf("%s stopped while deleting the data", p.Manifest.Name)
	}
	return err
}

// discardParams builds job.discard's params: the job to forget, what it was
// (a canceled update must never take the installed game with it) and the
// plugin install folder it ran in. The plugin finds the game's own subfolder
// itself.
func discardParams(j types.Job, gamesRoot string) (map[string]any, error) {
	if j.Kind == jobs.KindUninstall {
		return nil, fmt.Errorf("an uninstall has no downloaded data")
	}
	if err := checkDiscardDir(j.InstallDir, gamesRoot); err != nil {
		return nil, err
	}
	return map[string]any{"jobId": j.ID, "id": j.StoreGameID, "kind": j.Kind, "installDir": j.InstallDir}, nil
}

// checkDiscardDir refuses install folders that no plugin may clean inside:
// anything relative or unclean, the filesystem root, system folders, the home
// folder or a parent of it, and the Games folder itself (a plugin's folder is
// always a subfolder of it, or a folder the user picked on purpose).
func checkDiscardDir(dir, gamesRoot string) error {
	if dir == "" || !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return fmt.Errorf("refusing to clean %q: not a clean absolute path", dir)
	}
	if dir == "/" {
		return fmt.Errorf("refusing to clean the filesystem root")
	}
	for _, sys := range []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib32", "/lib64", "/proc", "/run", "/sbin", "/sys", "/usr", "/var"} {
		if dir == sys || strings.HasPrefix(dir, sys+"/") {
			return fmt.Errorf("refusing to clean system folder %s", dir)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		home = filepath.Clean(home)
		if dir == home || strings.HasPrefix(home, dir+"/") {
			return fmt.Errorf("refusing to clean your home folder or one above it (%s)", dir)
		}
	}
	if root := filepath.Clean(gamesRoot); gamesRoot != "" && dir == root {
		return fmt.Errorf("refusing to clean the Games folder itself (%s)", dir)
	}
	return nil
}
