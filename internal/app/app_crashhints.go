package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	prefix := runner.PrefixPath(game, a.homeDir(), a.prefixesFolder())
	if _, err := os.Stat(prefix); err != nil {
		return apperr.New("crash.noPrefix", fmt.Sprintf("%s has no Wine prefix yet", game.Title), map[string]string{"title": game.Title})
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
	cmd.Env = append(os.Environ(), "WINEPREFIX="+prefix)
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
