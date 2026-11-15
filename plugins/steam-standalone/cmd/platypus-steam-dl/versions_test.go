package main

import (
	"strings"
	"testing"

	"platypus/internal/steam"
)

// A PICS-shaped app: public plus a password branch, two depots. Mirrors the
// fields live ProductInfo carries under depots/branches/<name> (buildid,
// timeupdated, description, pwdrequired) and per-depot manifests/<branch>.
const versionsKV = `"depots" {` +
	`"branches" {` +
	`"public" { "buildid" "100" "timeupdated" "1700000000" "description" "Default" }` +
	`"beta" { "buildid" "200" "timeupdated" "1700001000" "description" "Beta" "pwdrequired" "1" }` +
	`"old" { "buildid" "50" "description" "Old" }` +
	`}` +
	`"11" { "config" { "oslist" "linux" } "manifests" { "public" { "gid" "111" } "beta" { "gid" "112" } "old" { "gid" "113" } } }` +
	`"12" { "config" { "oslist" "linux" } "manifests" { "public" { "gid" "121" } "beta" { "gid" "122" } } }` +
	`}`

func parseTestBranches(t *testing.T) []branchInfo {
	t.Helper()
	kv, err := steam.ParseKV(versionsKV)
	if err != nil {
		t.Fatal(err)
	}
	branches, err := parseBranches(kv)
	if err != nil {
		t.Fatal(err)
	}
	return branches
}

func TestParseBranchesPublicFirst(t *testing.T) {
	branches := parseTestBranches(t)
	if len(branches) != 3 {
		t.Fatalf("branches = %v, want 3", branches)
	}
	if branches[0].Name != "public" {
		t.Fatalf("first branch = %q, want public", branches[0].Name)
	}
	beta := findBranch(branches, "beta")
	if beta == nil {
		t.Fatal("no beta branch")
	}
	if !beta.PwdRequired || beta.BuildID != "200" || beta.Description != "Beta" {
		t.Fatalf("beta = %+v", beta)
	}
	if beta.TimeUpdated == "" {
		t.Fatal("beta timeupdated didn't convert to a date")
	}
	if beta.Manifests[11] != 112 || beta.Manifests[12] != 122 {
		t.Fatalf("beta manifests = %v", beta.Manifests)
	}
	if got := findBranch(branches, "public").Manifests[11]; got != 111 {
		t.Fatalf("public depot 11 = %d, want 111", got)
	}
}

// Branches with no manifests anywhere (a name with no gid) are dropped:
// they can't be installed.
func TestParseBranchesDropsEmpty(t *testing.T) {
	kv, err := steam.ParseKV(`"depots" { ` +
		`"branches" { "ghost" { "buildid" "9" } } ` +
		`"11" { "manifests" { "public" { "gid" "111" } } } ` +
		`}`)
	if err != nil {
		t.Fatal(err)
	}
	branches, err := parseBranches(kv)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 1 || branches[0].Name != "public" {
		t.Fatalf("branches = %+v, want only public", branches)
	}
}

