// Command platypus-runner provides Wine, Proton, umu and native launch wrapping.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"platypus/internal/crashhints"
	"platypus/internal/plugins"
	"platypus/internal/runner"
	"platypus/internal/types"
	"platypus/internal/winetricks"
)

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// codedError is a runner failure the core shows with its own translated
// message: code is the apperr code and params its values (protocol.md, Runner).
type codedError struct {
	code   string
	params map[string]string
	msg    string
}

func (e *codedError) Error() string { return e.msg }

// rpcFailure answers err with code num. A coded failure carries its apperr code
// and params in error.data; any other failure is only its message.
func rpcFailure(num int, err error) *rpcError {
	out := &rpcError{Code: num, Message: err.Error()}
	var ce *codedError
	if errors.As(err, &ce) {
		out.Data, _ = json.Marshal(map[string]any{"code": ce.code, "params": ce.params})
	}
	return out
}

type wrapParams struct {
	Spec struct {
		Command   []string          `json:"command"`
		Env       map[string]string `json:"env"`
		Cwd       string            `json:"cwd"`
		NeedsWine bool              `json:"needsWine"`
	} `json:"spec"`
	Game           types.Game `json:"game"`
	RunnerID       string     `json:"runnerId"`
	PrefixesFolder string     `json:"prefixesFolder"`
	Preview        bool       `json:"preview"`
}

type prefixParams struct {
	Game           types.Game `json:"game"`
	RunnerID       string     `json:"runnerId"`
	PrefixesFolder string     `json:"prefixesFolder"`
}

type actionParams struct {
	ID             string     `json:"id"`
	Verb           string     `json:"verb"`
	Game           types.Game `json:"game"`
	RunnerID       string     `json:"runnerId"`
	PrefixesFolder string     `json:"prefixesFolder"`
	Force          bool       `json:"force"`
	// Launch is the runner.wrap result of the launch being stopped, handed back
	// unread by the core. Only the stop action reads it.
	Launch struct {
		Env    map[string]string  `json:"env"`
		Runner plugins.RunnerInfo `json:"runner"`
	} `json:"launch"`
}

type hintsParams struct {
	Game     types.Game `json:"game"`
	RunnerID string     `json:"runnerId"`
	Log      string     `json:"log"`
}

