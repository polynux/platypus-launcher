package main

import (
	"testing"

	"hydra-launcher/internal/steam"
)

// Depot filters mirror DepotDownloader's defaults: oslist must contain the
// OS, osarch the machine arch, language english; lowviolence is skipped.
// Caught live: Pressure's 32-bit depots must not join the download.
func TestSelectDepotsFilters(t *testing.T) {
	src := `"depots" {` +
		`"11" { "config" { "oslist" "linux" } "manifests" { "public" { "gid" "111" } } }` +
		`"12" { "config" { "oslist" "linux" "osarch" "32" } "manifests" { "public" { "gid" "112" } } }` +
		`"13" { "config" { "oslist" "linux" "language" "french" } "manifests" { "public" { "gid" "113" } } }` +
		`"14" { "config" { "oslist" "linux" "lowviolence" "1" } "manifests" { "public" { "gid" "114" } } }` +
		`"15" { "manifests" { "public" { "gid" "115" } } }` +
		`"21" { "config" { "oslist" "windows" } "manifests" { "public" { "gid" "121" } } }` +
		`"22" { "config" { "oslist" "linux" } "depotfromapp" "543" }` +
		`"23" { "config" { "oslist" "windows" } "depotfromapp" "543" }` +
		`}`
	kv, err := steam.ParseKV(src)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := selectDepots(kv, 4321, "public", "linux")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[uint32]bool{}
	for _, r := range refs {
		ids[r.DepotID] = true
	}
	if !ids[11] || !ids[15] {
		t.Fatalf("plain and config-less depots must match: %v", ids)
	}
	if steamArch() == "64" && ids[12] {
		t.Fatalf("32-bit depot must not match on 64-bit: %v", ids)
	}
	if ids[13] {
		t.Fatalf("french depot must not match: %v", ids)
	}
	if ids[14] {
		t.Fatalf("lowviolence depot must not match: %v", ids)
	}
	if ids[21] {
		t.Fatalf("windows depot must not match linux: %v", ids)
	}
	shared := false
	for _, r := range refs {
		if r.DepotID == 22 && r.FromApp == 543 {
			shared = true
		}
		if r.DepotID == 23 {
			t.Fatalf("windows shared depot must not match linux: %v", ids)
		}
	}
	if !shared {
		t.Fatalf("linux shared depot must resolve later: %v", ids)
	}
	// Config-less depots belong to every platform (DepotDownloader skips
	// no filter when the config section is absent).
	macos, err := selectDepots(kv, 4321, "public", "macos")
	if err != nil {
		t.Fatal(err)
	}
	if len(macos) != 1 || macos[0].DepotID != 15 {
		t.Fatalf("macos should have only the config-less depot: %+v", macos)
	}
}
