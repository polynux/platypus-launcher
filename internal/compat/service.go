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
	// wikiFirstWait bounds the wait for a PCGamingWiki answer: a lookup is two
	// requests, one a second.
	wikiFirstWait = 12 * time.Second
	// refreshWait bounds a background refresh.
	refreshWait = 30 * time.Second
)

// Service caches and serves the compat data. The fields are set by the
// launcher; tests replace the sources and the clock.
type Service struct {
	Cache     *Cache
	Ratings   RatingSource
	Deck      DeckSource
	AntiCheat *AntiCheatStore
	// Wiki is the PCGamingWiki source (see pcgw_service.go).
	Wiki WikiSource
	// Online says whether network fetches are allowed (the setting). Read on
	// every call, so switching it off takes effect at once.
	Online func() bool
	Now    func() time.Time
	// Scan looks for markers in an install folder (ScanInstall unless a test
	// replaces it).
	Scan func(root string) ([]LocalHit, bool)
	// Warn reports a source that failed to answer. Optional.
	Warn func(format string, args ...any)
	// FirstWait bounds how long a look at a source waits for its first
	// answer; zero means firstWait.
	FirstWait time.Duration

	mu         sync.Mutex
	refreshing map[string]chan struct{} // a refresh is running; closed when it ends
	failed     map[string]time.Time     // last failed fetch, per source key
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
	key := "protondb:" + appID
	if r, ok := s.cachedProton(appID); ok {
		return r, true
	}
	if !s.online() || s.cooling(key) {
		return Rating{}, false
	}
	// Nothing cached: the fetch runs in the background (it may queue behind
	// others), and the caller waits for it only up to firstWait. A fetch that
	// is still running fills the cache for the next look.
	done := s.start(key, func(ctx context.Context) error { return s.refreshProton(ctx, appID) })
	select {
	case <-done:
	case <-time.After(s.firstWait()):
	case <-ctx.Done():
	}
	if r, ok := s.cachedProton(appID); ok {
		return r, true
	}
	return Rating{}, false
}

// ProtonDBCached is ProtonDB without the wait: the cached rating when there is
// one (a stale one is refreshed in the background), otherwise nothing, with a
// background fetch started for the next look. The library lists every game
// with it, so it never blocks.
func (s *Service) ProtonDBCached(appID string) (Rating, bool) {
	if !validAppID(appID) {
		return Rating{}, false
	}
	if r, ok := s.cachedProton(appID); ok {
		return r, true
	}
	key := "protondb:" + appID
	if s.online() && !s.cooling(key) {
		s.start(key, func(ctx context.Context) error { return s.refreshProton(ctx, appID) })
	}
	return Rating{}, false
}

// cachedProton is the saved rating of an AppID. A stale one is served at once
// and refreshed in the background when the network is on.
func (s *Service) cachedProton(appID string) (Rating, bool) {
	key := "protondb:" + appID
	e, have := s.Cache.ProtonDB(appID)
	if !have {
		return Rating{}, false
	}
	stale := s.now().Sub(e.FetchedAt) >= protonTTL(e)
	if stale && s.online() && !s.cooling(key) {
		s.start(key, func(ctx context.Context) error { return s.refreshProton(ctx, appID) })
	}
	return e.rating(stale || !s.online()), true
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
		// A wait that ran out is our own deadline, not the source failing.
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			s.setFailed("protondb:" + appID)
			s.warn("ProtonDB for %s: %v", appID, err)
		}
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
	done := s.start("awacy", func(ctx context.Context) error { return s.refreshAntiCheat(ctx) })
	if d != nil {
		return d
	}
	select {
	case <-done:
	case <-time.After(s.firstWait()):
	case <-ctx.Done():
	}
	return s.AntiCheat.Load()
}

func (s *Service) refreshAntiCheat(ctx context.Context) error {
	if _, err := s.AntiCheat.Refresh(ctx); err != nil {
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			s.setFailed("awacy")
			s.warn("AreWeAntiCheatYet: %v", err)
		}
		return err
	}
	return nil
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) firstWait() time.Duration {
	if s.FirstWait > 0 {
		return s.FirstWait
	}
	return firstWait
}

func (s *Service) warn(format string, args ...any) {
	if s.Warn != nil {
		s.Warn(format, args...)
	}
}

func (s *Service) online() bool { return s.Online == nil || s.Online() }

