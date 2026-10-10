package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// asideDir is where plugin builds that predate the rename are kept.
const asideDir = "plugins-before-rename"

// legacyEnv are the variables of the plugin protocol before the rename. A
// plugin that reads them would find none of its data folder under the new
// names (it would start logged out), so it must not run until reinstalled.
var legacyEnv = [][]byte{[]byte("HYDRA_PLUGIN_DATA"), []byte("HYDRA_PLUGIN_API")}

const maxExecScan = 64 << 20

// SetAsideLegacyPlugins moves each plugin folder under root/plugins whose
// executable predates the rename (named hydra-*, or mentioning the old
// protocol variables) to root/plugins-before-rename/<id>. The plugin's data
// folder and the user's approval stay where they are, keyed by plugin id: once
// the new build is installed (plugins/install.sh) its content hash differs, so
// Platypus asks for approval once, and the plugin finds its login again. It
// returns the ids it moved.
func SetAsideLegacyPlugins(root string) ([]string, error) {
	pdir := filepath.Join(root, "plugins")
	entries, err := os.ReadDir(pdir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var moved []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(pdir, e.Name())
		if !isLegacyPlugin(dir) {
			continue
		}
		dst, err := freeName(filepath.Join(root, asideDir), e.Name())
		if err != nil {
			return moved, err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return moved, err
		}
		if err := os.Rename(dir, dst); err != nil {
			return moved, err
		}
		moved = append(moved, e.Name())
	}
	if len(moved) > 0 {
		note := "These plugin folders were built before Platypus was renamed and still read the old\n" +
			"environment variables, so they were moved out of plugins/. Install the current build of each\n" +
			"with plugins/install.sh <name> from the Platypus source, approve it again in the Plugins view,\n" +
			"and delete the folder here once it works. Each plugin's login and settings were not touched.\n"
		_ = os.WriteFile(filepath.Join(root, asideDir, "README.txt"), []byte(note), 0o644)
	}
	return moved, nil
}

func freeName(dir, name string) (string, error) {
	p := filepath.Join(dir, name)
	for i := 2; ; i++ {
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			return p, nil
		} else if err != nil {
			return "", err
		}
		p = filepath.Join(dir, fmt.Sprintf("%s-%d", name, i))
	}
}

// isLegacyPlugin reports whether a plugin folder holds a build from before
// the rename. Data plugins (no exec) are plain JSON and never are.
func isLegacyPlugin(dir string) bool {
	raw, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return false
	}
	var m struct {
		Exec string `json:"exec"`
	}
	if json.Unmarshal(raw, &m) != nil || m.Exec == "" {
		return false
	}
	if strings.HasPrefix(filepath.Base(m.Exec), legacyName+"-") {
		return true
	}
	exe := filepath.Join(dir, filepath.FromSlash(m.Exec))
	if !strings.HasPrefix(filepath.Clean(exe), filepath.Clean(dir)+string(filepath.Separator)) {
		return false
	}
	fi, err := os.Stat(exe)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxExecScan {
		return false
	}
	body, err := os.ReadFile(exe)
	if err != nil {
		return false
	}
	for _, v := range legacyEnv {
		if bytes.Contains(body, v) {
			return true
		}
	}
	return false
}
