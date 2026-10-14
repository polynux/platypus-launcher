// Package winetricks knows the real winetricks verb names, pinned to
// Winetricks/winetricks commit f3890f6 (2026-08-27): every w_metadata line
// in src/winetricks at that commit. The dataplug fix validator and the
// prefix preset validator share this one list, so an unknown verb is
// refused everywhere instead of failing at install time.
package winetricks

import (
	_ "embed"
	"strings"
)

//go:embed verbs.txt
var verbsData string

// known is the pinned verb set, parsed once from the embedded list.
var known = func() map[string]bool {
	m := map[string]bool{}
	for _, v := range strings.Fields(verbsData) {
		m[v] = true
	}
	return m
}()

// KnownVerb reports whether name is a real winetricks verb: an exact
// w_metadata name from the pinned winetricks source. The match is exact;
// "ddraw" is not a verb (only cnc_ddraw is), nor is a DLL name.
func KnownVerb(name string) bool {
	return known[name]
}
