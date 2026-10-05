// Command hydra-echo is the smallest store plugin: it answers initialize and
// echoes requests back. Hydra's tests drive it through the JSON-RPC client.
//
// Build and install it by hand with:
//
//	go build -o ~/.local/share/hydra/plugins/echo/bin/hydra-echo ./internal/plugins/testdata/echo
//	cp internal/plugins/testdata/echo/plugin.json ~/.local/share/hydra/plugins/echo/
//
// Methods: initialize, echo (returns its params), pid, crash (exits without
// answering). It sends a `log` notification on initialize and exits when
// stdin closes, as every plugin must.
package main

import (
	"bufio"
	"encoding/json"
	"os"
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

func main() {
	out := json.NewEncoder(os.Stdout)
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64<<10), 32<<20)
	for in.Scan() {
		var m message
		if err := json.Unmarshal(in.Bytes(), &m); err != nil {
			out.Encode(message{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		if m.ID == nil {
			continue // notifications need no answer
		}
		resp := message{JSONRPC: "2.0", ID: m.ID}
		switch m.Method {
		case "initialize":
			out.Encode(message{JSONRPC: "2.0", Method: "log", Params: json.RawMessage(`{"level":"info","message":"echo initialized"}`)})
			resp.Result = map[string]any{
				"name":         "Echo (test plugin)",
				"version":      "0.1.0",
				"api":          1,
				"capabilities": []string{},
				"missing":      []any{},
			}
		case "echo":
			resp.Result = m.Params
			if len(m.Params) == 0 {
				resp.Result = json.RawMessage("null")
			}
		case "pid":
			resp.Result = os.Getpid()
		case "crash":
			os.Exit(3)
		default:
			resp.Error = &rpcError{Code: -32601, Message: "method not found: " + m.Method}
		}
		out.Encode(resp)
	}
}
