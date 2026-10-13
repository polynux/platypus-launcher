package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"platypus/internal/plugins"
)

// Bundled store plugins (issue #54): the core ships the steam plugin and
// installs it on startup when it's missing, so existing users lose nothing.
// The bundled copy is a folder of ready plugin folders: $PLATYPUS_BUNDLED_PLUGINS
// when set to an absolute path, else <dir of the core executable>/plugins
// (see docs/packaging.md). Installs are atomic (temp dir + rename) and never
// touch a folder that isn't the same plugin.

// bundledPluginsDir names the folder of ready plugin folders, or "".
func bundledPluginsDir() string {
	if v := os.Getenv("PLATYPUS_BUNDLED_PLUGINS"); v != "" && filepath.IsAbs(v) {
		if st, err := os.Stat(v); err == nil && st.IsDir() {
			return v
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Join(filepath.Dir(exe), "plugins")
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir
	}
	return ""
}

// ensureBundledSteam installs or updates the bundled steam plugin and
// returns the plugins to list. When nothing needs doing it returns found.
func (a *App) ensureBundledSteam(found []plugins.Plugin) []plugins.Plugin {
	const id = "steam"
	bundledDir := bundledPluginsDir()
	if bundledDir == "" {
		return found
	}
	src := filepath.Join(bundledDir, id)
	bundled, err := plugins.LoadManifest(src)
	if err != nil {
		a.host.Warnf("bundled steam plugin: %v", err)
		return found
	}
	if bundled.ID != id {
		a.host.Warnf("bundled steam plugin has id %q, want %q", bundled.ID, id)
		return found
	}
	cur, haveCur := a.plugins.Plugin(id)
	if !haveCur {
		// Something sits where the plugin goes but the manager doesn't
		// know it as steam: never touch a different folder.
		if _, err := os.Stat(filepath.Join(a.plugins.Dir(), id)); err == nil {
			a.host.Warnf("bundled steam plugin: %s exists but isn't the steam plugin, leaving it alone", filepath.Join(a.plugins.Dir(), id))
			return found
		}
		if err := installBundledDir(a.plugins.Dir(), id, src); err != nil {
			a.host.Warnf("bundled steam plugin: %v", err)
			return found
		}
		updated, err := a.plugins.Discover()
		if err != nil {
			a.host.Warnf("bundled steam plugin: %v", err)
			return found
		}
		if err := a.plugins.Approve(id); err != nil {
			a.host.Warnf("bundled steam plugin: %v", err)
			return updated
		}
		a.host.Infof("installed the bundled Steam plugin (v%s)", bundled.Version)
		updated, _ = a.plugins.Discover()
		return updated
	}
	installed, err := plugins.LoadManifest(cur.Dir)
	if err != nil || installed.ID != id {
		// Not the same plugin folder: never touch it.
		a.host.Warnf("bundled steam plugin: %s is not the steam plugin, leaving it alone", cur.Dir)
		return found
	}
	if compareVersions(bundled.Version, installed.Version) <= 0 {
		return found // up to date, or newer (a user install from install.sh): leave it alone
	}
	if err := installBundledDir(a.plugins.Dir(), id, src); err != nil {
		a.host.Warnf("bundled steam plugin: %v", err)
		return found
	}
	updated, err := a.plugins.Discover()
	if err != nil {
		a.host.Warnf("bundled steam plugin: %v", err)
		return found
	}
	// Re-pin the new hash, keeping the user's choices (issue #54): an
	// update must not re-enable a plugin the user turned off, nor drop
	// its install folder or allowed folders.
	state, _ := a.settings.PluginState(id)
	if next, ok := a.plugins.Plugin(id); ok {
		state.Hash = next.Hash
		if _, had := a.settings.PluginState(id); !had {
			state.Enabled = true
		}
		if err := a.settings.SetPluginState(id, state); err != nil {
			a.host.Warnf("bundled steam plugin: %v", err)
			return updated
		}
		updated, _ = a.plugins.Discover()
	}
	a.host.Infof("updated the bundled Steam plugin (v%s -> v%s)", installed.Version, bundled.Version)
	return updated
}

// installBundledDir copies the ready plugin folder src into pluginsDir/id
// atomically: a temp dir next to the target, then renames. An existing
// target is only replaced after the caller verified it is the same plugin;
// it is moved aside first so a failed rename can roll back, and never
// deleted outright.
func installBundledDir(pluginsDir, id, src string) error {
	if pluginsDir == "" || id == "" || src == "" {
		return fmt.Errorf("bundled %s: bad paths", id)
	}
	if strings.ContainsAny(id, `/\.`) || id == "." || id == ".." {
		return fmt.Errorf("bundled %s: bad plugin id", id)
	}
	dst := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return fmt.Errorf("bundled %s: %w", id, err)
	}
	tmp, err := os.MkdirTemp(pluginsDir, ".bundled-"+id+".*")
	if err != nil {
		return fmt.Errorf("bundled %s: %w", id, err)
	}
	// From here on every failure removes the temp dir; the target is only
	// touched by renames.
	if err := copyBundledDir(src, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("bundled %s: %w", id, err)
	}
	if _, err := os.Stat(dst); err == nil {
		bak, err := os.MkdirTemp(pluginsDir, ".bundled-"+id+".bak.*")
		if err != nil {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("bundled %s: %w", id, err)
		}
		_ = os.Remove(bak)
		if err := os.Rename(dst, bak); err != nil {
			_ = os.RemoveAll(tmp)
			_ = os.Remove(bak)
			return fmt.Errorf("bundled %s: %w", id, err)
		}
		if err := os.Rename(tmp, dst); err != nil {
			_ = os.Rename(bak, dst)
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("bundled %s: %w", id, err)
		}
		_ = os.RemoveAll(bak)
		return nil
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("bundled %s: %w", id, err)
	}
	return nil
}

// copyBundledDir replicates the ready plugin folder: dirs, regular files
// with their mode, and symlinks as symlinks (so the installed hash matches
// what the user would approve).
func copyBundledDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		out := filepath.Join(dst, rel)
		fi, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case fi.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, out)
		case fi.IsDir():
			return os.MkdirAll(out, fi.Mode().Perm())
		case fi.Mode().IsRegular():
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.WriteFile(out, body, fi.Mode().Perm()); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("bundled: refusing special file %s", rel)
		}
	})
}

// compareVersions orders manifest versions ("1.0.0", "0.4.0"): numeric
// parts compare numerically, other parts lexically, a missing part counts
// as zero. It is only for the bundled-newer check, not a semver promise.
func compareVersions(a, b string) int {
	pa, pb := splitVersion(a), splitVersion(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		x, y := "0", "0"
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x == y {
			continue
		}
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case xerr == nil && yerr == nil:
			if xn != yn {
				if xn < yn {
					return -1
				}
				return 1
			}
		case x == "":
			return -1
		case y == "":
			return 1
		default:
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func splitVersion(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' || r == '+' })
}
