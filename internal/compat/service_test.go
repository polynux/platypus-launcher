package compat

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRatings answers ProtonDB from a table and counts the asks.
type fakeRatings struct {
	mu    sync.Mutex
	sums  map[string]Summary
	err   error
	gate  chan struct{} // when set, Summary waits for it to close
	calls atomic.Int32
}

func (f *fakeRatings) Summary(ctx context.Context, appID string) (Summary, error) {
	f.calls.Add(1)
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return Summary{}, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return Summary{}, f.err
	}
	s, ok := f.sums[appID]
	if !ok {
		return Summary{}, ErrNotFound
	}
	return s, nil
}

// clock is a settable time for the service.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func newTestService(t *testing.T, online *bool) (*Service, *fakeRatings, *clock, string) {
	t.Helper()
	dir := t.TempDir()
	cache, err := OpenCache(filepath.Join(dir, "cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	ck := &clock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	rt := &fakeRatings{sums: map[string]Summary{
		"620": {Tier: "platinum", Confidence: "strong", Total: 343, Score: 0.88},
	}}
	s := &Service{
		Cache:   cache,
		Ratings: rt,
		Online:  func() bool { return *online },
		Now:     ck.now,
	}
	return s, rt, ck, dir
}

func TestProtonFreshHitIsServedFromCacheWithoutAsking(t *testing.T) {
	on := true
	s, rt, _, _ := newTestService(t, &on)
	r, ok := s.ProtonDB(context.Background(), "620")
	if !ok || r.Summary.Tier != "platinum" || r.Stale {
		t.Fatalf("first answer %+v ok %v", r, ok)
	}
	if rt.calls.Load() != 1 {
		t.Fatalf("%d asks, want 1", rt.calls.Load())
	}
	r, ok = s.ProtonDB(context.Background(), "620")
	if !ok || r.Stale || rt.calls.Load() != 1 {
		t.Fatalf("fresh hit asked again: %+v asks %d", r, rt.calls.Load())
	}
}

func TestProtonStaleHitIsServedAtOnceAndRefreshedInBackground(t *testing.T) {
	on := true
	s, rt, ck, _ := newTestService(t, &on)
	s.ProtonDB(context.Background(), "620")
	ck.add(ProtonHitTTL + time.Hour)
	r, ok := s.ProtonDB(context.Background(), "620")
	if !ok || !r.Stale || r.Summary.Tier != "platinum" {
		t.Fatalf("stale answer %+v ok %v, want the cached one, marked stale", r, ok)
	}
	// The refresh runs in the background: wait for its ask, then the cache is fresh.
	deadline := time.Now().Add(2 * time.Second)
	for rt.calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if rt.calls.Load() != 2 {
		t.Fatalf("%d asks, want the background refresh", rt.calls.Load())
	}
}

func TestProtonOfflineServesCacheWithItsAge(t *testing.T) {
	on := true
	s, rt, ck, _ := newTestService(t, &on)
	s.ProtonDB(context.Background(), "620")
	on = false
	ck.add(30 * 24 * time.Hour)
	r, ok := s.ProtonDB(context.Background(), "620")
	if !ok || !r.Stale || r.FetchedAt.IsZero() {
		t.Fatalf("offline answer %+v ok %v", r, ok)
	}
	if rt.calls.Load() != 1 {
		t.Fatalf("asked the network while offline (%d asks)", rt.calls.Load())
	}
}

func TestProtonNothingCachedAndOfflineGivesNoAnswer(t *testing.T) {
	on := false
	s, rt, _, _ := newTestService(t, &on)
	if _, ok := s.ProtonDB(context.Background(), "620"); ok {
		t.Fatal("an answer with no cache and no network")
	}
	if rt.calls.Load() != 0 {
		t.Fatal("asked the network while offline")
	}
}

