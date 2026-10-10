package main

import (
	"encoding/json"
	"sync"
	"time"

	"platypus/internal/types"
)

// How long the size of an install is remembered. A store's figure changes with
// an update, not by the minute; a failure is remembered briefly, so a store
// that can't tell isn't asked again at every visit of the page.
const (
	sizeTTL     = 10 * time.Minute
	sizeFailTTL = time.Minute
)

// sizeCache remembers the size of a fresh install by game and version. The
// zero value is ready to use. Asking for the same size twice at once runs one
// call.
type sizeCache struct {
	mu  sync.Mutex
	now func() time.Time // tests replace it
	m   map[string]*sizeEntry
}

type sizeEntry struct {
	bytes int64
	err   error
	at    time.Time
	done  chan struct{} // closed when the call is over
}

func (c *sizeCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// get returns the cached size of (gameID, target), or runs fetch for it.
func (c *sizeCache) get(gameID string, target *types.VersionTarget, fetch func() (int64, error)) (int64, error) {
	raw, _ := json.Marshal(target) // nil is "null": the latest version
	key := gameID + "\x00" + string(raw)
	c.mu.Lock()
	if e, ok := c.m[key]; ok {
		select {
		case <-e.done:
			ttl := sizeTTL
			if e.err != nil {
				ttl = sizeFailTTL
			}
			if c.clock().Sub(e.at) < ttl {
				c.mu.Unlock()
				return e.bytes, e.err
			}
		default: // a call is running: wait for it
			c.mu.Unlock()
			<-e.done
			return e.bytes, e.err
		}
	}
	e := &sizeEntry{done: make(chan struct{})}
	if c.m == nil {
		c.m = map[string]*sizeEntry{}
	}
	c.m[key] = e
	c.mu.Unlock()

	e.bytes, e.err = fetch()
	e.at = c.clock()
	close(e.done)
	return e.bytes, e.err
}
