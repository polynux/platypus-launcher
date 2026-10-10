// Package corerpc serves one Go value over JSON-RPC 2.0 on a byte stream, one
// JSON object per line: the protocol between the Platypus core process and the
// UI shell (Electron) that spawns it. Requests name an exported method of the
// served value and carry its arguments as a positional JSON array; the result
// and the error come back as the response. Events go the other way as
// notifications, and the core can call the shell (native dialogs).
//
// The framing is the one the store plugins use (internal/jsonrpc). Batches are
// not supported. The first request must be "initialize", which settles the
// protocol version.
package corerpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// Protocol is the core protocol version, major.minor. A minor bump is
// additive only (new methods, optional fields); a major bump may break. A
// shell must speak the same major and a minor no newer than the core's.
const Protocol = "1.0"

// MaxMessage bounds one line; library listings can be large.
const MaxMessage = 32 << 20

// Error codes, besides the JSON-RPC standard ones.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	// CodeNotInitialized answers a request that came before initialize.
	CodeNotInitialized = -32002
	// CodeAppError answers a method that returned an error; the message is the error's.
	CodeAppError = -32000
)

// Error is an error response.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *Error           `json:"error,omitempty"`
}

// Hello is the params of initialize, and what the core answers with.
type Hello struct {
	Protocol string `json:"protocol"`
	// Methods, in the answer, lists what the core serves.
	Methods []string `json:"methods,omitempty"`
	// Info, in the answer, is what the core tells the shell about itself.
	Info map[string]any `json:"info,omitempty"`
}

// Server serves target's exported methods.
type Server struct {
	target  reflect.Value
	methods map[string]reflect.Value

	// OnInitialize runs once, while handling initialize and before it answers;
	// an error fails the handshake. The core starts the App here, so every
	// later call finds it ready.
	OnInitialize func() error

	// Info is sent to the shell in the answer to initialize.
	Info map[string]any

	w       io.Writer
	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  uint64
	pending map[uint64]chan *message
	ready   bool
	closed  bool
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// New serves the exported methods of target (typically a pointer to a struct).
// Methods with parameters or results JSON cannot carry (functions, channels)
// are left out.
func New(target any) *Server {
	s := &Server{target: reflect.ValueOf(target), methods: map[string]reflect.Value{}, pending: map[uint64]chan *message{}}
	t := s.target.Type()
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		if !servable(m.Type) {
			continue
		}
		s.methods[m.Name] = s.target.Method(i)
	}
	return s
}

func servable(t reflect.Type) bool {
	ok := func(t reflect.Type) bool {
		switch t.Kind() {
		case reflect.Func, reflect.Chan, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
			return false
		}
		return true
	}
	for i := 0; i < t.NumIn(); i++ {
		if !ok(t.In(i)) {
			return false
		}
	}
	switch t.NumOut() {
	case 0:
	case 1:
		if !ok(t.Out(0)) && t.Out(0) != errorType {
			return false
		}
	case 2:
		if t.Out(1) != errorType || !ok(t.Out(0)) {
			return false
		}
	default:
		return false
	}
	return true
}

// Methods lists the served method names, sorted.
func (s *Server) Methods() []string {
	names := make([]string, 0, len(s.methods))
	for n := range s.methods {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Serve reads requests from r and answers on w until r ends. It returns nil on
// a clean EOF. Requests run concurrently; the responses may come in any order.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	s.writeMu.Lock()
	s.w = w
	s.writeMu.Unlock()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), MaxMessage)
	var wg sync.WaitGroup
	defer func() {
		s.mu.Lock()
		s.closed = true
		for id, ch := range s.pending {
			close(ch)
			delete(s.pending, id)
		}
		s.mu.Unlock()
		wg.Wait()
	}()
	for sc.Scan() {
		line := append([]byte(nil), sc.Bytes()...)
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var m message
		if err := json.Unmarshal(line, &m); err != nil {
			s.respond(nil, nil, &Error{CodeParseError, "parse error: " + err.Error()})
			continue
		}
		switch {
		case m.Method == "" && m.ID != nil:
			s.deliver(&m)
		case m.Method == "":
			s.respond(nil, nil, &Error{CodeInvalidRequest, "neither a method nor a response"})
		case m.ID == nil:
			// Notifications from the shell have no meaning yet.
		case m.Method == "initialize":
			// Handled in line: nothing else may run before it has answered.
			s.initialize(&m)
		default:
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.handle(&m)
			}()
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}

func (s *Server) initialize(m *message) {
	var args []Hello
	if err := json.Unmarshal(m.Params, &args); err != nil || len(args) != 1 {
		s.respond(m.ID, nil, &Error{CodeInvalidParams, "initialize takes one argument: {protocol}"})
		return
	}
	if err := compatible(args[0].Protocol); err != nil {
		s.respond(m.ID, nil, &Error{CodeAppError, err.Error()})
		return
	}
	s.mu.Lock()
	already := s.ready
	s.mu.Unlock()
	if already {
		s.respond(m.ID, nil, &Error{CodeInvalidRequest, "already initialized"})
		return
	}
	if s.OnInitialize != nil {
		if err := s.OnInitialize(); err != nil {
			s.respond(m.ID, nil, &Error{CodeAppError, "starting the core: " + err.Error()})
			return
		}
	}
	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()
	s.respond(m.ID, Hello{Protocol: Protocol, Methods: s.Methods(), Info: s.Info}, nil)
}

