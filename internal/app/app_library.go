package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"platypus/internal/stores"
	"platypus/internal/types"
)

// Library rescan cache (issue #27): a full GetLibrary lists every source,
// which costs seconds on a real library (each store plugin spawns its CLI).
// GetLibrary therefore keeps one listing per source and re-lists only the
// sources without a fresh entry. Invalidation is explicit:
//
//   - a finished job drops its source (jobFinished runs before the job reads
//     as done, so the rescan the jobs event triggers can't serve the old one),
//   - a plugin change drops its plugin (approve, enable, retry, login,
//     logout, install folders),
//   - RescanPlugins and a manual rescan drop everything,
//   - playtime reports invalidate nothing: the cached listing is merged with
//     the new playtime instead of being re-listed.
//
// libraryCacheTTL is the safety net: a source whose change notification was
// missed is re-listed at the latest this long after its last listing.
const libraryCacheTTL = 30 * time.Second

// libraryCacheEntry is one source's last listing, failure included: a source
// that failed (logged out, store down) isn't retried on every read. lastGood
// is the last successful listing (issue #44): when a source fails, its games
// come from lastGood or the on-disk snapshot, with a stale warning, so the
// library survives an offline restart.
type libraryCacheEntry struct {
	games []types.Game
	err   error
	at    time.Time
	// lastGood is the last successful listing of this source.
	lastGood []types.Game
}

// libraryCache is the per-source library cache. The zero value is ready to
// use; a test may set ttl to shrink the safety net.
type libraryCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]libraryCacheEntry
	// snapDir is where per-source snapshots live
	// (<snapDir>/<source>.json), written after each success. Empty means
	// memory only (tests).
	snapDir string
}

func (c *libraryCache) age() time.Duration {
	if c.ttl > 0 {
		return c.ttl
	}
	return libraryCacheTTL
}

// fresh returns the source's listing when it is younger than the TTL.
func (c *libraryCache) fresh(source string) ([]types.Game, error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[source]
	if !ok || time.Since(e.at) > c.age() {
		return nil, nil, false
	}
	return e.games, e.err, true
}

func (c *libraryCache) set(source string, games []types.Game, err error) {
	c.mu.Lock()
	e := c.entries[source]
	if err == nil {
		e.lastGood = games
	} else if e.lastGood == nil {
		// No success yet in this run: the on-disk snapshot (an earlier
		// run's last good listing) stands in, so the library survives an
		// offline restart.
		e.lastGood = loadLibrarySnapshot(c.snapDir, source)
	}
	e.games, e.err, e.at = games, err, time.Now()
	if c.entries == nil {
		c.entries = map[string]libraryCacheEntry{}
	}
	c.entries[source] = e
	c.mu.Unlock()
	if err == nil {
		saveLibrarySnapshot(c.snapDir, source, games)
	}
}

// lastGood returns the source's last successful listing: the in-memory one,
// else the on-disk snapshot. ok is false when there is none.
func (c *libraryCache) lastGood(source string) ([]types.Game, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[source]; ok && len(e.lastGood) > 0 {
		return e.lastGood, true
	}
	if games := loadLibrarySnapshot(c.snapDir, source); len(games) > 0 {
		return games, true
	}
	return nil, false
}

func (c *libraryCache) invalidate(source string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, source)
}

func (c *libraryCache) invalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]libraryCacheEntry{}
}

// prune drops the entries of sources that are gone (a disabled or removed
// plugin), so a re-enabled one is listed again even within the TTL.
func (c *libraryCache) prune(all []stores.Store) {
	c.mu.Lock()
	defer c.mu.Unlock()
	keep := map[string]bool{}
	for _, s := range all {
		keep[s.ID()] = true
	}
	for id := range c.entries {
		if !keep[id] {
			delete(c.entries, id)
		}
	}
}

// invalidateLibrary drops one source's cached listing, so the next GetLibrary
// lists it again. Call it wherever that source's games may have changed.
func (a *App) invalidateLibrary(source string) {
	a.library.invalidate(source)
}

