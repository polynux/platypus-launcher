// Login-id registry and per-connection write serialization for the
// duplicate-kick behaviour, plus small helpers.
package steamcmtest

import (
	"encoding/base64"
	"sync"

	"github.com/gorilla/websocket"
)

// loginRegistry tracks a Fake's live connections per login id so a
// second logon with the same id can kick the first (ClientLoggedOff
// 34). One registry per Fake: tests never share state.
type loginRegistry struct {
	mu    sync.Mutex
	conns map[uint32]*fakeConn
}

// claim registers c for loginID and returns the connection it replaced
// (nil when the id was free).
func (r *loginRegistry) claim(loginID uint32, c *fakeConn) *fakeConn {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns == nil {			r.conns = map[uint32]*fakeConn{}
	}
	old := r.conns[loginID]
	r.conns[loginID] = c
	return old
}

func (r *loginRegistry) release(loginID uint32, c *fakeConn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns != nil && r.conns[loginID] == c {
		delete(r.conns, loginID)
	}
}

func (r *loginRegistry) count(loginID uint32) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns != nil && r.conns[loginID] != nil {
		return 1
	}
	return 0
}

// LiveLogins reports how many of this fake's live sessions use loginID.
func (f *Fake) LiveLogins(loginID uint32) int { return f.logins.count(loginID) }

func base64Encode(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

// fakeConn is one served connection: a websocket whose writes go
// through a per-connection mutex, because a kick can be written from
// another connection's serve goroutine (gorilla/websocket allows only
// one concurrent writer).
type fakeConn struct {
	*websocket.Conn
	writeMu sync.Mutex
}

func (fc *fakeConn) write(msg []byte) {
	fc.writeMu.Lock()
	_ = fc.Conn.WriteMessage(websocket.BinaryMessage, msg)
	fc.writeMu.Unlock()
}

// close kicks the reader loose; safe next to a write in progress.
func (fc *fakeConn) close() { _ = fc.Conn.Close() }
