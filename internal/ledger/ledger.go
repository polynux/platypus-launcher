// Package ledger keeps the install history of every plugin game, in
// ~/.local/share/hydra/install-history.json: which build was installed when,
// and the opaque reference its plugin needs to fetch it again. Stores like
// Steam and Epic don't list old builds, so this is the only way back to one
// the user once had (docs/plans/launcher-parity.md §2.2).
package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"hydra-launcher/internal/types"
)

const (
	fileVersion = 1
	// maxPerGame bounds a game's history; the oldest entries go first.
	maxPerGame = 100
	// maxArchiveRef bounds the opaque reference a plugin may attach.
	maxArchiveRef = 64 << 10
)

type file struct {
	Version int                            `json:"version"`
	Games   map[string][]types.LedgerEntry `json:"games"`
}

// Ledger is safe for concurrent use and writes through on every change.
type Ledger struct {
	path string
	now  func() time.Time

	mu   sync.Mutex
	data file
}

// Open loads the ledger at path. A missing file is an empty ledger; a corrupt
// one is moved aside (…bad-<time>) and Open returns an empty ledger with the
// error, so history is never silently overwritten.
func Open(path string) (*Ledger, error) {
	l := &Ledger{path: path, now: time.Now, data: file{Games: map[string][]types.LedgerEntry{}}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return l, err
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		bad := fmt.Sprintf("%s.bad-%d", path, time.Now().Unix())
		_ = os.Rename(path, bad)
		return l, fmt.Errorf("install history is corrupt, moved to %s: %w", bad, err)
	}
	if f.Games != nil {
		l.data.Games = f.Games
	}
	return l, nil
}

// Entries returns a game's history, oldest first.
func (l *Ledger) Entries(game string) []types.LedgerEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]types.LedgerEntry{}, l.data.Games[game]...)
}

// Append records an install. Installing the build the history already ends
// with refreshes that row (new date, name, reference) instead of adding one,
// so a repeated update doesn't flood the list.
func (l *Ledger) Append(e types.LedgerEntry) error {
	if e.Store == "" || e.Game == "" {
		return errors.New("ledger entry needs a store and a game")
	}
	if e.Build == "" {
		return errors.New("ledger entry needs a build")
	}
	if len(e.ArchiveRef) > maxArchiveRef {
		return fmt.Errorf("archiveRef is %d bytes; the limit is %d", len(e.ArchiveRef), maxArchiveRef)
	}
	if len(e.ArchiveRef) > 0 && !json.Valid(e.ArchiveRef) {
		return errors.New("archiveRef is not valid JSON")
	}
	if len(e.ArchiveRef) > 0 {
		var buf bytes.Buffer
		if err := json.Compact(&buf, e.ArchiveRef); err == nil {
			e.ArchiveRef = buf.Bytes()
		}
	}
	if e.Date == "" {
		e.Date = l.now().UTC().Format(time.RFC3339)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	list := l.data.Games[e.Game]
	prev := append([]types.LedgerEntry{}, list...)
	if n := len(list); n > 0 && list[n-1].Build == e.Build && list[n-1].Channel == e.Channel {
		list[n-1] = e
	} else {
		list = append(list, e)
		if len(list) > maxPerGame {
			list = list[len(list)-maxPerGame:]
		}
	}
	l.data.Games[e.Game] = list
	if err := l.save(); err != nil {
		l.data.Games[e.Game] = prev
		return err
	}
	return nil
}

// save writes the file atomically. Callers hold l.mu.
func (l *Ledger) save() error {
	l.data.Version = fileVersion
	raw, err := json.MarshalIndent(l.data, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".history-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), l.path)
}