// InvalidateLibrary drops one source's cached listing, so the next GetLibrary
// lists it again. The store view calls it when it closes (issue #17): what
// was bought in the embedded store appears in the library.
func (a *App) InvalidateLibrary(source string) {
	a.invalidateLibrary(source)
}

// invalidateLibraries drops every cached listing: the plugin set itself may
// have changed, so no entry can be trusted.
func (a *App) invalidateLibraries() {
	a.library.invalidateAll()
}

// GetLibrary lists every source without a fresh cached listing and returns
// the merged library. Steam failing fails the scan, as before; a plugin
// failing (logged out, crashed) only adds a warning, so one broken plugin
// can't hide the rest of the library.
//
// Sources answer at their own pace (Steam reads local files, a store plugin
// may wait on its store): each time one finishes while others are still out,
// the library of those finished is sent as a "library-partial" event, so the
// window can show it before the slowest plugin answers.
func (a *App) GetLibrary() (types.Library, error) {
	return a.getLibrary(false)
}

// RescanLibrary drops the whole library cache and lists every source again:
// the manual "Rescan". GetLibrary serves fresh entries from the cache.
func (a *App) RescanLibrary() (types.Library, error) {
	return a.getLibrary(true)
}

func (a *App) getLibrary(force bool) (types.Library, error) {
	// Concurrent scans share one refresh: the second serves the first one's
	// fresh cache instead of listing the same slow stores twice.
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	if force {
		a.library.invalidateAll()
	}
	all := a.stores.All()
	a.library.prune(all)
	results := make([][]types.Game, len(all))
	errs := make([]error, len(all))
	done := make([]bool, len(all))
	var stale []int
	for i, s := range all {
		if games, err, ok := a.library.fresh(s.ID()); ok {
			results[i], errs[i], done[i] = games, err, true
			continue
		}
		stale = append(stale, i)
	}
	if len(stale) == 0 {
		lib, err := a.assembleLibrary(all, results, errs, done)
		if err != nil {
			return types.Library{}, err
		}
		a.forgetRemovedGameSettings(all, results, errs)
		return lib, nil
	}
	finished := make(chan int, len(stale))
	for _, i := range stale {
		go func() {
			ctx, cancel := context.WithTimeout(a.ctx, libraryTimeout)
			defer cancel()
			games, err := all[i].Library(ctx)
			a.library.set(all[i].ID(), games, err)
			if err != nil && len(games) == 0 {
				// The source failed: serve its last good listing (or the
				// on-disk snapshot) with a stale warning, so one broken
				// source — or an offline restart — can't hide the library.
				if good, ok := a.library.lastGood(all[i].ID()); ok {
					games = good
				}
			}
			results[i], errs[i] = games, err
			finished <- i
		}()
	}
	for left := len(stale); left > 0; left-- {
		i := <-finished // the channel hands over the writes of results[i] and errs[i]
		done[i] = true
		if left > 1 && a.libraryEmit != nil {
			if lib, err := a.assembleLibrary(all, results, errs, done); err == nil && len(lib.Games) > 0 {
				a.libraryEmit(lib)
			}
		}
	}
	lib, err := a.assembleLibrary(all, results, errs, done)
	if err != nil {
		return types.Library{}, err
	}
	a.forgetRemovedGameSettings(all, results, errs)
	return lib, nil
}

// sourceLibrary lists one source, serving its fresh cache entry when it has
// one. The versions gate and move planning read through it instead of asking
// the plugin again, so opening the versions dialog doesn't re-list the store.
func (a *App) sourceLibrary(source string) ([]types.Game, error) {
	if games, err, ok := a.library.fresh(source); ok {
		return games, err
	}
	s, ok := a.stores.Get(source)
	if !ok {
		return nil, fmt.Errorf("unknown source %q", source)
	}
	ctx, cancel := context.WithTimeout(a.ctx, libraryTimeout)
	defer cancel()
	games, err := s.Library(ctx)
	a.library.set(source, games, err)
	return games, err
}