// compatible accepts a shell that speaks the core's major and a minor no newer.
func compatible(shell string) error {
	maj, min, ok := parse(shell)
	cmaj, cmin, _ := parse(Protocol)
	switch {
	case !ok:
		return fmt.Errorf("protocol %q must look like 1.0 (major.minor)", shell)
	case maj != cmaj:
		return fmt.Errorf("the shell speaks core protocol %s, this core speaks %s", shell, Protocol)
	case min > cmin:
		return fmt.Errorf("the shell speaks core protocol %s, newer than this core's %s", shell, Protocol)
	}
	return nil
}

func parse(v string) (maj, min int, ok bool) {
	a, b, found := strings.Cut(v, ".")
	if !found {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(a, "%d", &maj); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(b, "%d", &min); err != nil {
		return 0, 0, false
	}
	return maj, min, true
}

func (s *Server) handle(m *message) {
	s.mu.Lock()
	ready := s.ready
	s.mu.Unlock()
	if !ready {
		s.respond(m.ID, nil, &Error{CodeNotInitialized, "send initialize first"})
		return
	}
	fn, ok := s.methods[m.Method]
	if !ok {
		s.respond(m.ID, nil, &Error{CodeMethodNotFound, "unknown method " + m.Method})
		return
	}
	result, err := invoke(fn, m.Params)
	s.respond(m.ID, result, err)
}

// invoke calls fn with the positional arguments in params.
func invoke(fn reflect.Value, params json.RawMessage) (result any, rerr *Error) {
	defer func() {
		if p := recover(); p != nil {
			result, rerr = nil, &Error{CodeInternalError, fmt.Sprintf("panic: %v", p)}
		}
	}()
	t := fn.Type()
	var raw []json.RawMessage
	if len(params) > 0 {
		if err := json.Unmarshal(params, &raw); err != nil {
			return nil, &Error{CodeInvalidParams, "params must be a JSON array of the arguments"}
		}
	}
	if len(raw) != t.NumIn() {
		return nil, &Error{CodeInvalidParams, fmt.Sprintf("takes %d arguments, got %d", t.NumIn(), len(raw))}
	}
	in := make([]reflect.Value, len(raw))
	for i, r := range raw {
		v := reflect.New(t.In(i))
		if err := json.Unmarshal(r, v.Interface()); err != nil {
			return nil, &Error{CodeInvalidParams, fmt.Sprintf("argument %d: %v", i+1, err)}
		}
		in[i] = v.Elem()
	}
	out := fn.Call(in)
	if n := len(out); n > 0 && t.Out(n-1) == errorType && !out[n-1].IsNil() {
		return nil, &Error{CodeAppError, out[n-1].Interface().(error).Error()}
	}
	if len(out) == 0 || t.Out(0) == errorType {
		return nil, nil
	}
	return out[0].Interface(), nil
}

// Notify sends a notification to the shell (the core's events).
func (s *Server) Notify(method string, params any) error {
	return s.send(message{Method: method}, params)
}

// Call asks the shell to do something and waits for its answer (native
// dialogs). result may be nil.
func (s *Server) Call(ctx context.Context, method string, params, result any) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("corerpc: connection closed")
	}
	s.nextID++
	id := s.nextID
	ch := make(chan *message, 1)
	s.pending[id] = ch
	s.mu.Unlock()
	forget := func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}
	raw := json.RawMessage(fmt.Sprint(id))
	if err := s.send(message{ID: &raw, Method: method}, params); err != nil {
		forget()
		return err
	}
	select {
	case resp, ok := <-ch:
		if !ok {
			return errors.New("corerpc: connection closed")
		}
		if resp.Error != nil {
			return resp.Error
		}
		if result == nil || len(resp.Result) == 0 {
			return nil
		}
		return json.Unmarshal(resp.Result, result)
	case <-ctx.Done():
		forget()
		return ctx.Err()
	}
}

func (s *Server) deliver(m *message) {
	var id uint64
	if err := json.Unmarshal(*m.ID, &id); err != nil {
		return
	}
	s.mu.Lock()
	ch, ok := s.pending[id]
	delete(s.pending, id)
	s.mu.Unlock()
	if ok {
		ch <- m
	}
}

func (s *Server) respond(id *json.RawMessage, result any, rerr *Error) {
	m := message{ID: id, Error: rerr}
	if id == nil {
		null := json.RawMessage("null")
		m.ID = &null
	}
	if rerr == nil {
		b, err := json.Marshal(result)
		if err != nil {
			m.Error = &Error{CodeInternalError, "encoding the result: " + err.Error()}
		} else {
			m.Result = b
		}
	}
	_ = s.send(m, nil)
}

func (s *Server) send(m message, params any) error {
	m.JSONRPC = "2.0"
	if params != nil {
		p, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("corerpc: encoding %s params: %w", m.Method, err)
		}
		m.Params = p
	}
	if m.Method == "" && m.Error == nil && m.Result == nil {
		m.Result = json.RawMessage("null") // a response always carries one of the two
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.w == nil {
		return errors.New("corerpc: not serving yet")
	}
	_, err = s.w.Write(append(data, '\n'))
	return err
}
