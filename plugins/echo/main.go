// Command hydra-echo is a fake store plugin for tests and for trying the
// plugin UI without a real store: a login that accepts the code "letmein", a
// small library, installs that write a tiny shell-script "game" with fake
// progress, and launch specs for each launch path. It never downloads
// anything.
//
// Install it with plugins/install.sh echo.
//
// Library:
//
//	tiny     native script, installs in ~1 s
//	slow     native script, installs in ~20 s (try cancel)
//	huge     claims 32 PiB, so the free-space check refuses it
//	flaky    the plugin crashes once mid-install, then resumes
//	client   always installed; "launches" through a store client (sleep 5.0421)
//	escape   always installed outside the install folder; Hydra must refuse it
//
// Besides the store protocol it answers echo (returns its params), pid and
// crash (exits without answering). It exits when stdin closes.
//
// Capability knobs: echo is the test store for the capability system
// (docs/plugins/protocol.md, "Capabilities"), so it can declare a capability in
// plugin.json and still switch it off, report features, narrow them per game,
// and fail a method on demand. Each knob is an environment variable, read at
// start, and can be changed while running with the `config` method:
//
//	ECHO_OFF           capabilities initialize leaves out although the manifest declares them
//	ECHO_EXTRA         capabilities initialize adds that the manifest doesn't declare (a protocol violation)
//	ECHO_FEATURES      JSON {"versions": {"channels": true}} - the features initialize reports
//	ECHO_GAME_FEATURES JSON {"tiny": ["install"]} - library.list `features` for those games
//	ECHO_UNAVAILABLE   methods that answer -32002 "unavailable right now"
//	ECHO_MISSING       methods that answer -32601 "method not found"
//	ECHO_INIT_OMIT     set: initialize leaves `capabilities` out (means "all declared")
//	ECHO_PROTOCOLS     protocol versions echo speaks (default 1.0 to 1.5); it answers the highest Hydra also offers
//	ECHO_NO_PROTOCOL   set: initialize leaves `protocol` out, like a plugin from before negotiation
//	ECHO_PROTOCOL      protocol version initialize answers whatever was negotiated (a protocol violation)
//	ECHO_FUTURE        set: initialize carries a field, and echo sends a notification, that only a newer Hydra knows
//
// Cloud saves (cloud.go) have their own ECHO_CLOUD* knobs, listed there.
//
// `config` takes {off, extra, features, gameFeatures, unavailable, missing,
// initOmit} (absent keys are left alone) and `calls` returns how many times
// each method was received, so tests can tell a cached -32601 from a retry.
// The methods of capabilities that core doesn't use yet answer canned data.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
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

const (
	codeAuthRequired = -32001
	codeUnavailable  = -32002
)

type game struct {
	title    string
	size     int64 // download == disk size
	steps    int
	stepMs   int
	native   bool
	note     string
	external string // always installed here, outside Hydra's folder
	client   bool
	details  bool // answers game.details with store metadata
}

var games = map[string]game{
	"tiny":   {title: "Echo Tiny", size: 3 << 20, steps: 10, stepMs: 100, native: true, details: true},
	"slow":   {title: "Echo Slow", size: 40 << 20, steps: 100, stepMs: 200, native: true},
	"huge":   {title: "Echo Huge", size: 32 << 50, steps: 10, stepMs: 100, native: true},
	"flaky":  {title: "Echo Flaky", size: 8 << 20, steps: 20, stepMs: 100, native: true, note: "The plugin crashes once while installing this."},
	"client": {title: "Echo Client", client: true, note: "Launches through a store client (a 5 s sleep)."},
	"escape": {title: "Echo Escape", external: "/usr/bin", note: "Installed outside the plugin's folder: Hydra must refuse to launch it."},
}

var order = []string{"tiny", "slow", "huge", "flaky", "client", "escape"}