func TestProtonNoReportsIsCachedForADay(t *testing.T) {
	on := true
	s, rt, ck, _ := newTestService(t, &on)
	r, ok := s.ProtonDB(context.Background(), "999")
	if !ok || r.Found {
		t.Fatalf("no-report answer %+v ok %v", r, ok)
	}
	ck.add(ProtonMissTTL / 2)
	s.ProtonDB(context.Background(), "999")
	if rt.calls.Load() != 1 {
		t.Fatalf("a miss was asked again within its day (%d asks)", rt.calls.Load())
	}
	ck.add(ProtonMissTTL)
	s.ProtonDB(context.Background(), "999")
	deadline := time.Now().Add(2 * time.Second)
	for rt.calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if rt.calls.Load() != 2 {
		t.Fatalf("a miss older than a day wasn't refreshed (%d asks)", rt.calls.Load())
	}
}

func TestProtonFailureIsNotCachedAndCoolsDown(t *testing.T) {
	on := true
	s, rt, ck, _ := newTestService(t, &on)
	rt.err = errors.New("network down")
	if _, ok := s.ProtonDB(context.Background(), "620"); ok {
		t.Fatal("an answer from a failed fetch")
	}
	if _, ok := s.Cache.ProtonDB("620"); ok {
		t.Fatal("a failure was cached")
	}
	// Within the back-off, no second ask.
	s.ProtonDB(context.Background(), "620")
	if rt.calls.Load() != 1 {
		t.Fatalf("%d asks during the back-off, want 1", rt.calls.Load())
	}
	rt.err = nil
	ck.add(failBackoff + time.Second)
	if r, ok := s.ProtonDB(context.Background(), "620"); !ok || r.Summary.Tier != "platinum" {
		t.Fatalf("after the back-off: %+v ok %v", r, ok)
	}
}

func TestShownTierHidesThinAnswers(t *testing.T) {
	cases := []struct {
		name string
		s    Summary
		want string
	}{
		{"strong gold", Summary{Tier: "gold", Confidence: "strong", Total: 40}, "gold"},
		{"moderate silver", Summary{Tier: "silver", Confidence: "moderate", Total: 5}, "silver"},
		{"few reports", Summary{Tier: "gold", Confidence: "strong", Total: 2}, ""},
		{"low confidence", Summary{Tier: "gold", Confidence: "inadequate", Total: 40}, ""},
		{"pending is not a medal", Summary{Tier: "pending", Confidence: "strong", Total: 40}, ""},
		{"native is not a medal", Summary{Tier: "native", Confidence: "strong", Total: 40}, ""},
	}
	for _, c := range cases {
		if got := ShownTier(c.s); got != c.want {
			t.Errorf("%s: ShownTier = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestLocalScanIsCachedPerInstall(t *testing.T) {
	on := true
	s, _, ck, _ := newTestService(t, &on)
	scans := 0
	s.Scan = func(root string) ([]LocalHit, bool) {
		scans++
		return []LocalHit{{Kind: "anticheat", Label: "BattlEye", Name: "BEClient_x64.dll"}}, false
	}
	e := s.Local("/games/a")
	if len(e.Hits) != 1 || scans != 1 {
		t.Fatalf("first scan %+v scans %d", e, scans)
	}
	s.Local("/games/a")
	if scans != 1 {
		t.Fatal("rescanned inside the TTL")
	}
	ck.add(LocalTTL + time.Hour)
	s.Local("/games/a")
	if scans != 2 {
		t.Fatal("not rescanned after the TTL")
	}
}

func TestSlowFirstFetchKeepsRunningAndIsNotCoolingDown(t *testing.T) {
	on := true
	s, rt, _, _ := newTestService(t, &on)
	s.FirstWait = 20 * time.Millisecond
	rt.gate = make(chan struct{})
	// The caller gives up before the source answers: no answer yet, no failure.
	if _, ok := s.ProtonDB(context.Background(), "620"); ok {
		t.Fatal("an answer before the source responded")
	}
	if s.cooling("protondb:620") {
		t.Fatal("a wait that ran out put the source in cooldown")
	}
	// The fetch is still running; when it lands, the cache has it.
	close(rt.gate)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r, ok := s.ProtonDB(context.Background(), "620"); ok && r.Summary.Tier == "platinum" {
			if rt.calls.Load() != 1 {
				t.Fatalf("%d asks, want the one in-flight fetch", rt.calls.Load())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the slow fetch never filled the cache")
}
