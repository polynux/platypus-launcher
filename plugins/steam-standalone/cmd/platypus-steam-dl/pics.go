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
// Branch defaults to "public". Password branches need CheckAppBetaPassword
// first; the request code call then carries the branch and its hash.
// The exact key spellings are an assumption to check against a real
// product-info reply in phase 5.
package main

import (
	"crypto/sha1"
	"fmt"
	"runtime"
	"strings"

	"platypus/internal/steam"
)

// DepotRef is one depot to download: its id, manifest id and branch.
type DepotRef struct {
	DepotID  uint32
	GID      uint64
	Branch   string
	FromApp  uint32 // depotfromapp source, 0 when own
	KeyAppID uint32 // app the depot key/request code belong to (source for shared)
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
		if gidStr := d.String("manifests", branch, "gid"); gidStr != "" {
			var gid uint64
			fmt.Sscanf(gidStr, "%d", &gid)
			if gid != 0 {
				out = append(out, DepotRef{DepotID: depotID, GID: gid, Branch: branch})
				continue
			}
		}
		if from := d.String("depotfromapp"); from != "" {
			var src uint32
			fmt.Sscanf(from, "%d", &src)
			if src != 0 && src != appID {
				out = append(out, DepotRef{DepotID: depotID, Branch: branch, FromApp: src})
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
// it (SHA-1 hex, like DepotDownloader's branch_password_hash).
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
