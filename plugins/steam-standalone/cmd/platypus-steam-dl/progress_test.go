package main

import (
	"testing"

	"platypus/internal/depot"
)

// STEAM-11: multi-depot progress must not jump backwards at depot
// boundaries. Each depot's DownloadStream reports DoneBytes from zero;
// the caller adds a base offset per depot.
func TestMultiDepotProgressMonotonic(t *testing.T) {
	var emitted []uint64
	onProg := func(p depot.Progress) { emitted = append(emitted, p.DoneBytes) }
	var base uint64
	for _, depotBytes := range []uint64{50, 50} {
		wrapped := withBase(onProg, base)
		// Simulate one depot's 0..depotBytes reports.
		for _, done := range []uint64{0, 25, 50} {
			wrapped(depot.Progress{DoneBytes: done, TotalBytes: depotBytes})
		}
		base += depotBytes
	}
	for i := 1; i < len(emitted); i++ {
		if emitted[i] < emitted[i-1] {
			t.Fatalf("progress went backwards at %d: %v", i, emitted)
		}
	}
	if last := emitted[len(emitted)-1]; last != 100 {
		t.Fatalf("final bytes = %d, want 100", last)
	}
}