func handle(req message) message {
	resp := message{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		var p struct {
			Protocols []string `json:"protocols"`
			Locale    string   `json:"locale"`
		}
		_ = json.Unmarshal(req.Params, &p)
		setLocale(p.Locale)
		chosen := "1.0"
		for _, v := range p.Protocols {
			if v == "1.17" {
				chosen = v
			}
		}
		resp.Result = map[string]any{"protocol": chosen, "capabilities": []string{"runner"}}
	case "runner.list":
		rs := runner.ScanProtonVersionsFor(os.Getenv("HOME"))
		out := make([]plugins.RunnerInfo, 0, len(rs))
		for _, r := range rs {
			out = append(out, runnerInfo(r))
		}
		resp.Result = map[string]any{"runners": out}
	case "runner.options":
		resp.Result = runnerSchema()
	case "runner.env":
		var p struct {
			Game types.Game `json:"game"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Game.ID == "" {
			resp.Error = &rpcError{Code: -32602, Message: "game.id is required"}
			break
		}
		s := effectiveSettings(p.Game.Settings)
		var wrappers []string
		if s.UseGameMode {
			wrappers = append(wrappers, "gamemode")
		}
		if s.UseMangoHud {
			wrappers = append(wrappers, "mangohud")
		}
		resp.Result = map[string]any{"env": s.EnvVars, "wrappers": wrappers}
	case "runner.detectExecutable":
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Path == "" {
			resp.Error = &rpcError{Code: -32602, Message: "path is required"}
			break
		}
		d, err := runner.DetectExecutable(p.Path)
		if err != nil {
			resp.Error = &rpcError{Code: -32003, Message: err.Error()}
			break
		}
		resp.Result = map[string]any{"native": d.Native, "kind": string(d.Kind)}
	case "runner.wrap":
		var p wrapParams
		if err := json.Unmarshal(req.Params, &p); err != nil || len(p.Spec.Command) == 0 {
			resp.Error = &rpcError{Code: -32602, Message: "spec.command is required"}
			break
		}
		p.Game.Settings = effectiveSettings(p.Game.Settings)
		home := os.Getenv("HOME")
		target := runner.Target{Exe: p.Spec.Command[0], Args: p.Spec.Command[1:], Env: p.Spec.Env, Native: !p.Spec.NeedsWine}
		if target.Native {
			// A native game runs with no runner and no prefix; the wrappers still apply.
			command, env, err := runner.BuildCommandFor(p.Game, types.ProtonVersion{}, target, p.Preview, p.PrefixesFolder, home)
			if err != nil {
				resp.Error = &rpcError{Code: -32003, Message: err.Error()}
				break
			}
			resp.Result = map[string]any{"command": command, "env": env, "cwd": p.Spec.Cwd}
			break
		}
		// With no explicit runner, an installed umu-launcher is preferred and wraps
		// the auto Proton pick (issue #48); explicit "umu" picks it directly. The
		// runner reported is the umu entry, so cloud saves and the stop hook see
		// umu's prefix layout.
		rs := runner.ScanProtonVersionsFor(home)
		selected, proton, useUmu := runner.SelectUmu(p.RunnerID, rs)
		if !useUmu {
			selected, _ = runner.Resolve(p.RunnerID, rs)
		}
		if selected.ID == "" {
			if runner.IsAuto(p.RunnerID) {
				resp.Error = rpcFailure(-32003, &codedError{code: "game.noRunner", msg: "no Proton or Wine runner is installed"})
			} else {
				resp.Error = rpcFailure(-32003, &codedError{code: "game.runnerMissing", params: map[string]string{"id": p.RunnerID},
					msg: fmt.Sprintf("runner %q is not installed", p.RunnerID)})
			}
			break
		}
		var command []string
		var env map[string]string
		var err error
		if useUmu {
			compatID := ""
			if p.Game.Store == "steam" {
				compatID = p.Game.StoreAppID
			}
			command, env, err = runner.BuildUmuCommandFor(p.Game, proton, selected.Path, compatID, target, p.Preview, p.PrefixesFolder, home)
		} else {
			command, env, err = runner.BuildCommandFor(p.Game, selected, target, p.Preview, p.PrefixesFolder, home)
		}
		if err != nil {
			resp.Error = &rpcError{Code: -32003, Message: err.Error()}
			break
		}
		resp.Result = map[string]any{"command": command, "env": env, "cwd": p.Spec.Cwd, "runner": runnerInfo(selected)}
	case "runner.prefix":
		var p prefixParams
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Game.ID == "" {
			resp.Error = &rpcError{Code: -32602, Message: "game.id is required"}
			break
		}
		if p.Game.Native {
			resp.Result = map[string]any{}
			break
		}
		dir, user := gamePrefix(p.Game, p.RunnerID, p.PrefixesFolder, os.Getenv("HOME"))
		resp.Result = map[string]any{"dir": dir, "user": user}
	case "runner.actions":
		var p actionParams
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Game.ID == "" {
			resp.Error = &rpcError{Code: -32602, Message: "game.id is required"}
			break
		}
		prefix := runner.PrefixPath(p.Game, os.Getenv("HOME"), p.PrefixesFolder)
		actions := []any{}
		if _, err := os.Stat(prefix); err == nil {
			actions = append(actions, map[string]any{"id": "open-prefix", "label": "Open prefix"})
			if _, err := exec.LookPath("winetricks"); err == nil {
				actions = append(actions, map[string]any{"id": "winetricks", "label": "Run winetricks"})
			}
		}
		resp.Result = map[string]any{"actions": actions}
	case "runner.runAction":
		var p actionParams
		if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" || (p.ID != "stop" && p.Game.ID == "") {
			resp.Error = &rpcError{Code: -32602, Message: "id is required, and game.id unless the action is stop"}
			break
		}
		switch p.ID {
		case "stop":
			cmd := runner.WineserverStopCommand(types.ProtonVersion{ID: p.Launch.Runner.ID, Path: p.Launch.Runner.Path}, p.Launch.Env, p.Force)
			if cmd != nil {
				if err := cmd.Start(); err != nil {
					resp.Error = &rpcError{Code: -32003, Message: err.Error()}
					break
				}
				done := make(chan struct{})
				go func() { _ = cmd.Wait(); close(done) }()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					_ = cmd.Process.Kill()
				}
			}
			resp.Result = map[string]any{}
		case "open-prefix":
			prefix := runner.PrefixPath(p.Game, os.Getenv("HOME"), p.PrefixesFolder)
			cmd := exec.Command("xdg-open", prefix)
			if err := cmd.Start(); err != nil {
				resp.Error = &rpcError{Code: -32003, Message: err.Error()}
			} else {
				go func() { _ = cmd.Wait() }()
				resp.Result = map[string]any{}
			}
		case "winetricks":
			if !winetricks.KnownVerb(p.Verb) {
				resp.Error = rpcFailure(-32602, &codedError{code: "crash.badVerb", params: map[string]string{"verb": p.Verb},
					msg: fmt.Sprintf("%q is not a known winetricks verb", p.Verb)})
				break
			}
			if err := runWinetricks(p); err != nil {
				resp.Error = rpcFailure(-32003, err)
			} else {
				resp.Result = map[string]any{}
			}
		default:
			resp.Error = &rpcError{Code: -32602, Message: "unknown runner action"}
		}
	case "runner.crashHints":
		var p hintsParams
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Log == "" {
			resp.Error = &rpcError{Code: -32602, Message: "log is required"}
			break
		}
		resp.Result = crashhints.Match(p.Log)
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return resp
}

func runnerInfo(r types.ProtonVersion) plugins.RunnerInfo {
	kind := "wine"
	switch {
	case runner.IsUmu(r):
		kind = "umu"
	case strings.HasSuffix(r.Path, "proton"):
		kind = "proton"
	case r.IsCustom:
		kind = "custom"
	case r.IsSystem:
		kind = "system"
	}
	return plugins.RunnerInfo{ID: r.ID, Name: r.Name, Kind: kind, Version: r.Name, Default: r.IsDefault, Path: r.Path, System: r.IsSystem, Custom: r.IsCustom}
}

func prefixUser(r types.ProtonVersion) string {
	if runner.IsUmu(r) || strings.HasSuffix(r.Path, "proton") {
		return "steamuser"
	}
	return ""
}

// runWinetricks starts winetricks for the game's prefix, unattended and in the
// background (issue #46). Its output goes to logs/winetricks-<id>.log in the
// plugin's data folder. It refuses a verb outside the pinned list (checked by
// the caller), a game with no prefix yet, and a umu build without Proton.
func runWinetricks(p actionParams) error {
	bin, err := exec.LookPath("winetricks")
	if err != nil {
		return &codedError{code: "crash.noWinetricks", msg: "winetricks isn't installed"}
	}
	home := os.Getenv("HOME")
	runners := runner.ScanProtonVersionsFor(home)
	selected, proton, useUmu := runner.SelectUmu(p.RunnerID, runners)
	if !useUmu {
		var ok bool
		selected, ok = runner.Resolve(p.RunnerID, runners)
		if !ok {
			return &codedError{code: "game.runnerMissing", params: map[string]string{"id": p.RunnerID},
				msg: fmt.Sprintf("runner %q is not installed", p.RunnerID)}
		}
	}
	prefix := runner.PrefixPath(p.Game, home, p.PrefixesFolder)
	wineRunner := selected
	if useUmu {
		if proton.Path != "" {
			wineRunner = proton
		} else {
			return &codedError{code: "crash.umuUnsupported", params: map[string]string{"title": p.Game.Title},
				msg: "umu's Proton build isn't available for winetricks"}
		}
	}
	env, err := winetricksEnv(p.Game.Title, prefix, wineRunner, useUmu)
	if err != nil {
		return err
	}
	if st, err := os.Stat(env["WINEPREFIX"]); err != nil || !st.IsDir() {
		return &codedError{code: "crash.noPrefix", params: map[string]string{"title": p.Game.Title},
			msg: fmt.Sprintf("%s has no Wine prefix yet", p.Game.Title)}
	}
	data := os.Getenv("PLATYPUS_PLUGIN_DATA")
	if data == "" {
		return fmt.Errorf("plugin data folder is unavailable")
	}
	logDir := filepath.Join(data, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	fileID := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, p.Game.ID)
	log, err := os.OpenFile(filepath.Join(logDir, "winetricks-"+fileID+".log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "--unattended", p.Verb)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return err
	}
	go func() { _ = cmd.Wait(); _ = log.Close() }()
	return nil
}

// gamePrefix is the Windows side of a game's prefix, the folder its saves are
// read from, and the login inside it, for the runner it launches with. It reads
// the runner list and the disk only. With no runner installed, a pfx folder
// (Proton's layout) decides.
func gamePrefix(game types.Game, runnerID, prefixesFolder, home string) (dir, user string) {
	prefix := runner.PrefixPath(game, home, prefixesFolder)
	rs := runner.ScanProtonVersionsFor(home)
	selected, _, useUmu := runner.SelectUmu(runnerID, rs)
	if !useUmu {
		selected, _ = runner.Resolve(runnerID, rs)
	}
	if selected.ID == "" {
		if st, err := os.Stat(filepath.Join(prefix, "pfx")); err == nil && st.IsDir() {
			return protonPrefix(prefix), "steamuser"
		}
		return prefix, ""
	}
	return prefixOf(prefix, selected, useUmu)
}

// prefixOf is the Windows side of prefix for the runner selected: umu runs the
// pfx dir itself when there is one, Proton keeps it in pfx, system Wine has none.
// The user inside a Proton or umu prefix is Proton's steamuser.
func prefixOf(prefix string, selected types.ProtonVersion, umu bool) (dir, user string) {
	switch {
	case umu:
		return runner.EffectiveUmuPrefix(prefix), prefixUser(selected)
	case strings.HasSuffix(selected.Path, "proton"):
		return protonPrefix(prefix), prefixUser(selected)
	default:
		return prefix, ""
	}
}

// protonBinDirs are where a Proton build keeps its Wine binaries, relative to
// its folder: newer builds ship files/, older ones dist/.
var protonBinDirs = []string{"files/bin", "dist/bin"}

// protonPrefix is the folder Proton runs a prefix in: the pfx folder of its
// compat data. A prefix already named pfx (a custom wine_prefix pointing at
// it) is used as it is, never pfx/pfx (issue #60).
func protonPrefix(prefix string) string {
	if filepath.Base(filepath.Clean(prefix)) == "pfx" {
		return prefix
	}
	return filepath.Join(prefix, "pfx")
}

// winetricksEnv is the environment winetricks runs with, so it works on the
// prefix the game runs in (issue #46). prefix is the game's prefix. A Proton
// build keeps its Wine prefix in <prefix>/pfx and runs with its own wine and
// wineserver. A plain Wine runner runs with its own binary. The system Wine (no
// path) is left to PATH. With umu, winetricks uses umu's Proton build and umu's
// prefix layout.
func winetricksEnv(title, prefix string, wineRunner types.ProtonVersion, umu bool) (map[string]string, error) {
	env := map[string]string{"WINEPREFIX": prefix}
	if umu {
		env["WINEPREFIX"] = runner.EffectiveUmuPrefix(prefix)
	}
	if strings.HasSuffix(wineRunner.Path, "proton") {
		if !umu {
			env["WINEPREFIX"] = protonPrefix(prefix)
		}
		wine, server, ok := protonWine(filepath.Dir(wineRunner.Path))
		if !ok {
			return nil, &codedError{code: "crash.noProtonWine", params: map[string]string{"title": title},
				msg: "the selected Proton build has no Wine binaries for winetricks"}
		}
		env["WINE"], env["WINESERVER"] = wine, server
	} else if wineRunner.Path != "" {
		env["WINE"] = wineRunner.Path
	}
	return env, nil
}

// protonWine finds the wine and wineserver binaries of the Proton build in
// protonDir, in the first bin folder that has both.
func protonWine(protonDir string) (wine, server string, ok bool) {
	for _, d := range protonBinDirs {
		w, s := filepath.Join(protonDir, d, "wine"), filepath.Join(protonDir, d, "wineserver")
		if wi, e := os.Stat(w); e == nil && !wi.IsDir() {
			if si, e := os.Stat(s); e == nil && !si.IsDir() {
				return w, s, true
			}
		}
	}
	return "", "", false
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	for scanner.Scan() {
		var req message
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		if req.Method == "locale.changed" {
			var p struct {
				Locale string `json:"locale"`
			}
			_ = json.Unmarshal(req.Params, &p)
			setLocale(p.Locale)
			continue
		}
		if req.ID == nil {
			continue
		}
		resp := handle(req)
		if err := json.NewEncoder(writer).Encode(resp); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		_ = writer.Flush()
	}
}
