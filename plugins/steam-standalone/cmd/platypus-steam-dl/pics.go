// Depot selection from PICS app info (text KeyValues, confirmed live).
// The buffer is wrapped in an "appinfo" section (stripped by ProductInfo,
// which hands us the section) and looks like:
//
//	"depots"
//	{
//	  "11"
//	  {
//	    "config" { "oslist" "linux" ... }
//	    "manifests" { "public" { "gid" "123..." } }
//	    "depotfromapp" "543"   // optional: another app's depot
//	  }
//	}
//
// Branch defaults to "public". A password branch carries no plain
// manifests: privateBranchInfo unlocks its private-beta section with the
// branch key CheckBetaPassword returned, and the request code call then
// carries the branch and its SHA-1 password hash.
package main

import (
	"crypto/sha1"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"platypus/internal/steam"
)

// DepotRef is one depot to download: its id, manifest id and branch.
// DLCAppID names the add-on the depot belongs to (PICS depots.<id>.dlcappid);
// zero means the base game.
type DepotRef struct {
	DepotID  uint32
	GID      uint64
	Branch   string
	FromApp  uint32 // depotfromapp source, 0 when own
	KeyAppID uint32 // app the depot key/request code belong to (source for shared)
	DLCAppID uint32 // DLC app the depot belongs to, 0 for the base game
}

// steamArch reports the machine architecture the way DepotDownloader's
// default does: "64" on 64-bit machines, "32" otherwise.
func steamArch() string {
	if runtime.GOARCH == "386" {
		return "32"
	}
	return "64"
}

// selectDepots picks the app's depots for one OS name, applying the same
// filters DepotDownloader uses by default: the entry's own config must
// carry the OS in oslist, the machine architecture in osarch and english
// in language, and lowviolence entries are skipped. An absent or empty key
// means "all" and is included. Shared (depotfromapp) entries are filtered
// by their own entry first, exactly like DepotDownloader filters before
// resolving; entries with their own manifests use them, others resolve
// against the source app in flow.go.
func selectDepots(kv *steam.KV, appID uint32, branch, osname string) ([]DepotRef, error) {
	if branch == "" {
		branch = "public"
	}
	depots := kv.Get("depots")
	if depots == nil || len(depots.Children) == 0 {
		return nil, fmt.Errorf("app has no depots section")
	}
	var out []DepotRef
	for id, d := range depots.Children {
		var depotID uint32
		fmt.Sscanf(id, "%d", &depotID)
		if depotID == 0 {
			continue
		}
		cfg := d.Get("config")
		if !listMatches(cfg, "oslist", osname) {
			continue
		}
		if !listMatches(cfg, "osarch", steamArch()) {
			continue
		}
		if !listMatches(cfg, "language", "english") {
			continue
		}
		if cfg != nil && cfg.String("lowviolence") == "1" {
			continue
		}
		// DepotDownloader reads the same field: depots.<id>.dlcappid names
		// the add-on the depot ships; absent (or unparsable) is the base.
		var dlcAppID uint32
		fmt.Sscanf(d.String("dlcappid"), "%d", &dlcAppID)
		if gidStr := d.String("manifests", branch, "gid"); gidStr != "" {
			var gid uint64
			fmt.Sscanf(gidStr, "%d", &gid)
			if gid != 0 {
				out = append(out, DepotRef{DepotID: depotID, GID: gid, Branch: branch, DLCAppID: dlcAppID})
				continue
			}
		}
		if from := d.String("depotfromapp"); from != "" {
			var src uint32
			fmt.Sscanf(from, "%d", &src)
			if src != 0 && src != appID {
				out = append(out, DepotRef{DepotID: depotID, Branch: branch, FromApp: src, DLCAppID: dlcAppID})
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no depots for this platform")
	}
	return out, nil
}

// listMatches reports whether a config list key (comma-separated, like
// oslist) contains the wanted value. Missing or empty means "all".
func listMatches(cfg *steam.KV, key, want string) bool {
	if cfg == nil {
		return true
	}
	raw := cfg.String(key)
	if strings.TrimSpace(raw) == "" {
		return true
	}
	for _, o := range strings.Split(raw, ",") {
		if strings.TrimSpace(o) == want {
			return true
		}
	}
	return false
}

// betaHash hashes a branch password the way the request-code call wants
// it (SHA-1 hex: proven live 2026-11-16, a Valheim public-test manifest
// downloaded with the SHA-1 of its password).
func betaHash(password string) string {
	sum := sha1.Sum([]byte(password))
	return fmt.Sprintf("%x", sum)
}

// branchPasswordHash returns the request-code hash for a job: SHA-1 hex
// for non-public branches with a password, else "" (no hash sent).
func branchPasswordHash(req request) string {
	branch := req.Branch
	if branch == "" {
		branch = "public"
	}
	if branch == "public" || req.BranchPassword == "" {
		return ""
	}
	return betaHash(req.BranchPassword)
}

// listDLC reads the game's DLC app ids from PICS app info
// (extended.listofdlc, comma-separated): the Steam client's DLC list for
// the game. Missing or unparsable entries are skipped; the result is
// sorted. An app with no DLC answers an empty list, not an error.
func listDLC(kv *steam.KV) []uint32 {
	raw := kv.String("extended", "listofdlc")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := map[uint32]bool{}
	var out []uint32
	for _, part := range strings.Split(raw, ",") {
		n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 32)
		if err != nil || n == 0 || seen[uint32(n)] {
			continue
		}
		seen[uint32(n)] = true
		out = append(out, uint32(n))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// filterDepotsByDLC keeps the base-game depots (no dlcappid) and the depots
// of the selected DLC apps. keep names DLC app ids; a nil keep installs
// the old set (everything selected: the legacy request shape without a
// dlc field). The second result is the dropped depots, so callers can
// forget overrides pinned to them instead of erroring.
func filterDepotsByDLC(refs []DepotRef, keep map[uint32]bool) (kept, dropped []DepotRef) {
	if keep == nil {
		return refs, nil
	}
	for _, r := range refs {
		if r.DLCAppID == 0 || keep[r.DLCAppID] {
			kept = append(kept, r)
		} else {
			dropped = append(dropped, r)
		}
	}
	return kept, dropped
}

// dlcKeepSet parses the request's dlc value ids (protocol 1.5: strings)
// into the keep set for filterDepotsByDLC. A nil request field means no
// filtering; anything else (even empty: base game only) filters.
func dlcKeepSet(raw []string) (map[uint32]bool, error) {
	if raw == nil {
		return nil, nil
	}
	keep := map[uint32]bool{}
	for _, id := range raw {
		n, err := strconv.ParseUint(strings.TrimSpace(id), 10, 32)
		if err != nil || n == 0 {
			return nil, fmt.Errorf("bad dlc selection %q", id)
		}
		keep[uint32(n)] = true
	}
	return keep, nil
}
