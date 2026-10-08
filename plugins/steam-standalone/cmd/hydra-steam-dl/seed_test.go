package main

import (
	"testing"
)

// STEAM-5: small-depot tail is serial startup, not per-chunk cost.
// Seed connections by hosts*perHost and chunks so small jobs start wide.
func TestSeededInitialConns(t *testing.T) {
	if got := seededInitialConns(32, 4, 55); got != 32 {
		t.Fatalf("32 hosts x4, 55 chunks: got %d, want 32", got)
	}
	if got := seededInitialConns(2, 4, 55); got != 8 {
		t.Fatalf("2 hosts x4: got %d, want 8", got)
	}
	if got := seededInitialConns(32, 4, 5); got != 5 {
		t.Fatalf("5 chunks: got %d, want 5", got)
	}
	if got := seededInitialConns(0, 4, 55); got != 8 {
		t.Fatalf("no hosts: got %d, want default 8", got)
	}
}
