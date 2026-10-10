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

// fakeWiki source: answers from a table, counts the asks.
type fakeWikiSource struct {
	mu    sync.Mutex
	pages map[string]WikiRecord
	err   error
	calls atomic.Int32
}

func (f *fakeWikiSource) Lookup(ctx context.Context, appID string) (WikiRecord, error) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return WikiRecord{}, f.err
	}
	rec, ok := f.pages[appID]
	if !ok {
		return WikiRecord{}, ErrNotFound
	}
	return rec, nil
}

func newWikiService(t *testing.T, online *bool) (*Service, *fakeWikiSource, *clock, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cache.json")
	cache, err := OpenCache(path)
	if err != nil {
		t.Fatal(err)
	}
	ck := &clock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	src := &fakeWikiSource{pages: map[string]WikiRecord{
		"900101": {Title: "Example Horror Game", DRM: []string{"Steam"}, Removed: []RemovedDRM{{Name: "Denuvo", Date: "2023-04-10"}}},
	}}
	s := &Service{Cache: cache, Wiki: src, Online: func() bool { return *online }, Now: ck.now, FirstWait: time.Second}
	return s, src, ck, path
}

// settle waits for the background refresh of key to end.
func settle(t *testing.T, s *Service, key string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		_, running := s.refreshing[key]
		s.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("refresh of %s did not end", key)
}

func TestWikiFirstLookWaitsForTheAnswerThenCaches(t *testing.T) {
	on := true
	s, src, _, _ := newWikiService(t, &on)
	a, ok := s.WikiPage(context.Background(), "900101")
	if !ok || !a.Found || a.Record.Title != "Example Horror Game" || a.Stale {
		t.Fatalf("first answer %+v ok %v", a, ok)
	}
	if _, ok := s.WikiPage(context.Background(), "900101"); !ok || src.calls.Load() != 1 {
		t.Fatalf("second look asked %d times, want 1", src.calls.Load())
	}
}

func TestWikiHitIsKeptAMonthAndMissAWeek(t *testing.T) {
	on := true
	s, src, ck, _ := newWikiService(t, &on)
	ctx := context.Background()
	s.WikiPage(ctx, "900101")
	s.WikiPage(ctx, "42") // no page
	if src.calls.Load() != 2 {
		t.Fatalf("%d asks, want 2", src.calls.Load())
	}

	ck.add(8 * 24 * time.Hour)
	a, _ := s.WikiPage(ctx, "42")
	if a.Found || !a.Stale {
		t.Fatalf("a miss after 8 days is stale: %+v", a)
	}
	settle(t, s, wikiKey("42"))
	if src.calls.Load() != 3 {
		t.Fatalf("%d asks, want the miss refreshed after a week", src.calls.Load())
	}

	ck.add(23 * 24 * time.Hour) // day 31: the hit is past its month
	a, ok := s.WikiPage(ctx, "900101")
	if !ok || !a.Found || !a.Stale {
		t.Fatalf("hit at day 31 served stale: %+v ok %v", a, ok)
	}
	settle(t, s, wikiKey("900101"))
	if src.calls.Load() != 4 {
		t.Fatalf("%d asks, want the hit refreshed in the background", src.calls.Load())
	}
}

func TestWikiOfflineServesTheCacheAndAsksNothing(t *testing.T) {
	on := true
	s, src, ck, _ := newWikiService(t, &on)
	s.WikiPage(context.Background(), "900101")
	on = false
	ck.add(90 * 24 * time.Hour)
	a, ok := s.WikiPage(context.Background(), "900101")
	if !ok || !a.Found || !a.Stale || a.Record.Removed[0].Date != "2023-04-10" {
		t.Fatalf("offline answer %+v ok %v", a, ok)
	}
	if src.calls.Load() != 1 {
		t.Fatalf("%d asks while offline, want the one from before", src.calls.Load())
	}
	if _, ok := s.WikiPage(context.Background(), "42"); ok {
		t.Fatal("offline with nothing saved must give no answer")
	}
}

func TestWikiFailureCoolsDownAndIsNotAskedAgainAtOnce(t *testing.T) {
	on := true
	s, src, ck, _ := newWikiService(t, &on)
	src.err = errors.New("PCGamingWiki answered 429")
	if _, ok := s.WikiPage(context.Background(), "900101"); ok {
		t.Fatal("a failed lookup with nothing saved is no answer")
	}
	s.WikiPage(context.Background(), "900101")
	if src.calls.Load() != 1 {
		t.Fatalf("%d asks during the cool-down, want 1", src.calls.Load())
	}
	ck.add(failBackoff + time.Second)
	src.err = nil
	s.WikiPage(context.Background(), "900101")
	settle(t, s, wikiKey("900101"))
	if src.calls.Load() != 2 {
		t.Fatalf("%d asks after the cool-down, want 2", src.calls.Load())
	}
}

func TestWikiAnswerSurvivesARestart(t *testing.T) {
	on := true
	s, src, _, path := newWikiService(t, &on)
	s.WikiPage(context.Background(), "900101")
	reopened, err := OpenCache(path)
	if err != nil {
		t.Fatal(err)
	}
	s2 := &Service{Cache: reopened, Wiki: src, Online: func() bool { return false }, Now: s.Now}
	a, ok := s2.WikiPage(context.Background(), "900101")
	if !ok || a.Record.Title != "Example Horror Game" || src.calls.Load() != 1 {
		t.Fatalf("after restart %+v ok %v, asks %d", a, ok, src.calls.Load())
	}
}

func TestWikiRejectsNonNumericAppIDWithoutAsking(t *testing.T) {
	on := true
	s, src, _, _ := newWikiService(t, &on)
	if _, ok := s.WikiPage(context.Background(), "Example Puzzle Game"); ok || src.calls.Load() != 0 {
		t.Fatalf("title used as AppID: ok %v asks %d", ok, src.calls.Load())
	}
}