// state is persisted in $HYDRA_PLUGIN_DATA/state.json.
type state struct {
	LoggedIn  bool              `json:"loggedIn"`
	Installed map[string]string `json:"installed"` // id → install path
	Partial   map[string]int64  `json:"partial"`   // id → bytes done (resume)
	Crashed   map[string]bool   `json:"crashed"`
}

var (
	outMu   sync.Mutex
	out     = json.NewEncoder(os.Stdout)
	stMu    sync.Mutex
	st      = state{Installed: map[string]string{}, Partial: map[string]int64{}, Crashed: map[string]bool{}}
	jobsMu  sync.Mutex
	cancels = map[string]chan struct{}{}
)

// knobs are the capability test settings (see the package comment).
type knobs struct {
	Off          []string                   `json:"off"`
	Extra        []string                   `json:"extra"`
	Features     map[string]map[string]bool `json:"features"`
	GameFeatures map[string][]string        `json:"gameFeatures"`
	Unavailable  []string                   `json:"unavailable"`
	Missing      []string                   `json:"missing"`
	InitOmit     bool                       `json:"initOmit"`
}

// defaultProtocols is what echo speaks: more than any Hydra offers yet, so
// the negotiation has something to pick from.
var defaultProtocols = []string{"1.0", "1.1", "1.2", "1.3", "1.4", "1.5"}

var (
	knobMu  sync.Mutex
	kn      knobs
	callsMu sync.Mutex
	calls   = map[string]int{}
)