// No branches section and no manifests at all errors instead of answering
// an empty list that could install the wrong build.
func TestParseBranchesEmptyErrors(t *testing.T) {
	kv, err := steam.ParseKV(`"depots" { "11" { "config" { "oslist" "linux" } } }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseBranches(kv); err == nil {
		t.Fatal("expected an error for a depot section with no manifests")
	}
}

// Old fixtures without a branches section still install: public is
// synthesized from the depots' public manifests.
func TestParseBranchesSynthesizesPublic(t *testing.T) {
	kv, err := steam.ParseKV(`"depots" {` +
		`"11" { "config" { "oslist" "linux" } "manifests" { "public" { "gid" "111" } } }` +
		`}`)
	if err != nil {
		t.Fatal(err)
	}
	branches, err := parseBranches(kv)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 1 || branches[0].Name != "public" || branches[0].Manifests[11] != 111 {
		t.Fatalf("branches = %+v", branches)
	}
}

func TestResolveTargetLegacyBranch(t *testing.T) {
	branches := parseTestBranches(t)
	rt, err := resolveTarget(branches, request{Branch: "old"})
	if err != nil {
		t.Fatal(err)
	}
	if rt.branch != "old" || rt.build != "50" {
		t.Fatalf("rt = %+v", rt)
	}
	// No branch means public.
	rt, err = resolveTarget(branches, request{})
	if err != nil {
		t.Fatal(err)
	}
	if rt.branch != "public" || rt.build != "100" {
		t.Fatalf("rt = %+v", rt)
	}
}

func TestResolveTargetUnknownBranchErrors(t *testing.T) {
	branches := parseTestBranches(t)
	if _, err := resolveTarget(branches, request{Target: &versionTarget{Channel: "nope"}}); err == nil {
		t.Fatal("expected an error for an unknown branch")
	}
}

func TestResolveTargetLockedChannel(t *testing.T) {
	branches := parseTestBranches(t)
	// A locked channel without its password is refused, never installed
	// as public instead.
	if _, err := resolveTarget(branches, request{Target: &versionTarget{Channel: "beta"}}); err == nil {
		t.Fatal("expected a locked-channel error")
	}
	rt, err := resolveTarget(branches, request{Target: &versionTarget{Channel: "beta"}, BranchPasswords: map[string]string{"beta": "pw"}})
	if err != nil {
		t.Fatal(err)
	}
	if rt.branch != "beta" || rt.password != "pw" {
		t.Fatalf("rt = %+v", rt)
	}
	// The legacy single password still works.
	rt, err = resolveTarget(branches, request{Target: &versionTarget{Channel: "beta"}, BranchPassword: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	if rt.password != "pw" {
		t.Fatalf("rt = %+v", rt)
	}
}

func TestResolveTargetBuild(t *testing.T) {
	branches := parseTestBranches(t)
	rt, err := resolveTarget(branches, request{Target: &versionTarget{Build: "50"}})
	if err != nil {
		t.Fatal(err)
	}
	if rt.branch != "old" || rt.build != "50" {
		t.Fatalf("rt = %+v", rt)
	}
	if _, err := resolveTarget(branches, request{Target: &versionTarget{Build: "999"}}); err == nil {
		t.Fatal("expected an error for an unknown build")
	}
	// An explicit build that is no longer the branch's tip is refused,
	// not installed as the tip.
	if _, err := resolveTarget(branches, request{Target: &versionTarget{Channel: "old", Build: "51"}}); err == nil {
		t.Fatal("expected a stale-build error")
	}
}

func TestParseManualManifests(t *testing.T) {
	m, err := parseManualManifests("11: 111\n12 = 122\n")
	if err != nil {
		t.Fatal(err)
	}
	if m[11] != 111 || m[12] != 122 {
		t.Fatalf("manifests = %v", m)
	}
	for _, bad := range []string{"", "abc", "11", "11: 0", "0: 5", "11: xyz"} {
		if _, err := parseManualManifests(bad); err == nil {
			t.Fatalf("input %q must error", bad)
		}
	}
}

func TestResolveTargetManual(t *testing.T) {
	branches := parseTestBranches(t)
	rt, err := resolveTarget(branches, request{Target: &versionTarget{Manual: map[string]string{"input": "11: 999"}}})
	if err != nil {
		t.Fatal(err)
	}
	if rt.manifests[11] != 999 {
		t.Fatalf("rt = %+v", rt)
	}
	if !strings.HasPrefix(rt.build, "manual-") {
		t.Fatalf("manual build id = %q", rt.build)
	}
	// Same input names the same build.
	rt2, err := resolveTarget(branches, request{Target: &versionTarget{Manual: map[string]string{"input": "11: 999"}}})
	if err != nil {
		t.Fatal(err)
	}
	if rt.build != rt2.build {
		t.Fatalf("manual builds differ: %q vs %q", rt.build, rt2.build)
	}
	// The archive reference travels the same road.
	rt, err = resolveTarget(branches, request{Target: &versionTarget{Build: "b1", ArchiveRef: []byte(`{"depots": {"11": "999"}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	if rt.manifests[11] != 999 || rt.build != "b1" {
		t.Fatalf("rt = %+v", rt)
	}
	if _, err := resolveTarget(branches, request{Target: &versionTarget{ArchiveRef: []byte(`{"depots": {"11": "x"}}`)}}); err == nil {
		t.Fatal("expected an error for a bad archive reference")
	}
}

// A pinned set from a branch reinstalls under that branch.
func TestBranchForManifests(t *testing.T) {
	branches := parseTestBranches(t)
	if bi := branchForManifests(branches, map[uint32]uint64{11: 112, 12: 122}); bi == nil || bi.Name != "beta" {
		t.Fatalf("branch = %+v", bi)
	}
	if bi := branchForManifests(branches, map[uint32]uint64{11: 1}); bi != nil {
		t.Fatalf("branch = %+v, want nil", bi)
	}
}
