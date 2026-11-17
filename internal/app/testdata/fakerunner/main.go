// Command fake-runner is a test-only runner plugin: it records every
// runner.wrap and runner.runAction call to $FAKE_RUNNER_RECORD (one JSON
// object per line) and wraps launches by passing the command through,
// pinning WINEPREFIX to the game's pinned prefix. It runs installers and
// fake clients natively, so the wine-client flow is testable without Wine.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type wrapSpec struct {
	Command   []string          `json:"command"`
	Env       map[string]string `json:"env"`
	Cwd       string            `json:"cwd"`
	NeedsWine bool              `json:"needsWine"`
}

func record(entry map[string]any) {
	path := os.Getenv("FAKE_RUNNER_RECORD")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	raw, _ := json.Marshal(entry)
	fmt.Fprintln(f, string(raw))
}

func handle(m message) message {
	resp := message{JSONRPC: "2.0", ID: m.ID}
	switch m.Method {
	case "initialize":
		resp.Result = map[string]any{"protocol": "1.17", "capabilities": []string{"runner"}}
	case "runner.wrap":
		var p struct {
			Spec           wrapSpec       `json:"spec"`
			Game           map[string]any `json:"game"`
			RunnerID       string         `json:"runnerId"`
			DefaultRunner  string         `json:"defaultRunnerId"`
			Options        map[string]any `json:"options"`
			Tweaks         []any          `json:"tweaks"`
			PrefixesFolder string         `json:"prefixesFolder"`
			Preview        bool           `json:"preview"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			resp.Error = &rpcError{Code: -32602, Message: err.Error()}
			break
		}
		var gameID, winePrefix string
		if g, ok := p.Game["id"].(string); ok {
			gameID = g
		}
		if s, ok := p.Game["settings"].(map[string]any); ok {
			winePrefix, _ = s["wine_prefix"].(string)
		}
		if winePrefix == "" {
			winePrefix = filepath.Join(p.PrefixesFolder, gameID)
		}
		record(map[string]any{
			"method": "runner.wrap", "command": p.Spec.Command, "env": p.Spec.Env,
			"cwd": p.Spec.Cwd, "needsWine": p.Spec.NeedsWine, "game_id": gameID,
			"wine_prefix": winePrefix, "runner_id": p.RunnerID,
			"default_runner_id": p.DefaultRunner, "options": p.Options,
			"tweaks": p.Tweaks, "prefixes_folder": p.PrefixesFolder, "preview": p.Preview,
		})
		env := map[string]string{"WINEPREFIX": winePrefix}
		for k, v := range p.Spec.Env {
			env[k] = v
		}
		resp.Result = map[string]any{
			"command": p.Spec.Command, "env": env, "cwd": p.Spec.Cwd,
			"runner": map[string]any{"id": "fake", "name": "Fake runner", "kind": "wine", "system": true},
		}
	case "runner.runAction":
		var p struct {
			ID     string         `json:"id"`
			Verb   string         `json:"verb"`
			Force  bool           `json:"force"`
			Launch map[string]any `json:"launch"`
		}
		_ = json.Unmarshal(m.Params, &p)
		record(map[string]any{"method": "runner.runAction", "id": p.ID, "verb": p.Verb, "force": p.Force, "launch": p.Launch})
		resp.Result = map[string]any{}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + m.Method}
	}
	return resp
}

var (
	outMu sync.Mutex
	out   = json.NewEncoder(os.Stdout)
)

func send(m message) {
	m.JSONRPC = "2.0"
	outMu.Lock()
	defer outMu.Unlock()
	_ = out.Encode(m)
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64<<10), 32<<20)
	for in.Scan() {
		raw := append([]byte(nil), in.Bytes()...)
		go func() {
			var m message
			if err := json.Unmarshal(raw, &m); err != nil {
				send(message{Error: &rpcError{Code: -32700, Message: err.Error()}})
				return
			}
			if m.ID == nil {
				return
			}
			resp := handle(m)
			resp.ID = m.ID
			send(resp)
		}()
	}
}