func csv(name string) []string {
	var out []string
	for _, f := range strings.Split(os.Getenv(name), ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func loadKnobs() {
	kn = knobs{Off: csv("ECHO_OFF"), Extra: csv("ECHO_EXTRA"), Unavailable: csv("ECHO_UNAVAILABLE"),
		Missing: csv("ECHO_MISSING"), InitOmit: os.Getenv("ECHO_INIT_OMIT") != ""}
	if v := os.Getenv("ECHO_FEATURES"); v != "" {
		_ = json.Unmarshal([]byte(v), &kn.Features)
	}
	if v := os.Getenv("ECHO_GAME_FEATURES"); v != "" {
		_ = json.Unmarshal([]byte(v), &kn.GameFeatures)
	}
}

// configure applies a `config` request: only the keys present change.
func configure(raw json.RawMessage) {
	var present map[string]json.RawMessage
	_ = json.Unmarshal(raw, &present)
	knobMu.Lock()
	defer knobMu.Unlock()
	set := func(key string, dst any) {
		if v, ok := present[key]; ok {
			_ = json.Unmarshal(v, dst)
		}
	}
	set("off", &kn.Off)
	set("extra", &kn.Extra)
	set("features", &kn.Features)
	set("gameFeatures", &kn.GameFeatures)
	set("unavailable", &kn.Unavailable)
	set("missing", &kn.Missing)
	set("initOmit", &kn.InitOmit)
}

// manifestCapabilities reads the capabilities plugin.json declares; Hydra runs
// the plugin from its folder.
func manifestCapabilities() []string {
	var m struct {
		Capabilities []string `json:"capabilities"`
	}
	if data, err := os.ReadFile("plugin.json"); err == nil && json.Unmarshal(data, &m) == nil {
		return m.Capabilities
	}
	return []string{"library", "install", "launch", "auth", "discard"}
}

// manifestAccepts reports whether plugin.json declares protocol version v: in
// its list, inside its range, or (a legacy manifest with only "api") 1.0.
func manifestAccepts(v string) bool {
	var m struct {
		Protocol struct {
			Versions json.RawMessage `json:"versions"`
		} `json:"protocol"`
	}
	data, err := os.ReadFile("plugin.json")
	if err != nil || json.Unmarshal(data, &m) != nil || len(m.Protocol.Versions) == 0 {
		return v == "1.0" || err != nil
	}
	var list []string
	if json.Unmarshal(m.Protocol.Versions, &list) == nil {
		return slices.Contains(list, v)
	}
	var rng struct{ Min, Max string }
	if json.Unmarshal(m.Protocol.Versions, &rng) != nil {
		return false
	}
	return (rng.Min == "" || !versionLess(v, rng.Min)) && (rng.Max == "" || !versionLess(rng.Max, v))
}

// negotiate answers the highest protocol version that echo speaks, its
// manifest declares and Hydra offered ("major.minor", compared numerically).
func negotiate(offered []string) (string, bool) {
	speaks := csv("ECHO_PROTOCOLS")
	if len(speaks) == 0 {
		speaks = defaultProtocols
	}
	best, found := "", false
	for _, o := range offered {
		if slices.Contains(speaks, o) && manifestAccepts(o) && (!found || versionLess(best, o)) {
			best, found = o, true
		}
	}
	return best, found
}

func versionLess(a, b string) bool {
	var a1, a2, b1, b2 int
	fmt.Sscanf(a, "%d.%d", &a1, &a2)
	fmt.Sscanf(b, "%d.%d", &b1, &b2)
	return a1 < b1 || (a1 == b1 && a2 < b2)
}

func initializeResult(p params) (map[string]any, error) {
	knobMu.Lock()
	defer knobMu.Unlock()
	res := map[string]any{"name": "Echo (test plugin)", "version": "0.2.0", "api": 1, "missing": []any{}}
	if os.Getenv("ECHO_NO_PROTOCOL") == "" {
		picked, ok := negotiate(p.Protocols)
		if v := os.Getenv("ECHO_PROTOCOL"); v != "" {
			picked, ok = v, true
		}
		if !ok {
			return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("no protocol version in common with Hydra's %v", p.Protocols)}
		}
		res["protocol"] = picked
	}
	if os.Getenv("ECHO_FUTURE") != "" {
		res["flux"] = map[string]any{"capacitor": true}
	}
	if !kn.InitOmit {
		caps := []string{}
		for _, c := range manifestCapabilities() {
			if !slices.Contains(kn.Off, c) {
				caps = append(caps, c)
			}
		}
		res["capabilities"] = append(caps, kn.Extra...)
	}
	if kn.Features != nil {
		res["features"] = kn.Features
	}
	return res, nil
}

// forced says whether a test knob makes this method fail, and how.
func forced(method string) error {
	knobMu.Lock()
	defer knobMu.Unlock()
	switch {
	case slices.Contains(kn.Missing, method):
		return &rpcError{Code: -32601, Message: "method not found: " + method}
	case slices.Contains(kn.Unavailable, method):
		return &rpcError{Code: codeUnavailable, Message: "Echo can't do that right now: the fake service is down (test knob)"}
	}
	return nil
}

func gameFeatures(id string) (list []string, set bool) {
	knobMu.Lock()
	defer knobMu.Unlock()
	list, set = kn.GameFeatures[id]
	if list == nil {
		list = []string{}
	}
	return list, set
}

func send(m message) {
	m.JSONRPC = "2.0"
	outMu.Lock()
	defer outMu.Unlock()
	_ = out.Encode(m)
}

func notify(method string, params any) {
	raw, _ := json.Marshal(params)
	send(message{Method: method, Params: raw})
}

func statePath() string {
	dir := os.Getenv("HYDRA_PLUGIN_DATA")
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "state.json")
}

func load() {
	if p := statePath(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &st)
		}
	}
	if st.Installed == nil {
		st.Installed = map[string]string{}
	}
	if st.Partial == nil {
		st.Partial = map[string]int64{}
	}
	if st.Crashed == nil {
		st.Crashed = map[string]bool{}
	}
}

// save writes the state. Callers hold stMu.
func save() {
	if p := statePath(); p != "" {
		data, _ := json.MarshalIndent(st, "", "  ")
		_ = os.WriteFile(p, data, 0o600)
	}
}