// start runs a refresh of key in the background, unless one is already
// running. The returned channel is closed when the refresh ends.
func (s *Service) start(key string, run func(ctx context.Context) error) <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refreshing == nil {
		s.refreshing = map[string]chan struct{}{}
	}
	if ch, ok := s.refreshing[key]; ok {
		return ch
	}
	ch := make(chan struct{})
	s.refreshing[key] = ch
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), refreshWait)
		defer cancel()
		_ = run(ctx)
		s.mu.Lock()
		delete(s.refreshing, key)
		s.mu.Unlock()
		close(ch)
	}()
	return ch
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

// DeckResult is a Steam Deck answer as the service holds it.
type DeckResult struct {
	// Found is false when Steam confirmed it has no verdict for the app.
	Found     bool
	Report    DeckReport
	FetchedAt time.Time
	// Stale is set when the answer is past its TTL: a refresh is running or
	// the network is off.
	Stale bool
}

// DeckRating returns the Deck verdict for a Steam AppID, with the same rules
// as ProtonDB: a saved answer is served at once (refreshed in the background
// when stale), otherwise it waits up to firstWait. ok is false when there is
// no answer at all.
func (s *Service) DeckRating(ctx context.Context, appID string) (DeckResult, bool) {
	if !validAppID(appID) || s.Deck == nil {
		return DeckResult{}, false
	}
	if r, ok := s.cachedDeck(appID); ok {
		return r, true
	}
	key := deckKey(appID)
	if !s.online() || s.cooling(key) {
		return DeckResult{}, false
	}
	done := s.start(key, func(ctx context.Context) error { return s.refreshDeck(ctx, appID) })
	select {
	case <-done:
	case <-time.After(s.firstWait()):
	case <-ctx.Done():
	}
	if r, ok := s.cachedDeck(appID); ok {
		return r, true
	}
	return DeckResult{}, false
}

// DeckRatingCached is DeckRating without the wait: the cached verdict when
// there is one (a stale one is refreshed in the background), otherwise
// nothing, with a background fetch started for the next look.
func (s *Service) DeckRatingCached(appID string) (DeckResult, bool) {
	if !validAppID(appID) || s.Deck == nil {
		return DeckResult{}, false
	}
	if r, ok := s.cachedDeck(appID); ok {
		return r, true
	}
	key := deckKey(appID)
	if s.online() && !s.cooling(key) {
		s.start(key, func(ctx context.Context) error { return s.refreshDeck(ctx, appID) })
	}
	return DeckResult{}, false
}

func deckKey(appID string) string { return "deck:" + appID }

// cachedDeck is the saved Deck verdict of an AppID. A stale one is served at
// once and refreshed in the background when the network is on.
func (s *Service) cachedDeck(appID string) (DeckResult, bool) {
	if s.Cache == nil {
		return DeckResult{}, false
	}
	e, have := s.Cache.Deck(appID)
	if !have {
		return DeckResult{}, false
	}
	key := deckKey(appID)
	stale := s.now().Sub(e.FetchedAt) >= deckTTL(e)
	if stale && s.online() && !s.cooling(key) {
		s.start(key, func(ctx context.Context) error { return s.refreshDeck(ctx, appID) })
	}
	return e.result(stale || !s.online()), true
}

// refreshDeck asks Steam and saves the answer. "No verdict" is an answer;
// any other failure is remembered for failBackoff.
func (s *Service) refreshDeck(ctx context.Context, appID string) error {
	rep, err := s.Deck.Report(ctx, appID)
	now := s.now()
	switch {
	case errors.Is(err, ErrNotFound):
		return s.Cache.SetDeck(appID, DeckEntry{FetchedAt: now})
	case err != nil:
		// A wait that ran out is our own deadline, not the source failing.
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			s.setFailed(deckKey(appID))
			s.warn("Steam Deck for %s: %v", appID, err)
		}
		return err
	}
	return s.Cache.SetDeck(appID, DeckEntry{
		Found: true, FetchedAt: now, Rating: rep.Rating, Notes: rep.Notes,
	})
}

func deckTTL(e DeckEntry) time.Duration {
	if e.Found {
		return ProtonHitTTL
	}
	return ProtonMissTTL
}

// result is the DeckResult of a saved entry.
func (e DeckEntry) result(stale bool) DeckResult {
	return DeckResult{Found: e.Found, Report: DeckReport{Rating: e.Rating, Notes: e.Notes}, FetchedAt: e.FetchedAt, Stale: stale}
}
