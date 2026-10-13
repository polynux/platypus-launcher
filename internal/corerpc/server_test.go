package corerpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"platypus/internal/apperr"
)

type thing struct{ started bool }

type pair struct {
	A string `json:"a"`
	B []int  `json:"b"`
}

func (t *thing) Add(a, b int) int { return a + b }
func (t *thing) Echo(p pair) pair { return p }
func (t *thing) Fail() error      { return errors.New("nope") }
func (t *thing) Coded() error {
	return apperr.New("game.running", "Doom is running", map[string]string{"title": "Doom"})
}
func (t *thing) Wrapped() error {
	return apperr.Wrap("test.wrapped", errors.New("no such file"), nil)
}
func (t *thing) Both(s string) (string, error) { return s + "!", nil }
func (t *thing) Boom()                         { panic("kaboom") }
func (t *thing) Nothing()                      {}
func (t *thing) Nil() []string                 { return nil }
func (t *thing) hidden()                       {}
func (t *thing) Callback(f func())             {}

// harness wires a Server to a line-based test client.
type harness struct {
	t   *testing.T
	in  io.WriteCloser
	out *bufio.Scanner
	srv *Server
	end chan error
}

func start(t *testing.T) *harness {
	t.Helper()
	srv := New(&thing{})
	cr, cw := io.Pipe()
	sr, sw := io.Pipe()
	h := &harness{t: t, in: cw, out: bufio.NewScanner(sr), srv: srv, end: make(chan error, 1)}
	go func() { h.end <- srv.Serve(cr, sw); sw.Close() }()
	t.Cleanup(func() { cw.Close() })
	return h
}

func (h *harness) send(line string) { h.t.Helper(); io.WriteString(h.in, line+"\n") }

