package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"platypus/internal/apperr"
	"platypus/internal/crashhints"
	"platypus/internal/runner"
	"platypus/internal/types"
	"platypus/internal/winetricks"
)

// GameCrashHints returns the suggestions for the game's last launch when that
// run ended abnormally (issue #46). It is nil when the run was fine or nothing
// in its log matches a known pattern. The log is read here and never leaves
// the machine.
func (a *App) GameCrashHints(game types.Game) *types.CrashHints {
	path := a.sessions.CrashLog(game.ID)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	hints := crashhints.Match(string(raw))
	if len(hints) == 0 {
		return nil
	}
	_, lookErr := exec.LookPath("winetricks")
	return &types.CrashHints{Log: path, Winetricks: lookErr == nil, Hints: hints}
}

// RunWinetricksVerb installs a winetricks verb into the game's prefix: the
// one-click fix for a crash hint (issue #46). winetricks runs unattended in the
// background with its output in logs/winetricks-<id>.log. It refuses a verb
// outside the pinned list, a running game, and a game with no prefix yet.
func (a *App) RunWinetricksVerb(game types.Game, verb string) error {
	if !winetricks.KnownVerb(verb) {
		return apperr.New("crash.badVerb", fmt.Sprintf("%q is not a winetricks verb", verb), map[string]string{"verb": verb})
	}
	if a.isRunning(game.ID) {
		return apperr.New("game.running", fmt.Sprintf("%s is already running", game.Title), map[string]string{"title": game.Title})
	}
	bin, err := exec.LookPath("winetricks")
	if err != nil {
		return apperr.New("crash.noWinetricks", "winetricks isn't installed", nil)
	}
	r, err := a.selectRunner(game)
	if err != nil {
		return err
	}
	env, err := winetricksEnv(game, runner.PrefixPath(game, a.homeDir(), a.prefixesFolder()), r)
	if err != nil {
		return err
	}
	logPath := filepath.Join(a.resolvedDataDir(), "logs", "winetricks-"+game.ID+".log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "--unattended", verb)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		_ = out.Close()
		return apperr.Wrap("crash.winetricksFailed", err, map[string]string{"verb": verb})
	}
	go func() {
		_ = cmd.Wait()
		_ = out.Close()
	}()
	return nil
}

// protonBinDirs are where a Proton build keeps its Wine binaries, relative to
// its folder: newer builds ship files/, older ones dist/.
var protonBinDirs = []string{"files/bin", "dist/bin"}

// winetricksEnv is the environment winetricks runs with, so it works on the
// prefix the game runs in (issue #46). prefix is the game's Platypus prefix. A
// Proton build keeps its Wine prefix in <prefix>/pfx and runs with its own wine
// and wineserver. A plain Wine runner runs with its own binary. The system Wine
// (no path) is left to PATH. umu picks its own prefix and Proton, so it is
// refused rather than guessed at.
func winetricksEnv(game types.Game, prefix string, r types.ProtonVersion) (map[string]string, error) {
	if runner.IsUmu(r) {
		return nil, apperr.New("crash.umuUnsupported", fmt.Sprintf("%s starts through umu, which winetricks can't reach yet", game.Title), map[string]string{"title": game.Title})
	}
	isProton := strings.HasSuffix(r.Path, "proton")
	wineprefix := prefix
	if isProton {
		wineprefix = filepath.Join(prefix, "pfx")
	}
	if !isDir(wineprefix) {
		return nil, apperr.New("crash.noPrefix", fmt.Sprintf("%s has no Wine prefix yet", game.Title), map[string]string{"title": game.Title})
	}
	env := map[string]string{"WINEPREFIX": wineprefix}
	switch {
	case isProton:
		wine, server, ok := protonWine(filepath.Dir(r.Path))
		if !ok {
			return nil, apperr.New("crash.noProtonWine", fmt.Sprintf("the Proton build of %s has no Wine binaries", game.Title), map[string]string{"title": game.Title})
		}
		env["WINE"], env["WINESERVER"] = wine, server
	case r.Path != "":
		env["WINE"] = r.Path
	}
	return env, nil
}

// protonWine finds the wine and wineserver binaries of the Proton build in
// protonDir, in the first bin folder that has both.
func protonWine(protonDir string) (wine, wineserver string, ok bool) {
	for _, d := range protonBinDirs {
		w, s := filepath.Join(protonDir, d, "wine"), filepath.Join(protonDir, d, "wineserver")
		if isFile(w) && isFile(s) {
			return w, s, true
		}
	}
	return "", "", false
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// OpenCrashLog opens the log of the game's last abnormal run in the default
// viewer (issue #46).
func (a *App) OpenCrashLog(game types.Game) error {
	path := a.sessions.CrashLog(game.ID)
	if path == "" {
		return apperr.New("crash.noLog", fmt.Sprintf("%s has no failed launch to show", game.Title), map[string]string{"title": game.Title})
	}
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return apperr.Wrap("crash.openLogFailed", err, map[string]string{"title": game.Title})
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
