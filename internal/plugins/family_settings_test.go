package plugins

import (
	"slices"
	"testing"
)

// Family and plugin settings are protocol 1.8: a plugin that settled on 1.7
// or older never gets their methods, whatever its manifest lists.
func TestFamilyAndSettingsAreProtocol18(t *testing.T) {
	for _, name := range []string{"family", "settings"} {
		c, ok := LookupCapability(name)
		if !ok || c.Since != "1.8" {
			t.Fatalf("%s: since %q, want 1.8", name, c.Since)
		}
	}
	v17, _ := ParseVersion("1.7")
	kept, later := WithinProtocol([]string{"library", "family", "settings"}, v17)
	if !slices.Equal(kept, []string{"library"}) || !slices.Equal(later, []string{"family", "settings"}) {
		t.Fatalf("over 1.7: kept %v, later %v", kept, later)
	}
	v18, _ := ParseVersion("1.8")
	if kept, later := WithinProtocol([]string{"family", "settings"}, v18); len(later) != 0 || len(kept) != 2 {
		t.Fatalf("over 1.8: kept %v, later %v", kept, later)
	}
	if owner, optional, ok := CapabilityForMethod("settings.apply"); !ok || owner != "settings" || optional {
		t.Fatalf("settings.apply belongs to %q (optional %v, known %v)", owner, optional, ok)
	}
	if owner, _, ok := CapabilityForMethod("family.members"); !ok || owner != "family" {
		t.Fatalf("family.members belongs to %q (known %v)", owner, ok)
	}
	if owner, optional, ok := CapabilityForMethod("family.setPreferredCopy"); !ok || owner != "family" || !optional {
		t.Fatalf("family.setPreferredCopy belongs to %q (optional %v, known %v): a plugin without copies must not fail the family", owner, optional, ok)
	}
	if !slices.Contains(SupportedProtocols, Version{1, 8}) {
		t.Fatal("1.8 is not in SupportedProtocols")
	}
}