func (h *harness) read() map[string]json.RawMessage {
	h.t.Helper()
	done := make(chan bool, 1)
	go func() { done <- h.out.Scan() }()
	select {
	case ok := <-done:
		if !ok {
			h.t.Fatal("stream ended")
		}
	case <-time.After(3 * time.Second):
		h.t.Fatal("no message")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(h.out.Bytes(), &m); err != nil {
		h.t.Fatalf("bad line %q: %v", h.out.Text(), err)
	}
	return m
}

func (h *harness) call(id int, method, params string) map[string]json.RawMessage {
	h.t.Helper()
	h.send(`{"jsonrpc":"2.0","id":` + string(rune('0'+id)) + `,"method":"` + method + `","params":` + params + `}`)
	return h.read()
}

func (h *harness) init() {
	h.t.Helper()
	r := h.call(1, "initialize", `[{"protocol":"1.0"}]`)
	if r["error"] != nil {
		h.t.Fatalf("initialize: %s", r["error"])
	}
}

func errMsg(m map[string]json.RawMessage) (int, string) {
	var e Error
	json.Unmarshal(m["error"], &e)
	return e.Code, e.Message
}

func TestInitializeListsMethods(t *testing.T) {
	h := start(t)
	r := h.call(1, "initialize", `[{"protocol":"1.0"}]`)
	var hello Hello
	json.Unmarshal(r["result"], &hello)
	if hello.Protocol != Protocol {
		t.Errorf("protocol = %q", hello.Protocol)
	}
	got := strings.Join(hello.Methods, ",")
	if got != "Add,Boom,Both,Coded,Echo,Fail,Nil,Nothing,Wrapped" {
		t.Errorf("methods = %s (unexported and function-taking methods must be left out)", got)
	}
}

func TestInitializeRunsHookAndRefusesIncompatibleShells(t *testing.T) {
	h := start(t)
	ran := 0
	h.srv.OnInitialize = func() error { ran++; return nil }
	for _, v := range []string{"2.0", "1.1", "x"} {
		if code, _ := errMsg(h.call(1, "initialize", `[{"protocol":"`+v+`"}]`)); code != CodeAppError {
			t.Errorf("protocol %s: code = %d", v, code)
		}
	}
	if ran != 0 {
		t.Fatal("OnInitialize ran for an incompatible shell")
	}
	h.init()
	if ran != 1 {
		t.Errorf("OnInitialize ran %d times", ran)
	}
	if code, _ := errMsg(h.call(1, "initialize", `[{"protocol":"1.0"}]`)); code != CodeInvalidRequest {
		t.Errorf("second initialize: code = %d", code)
	}
}

func TestInitializeFailureStaysUninitialized(t *testing.T) {
	h := start(t)
	h.srv.OnInitialize = func() error { return errors.New("no disk") }
	if _, msg := errMsg(h.call(1, "initialize", `[{"protocol":"1.0"}]`)); !strings.Contains(msg, "no disk") {
		t.Errorf("message = %q", msg)
	}
	if code, _ := errMsg(h.call(2, "Add", `[1,2]`)); code != CodeNotInitialized {
		t.Errorf("code = %d", code)
	}
}

func TestCallsBeforeInitializeAreRefused(t *testing.T) {
	h := start(t)
	if code, _ := errMsg(h.call(2, "Add", `[1,2]`)); code != CodeNotInitialized {
		t.Errorf("code = %d", code)
	}
}

func TestMethodResults(t *testing.T) {
	h := start(t)
	h.init()
	cases := []struct{ method, params, want string }{
		{"Add", `[2,3]`, `5`},
		{"Echo", `[{"a":"x","b":[1,2]}]`, `{"a":"x","b":[1,2]}`},
		{"Both", `["hi"]`, `"hi!"`},
		{"Nothing", `[]`, `null`},
		{"Nil", `[]`, `null`},
	}
	for _, c := range cases {
		r := h.call(2, c.method, c.params)
		if string(r["result"]) != c.want {
			t.Errorf("%s = %s (error %s), want %s", c.method, r["result"], r["error"], c.want)
		}
	}
}

func TestErrorsAreResponses(t *testing.T) {
	h := start(t)
	h.init()
	cases := []struct {
		method, params string
		code           int
		msg            string
	}{
		{"Fail", `[]`, CodeAppError, "nope"},
		{"Boom", `[]`, CodeInternalError, "kaboom"},
		{"Missing", `[]`, CodeMethodNotFound, "Missing"},
		{"hidden", `[]`, CodeMethodNotFound, "hidden"},
		{"Add", `[1]`, CodeInvalidParams, "takes 2"},
		{"Add", `["a","b"]`, CodeInvalidParams, "argument 1"},
		{"Add", `{"a":1}`, CodeInvalidParams, "array"},
	}
	for _, c := range cases {
		code, msg := errMsg(h.call(2, c.method, c.params))
		if code != c.code || !strings.Contains(msg, c.msg) {
			t.Errorf("%s %s: got %d %q, want %d containing %q", c.method, c.params, code, msg, c.code, c.msg)
		}
	}
	// The server survives all of that.
	if r := h.call(2, "Add", `[1,1]`); string(r["result"]) != "2" {
		t.Errorf("after errors: %s", r["result"])
	}
}

func TestCodedErrorsCarryDataAndKeepTheMessage(t *testing.T) {
	h := start(t)
	h.init()

	r := h.call(2, "Coded", `[]`)
	var e Error
	if err := json.Unmarshal(r["error"], &e); err != nil {
		t.Fatalf("bad error: %v", err)
	}
	if e.Code != CodeAppError || e.Message != "Doom is running" {
		t.Errorf("error = %+v, want code %d message %q", e, CodeAppError, "Doom is running")
	}
	if e.Data == nil || e.Data.Code != "game.running" || e.Data.Params["title"] != "Doom" {
		t.Errorf("data = %+v, want code game.running with title Doom", e.Data)
	}

	r = h.call(2, "Wrapped", `[]`)
	e = Error{}
	json.Unmarshal(r["error"], &e)
	if e.Message != "no such file" || e.Data == nil || e.Data.Code != "test.wrapped" {
		t.Errorf("wrapped error = %+v", e)
	}

	// A plain error carries no data, as before.
	r = h.call(2, "Fail", `[]`)
	e = Error{}
	json.Unmarshal(r["error"], &e)
	if e.Message != "nope" || e.Data != nil {
		t.Errorf("plain error = %+v, want message only", e)
	}
}

func TestGarbageLineIsAParseError(t *testing.T) {
	h := start(t)
	h.send("{nope")
	if code, _ := errMsg(h.read()); code != CodeParseError {
		t.Errorf("code = %d", code)
	}
}

func TestNotifyAndCallToTheShell(t *testing.T) {
	h := start(t)
	h.init()
	go h.srv.Notify("jobs", []string{"a"})
	n := h.read()
	if string(n["method"]) != `"jobs"` || n["id"] != nil || string(n["params"]) != `["a"]` {
		t.Errorf("notification = %v", n)
	}

	got := make(chan string, 1)
	go func() {
		var out string
		err := h.srv.Call(context.Background(), "dialog.openDirectory", map[string]string{"title": "t"}, &out)
		if err != nil {
			out = "error: " + err.Error()
		}
		got <- out
	}()
	req := h.read()
	if string(req["method"]) != `"dialog.openDirectory"` || req["id"] == nil {
		t.Fatalf("request = %v", req)
	}
	h.send(`{"jsonrpc":"2.0","id":` + string(req["id"]) + `,"result":"/home/x"}`)
	if v := <-got; v != "/home/x" {
		t.Errorf("Call returned %q", v)
	}
}

func TestShellErrorReachesTheCaller(t *testing.T) {
	h := start(t)
	h.init()
	got := make(chan error, 1)
	go func() { got <- h.srv.Call(context.Background(), "dialog.openFile", nil, nil) }()
	req := h.read()
	h.send(`{"jsonrpc":"2.0","id":` + string(req["id"]) + `,"error":{"code":-1,"message":"window closed"}}`)
	if err := <-got; err == nil || err.Error() != "window closed" {
		t.Errorf("err = %v", err)
	}
}

func TestEndOfStreamEndsServeAndFailsPendingCalls(t *testing.T) {
	h := start(t)
	h.init()
	got := make(chan error, 1)
	go func() { got <- h.srv.Call(context.Background(), "dialog.openFile", nil, nil) }()
	h.read()
	h.in.Close()
	if err := <-got; err == nil {
		t.Error("pending call survived the end of the stream")
	}
	if err := <-h.end; err != nil {
		t.Errorf("Serve = %v", err)
	}
}
