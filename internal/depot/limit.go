// Download speed cap: a token bucket the scheduler feeds each landed chunk
// through, so the whole install (every depot, every chunk) holds the user's
// rate. A nil Limiter is unlimited.
package depot

import (
	"context"
	"math"
	"sync"
	"time"
)

// Limiter caps downloaded bytes per second (issue #207). The bucket holds at
// most one second of the cap, so a run starts at full speed for a moment and
// then settles on the rate; a single chunk larger than the cap still lands,
// it just waits for its bytes first.
type Limiter struct {
	rate   float64 // bytes per second
	burst  float64 // tokens saved up, one second of rate
	tokens float64
	last   time.Time
	// now and sleep are the clock, swapped by tests; WaitN only ever waits
	// through sleep.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error

	mu sync.Mutex
}

// NewLimiter caps a run at bytesPerSec; 0 or less means unlimited, which is
// a nil *Limiter (WaitN on nil returns at once).
func NewLimiter(bytesPerSec int64) *Limiter {
	if bytesPerSec <= 0 {
		return nil
	}
	rate := float64(bytesPerSec)
	now := time.Now
	return &Limiter{rate: rate, burst: rate, tokens: rate, last: now(), now: now, sleep: sleepCtx}
}

// WaitN blocks until n more bytes fit under the cap, or ctx ends.
func (l *Limiter) WaitN(ctx context.Context, n int) error {
	if l == nil || n <= 0 {
		return nil
	}
	need := float64(n)
	for {
		l.mu.Lock()
		now := l.now()
		// A chunk bigger than the bucket may still accumulate enough; the
		// average after it is back on the rate.
		limit := math.Max(l.burst, need)
		l.tokens = math.Min(limit, l.tokens+now.Sub(l.last).Seconds()*l.rate)
		l.last = now
		if l.tokens >= need {
			l.tokens -= need
			l.mu.Unlock()
			return nil
		}
		wait := time.Duration((need - l.tokens) / l.rate * float64(time.Second))
		l.mu.Unlock()
		if wait < time.Millisecond {
			wait = time.Millisecond // float rounding must not spin
		}
		if err := l.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
