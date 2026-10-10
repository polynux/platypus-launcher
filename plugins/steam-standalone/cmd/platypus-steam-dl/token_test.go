package main

import (
	"testing"
	"time"
)

// STEAM-14: CDN tokens expire; long jobs must refresh past expiry-margin
// instead of reusing the first token for hours.
func TestTokenFreshnessMargin(t *testing.T) {
	now := time.Now()
	if !tokenFresh(now.Add(time.Hour), now) {
		t.Fatal("hour-long token should be fresh")
	}
	if tokenFresh(now.Add(60*time.Second), now) {
		t.Fatal("token expiring in 60s should refresh (5m margin)")
	}
	if tokenFresh(now.Add(-time.Second), now) {
		t.Fatal("expired token should not be fresh")
	}
	if !tokenFresh(time.Time{}, now) {
		t.Fatal("zero expiry (no TTL) stays cached")
	}
}
