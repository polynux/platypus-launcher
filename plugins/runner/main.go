// Command platypus-runner provides Wine, Proton, umu and native launch wrapping.
package main

import (
	"bufio"
	"encoding/json"
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
	Code    int    `json:"code"`
	Message string `json:"message"`
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
		}
		_ = json.Unmarshal(req.Params, &p)
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
		rs := runner.ScanProtonVersionsFor(home)
		selected, proton, useUmu := runner.SelectUmu(p.RunnerID, rs)
		if !useUmu {
			selected, _ = runner.Resolve(p.RunnerID, rs)
		}
		if selected.ID == "" {
			if runner.IsAuto(p.RunnerID) {
				resp.Error = &rpcError{Code: -32003, Message: "no Proton or Wine runner is installed"}
			} else {
				resp.Error = &rpcError{Code: -32003, Message: fmt.Sprintf("runner %q is not installed", p.RunnerID)}
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
		prefix := runner.PrefixPath(p.Game, home, p.PrefixesFolder)
		if useUmu {
			prefix = runner.EffectiveUmuPrefix(prefix)
		} else if strings.HasSuffix(selected.Path, "proton") {
			prefix = protonPrefix(prefix)
		}
		resp.Result = map[string]any{"command": command, "env": env, "cwd": p.Spec.Cwd, "runner": runnerInfo(selected), "prefixDir": prefix, "prefixUser": prefixUser(selected)}
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
				resp.Error = &rpcError{Code: -32602, Message: fmt.Sprintf("%q is not a known winetricks verb", p.Verb)}
				break
			}
			if err := runWinetricks(p); err != nil {
				resp.Error = &rpcError{Code: -32003, Message: err.Error()}
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

// runnerSchema is the per-game runner settings, shown by the game settings UI.
// Each key is a GameSettings field, so saved settings keep their names.
func runnerSchema() map[string]any {
	return map[string]any{"groups": []any{
		map[string]any{"id": "runner", "label": "Runner", "fields": []any{
			map[string]any{"id": "runner_id", "kind": "select", "label": "Runner", "scope": "game", "default": "auto", "options": runnerOptions()},
			map[string]any{"id": "wine_prefix", "kind": "text", "label": "Wine prefix", "scope": "game", "default": ""},
		}},
		map[string]any{"id": "wrappers", "label": "Wrappers", "fields": []any{
			map[string]any{"id": "use_gamemode", "kind": "bool", "label": "GameMode", "scope": "game", "default": true},
			map[string]any{"id": "use_mangohud", "kind": "bool", "label": "MangoHud", "scope": "game", "default": false},
			map[string]any{"id": "use_gamescope", "kind": "bool", "label": "Gamescope", "scope": "game", "default": false},
		}},
		map[string]any{"id": "gamescope", "label": "Gamescope", "fields": []any{
			map[string]any{"id": "gamescope_w", "kind": "number", "label": "Width", "scope": "game", "default": 0, "min": 0},
			map[string]any{"id": "gamescope_h", "kind": "number", "label": "Height", "scope": "game", "default": 0, "min": 0},
			map[string]any{"id": "gamescope_r", "kind": "number", "label": "Refresh rate", "scope": "game", "default": 0, "min": 0},
			map[string]any{"id": "gamescope_args", "kind": "text", "label": "Arguments", "scope": "game", "default": ""},
		}},
		map[string]any{"id": "anticheat", "label": "Anti-cheat", "fields": []any{
			map[string]any{"id": "eac_runtime", "kind": "bool", "label": "EasyAntiCheat runtime", "scope": "game", "default": false},
			map[string]any{"id": "battleye_runtime", "kind": "bool", "label": "BattlEye runtime", "scope": "game", "default": false},
		}},
	}}
}

func runnerOptions() []any {
	out := []any{map[string]any{"id": "auto", "label": "Automatic"}}
	for _, r := range runner.ScanProtonVersionsFor(os.Getenv("HOME")) {
		if strings.EqualFold(r.ID, "auto") {
			continue
		}
		out = append(out, map[string]any{"id": r.ID, "label": r.Name})
	}
	return out
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

func runWinetricks(p actionParams) error {
	bin, err := exec.LookPath("winetricks")
	if err != nil {
		return fmt.Errorf("winetricks isn't installed")
	}
	home := os.Getenv("HOME")
	runners := runner.ScanProtonVersionsFor(home)
	selected, proton, useUmu := runner.SelectUmu(p.RunnerID, runners)
	if !useUmu {
		var ok bool
		selected, ok = runner.Resolve(p.RunnerID, runners)
		if !ok {
			return fmt.Errorf("runner %q is not installed", p.RunnerID)
		}
	}
	prefix := runner.PrefixPath(p.Game, home, p.PrefixesFolder)
	wineRunner := selected
	if useUmu {
		if proton.Path != "" {
			wineRunner = proton
		} else {
			return fmt.Errorf("umu's Proton build isn't available for winetricks")
		}
	}
	env, err := winetricksEnv(prefix, wineRunner, useUmu)
	if err != nil {
		return err
	}
	if st, err := os.Stat(env["WINEPREFIX"]); err != nil || !st.IsDir() {
		return fmt.Errorf("%s has no Wine prefix yet", p.Game.Title)
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

func winetricksEnv(prefix string, wineRunner types.ProtonVersion, umu bool) (map[string]string, error) {
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
			return nil, fmt.Errorf("the selected Proton build has no Wine binaries for winetricks")
		}
		env["WINE"], env["WINESERVER"] = wine, server
	} else if wineRunner.Path != "" {
		env["WINE"] = wineRunner.Path
	}
	return env, nil
}

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
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil || req.ID == nil {
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
