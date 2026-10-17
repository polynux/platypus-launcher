// Command platypus-runner provides Wine, Proton, umu and native launch wrapping.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

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
		Command []string          `json:"command"`
		Env     map[string]string `json:"env"`
		Cwd     string            `json:"cwd"`
	} `json:"spec"`
	Game           types.Game `json:"game"`
	RunnerID       string     `json:"runnerId"`
	PrefixesFolder string     `json:"prefixesFolder"`
	Preview        bool       `json:"preview"`
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
	case "runner.wrap":
		var p wrapParams
		if err := json.Unmarshal(req.Params, &p); err != nil || len(p.Spec.Command) == 0 {
			resp.Error = &rpcError{Code: -32602, Message: "spec.command is required"}
			break
		}
		rs := runner.ScanProtonVersionsFor(os.Getenv("HOME"))
		selected, ok := runner.Resolve(p.RunnerID, rs)
		if !ok && !runner.IsAuto(p.RunnerID) {
			resp.Error = &rpcError{Code: -32003, Message: fmt.Sprintf("runner %q is not installed", p.RunnerID)}
			break
		}
		if !ok && !p.Game.Native {
			resp.Error = &rpcError{Code: -32003, Message: "no Proton or Wine runner is installed"}
			break
		}
		target := runner.Target{Exe: p.Spec.Command[0], Args: p.Spec.Command[1:], Env: p.Spec.Env, Native: p.Game.Native}
		command, env, err := runner.BuildCommandFor(p.Game, selected, target, p.Preview, p.PrefixesFolder, os.Getenv("HOME"))
		if err != nil {
			resp.Error = &rpcError{Code: -32003, Message: err.Error()}
			break
		}
		resp.Result = map[string]any{"command": command, "env": env, "cwd": p.Spec.Cwd}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return resp
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
