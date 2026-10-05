// Package jsonrpc is a JSON-RPC 2.0 client over a byte stream with one JSON
// object per line, which is how Hydra talks to store plugins over stdio.
// Batches are not supported.
package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// MaxMessage bounds one line. Library listings can be large, but a plugin
// streaming garbage without newlines must not exhaust memory.
const MaxMessage = 32 << 20

// Standard error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// ErrClosed is returned for calls on a closed connection, and for calls still
// pending when the connection closes.
var ErrClosed = errors.New("jsonrpc: connection closed")

// Error is an error response from the other side.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error is the other side's message, which users see; the code is for
// errors.As.
func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("error %d", e.Code)
	}
	return e.Message
}

// message covers requests, notifications and responses.
type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *Error           `json:"error,omitempty"`
}

// NotifyFunc receives notifications from the other side. It runs on the read
// loop, so it must not block or call back into the connection synchronously.
type NotifyFunc func(method string, params json.RawMessage)

// Conn is the client end of a connection. Calls may run concurrently.
type Conn struct {
	w        io.Writer
	onNotify NotifyFunc

	writeMu sync.Mutex
	mu      sync.Mutex
	nextID  uint64
	pending map[uint64]chan *message
	closed  bool
	err     error // why the read loop stopped
	done    chan struct{}
}

// NewConn starts reading responses from r. The connection closes when r hits
// EOF or a read error, or on Close.
func NewConn(r io.Reader, w io.Writer, onNotify NotifyFunc) *Conn {
	c := &Conn{w: w, onNotify: onNotify, pending: map[uint64]chan *message{}, done: make(chan struct{})}
	go c.read(r)
	return c
}

// Call sends a request and decodes the result into result (which may be nil).
// It returns an *Error if the other side answered with one.
func (c *Conn) Call(ctx context.Context, method string, params, result any) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	c.nextID++
	id := c.nextID
	ch := make(chan *message, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	forget := func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}
	rawID := json.RawMessage(fmt.Sprint(id))
	if err := c.send(message{ID: &rawID, Method: method}, params); err != nil {
		forget()
		return err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return ErrClosed
		}
		if resp.Error != nil {
			return resp.Error
		}
		if result == nil || len(resp.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("jsonrpc: decoding %s result: %w", method, err)
		}
		return nil
	case <-ctx.Done():
		forget()
		return ctx.Err()
	}
}

// Notify sends a notification, which gets no response.
func (c *Conn) Notify(method string, params any) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return ErrClosed
	}
	return c.send(message{Method: method}, params)
}

func (c *Conn) send(m message, params any) error {
	m.JSONRPC = "2.0"
	if params != nil {
		p, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("jsonrpc: encoding %s params: %w", m.Method, err)
		}
		m.Params = p
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("jsonrpc: write: %w", err)
	}
	return nil
}

// Close fails pending calls with ErrClosed. It doesn't close the underlying
// stream; the owner does that.
func (c *Conn) Close() {
	c.shutdown(nil)
}

// Done is closed once the connection is closed.
func (c *Conn) Done() <-chan struct{} {
	return c.done
}

// Err says why the connection closed: nil after Close or a clean EOF.
func (c *Conn) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *Conn) shutdown(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.err = err
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	close(c.done)
}

func (c *Conn) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), MaxMessage)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var m message
		if err := json.Unmarshal(line, &m); err != nil {
			// Not a protocol message (stray print from the plugin). Skip it
			// rather than drop the connection.
			continue
		}
		switch {
		case m.Method != "" && m.ID != nil:
			// Plugins may only send notifications to Hydra.
			c.reply(m.ID, &Error{Code: CodeMethodNotFound, Message: "hydra accepts no requests: " + m.Method})
		case m.Method != "":
			if c.onNotify != nil {
				c.onNotify(m.Method, m.Params)
			}
		case m.ID != nil:
			var id uint64
			if json.Unmarshal(*m.ID, &id) != nil {
				continue
			}
			c.mu.Lock()
			ch, ok := c.pending[id]
			delete(c.pending, id)
			c.mu.Unlock()
			if ok {
				ch <- &m
			}
		}
	}
	err := sc.Err()
	if err != nil {
		err = fmt.Errorf("jsonrpc: read: %w", err)
	}
	c.shutdown(err)
}

func (c *Conn) reply(id *json.RawMessage, e *Error) {
	_ = c.send(message{ID: id, Error: e}, nil)
}
