package cloudsync

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

func newJobID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "cloud-" + hex.EncodeToString(b[:])
}

func (c *Orchestrator) load() {
	if c.o.StatePath == "" {
		return
	}
	data, err := os.ReadFile(c.o.StatePath)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &c.state); err != nil {
		bad := c.o.StatePath + ".bad"
		_ = os.Rename(c.o.StatePath, bad)
		c.logf("cloud state file is corrupt, moved aside: %v", err)
		c.state = fileState{}
	}
}

// persist writes the state atomically. Callers hold c.mu.
func (c *Orchestrator) persist() {
	if c.o.StatePath == "" {
		return
	}
	c.state.Version = 1
	data, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(c.o.StatePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.logf("cloud state: %v", err)
		return
	}
	tmp, err := os.CreateTemp(dir, ".cloud-*.json")
	if err != nil {
		c.logf("cloud state: %v", err)
		return
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.Write(append(data, '\n'))
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		c.logf("cloud state: write failed")
		return
	}
	if err := os.Rename(tmp.Name(), c.o.StatePath); err != nil {
		c.logf("cloud state: %v", err)
	}
}

// PendingPushes lists the pushes owed, for tests and diagnostics.
func (c *Orchestrator) PendingPushes() []Pending {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Pending(nil), c.state.Pending...)
}

// pendingFor finds the owed push of a game. Callers hold c.mu.
func (c *Orchestrator) pendingFor(gameID string) *Pending {
	for i := range c.state.Pending {
		if c.state.Pending[i].GameID == gameID {
			return &c.state.Pending[i]
		}
	}
	return nil
}

// dropPending forgets the owed push of a game.
func (c *Orchestrator) dropPending(gameID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.state.Pending {
		if c.state.Pending[i].GameID == gameID {
			c.state.Pending = append(c.state.Pending[:i], c.state.Pending[i+1:]...)
			c.persist()
			return
		}
	}
}
