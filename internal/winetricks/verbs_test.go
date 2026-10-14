package winetricks

import (
	"strings"
	"testing"
)

// The accessor knows common verbs and refuses what isn't one. Tests never
// touch the network: the list is embedded.
func TestKnownVerb(t *testing.T) {
	for _, v := range []string{"d3dx9", "dotnet48", "vcrun2019", "corefonts"} {
		if !KnownVerb(v) {
			t.Errorf("KnownVerb(%q) = false, want true", v)
		}
	}
	// ddraw is not a winetricks verb (only cnc_ddraw is, for C&C games),
	// and neither are DLL names, blanks or invented verbs.
	for _, v := range []string{"ddraw", "xaudio2_7", "", "bogus-verb"} {
		if KnownVerb(v) {
			t.Errorf("KnownVerb(%q) = true, want false", v)
		}
	}
}

// The embedded list is the pinned source: one verb per line, no blanks.
func TestVerbsListShape(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(verbsData), "\n")
	if len(lines) < 400 {
		t.Fatalf("embedded verbs = %d lines, want the full winetricks list", len(lines))
	}
	seen := map[string]bool{}
	for _, l := range lines {
		if strings.TrimSpace(l) != l || l == "" {
			t.Fatalf("verbs.txt line %q is blank or padded", l)
		}
		if seen[l] {
			t.Fatalf("verbs.txt lists %q twice", l)
		}
		seen[l] = true
	}
}
