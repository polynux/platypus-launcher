package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// serve answers requests from r on w: "add" sums two ints, "hang" never
// answers, "notify" sends a notification before answering, anything else is
// method-not-found.
func serve(t *testing.T, r io.Reader, w io.Writer) {
	t.Helper()
	sc := bufio.NewScanner(r)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		var m message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Errorf("server got bad line %q", sc.Text())
			return
		}
		switch m.Method {
		case "add":
			var p [2]int
			json.Unmarshal(m.Params, &p)
			res, _ := json.Marshal(p[0] + p[1])
			enc.Encode(message{JSONRPC: "2.0", ID: m.ID, Result: res})
		case "hang":
		case "notify":
			enc.Encode(message{JSONRPC: "2.0", Method: "log", Params: json.RawMessage(`{"msg":"hi"}`)})
			enc.Encode(message{JSONRPC: "2.0", ID: m.ID, Result: json.RawMessage(`true`)})
		default:
			enc.Encode(message{JSONRPC: "2.0", ID: m.ID, Error: &Error{Code: CodeMethodNotFound, Message: "no " + m.Method}})
		}
	}
}

func pipe(t *testing.T, onNotify NotifyFunc) (*Conn, func()) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	go serve(t, sr, sw)
	c := NewConn(cr, cw, onNotify)
	return c, func() { cw.Close(); sw.Close() }
}

func TestCallConcurrent(t *testing.T) {
	c, stop := pipe(t, nil)
	defer stop()
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var sum int
			if err := c.Call(context.Background(), "add", [2]int{i, 1000}, &sum); err != nil {
				t.Error(err)
			} else if sum != i+1000 {
				t.Errorf("add(%d, 1000) = %d", i, sum)
			}
		}()
	}
	wg.Wait()
}

func TestErrorResponse(t *testing.T) {
	c, stop := pipe(t, nil)
	defer stop()
	err := c.Call(context.Background(), "nope", nil, nil)
	var rpcErr *Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != CodeMethodNotFound {
		t.Fatalf("want method-not-found, got %v", err)
	}
}

func TestNotification(t *testing.T) {
	got := make(chan string, 1)
	c, stop := pipe(t, func(method string, params json.RawMessage) { got <- method + " " + string(params) })
	defer stop()
	if err := c.Call(context.Background(), "notify", nil, nil); err != nil {
		t.Fatal(err)
	}
	if n := <-got; n != `log {"msg":"hi"}` {
		t.Errorf("notification = %q", n)
	}
}

func TestContextCancel(t *testing.T) {
	c, stop := pipe(t, nil)
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := c.Call(ctx, "hang", nil, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline exceeded, got %v", err)
	}
}

func TestPendingFailOnClose(t *testing.T) {
	c, stop := pipe(t, nil)
	errc := make(chan error, 1)
	go func() { errc <- c.Call(context.Background(), "hang", nil, nil) }()
	time.Sleep(20 * time.Millisecond)
	stop() // the other side goes away: EOF on our reader
	select {
	case err := <-errc:
		if !errors.Is(err, ErrClosed) {
			t.Fatalf("want ErrClosed, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pending call never failed")
	}
	if err := c.Call(context.Background(), "add", [2]int{1, 2}, nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("call after close: %v", err)
	}
}
