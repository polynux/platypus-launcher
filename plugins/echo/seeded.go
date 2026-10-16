package main

// Seeded library facts for trying collections and smart filters (#67, part of
// #65): deterministic store-like data on top of the ECHO_GAMES bulk games
// (bulk.go) and the ECHO_COMPAT facts (compat.go).
//
//	ECHO_SEED          integer seed for the bulk extras' facts (default unset:
//	                   bulk games as before, no seeded facts)
//
// When set, every bulk game (bulk-0000 …) also carries a deterministic
// lastPlayed/store status suggestion (library.list, protocols 1.15/1.16), a
// store playtime (playtime.list, protocol 1.7) and game.details facts
// (releaseDate plus the compat steamAppId/platforms/drm, protocol 1.10).
// Explicit ECHO_COMPAT entries win over the seeded compat facts. Installed,
// native, size and titles stay exactly what ECHO_GAMES gives: the seed only
// adds facts, so scale measurements don't move. The same seed lists the same
// facts every run (a fixed base time, hash-based picks, no clock reads).
//
// What the UI's smart filters read (apps/ui/src/lib/collections.ts) and
// where it comes from, so only real paths are seeded:
//   - installed/native/store/size: library.list (here, via ECHO_GAMES).
//   - last played: library.list lastPlayed (here) merged with the core's own
//     playlog (max wins); playtime: playtime.list (here) merged the same way.
//     Seeding the store side is the real store-report path.
//   - status/notes: library.list status/userNotes hints (here, 1.16).
//   - tier/deck (fix_info): the core derives them from the Steam AppID hint
//     (here) through ProtonDB/Deck or data-plugin fixes, so offline they stay
//     empty and the tier filter sees 'native' vs 'none'. The seeded AppIDs
//     are real-shaped so online runs and compat-cache seeds resolve.
//   - release year: store games carry none (ReleaseYear is sideloaded-only),
//     so game.details releaseDate is seeded for the game page while the year
//     filter still needs sideloaded games.

import (
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
)

// seededBase is the fixed "now" the seeded facts count back from (unix
// seconds, ~2026-10-14): lastPlayed values land within the year before it,
// so played-within-days filters with a generous window match.
const seededBase = 1792000000

// seededAppIDs are the Steam AppID hints seeded games draw from: real-shaped
// ids the compat tests already use.
var seededAppIDs = []string{"620", "730", "578080", "367520", "480"}

// seededSeed parses ECHO_SEED: the seed and whether seeding is on. Empty or
// unparsable means off (bulk games as before).
func seededSeed() (int64, bool) {
	v := strings.TrimSpace(os.Getenv("ECHO_SEED"))
	if v == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// seededHash is the deterministic pick for (seed, index, salt): FNV-1a over
// the three, so the same seed lists the same facts on every Go version.
func seededHash(seed int64, i int, salt string) uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d:%d:%s", seed, i, salt)
	return h.Sum64()
}

// seededBulkExtra adds the library.list facts of bulk game i to e: lastPlayed
// (1.15), status and userNotes hints (1.16). Installed, native, size and
// titles are ECHO_GAMES's own and stay untouched.
func seededBulkExtra(seed int64, i int, e map[string]any) {
	if h := seededHash(seed, i, "played"); h%5 >= 2 {
		e["lastPlayed"] = seededBase - int64(h%(90*86400))
	}
	if h := seededHash(seed, i, "status"); h%20 >= 15 {
		statuses := []string{"backlog", "playing", "finished", "abandoned"}
		st := statuses[h%uint64(len(statuses))]
		e["status"] = st
		if h%2 == 0 {
			e["userNotes"] = fmt.Sprintf("Seeded note for bulk-%04d.", i)
		}
	}
}

// seededBulkMinutes is the store playtime of bulk game i in minutes, and
// whether the store reports one at all. Unreported (false) leaves the core's
// local record alone; 0 would be a real "never played" report.
func seededBulkMinutes(seed int64, i int) (int, bool) {
	h := seededHash(seed, i, "playtime")
	if h%100 < 30 {
		return 0, false
	}
	return 1 + int(h%1500), true
}

// seededBulkIndex parses a bulk game id ("bulk-0007") to its index.
func seededBulkIndex(id string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(id, "bulk-"))
	if err != nil || n < 0 || id != bulkID(n) {
		return 0, false
	}
	return n, true
}

// seededBulkDetails is the game.details answer of a bulk game: its releaseDate
// and seeded compat facts (steamAppId/platforms/drm). Explicit ECHO_COMPAT
// facts are merged over these by the caller. Nil when seeding is off.
func seededBulkDetails(id string) map[string]any {
	seed, ok := seededSeed()
	if !ok {
		return nil
	}
	i, ok := seededBulkIndex(id)
	if !ok {
		return nil
	}
	out := map[string]any{}
	if h := seededHash(seed, i, "release"); h%10 < 9 {
		out["releaseDate"] = fmt.Sprintf("%04d-%02d-%02d", 1998+h%29, 1+h%12, 1+h%28)
	}
	if h := seededHash(seed, i, "appid"); h%10 >= 3 {
		out["steamAppId"] = seededAppIDs[h%uint64(len(seededAppIDs))]
	}
	if h := seededHash(seed, i, "platforms"); h%4 == 0 {
		out["platforms"] = []string{"linux", "windows"}
	} else {
		out["platforms"] = []string{"windows"}
	}
	if h := seededHash(seed, i, "drm"); h%10 >= 6 {
		drm := map[string]any{}
		if h%2 == 0 {
			drm["drmFree"] = false
		}
		if h%3 == 0 {
			drm["onlineRequired"] = true
		}
		if h%4 == 1 {
			drm["launcher"] = "echo-launcher"
		}
		if h%5 == 0 {
			drm["denuvoLikely"] = true
		}
		if h%3 == 1 {
			drm["notes"] = fmt.Sprintf("Seeded store facts for bulk-%04d.", i)
		}
		if len(drm) > 0 {
			out["drm"] = drm
		}
	}
	return out
}
