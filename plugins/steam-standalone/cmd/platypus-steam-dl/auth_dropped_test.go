package main

import (
	"fmt"
	"testing"

	"platypus/internal/steamcm"
)

// Only an error no Steam EResult answered restarts a QR login; a refusal
// (rate limit, expired session) or our own timeout never does (#182).
func TestAuthDroppedOnlyForUnansweredErrors(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&steamcm.CMError{Msg: "Steam CM connection dropped"}, true},
		{fmt.Errorf("poll: %w", &steamcm.CMError{Msg: "didn't answer"}), true},
		{&steamcm.CMError{Msg: "rate limited", EResult: 84}, false},
		{&steamcm.CMError{Msg: "expired", EResult: 27}, false},
		{fmt.Errorf("nobody scanned the QR code in time; try again"), false},
	}
	for _, c := range cases {
		if got := dropped(c.err); got != c.want {
			t.Errorf("dropped(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
