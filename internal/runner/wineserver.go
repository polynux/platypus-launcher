package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"platypus/internal/types"
)

// WineserverStopCommand builds the command that terminates the wineserver of
// a game's Wine prefix, or nil when there is nothing to stop (native game, no
// prefix, no wineserver next to the runner). env is the launch environment.
// Wine games outlive their launcher process group because wineserver
// daemonizes; "wineserver -k" ends every process of the prefix. With force it
// uses SIGKILL ("-k9").
func WineserverStopCommand(r types.ProtonVersion, env map[string]string, force bool) *exec.Cmd {
	prefix := env["WINEPREFIX"]
	if r.Path == "" || prefix == "" {
		return nil
	}
	if IsUmu(r) {
		// umu's WINEPREFIX is already the Wine prefix itself (see
		// EffectiveUmuPrefix): never append pfx. Its wineserver ships with
		// the PROTONPATH build; a codename (UMU-Latest) names no local
		// build, so there is nothing to stop.
		protonPath := env["PROTONPATH"]
		if protonPath == "" || !filepath.IsAbs(protonPath) {
			return nil
		}
		for _, c := range []string{
			filepath.Join(protonPath, "files", "bin", "wineserver"),
			filepath.Join(protonPath, "dist", "bin", "wineserver"),
		} {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return stopCmd(c, prefix, force)
			}
		}
		return nil
	}
	bin := wineserverFor(r.Path)
	if bin == "" {
		return nil
	}
	if strings.HasSuffix(r.Path, "proton") {
		// STEAM_COMPAT_DATA_PATH holds the real prefix under pfx/.
		prefix = filepath.Join(prefix, "pfx")
	}
	return stopCmd(bin, prefix, force)
}

func stopCmd(bin, prefix string, force bool) *exec.Cmd {
	arg := "-k"
	if force {
		arg = "-k9"
	}
	cmd := exec.Command(bin, arg)
	cmd.Env = append(os.Environ(), "WINEPREFIX="+prefix)
	return cmd
}

// wineserverFor locates the wineserver binary shipped with a runner.
func wineserverFor(runnerPath string) string {
	if !filepath.IsAbs(runnerPath) {
		// A system wine found through PATH.
		if p, err := exec.LookPath("wineserver"); err == nil {
			return p
		}
		return ""
	}
	dir := filepath.Dir(runnerPath)
	var candidates []string
	if strings.HasSuffix(runnerPath, "proton") {
		candidates = []string{
			filepath.Join(dir, "files", "bin", "wineserver"),
			filepath.Join(dir, "dist", "bin", "wineserver"),
		}
	} else {
		candidates = []string{filepath.Join(dir, "wineserver")}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}