func main() {
	load()
	loadKnobs()
	loadCloud()
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64<<10), 32<<20)
	for in.Scan() {
		var m message
		if err := json.Unmarshal(in.Bytes(), &m); err != nil {
			send(message{Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		if m.ID == nil {
			continue // notifications need no answer
		}
		go handle(m)
	}
}

func handle(m message) {
	result, err := dispatch(m.Method, m.Params)
	resp := message{ID: m.ID, Result: result}
	var re *rpcError
	switch {
	case errors.As(err, &re):
		resp.Error, resp.Result = re, nil
	case err != nil:
		resp.Error, resp.Result = &rpcError{Code: -32000, Message: err.Error()}, nil
	case result == nil:
		resp.Result = struct{}{}
	}
	send(resp)
}

func (e *rpcError) Error() string { return e.Message }

type params struct {
	Ctx        json.RawMessage   `json:"ctx"`     // cloud.*: the CloudContext
	Choices    map[string]string `json:"choices"` // cloud.resolve
	Via        string            `json:"via"`     // game.launchSpec: "client"
	ID         string            `json:"id"`
	JobID      string            `json:"jobId"`
	InstallDir string            `json:"installDir"`
	Preview    bool              `json:"preview"`
	Fields     map[string]string `json:"fields"`
	Protocols  []string          `json:"protocols"` // initialize: the versions Hydra speaks
}

func dispatch(method string, raw json.RawMessage) (any, error) {
	var p params
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p)
	}
	callsMu.Lock()
	calls[method]++
	callsMu.Unlock()
	if method != "config" && method != "calls" {
		if err := forced(method); err != nil {
			return nil, err
		}
	}
	switch method {
	case "initialize":
		notify("log", map[string]string{"level": "info", "message": "echo initialized"})
		res, err := initializeResult(p)
		if err == nil && os.Getenv("ECHO_FUTURE") != "" {
			notify("future.news", map[string]any{"headline": "a newer Hydra would show this"})
		}
		return res, err
	case "config":
		configure(raw)
		configureCloud(raw)
		return nil, nil
	case "calls":
		callsMu.Lock()
		defer callsMu.Unlock()
		return maps.Clone(calls), nil
	case "echo":
		if len(raw) == 0 {
			return json.RawMessage("null"), nil
		}
		return raw, nil
	case "pid":
		return os.Getpid(), nil
	case "env":
		return map[string]string{"PYTHONDONTWRITEBYTECODE": os.Getenv("PYTHONDONTWRITEBYTECODE")}, nil
	case "crash":
		os.Exit(3)

	case "auth.status":
		stMu.Lock()
		defer stMu.Unlock()
		return authStatus(), nil
	case "auth.begin":
		return map[string]any{
			"url":          "https://example.com/hydra-echo-login",
			"instructions": "Echo has no real login page. Type the code letmein.",
			"fields":       []map[string]any{{"id": "code", "label": "Code", "placeholder": "letmein"}},
		}, nil
	case "auth.complete":
		if p.Fields["code"] != "letmein" {
			return nil, &rpcError{Code: -32000, Message: "wrong code (it's letmein)"}
		}
		stMu.Lock()
		defer stMu.Unlock()
		st.LoggedIn = true
		save()
		return authStatus(), nil
	case "auth.logout":
		stMu.Lock()
		defer stMu.Unlock()
		st.LoggedIn = false
		save()
		return authStatus(), nil

	case "library.list":
		return library()
	case "game.details":
		g, ok := games[p.ID]
		if !ok {
			return nil, fmt.Errorf("unknown game %q", p.ID)
		}
		if !g.details {
			return nil, &rpcError{Code: -32601, Message: "method not found: game.details"}
		}
		return map[string]any{
			"summary":     "A tiny fake game from the Echo test store, shown on its game page.",
			"genres":      []string{"Test"},
			"developers":  []string{"Echo Software"},
			"publishers":  []string{"Echo Software"},
			"releaseDate": "2024-01-01",
			"achievements": map[string]any{
				"total": 2,
				"list": []map[string]any{
					{"id": "installed", "name": "Get installed", "description": "Install the game.", "globalPercent": 100},
					{"id": "played", "name": "Play it", "description": "Start the game once.", "globalPercent": 84.1},
				},
			},
		}, nil
	case "game.installSize":
		if p.ID == "" {
			return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
		}
		g, ok := games[p.ID]
		if !ok {
			return nil, fmt.Errorf("unknown game %q", p.ID)
		}
		return map[string]int64{"downloadBytes": g.size, "diskBytes": g.size}, nil
	case "game.install", "game.update":
		return startInstall(p)
	case "game.uninstall":
		return startUninstall(p)
	case "job.cancel":
		jobsMu.Lock()
		if c, ok := cancels[p.JobID]; ok {
			close(c)
			delete(cancels, p.JobID)
		}
		jobsMu.Unlock()
		return nil, nil
	case "job.discard":
		return discardJob(p)
	case "game.launchSpec":
		return launchSpec(p)
	case "cloud.status", "cloud.pull", "cloud.push", "cloud.resolve":
		return cloudCall(method, p)
	}
	if res, err, ok := capabilityStub(method, p); ok {
		return res, err
	}
	return nil, &rpcError{Code: -32601, Message: "method not found: " + method}
}

