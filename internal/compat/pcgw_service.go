package compat

import (
	"context"
	"errors"
	"time"
)

// How long a PCGamingWiki answer is trusted. The wiki asks for data to be
// kept on the client side for a while, so a page is kept a month; a miss is
// asked again in a week.
const (
	WikiHitTTL  = 30 * 24 * time.Hour
	WikiMissTTL = 7 * 24 * time.Hour
)

// WikiAnswer is a PCGamingWiki answer as the service holds it.
type WikiAnswer struct {
	// Found is false when the wiki has no page for the game.
	Found     bool
	Record    WikiRecord
	FetchedAt time.Time
	// Stale is set when the answer is past its TTL: a refresh is running or
	// the network is off.
	Stale bool
}

// WikiPage returns the PCGamingWiki answer for a Steam AppID, with the same
// rules as ProtonDB: a saved answer is served at once (refreshed in the
// background when stale), otherwise it waits up to wikiFirstWait. ok is false
// when there is no answer at all.
func (s *Service) WikiPage(ctx context.Context, appID string) (WikiAnswer, bool) {
	if !validAppID(appID) || s.Wiki == nil {
		return WikiAnswer{}, false
	}
	if a, ok := s.cachedWiki(appID); ok {
		return a, true
	}
	if !s.online() || s.cooling(wikiKey(appID)) {
		return WikiAnswer{}, false
	}
	done := s.start(wikiKey(appID), func(ctx context.Context) error { return s.refreshWiki(ctx, appID) })
	select {
	case <-done:
	case <-time.After(wikiFirstWait):
	case <-ctx.Done():
	}
	return s.cachedWiki(appID)
}

func wikiKey(appID string) string { return "pcgw:" + appID }

// cachedWiki is the saved answer of an AppID. A stale one is served at once
// and refreshed in the background when the network is on.
func (s *Service) cachedWiki(appID string) (WikiAnswer, bool) {
	if s.Cache == nil {
		return WikiAnswer{}, false
	}
	e, have := s.Cache.Wiki(appID)
	if !have {
		return WikiAnswer{}, false
	}
	stale := s.now().Sub(e.FetchedAt) >= wikiTTL(e)
	if stale && s.online() && !s.cooling(wikiKey(appID)) {
		s.start(wikiKey(appID), func(ctx context.Context) error { return s.refreshWiki(ctx, appID) })
	}
	return WikiAnswer{Found: e.Found, Record: e.Record, FetchedAt: e.FetchedAt, Stale: stale || !s.online()}, true
}

// refreshWiki asks PCGamingWiki and saves the answer. "No page" is an answer;
// any other failure is remembered for failBackoff.
func (s *Service) refreshWiki(ctx context.Context, appID string) error {
	rec, err := s.Wiki.Lookup(ctx, appID)
	now := s.now()
	switch {
	case errors.Is(err, ErrNotFound):
		return s.Cache.SetWiki(appID, WikiEntry{FetchedAt: now})
	case err != nil:
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			s.setFailed(wikiKey(appID))
			s.warn("PCGamingWiki for %s: %v", appID, err)
		}
		return err
	}
	return s.Cache.SetWiki(appID, WikiEntry{Found: true, FetchedAt: now, Record: rec})
}

func wikiTTL(e WikiEntry) time.Duration {
	if e.Found {
		return WikiHitTTL
	}
	return WikiMissTTL
}
