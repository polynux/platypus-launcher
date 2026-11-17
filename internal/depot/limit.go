// Download speed cap: a token bucket the scheduler feeds each chunk through
// before it asks the CDN, so the whole install (every depot, every chunk)
// holds the user's rate. A nil Limiter is unlimited.
package depot

import (
	"context"
	"math"
	"sync"
	"time"
)

// Limiter caps downloaded bytes per second (issue #207). It is a debt
// bucket: a chunk's bytes are subtracted at once, so one big chunk isn't
// starved behind small ones; the wait is the debt over the rate. The bucket
// banks at most one second of the cap, so a run starts at full speed for a
// moment and then settles on the rate.
type Limiter struct {
	rate   float64 // bytes per second
	burst  float64 // tokens saved up, one second of rate
	tokens float64 // may go negative: bytes already granted, not yet waited out
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

// WaitN blocks until n more bytes fit under the cap, or ctx ends. On ctx
// cancellation the bytes stay charged: a resume still pays for them.
func (l *Limiter) WaitN(ctx context.Context, n int) error {
	if l == nil || n <= 0 {
		return nil
	}
	l.mu.Lock()
	now := l.now()
	l.tokens = math.Min(l.burst, l.tokens+now.Sub(l.last).Seconds()*l.rate)
	l.last = now
	l.tokens -= float64(n)
	debt := -l.tokens
	l.mu.Unlock()
	if debt <= 0 {
		return nil
	}
	wait := time.Duration(debt / l.rate * float64(time.Second))
	if wait < time.Millisecond {
		wait = time.Millisecond // float rounding must not spin
	}
	return l.sleep(ctx, wait)
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