// discardJob throws away the data a canceled install left. For this fake
// store that is the saved resume point (state.json's partial map) and the
// "crashed mid-install" memory — a canceled flaky would otherwise resume from
// its own crash rather than from what Hydra knows — as well as an install
// folder the game never finished into. A game the store still reports as
// installed is never touched: its folder lives inside the same map.
func discardJob(p params) (any, error) {
	if p.ID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	stMu.Lock()
	defer stMu.Unlock()
	dir, installed := st.Installed[p.ID]
	if !installed && filepath.IsAbs(p.InstallDir) {
		dir = filepath.Join(p.InstallDir, p.ID)
		// Only a folder the game never finished into, and never one outside
		// the install folder (the external games'/client's path).
		if real, err := filepath.EvalSymlinks(dir); err == nil && real == filepath.Join(p.InstallDir, p.ID) {
			os.RemoveAll(dir)
		}
	}
	if installed {
		return nil, nil
	}
	delete(st.Partial, p.ID)
	delete(st.Crashed, p.ID)
	save()
	return nil, nil
}

func authStatus() map[string]any {
	if st.LoggedIn {
		return map[string]any{"loggedIn": true, "user": "echo-user"}
	}
	return map[string]any{"loggedIn": false}
}

func library() (any, error) {
	stMu.Lock()
	defer stMu.Unlock()
	if !st.LoggedIn {
		return nil, &rpcError{Code: codeAuthRequired, Message: "log in to Echo first"}
	}
	var list []map[string]any
	for _, id := range order {
		g := games[id]
		path, installed := st.Installed[id]
		if g.external != "" {
			path, installed = g.external, true
		}
		if g.client {
			installed = true
		}
		e := map[string]any{
			"id": id, "title": g.title, "installed": installed, "installable": g.external == "" && !g.client,
			"version": "1.0", "note": g.note, "native": g.native,
		}
		if f, set := gameFeatures(id); set {
			e["features"] = f
		}
		if g.size > 0 {
			e["sizeBytes"] = g.size
		}
		if installed && path != "" {
			e["installPath"] = path
		}
		list = append(list, e)
	}
	return map[string]any{"games": list}, nil
}

