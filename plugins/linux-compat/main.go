// Command platypus-linux-compat provides compatibility facts from ProtonDB.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"platypus/internal/compat"
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

type game struct {
	ID         string   `json:"id"`
	Store      string   `json:"store"`
	StoreAppID string   `json:"storeAppId"`
	SteamAppID string   `json:"steamAppId"`
	Platforms  []string `json:"platforms,omitempty"`
	InstallDir string   `json:"installDir,omitempty"`
}

type lookupParams struct {
	Games []game `json:"games"`
}
type badge struct {
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Tone      string `json:"tone,omitempty"`
	Link      string `json:"link,omitempty"`
	Source    string `json:"source"`
	CheckedAt int64  `json:"checkedAt"`
}
type fact struct {
	ID         string   `json:"id"`
	Badges     []badge  `json:"badges"`
	Report     *report  `json:"report,omitempty"`
	Links      []any    `json:"links"`
	Notes      []string `json:"notes,omitempty"`
	TTLSeconds int      `json:"ttlSeconds"`
}
type report struct {
	Count int    `json:"count"`
	State string `json:"state"`
	Tier  string `json:"tier,omitempty"`
}

var proton compat.RatingSource = &compat.ProtonDB{Fetch: compat.NewFetcher("Platypus/1.0 (Linux compatibility facts)")}

func lookup(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	var p lookupParams
	if err := json.Unmarshal(raw, &p); err != nil || len(p.Games) == 0 {
		return nil, &rpcError{Code: -32602, Message: "games is required"}
	}
	facts := make([]fact, 0, len(p.Games))
	sourceDown := false
	stopped := false
	for _, g := range p.Games {
		if g.ID == "" {
			return nil, &rpcError{Code: -32602, Message: "game id is required"}
		}
		if ctx.Err() != nil {
			stopped = true
			break
		}
		id := g.SteamAppID
		if id == "" && g.Store == "steam" {
			id = g.StoreAppID
		}
		f := fact{ID: g.ID, Badges: []badge{}, Links: []any{}, TTLSeconds: int((7 * 24 * time.Hour).Seconds())}
		if id != "" {
			summary, err := proton.Summary(ctx, id)
			if errors.Is(err, compat.ErrPaused) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || (err != nil && ctx.Err() != nil) {
				stopped = true
				break
			}
			if err != nil && !errors.Is(err, compat.ErrNotFound) {
				sourceDown = true
				continue
			}
			if err == nil {
				tier := compat.ShownTier(summary)
				state := "rated"
				if tier == "" {
					state = "few"
				}
				f.Report = &report{Count: summary.Total, State: state, Tier: tier}
				if tier != "" {
					label := strings.ToUpper(tier[:1]) + tier[1:]
					f.Badges = append(f.Badges, badge{Kind: "rating", Label: label, Tone: tier, Link: compat.ProtonDBPage(id), Source: "ProtonDB", CheckedAt: time.Now().Unix()})
				}
			}
			if errors.Is(err, compat.ErrNotFound) {
				f.Report = &report{State: "none"}
			}
		}
		facts = append(facts, f)
	}
	if len(facts) == 0 && sourceDown && !stopped {
		return nil, &rpcError{Code: -32002, Message: "ProtonDB is temporarily unavailable"}
	}
	return map[string]any{"facts": facts}, nil
}

func handle(ctx context.Context, req message) message {
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
		resp.Result = map[string]any{"protocol": chosen, "capabilities": []string{"compat-facts"}}
	case "compat.lookup":
		resp.Result, resp.Error = lookup(ctx, req.Params)
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
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		if req.ID == nil {
			continue
		}
		resp := handle(context.Background(), req)
		if err := json.NewEncoder(writer).Encode(resp); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		_ = writer.Flush()
	}
}
