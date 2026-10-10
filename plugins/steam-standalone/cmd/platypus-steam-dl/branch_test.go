package main

import (
	"crypto/sha1"
	"fmt"
	"testing"
)

// STEAM-4: branch passwords must travel end to end (Go side): the request
// carries branch_password, fetchDepots checks it via CheckBetaPassword and
// sends SHA1(password) as the request-code hash. Wrong passwords surface
// distinctly.
func TestBetaHashIsSHA1Hex(t *testing.T) {
	sum := sha1.Sum([]byte("secret"))
	want := fmt.Sprintf("%x", sum)
	if got := betaHash("secret"); got != want {
		t.Fatalf("betaHash = %q, want %q", got, want)
	}
	if betaHash("") == betaHash("x") {
		t.Fatal("empty password must hash differently")
	}
}

func TestBranchPasswordHashPlumbing(t *testing.T) {
	req := request{AppID: 4321, Branch: "betatest", BranchPassword: "secret"}
	got := branchPasswordHash(req)
	sum := sha1.Sum([]byte("secret"))
	if got != fmt.Sprintf("%x", sum) {
		t.Fatalf("branch hash = %q, want sha1", got)
	}
	if h := branchPasswordHash(request{Branch: "public", BranchPassword: "secret"}); h != "" {
		t.Fatalf("public branch must not send a hash: %q", h)
	}
	if h := branchPasswordHash(request{Branch: "betatest"}); h != "" {
		t.Fatalf("empty password must not send a hash: %q", h)
	}
}
