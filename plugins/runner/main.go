// Command platypus-runner provides Wine, Proton, umu and native launch wrapping.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"platypus/internal/crashhints"
	"platypus/internal/runner"
	"platypus/internal/types"
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
	ID       string     `json:"id"`
	Game     types.Game `json:"game"`
	RunnerID string     `json:"runnerId"`
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
		resp.Result = map[string]any{"runners": rs}
	case "runner.options":
		resp.Result = map[string]any{"groups": []any{
			map[string]any{"id": "runner", "label": "Runner", "fields": []any{
				map[string]any{"id": "runner_id", "kind": "select", "label": "Runner", "default": "auto", "options": runnerOptions()},
				map[string]any{"id": "wine_prefix", "kind": "text", "label": "Wine prefix", "default": ""},
			}},
			map[string]any{"id": "wrappers", "label": "Wrappers", "fields": []any{
				map[string]any{"id": "use_gamemode", "kind": "bool", "label": "GameMode", "default": true},
				map[string]any{"id": "use_mangohud", "kind": "bool", "label": "MangoHud", "default": false},
				map[string]any{"id": "use_gamescope", "kind": "bool", "label": "Gamescope", "default": false},
			}},
		}}
	case "runner.wrap":
		var p wrapParams
		if err := json.Unmarshal(req.Params, &p); err != nil || len(p.Spec.Command) == 0 {
			resp.Error = &rpcError{Code: -32602, Message: "spec.command is required"}
			break
		}
		rs := runner.ScanProtonVersionsFor(os.Getenv("HOME"))
		selected, proton, useUmu := runner.SelectUmu(p.RunnerID, rs)
		if !useUmu {
			selected, _ = runner.Resolve(p.RunnerID, rs)
		}
		ok := selected.ID != ""
		if !ok && !runner.IsAuto(p.RunnerID) {
			resp.Error = &rpcError{Code: -32003, Message: fmt.Sprintf("runner %q is not installed", p.RunnerID)}
			break
		}
		if p.Spec.NeedsWine && !ok {
			resp.Error = &rpcError{Code: -32003, Message: "no Proton or Wine runner is installed"}
			break
		}
		target := runner.Target{Exe: p.Spec.Command[0], Args: p.Spec.Command[1:], Env: p.Spec.Env, Native: !p.Spec.NeedsWine}
		var command []string
		var env map[string]string
		var err error
		if useUmu && p.Spec.NeedsWine {
			compatID := ""
			if p.Game.Store == "steam" {
				compatID = p.Game.StoreAppID
			}
			command, env, err = runner.BuildUmuCommandFor(p.Game, proton, selected.Path, compatID, target, p.Preview, p.PrefixesFolder, os.Getenv("HOME"))
		} else {
			command, env, err = runner.BuildCommandFor(p.Game, selected, target, p.Preview, p.PrefixesFolder, os.Getenv("HOME"))
		}
		if err != nil {
			resp.Error = &rpcError{Code: -32003, Message: err.Error()}
			break
		}
		resp.Result = map[string]any{"command": command, "env": env, "cwd": p.Spec.Cwd, "runner": selected}
	case "runner.actions":
		var p actionParams
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Game.ID == "" {
			resp.Error = &rpcError{Code: -32602, Message: "game.id is required"}
			break
		}
		prefix := runner.PrefixPath(p.Game, os.Getenv("HOME"), "")
		if _, err := os.Stat(prefix); err == nil {
			resp.Result = map[string]any{"actions": []any{map[string]any{"id": "open-prefix", "label": "Open prefix"}}}
		} else {
			resp.Result = map[string]any{"actions": []any{}}
		}
	case "runner.runAction":
		var p actionParams
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Game.ID == "" || p.ID == "" {
			resp.Error = &rpcError{Code: -32602, Message: "id and game.id are required"}
			break
		}
		if p.ID != "open-prefix" {
			resp.Error = &rpcError{Code: -32602, Message: "unknown runner action"}
			break
		}
		prefix := runner.PrefixPath(p.Game, os.Getenv("HOME"), "")
		cmd := exec.Command("xdg-open", prefix)
		if err := cmd.Start(); err != nil {
			resp.Error = &rpcError{Code: -32003, Message: err.Error()}
		} else {
			go func() { _ = cmd.Wait() }()
			resp.Result = map[string]any{}
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
