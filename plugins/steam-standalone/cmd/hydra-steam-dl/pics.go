// Depot selection from PICS app info (text KeyValues). The depots
// section looks like:
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
	"strings"

	"hydra-launcher/internal/steam"
)

// DepotRef is one depot to download: its id, manifest id and branch.
type DepotRef struct {
	DepotID uint32
	GID     uint64
	Branch  string
	FromApp uint32 // depotfromapp source, 0 when own
}

// selectDepots picks the app's depots for the wanted OS names, in order:
// the first OS name with any depots wins (the plugin tries linux, then
// falls back to windows, and the platform used decides `native`).
func selectDepots(kv *steam.KV, branch string, osnames []string) ([]DepotRef, string, error) {
	if branch == "" {
		branch = "public"
	}
	depots := kv.Get("depots")
	if depots == nil || len(depots.Children) == 0 {
		return nil, "", fmt.Errorf("app has no depots section")
	}
	for _, osname := range osnames {
		var out []DepotRef
		for id, d := range depots.Children {
			var depotID uint32
			fmt.Sscanf(id, "%d", &depotID)
			if depotID == 0 {
				continue
			}
			if from := d.String("depotfromapp"); from != "" {
				var src uint32
				fmt.Sscanf(from, "%d", &src)
				if src != 0 {
					// Shared depots resolve against the source app's
					// info; the caller refetches with the source id.
					out = append(out, DepotRef{DepotID: depotID, Branch: branch, FromApp: src})
					continue
				}
			}
			if !osMatches(d.String("config", "oslist"), osname) {
				continue
			}
			gidStr := d.String("manifests", branch, "gid")
			if gidStr == "" {
				continue
			}
			var gid uint64
			fmt.Sscanf(gidStr, "%d", &gid)
			if gid == 0 {
				continue
			}
			out = append(out, DepotRef{DepotID: depotID, GID: gid, Branch: branch})
		}
		if len(out) > 0 {
			return out, osname, nil
		}
	}
	return nil, "", fmt.Errorf("no depots for this platform")
}

// osMatches reports whether an oslist ("windows", "linux,macos", empty =
// all) includes the wanted OS.
func osMatches(oslist, want string) bool {
	if oslist == "" {
		return true
	}
	for _, o := range strings.Split(oslist, ",") {
		if strings.TrimSpace(strings.ToLower(o)) == want {
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
