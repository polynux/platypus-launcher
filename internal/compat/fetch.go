package compat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ErrNotFound is what a source answers for an app it has no data on.
var ErrNotFound = errors.New("no data for this app")

// ErrPaused is returned without asking the server: the source answered 429
// several times in a row, and Hydra waits before it asks again.
var ErrPaused = errors.New("paused after repeated rate limits")

// StatusError is an HTTP answer Hydra can't use.
type StatusError struct{ Status int }

func (e *StatusError) Error() string { return fmt.Sprintf("server answered %d", e.Status) }

const (
	maxBody       = 8 << 20 // a catalogue file is ~1 MB; anything bigger is wrong
	pauseAfter    = 3       // consecutive limited before the pause
	pauseFor      = 15 * time.Minute
	maxRetryAfter = 10 * time.Minute
)

// Fetcher is how Hydra talks to the compat sources: politely. At most
// Concurrency requests run at once, each one at least Interval after the
// last (plus up to Jitter), and a 429 or 5xx is retried with exponential
// backoff, honouring Retry-After. Repeated limited pause the whole fetcher.
type Fetcher struct {
	Client      *http.Client
	UserAgent   string
	Concurrency int
	Interval    time.Duration
	Jitter      time.Duration
	Retries     int
	Backoff     time.Duration

	// Now, Sleep and Rand are replaced in tests.
	Now   func() time.Time
	Sleep func(ctx context.Context, d time.Duration) error
	Rand  func(n int64) int64

	once    sync.Once
	slots   chan struct{}
	mu      sync.Mutex
	next    time.Time // earliest start of the next request
	pause   time.Time // no request before this
	limited int       // consecutive rate limits
}

// NewFetcher returns a fetcher with Hydra's defaults: two requests at once, about
// one a second, three retries from 2 s.
func NewFetcher(userAgent string) *Fetcher {
	return &Fetcher{
		Client:      &http.Client{Timeout: 15 * time.Second},
		UserAgent:   userAgent,
		Concurrency: 2,
		Interval:    time.Second,
		Jitter:      400 * time.Millisecond,
		Retries:     3,
		Backoff:     2 * time.Second,
	}
}

// NewNoRedirectClient is an HTTP client that hands a redirect back to the
// caller instead of following it (Fetcher.Location).
func NewNoRedirectClient() *http.Client {
	return &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func (f *Fetcher) init() {
	f.once.Do(func() {
		if f.Concurrency < 1 {
			f.Concurrency = 1
		}
		f.slots = make(chan struct{}, f.Concurrency)
		if f.Now == nil {
			f.Now = time.Now
		}
		if f.Sleep == nil {
			f.Sleep = sleepCtx
		}
		if f.Rand == nil {
			f.Rand = rand.Int64N
		}
		if f.Client == nil {
			f.Client = http.DefaultClient
		}
	})
}

// Get fetches url and returns the body. ErrNotFound is a 404; any other
// unusable answer is a *StatusError, and a transport error is returned as is.
func (f *Fetcher) Get(ctx context.Context, url string) ([]byte, error) {
	a, err := f.send(ctx, url)
	if err != nil {
		return nil, err
	}
	switch a.status {
	case http.StatusOK:
		return a.body, nil
	case http.StatusNotFound:
		return nil, ErrNotFound
	}
	return nil, &StatusError{Status: a.status}
}

// Location asks for url without following a redirect and returns where it
// points. A client used for this must not follow redirects itself (see
// NewNoRedirectClient). ErrNotFound is a 404; a non-redirect answer is a
// *StatusError.
func (f *Fetcher) Location(ctx context.Context, url string) (string, error) {
	a, err := f.send(ctx, url)
	if err != nil {
		return "", err
	}
	switch {
	case a.status >= 300 && a.status < 400 && a.location != "":
		return a.location, nil
	case a.status == http.StatusNotFound:
		return "", ErrNotFound
	}
	return "", &StatusError{Status: a.status}
}

// send makes the request and retries a 429 or 5xx with exponential backoff,
// honouring Retry-After. It returns the last answer, whatever its status.
func (f *Fetcher) send(ctx context.Context, url string) (answer, error) {
	f.init()
	wait := f.Backoff
	for attempt := 0; ; attempt++ {
		a, err := f.do(ctx, url)
		if err != nil {
			return answer{}, err
		}
		if a.status != http.StatusTooManyRequests && a.status < 500 {
			return a, nil
		}
		if attempt >= f.Retries {
			return a, nil
		}
		d := wait
		if a.retryAfter > d {
			d = a.retryAfter
		}
		if err := f.Sleep(ctx, d+f.jitter()); err != nil {
			return answer{}, err
		}
		wait *= 2
	}
}

// answer is one HTTP response, reduced to what Hydra reads.
type answer struct {
	status     int
	body       []byte
	retryAfter time.Duration
	location   string
}

// do makes one request: it takes a slot, waits its turn, and records a 429.
func (f *Fetcher) do(ctx context.Context, url string) (answer, error) {
	select {
	case f.slots <- struct{}{}:
	case <-ctx.Done():
		return answer{}, ctx.Err()
	}
	defer func() { <-f.slots }()

	if err := f.pace(ctx); err != nil {
		return answer{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return answer{}, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := f.Client.Do(req)
	if err != nil {
		return answer{}, err
	}
	defer resp.Body.Close()
	a := answer{status: resp.StatusCode, retryAfter: parseRetryAfter(resp.Header.Get("Retry-After")), location: resp.Header.Get("Location")}

	f.mu.Lock()
	if resp.StatusCode == http.StatusTooManyRequests {
		f.limited++
		if f.limited >= pauseAfter {
			f.pause = f.Now().Add(pauseFor)
		}
	} else {
		f.limited = 0
	}
	f.mu.Unlock()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBody))
		return a, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return answer{}, err
	}
	if len(body) > maxBody {
		return answer{}, errors.New("response is too large")
	}
	a.body = body
	return a, nil
}

// pace reserves the next start time and waits for it. A pause from repeated
// limited is refused outright.
func (f *Fetcher) pace(ctx context.Context) error {
	f.mu.Lock()
	now := f.Now()
	if now.Before(f.pause) {
		f.mu.Unlock()
		return ErrPaused
	}
	at := f.next
	if at.Before(now) {
		at = now
	}
	f.next = at.Add(f.Interval + f.jitter())
	f.mu.Unlock()
	if d := at.Sub(now); d > 0 {
		return f.Sleep(ctx, d)
	}
	return nil
}

// jitter is a random extra delay in [0, Jitter).
func (f *Fetcher) jitter() time.Duration {
	if f.Jitter <= 0 {
		return 0
	}
	return time.Duration(f.Rand(int64(f.Jitter)))
}

// parseRetryAfter reads a Retry-After given in seconds, capped. A date form
// or junk is ignored.
func parseRetryAfter(v string) time.Duration {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	d := time.Duration(n) * time.Second
	if d > maxRetryAfter {
		d = maxRetryAfter
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
