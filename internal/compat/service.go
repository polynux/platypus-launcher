// Package compat is the launcher's compatibility and protection metadata: the
// ProtonDB rating of a Steam AppID, the anti-cheat catalogue of
// AreWeAntiCheatYet, and the anti-cheat files found in an install. Every
// source is cached on disk and refreshed politely; a source that is down or
// switched off leaves the cached answer in place, with its age, and never
// blocks the caller for long.
package compat

import (
	"context"
	"errors"
	"sync"
	"time"
)

// How long an answer is trusted before it is refreshed. A "no reports" answer
// is asked again sooner: new reports come in all the time.
const (
	ProtonHitTTL  = 7 * 24 * time.Hour
	ProtonMissTTL = 24 * time.Hour
	LocalTTL      = 7 * 24 * time.Hour
	// failBackoff is how long a source that failed is left alone.
	failBackoff = 10 * time.Minute
	// firstWait bounds the wait for an answer when nothing is cached yet.
	firstWait = 6 * time.Second
	// refreshWait bounds a background refresh.
	refreshWait = 30 * time.Second
)

// Service caches and serves the compat data. The fields are set by the
// launcher; tests replace the sources and the clock.
type Service struct {
	Cache     *Cache
	Ratings   RatingSource
	AntiCheat *AntiCheatStore
	// Online says whether network fetches are allowed (the setting). Read on
	// every call, so switching it off takes effect at once.
	Online func() bool
	Now    func() time.Time
	// Scan looks for markers in an install folder (ScanInstall unless a test
	// replaces it).
	Scan func(root string) ([]LocalHit, bool)

	mu         sync.Mutex
	refreshing map[string]bool      // a background refresh is running
	failed     map[string]time.Time // last failed fetch, per source key
}

// Rating is a ProtonDB answer as the service holds it.
type Rating struct {
	// Found is false when ProtonDB confirmed it has no reports.
	Found     bool
	Summary   Summary
	FetchedAt time.Time
	// Stale is set when the answer is past its TTL: a refresh is running or
	// the network is off.
	Stale bool
}

// ProtonDB returns the rating for a Steam AppID. A cached answer is returned
// at once, even when it is stale (then a refresh starts in the background).
// With nothing cached it waits up to firstWait for ProtonDB. ok is false when
// there is no answer at all.
func (s *Service) ProtonDB(ctx context.Context, appID string) (Rating, bool) {
	if !validAppID(appID) {
		return Rating{}, false
	}
	e, have := s.Cache.ProtonDB(appID)
	if have {
		stale := s.now().Sub(e.FetchedAt) >= protonTTL(e)
		if stale && s.online() && s.begin("protondb:"+appID) {
			go func() {
				defer s.end("protondb:" + appID)
				ctx, cancel := context.WithTimeout(context.Background(), refreshWait)
				defer cancel()
				_ = s.refreshProton(ctx, appID)
			}()
		}
		return e.rating(stale || !s.online()), true
	}
	if !s.online() || s.cooling("protondb:"+appID) {
		return Rating{}, false
	}
	ctx, cancel := context.WithTimeout(ctx, firstWait)
	defer cancel()
	if err := s.refreshProton(ctx, appID); err != nil {
		return Rating{}, false
	}
	e, have = s.Cache.ProtonDB(appID)
	return e.rating(false), have
}

// refreshProton asks ProtonDB and saves the answer. "No reports" is an
// answer; any other failure is remembered for failBackoff.
func (s *Service) refreshProton(ctx context.Context, appID string) error {
	sum, err := s.Ratings.Summary(ctx, appID)
	now := s.now()
	switch {
	case errors.Is(err, ErrNotFound):
		return s.Cache.SetProtonDB(appID, ProtonEntry{FetchedAt: now})
	case err != nil:
		s.setFailed("protondb:" + appID)
		return err
	}
	return s.Cache.SetProtonDB(appID, ProtonEntry{
		Found: true, FetchedAt: now, Tier: sum.Tier, Confidence: sum.Confidence, Total: sum.Total, Score: sum.Score,
	})
}

// Local returns the marker scan of an install folder, from the cache when it
// is recent. The scan is local: it never needs the network.
func (s *Service) Local(root string) LocalEntry {
	if root == "" {
		return LocalEntry{}
	}
	if e, ok := s.Cache.Local(root); ok && s.now().Sub(e.FetchedAt) < LocalTTL {
		return e
	}
	scan := s.Scan
	if scan == nil {
		scan = ScanInstall
	}
	hits, truncated := scan(root)
	e := LocalEntry{FetchedAt: s.now(), Hits: hits, Truncated: truncated}
	_ = s.Cache.SetLocal(root, e) // a cache that can't be written is only slower
	return e
}

// AntiCheatCatalogue returns the AreWeAntiCheatYet catalogue, or nil when none
// was ever fetched. A catalogue older than AWACYTTL is refreshed: the first
// time within firstWait, later ones in the background.
func (s *Service) AntiCheatCatalogue(ctx context.Context) *AntiCheatData {
	d := s.AntiCheat.Load()
	if !s.AntiCheat.Stale(d) || !s.online() || s.cooling("awacy") {
		return d
	}
	if d != nil {
		if s.begin("awacy") {
			go func() {
				defer s.end("awacy")
				ctx, cancel := context.WithTimeout(context.Background(), refreshWait)
				defer cancel()
				s.refreshAntiCheat(ctx)
			}()
		}
		return d
	}
	ctx, cancel := context.WithTimeout(ctx, firstWait)
	defer cancel()
	if !s.refreshAntiCheat(ctx) {
		return nil
	}
	return s.AntiCheat.Load()
}

func (s *Service) refreshAntiCheat(ctx context.Context) bool {
	if _, err := s.AntiCheat.Refresh(ctx); err != nil {
		s.setFailed("awacy")
		return false
	}
	return true
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) online() bool { return s.Online == nil || s.Online() }

// begin claims a background refresh; false when one is already running.
func (s *Service) begin(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refreshing == nil {
		s.refreshing = map[string]bool{}
	}
	if s.refreshing[key] {
		return false
	}
	s.refreshing[key] = true
	return true
}

func (s *Service) end(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.refreshing, key)
}

func (s *Service) setFailed(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failed == nil {
		s.failed = map[string]time.Time{}
	}
	s.failed[key] = s.now()
}

// cooling says a source failed recently and is not asked again yet.
func (s *Service) cooling(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	at, ok := s.failed[key]
	return ok && s.now().Sub(at) < failBackoff
}

func protonTTL(e ProtonEntry) time.Duration {
	if e.Found {
		return ProtonHitTTL
	}
	return ProtonMissTTL
}

// rating is the Rating of a saved entry.
func (e ProtonEntry) rating(stale bool) Rating {
	return Rating{Found: e.Found, Summary: Summary{Tier: e.Tier, Confidence: e.Confidence, Total: e.Total, Score: e.Score}, FetchedAt: e.FetchedAt, Stale: stale}
}
