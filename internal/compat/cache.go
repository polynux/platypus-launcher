package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache is the on-disk record of what the compat sources answered, in
// Hydra's data folder. Hits and misses are both kept: a miss ("no reports")
// is an answer too, and it is asked again less often (see Service).
type Cache struct {
	mu   sync.Mutex
	path string
	data cacheFile
}

type cacheFile struct {
	Version  int                    `json:"version"`
	ProtonDB map[string]ProtonEntry `json:"protondb,omitempty"`
	Local    map[string]LocalEntry  `json:"local,omitempty"`
}

// ProtonEntry is one ProtonDB answer for a Steam AppID. Found false means
// ProtonDB has no reports for the app.
type ProtonEntry struct {
	Found      bool      `json:"found"`
	FetchedAt  time.Time `json:"fetched_at"`
	Tier       string    `json:"tier,omitempty"`
	Confidence string    `json:"confidence,omitempty"`
	Total      int       `json:"total,omitempty"`
	Score      float64   `json:"score,omitempty"`
}

// LocalEntry is the result of scanning one installed game's folder.
type LocalEntry struct {
	FetchedAt time.Time  `json:"fetched_at"`
	Hits      []LocalHit `json:"hits,omitempty"`
	Truncated bool       `json:"truncated,omitempty"`
}

// OpenCache loads the cache file at path. A missing file is an empty cache;
// a file that can't be parsed is moved aside and an empty cache returned with
// the error, so a bad file never blocks the launcher.
func OpenCache(path string) (*Cache, error) {
	c := &Cache{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c.data); err != nil {
		bad := fmt.Sprintf("%s.bad-%d", path, time.Now().Unix())
		_ = os.Rename(path, bad)
		c.data = cacheFile{}
		return c, fmt.Errorf("compat cache is corrupt, moved to %s: %w", bad, err)
	}
	return c, nil
}

// ProtonDB returns the saved ProtonDB answer for an AppID.
func (c *Cache) ProtonDB(appID string) (ProtonEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data.ProtonDB[appID]
	return e, ok
}

// SetProtonDB saves a ProtonDB answer and writes the file.
func (c *Cache) SetProtonDB(appID string, e ProtonEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data.ProtonDB == nil {
		c.data.ProtonDB = map[string]ProtonEntry{}
	}
	c.data.ProtonDB[appID] = e
	return c.saveLocked()
}

// Local returns the saved scan of an install folder.
func (c *Cache) Local(dir string) (LocalEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data.Local[dir]
	return e, ok
}

// SetLocal saves the scan of an install folder and writes the file.
func (c *Cache) SetLocal(dir string, e LocalEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data.Local == nil {
		c.data.Local = map[string]LocalEntry{}
	}
	c.data.Local[dir] = e
	return c.saveLocked()
}

// saveLocked writes the file atomically: a temp file renamed over the old one.
// The caller holds c.mu. A cache with no path (tests) is only in memory.
func (c *Cache) saveLocked() error {
	if c.path == "" {
		return nil
	}
	c.data.Version = 1
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
