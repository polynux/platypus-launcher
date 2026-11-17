package depot

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeClock is a Limiter clock the test advances through sleeps: nothing
// really waits, and the elapsed time is the one the cap allowed. The
// scheduler test calls WaitN from several workers, hence the lock.
type fakeClock struct {
	mu    sync.Mutex
	at    time.Time
	slept []time.Duration
}

func (c *fakeClock) limiter(rate int64) *Limiter {
	c.at = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(rate)
	l.now = func() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.at }
	l.last = c.at
	l.sleep = func(_ context.Context, d time.Duration) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.at = c.at.Add(d)
		c.slept = append(c.slept, d)
		return nil
	}
	return l
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *fakeClock) sleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration{}, c.slept...)
}

func TestLimiterHoldsTheRate(t *testing.T) {
	c := &fakeClock{}
	l := c.limiter(1000)
	start := c.now()
	// 5 x 600 bytes at 1000 B/s: the 1s burst covers the first 1000 bytes,
	// the rest takes 2s.
	for range 5 {
		if err := l.WaitN(context.Background(), 600); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := c.now().Sub(start), 2*time.Second; got != want {
		t.Fatalf("5 chunks took %v, want %v", got, want)
	}
}

// A chunk larger than the bucket still lands: it is granted at once and its
// debt is waited out, so it isn't starved behind the small chunks before it.
func TestLimiterMixedChunkSizes(t *testing.T) {
	c := &fakeClock{}
	l := c.limiter(1000)
	start := c.now()
	for _, n := range []int{100, 4000, 100} {
		if err := l.WaitN(context.Background(), n); err != nil {
			t.Fatal(err)
		}
	}
	// 4200 bytes minus the 1s burst (1000) at 1000 B/s: 3.2s in all. The
	// big chunk pays 3.1s (its own bytes), not 4s for the small one too.
	if got, want := c.now().Sub(start), 3200*time.Millisecond; got != want {
		t.Fatalf("mixed chunks took %v, want %v", got, want)
	}
	if got, want := c.sleeps(), []time.Duration{3100 * time.Millisecond, 100 * time.Millisecond}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("sleeps = %v, want %v", got, want)
	}
}

func TestLimiterUnlimited(t *testing.T) {
	if l := NewLimiter(0); l != nil {
		t.Fatalf("zero rate = %+v, want nil (unlimited)", l)
	}
	var l *Limiter
	if err := l.WaitN(context.Background(), 1<<20); err != nil {
		t.Fatalf("nil limiter waited: %v", err)
	}
}

func TestLimiterStopsWithContext(t *testing.T) {
	l := NewLimiter(1) // 1 B/s: a 1 KiB chunk waits ~17 minutes
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.WaitN(ctx, 1<<10); !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitN = %v, want context.Canceled", err)
	}
}

// DownloadStream feeds every chunk through the limiter before fetching it:
// the run's time is the payload over the rate, not the fake hosts' speed.
func TestSchedulerDownloadIsCapped(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 2, 3)
	hosts, _ := startHosts(t, m, encMap, []time.Duration{0, 0}, -1, -1, "")
	var payload uint64
	for _, c := range orderedChunks(m) {
		payload += uint64(c.CompressedLength)
	}
	c := &fakeClock{}
	lim := c.limiter(64)
	s := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 4, MaxConns: 4, Limiter: lim})
	start := c.now()
	var landed uint64
	sink := func(_ string, data []byte) error { landed += uint64(len(data)); return nil }
	if err := s.DownloadStream(context.Background(), m, 11, hosts, key, nil, sink, nil); err != nil {
		t.Fatal(err)
	}
	var wantLanded uint64
	for _, c := range orderedChunks(m) {
		wantLanded += uint64(c.UncompressedLength)
	}
	if landed != wantLanded {
		t.Fatalf("landed %d bytes, want %d", landed, wantLanded)
	}
	// One second's burst is free; the rest is payload over the rate.
	want := time.Duration(float64(payload-uint64(lim.burst)) / lim.rate * float64(time.Second))
	if got := c.now().Sub(start); got < want-time.Second || got > want+time.Second {
		t.Fatalf("capped run took %v, want about %v", got, want)
	}
}
