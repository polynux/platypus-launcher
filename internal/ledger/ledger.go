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
	// maxTotal bounds the history of all games together; the oldest entries
	// of any game go first.
	maxTotal = 2000
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
	// maxTotal is the cap on all entries together (a field so tests can lower it).
	maxTotal int
	// readErr is set when the file exists but couldn't be read. The history in
	// memory is then empty and not the file's, so Append refuses to write over it.
	readErr error

	mu   sync.Mutex
	data file
}

// Open loads the ledger at path. A missing file is an empty ledger; a corrupt
// one is moved aside (…bad-<time>) and Open returns an empty ledger with the
// error, so history is never silently overwritten. A file that can't be read
// at all also gives an empty ledger with the error, and that ledger is
// read-only: Append refuses, so the real file is left as it is.
func Open(path string) (*Ledger, error) {
	l := &Ledger{path: path, now: time.Now, maxTotal: maxTotal, data: file{Games: map[string][]types.LedgerEntry{}}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		l.readErr = fmt.Errorf("install history can't be read: %w", err)
		return l, l.readErr
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
	if l.readErr != nil {
		return l.readErr
	}
	before := l.snapshot()
	list := l.data.Games[e.Game]
	if n := len(list); n > 0 && list[n-1].Build == e.Build && list[n-1].Channel == e.Channel {
		list[n-1] = e
	} else {
		list = append(list, e)
		if len(list) > maxPerGame {
			list = list[len(list)-maxPerGame:]
		}
	}
	l.data.Games[e.Game] = list
	l.pruneTotal()
	if err := l.save(); err != nil {
		l.data.Games = before
		return err
	}
	return nil
}

// snapshot copies the history, so a failed save can be undone. Callers hold l.mu.
func (l *Ledger) snapshot() map[string][]types.LedgerEntry {
	out := make(map[string][]types.LedgerEntry, len(l.data.Games))
	for g, list := range l.data.Games {
		out[g] = append([]types.LedgerEntry{}, list...)
	}
	return out
}

// pruneTotal drops the oldest entries, of whichever game they belong to, until
// the whole history is within maxTotal. Each game's list is oldest first, so
// the oldest entry is the first of some game. Callers hold l.mu.
func (l *Ledger) pruneTotal() {
	total := 0
	for _, list := range l.data.Games {
		total += len(list)
	}
	for ; total > l.maxTotal; total-- {
		oldest := ""
		for g, list := range l.data.Games {
			if len(list) == 0 {
				continue
			}
			if oldest == "" || list[0].Date < l.data.Games[oldest][0].Date {
				oldest = g
			}
		}
		if oldest == "" {
			return
		}
		l.data.Games[oldest] = l.data.Games[oldest][1:]
	}
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