func startInstall(p params) (any, error) {
	g, ok := games[p.ID]
	if !ok || g.external != "" || g.client {
		return nil, fmt.Errorf("%q can't be installed", p.ID)
	}
	if !filepath.IsAbs(p.InstallDir) {
		return nil, fmt.Errorf("installDir %q is not absolute", p.InstallDir)
	}
	stop := make(chan struct{})
	jobsMu.Lock()
	if _, running := cancels[p.JobID]; running {
		// Hydra resends a job with the same jobId after a crash: run it once.
		jobsMu.Unlock()
		return map[string]any{}, nil
	}
	cancels[p.JobID] = stop
	jobsMu.Unlock()
	go func() {
		defer func() {
			jobsMu.Lock()
			delete(cancels, p.JobID)
			jobsMu.Unlock()
		}()
		stepMs := g.stepMs
		if v, err := strconv.Atoi(os.Getenv("ECHO_STEP_MS")); err == nil {
			stepMs = v
		}
		chunk := g.size / int64(g.steps)
		stMu.Lock()
		done := st.Partial[p.ID]
		stMu.Unlock()
		for done < g.size {
			select {
			case <-stop:
				return
			case <-time.After(time.Duration(stepMs) * time.Millisecond):
			}
			done = min(done+chunk, g.size)
			stMu.Lock()
			st.Partial[p.ID] = done
			crash := p.ID == "flaky" && !st.Crashed[p.ID] && done >= g.size/2
			if crash {
				st.Crashed[p.ID] = true
			}
			save()
			stMu.Unlock()
			if crash {
				os.Exit(4)
			}
			notify("job.progress", map[string]any{
				"jobId": p.JobID, "phase": "downloading", "bytes": done, "total": g.size,
				"speed": float64(chunk) / (float64(stepMs) / 1000),
			})
		}
		dir := filepath.Join(p.InstallDir, p.ID)
		script := "#!/bin/sh\necho \"echo game $*\"\nsleep \"${ECHO_GAME_SECONDS:-3}\"\n"
		if err := os.MkdirAll(dir, 0o755); err != nil {
			notify("job.failed", map[string]any{"jobId": p.JobID, "error": err.Error()})
			return
		}
		if err := os.WriteFile(filepath.Join(dir, "game.sh"), []byte(script), 0o755); err != nil {
			notify("job.failed", map[string]any{"jobId": p.JobID, "error": err.Error()})
			return
		}
		stMu.Lock()
		st.Installed[p.ID] = dir
		delete(st.Partial, p.ID)
		save()
		stMu.Unlock()
		notify("job.done", map[string]any{"jobId": p.JobID})
	}()
	return nil, nil
}

func startUninstall(p params) (any, error) {
	stMu.Lock()
	dir, ok := st.Installed[p.ID]
	stMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("%q is not installed", p.ID)
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		if err := os.RemoveAll(dir); err != nil {
			notify("job.failed", map[string]any{"jobId": p.JobID, "error": err.Error()})
			return
		}
		stMu.Lock()
		delete(st.Installed, p.ID)
		save()
		stMu.Unlock()
		notify("job.done", map[string]any{"jobId": p.JobID})
	}()
	return nil, nil
}

func launchSpec(p params) (any, error) {
	g, ok := games[p.ID]
	if !ok {
		return nil, fmt.Errorf("unknown game %q", p.ID)
	}
	switch {
	case g.client || p.Via == "client":
		return map[string]any{
			"delegate": "store-client", "client": "Echo client",
			"command": []string{"sleep", "5.0421"},
			"find":    map[string]any{"cmdline": []string{"sleep\x005.0421"}},
		}, nil
	case g.external != "":
		return map[string]any{"exe": filepath.Join(g.external, "true")}, nil
	}
	stMu.Lock()
	dir, installed := st.Installed[p.ID]
	stMu.Unlock()
	if !installed {
		return nil, fmt.Errorf("%s is not installed", g.title)
	}
	token := "preview"
	if !p.Preview {
		token = fmt.Sprintf("s3cret-%d", time.Now().UnixNano())
	}
	return map[string]any{
		"exe":    filepath.Join(dir, "game.sh"),
		"args":   []string{"--from-echo", "-AUTH_PASSWORD=" + token},
		"cwd":    dir,
		"env":    map[string]string{"ECHO_GAME": "1"},
		"redact": []string{"-AUTH_PASSWORD="},
	}, nil
}
