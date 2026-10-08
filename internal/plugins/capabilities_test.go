package plugins

import (
	"slices"
	"testing"
)

func TestRegistryHoldsTheExistingCapabilities(t *testing.T) {
	for _, name := range []string{"library", "install", "launch", "auth", "cloud-saves", "discard"} {
		c, ok := LookupCapability(name)
		if !ok {
			t.Fatalf("existing capability %q is not registered", name)
		}
		if c.Since == "" {
			t.Errorf("%s has no since", name)
		}
	}
	if c, _ := LookupCapability("library"); !slices.Contains(c.OptionalMethods, "game.details") {
		t.Errorf("game.details should be an optional method of library: %+v", c)
	}
	if c, _ := LookupCapability("discard"); !slices.Equal(c.Methods, []string{"job.discard"}) {
		t.Errorf("discard methods: %v", c.Methods)
	}
}

func TestRegistryHoldsThePlannedCapabilitiesWithoutOrchestrators(t *testing.T) {
	for _, name := range []string{"install-options", "versions", "verify", "move", "social",
		"achievements", "playtime", "redeem", "family"} {
		c, ok := LookupCapability(name)
		if !ok {
			t.Fatalf("planned capability %q is not registered", name)
		}
		if c.Orchestrated {
			t.Errorf("%s has no orchestrator yet but is marked orchestrated", name)
		}
	}
	for _, name := range []string{"library", "install", "launch", "auth", "discard"} {
		if c, _ := LookupCapability(name); !c.Orchestrated {
			t.Errorf("%s is used by core today but is not marked orchestrated", name)
		}
	}
}

func TestRegistryIsConsistent(t *testing.T) {
	seenName := map[string]bool{}
	owner := map[string]string{}
	for _, c := range Registry() {
		if !ValidCapabilityName(c.Name) {
			t.Errorf("registered name %q is malformed", c.Name)
		}
		if seenName[c.Name] {
			t.Errorf("%s registered twice", c.Name)
		}
		seenName[c.Name] = true
		if len(c.Methods)+len(c.OptionalMethods) == 0 {
			t.Errorf("%s has no methods", c.Name)
		}
		if len(c.Surfaces) == 0 {
			t.Errorf("%s names no surface", c.Name)
		}
		for _, m := range append(slices.Clone(c.Methods), c.OptionalMethods...) {
			if other, dup := owner[m]; dup {
				t.Errorf("method %s belongs to both %s and %s", m, other, c.Name)
			}
			owner[m] = c.Name
		}
	}
}

func TestCapabilityForMethod(t *testing.T) {
	cases := []struct {
		method, capability string
		optional, ok       bool
	}{
		{"library.list", "library", false, true},
		{"game.details", "library", true, true},
		{"versions.unlock", "versions", true, true},
		{"versions.list", "versions", false, true},
		{"initialize", "", false, false},
		{"no.such.method", "", false, false},
	}
	for _, c := range cases {
		cap, optional, ok := CapabilityForMethod(c.method)
		if cap != c.capability || optional != c.optional || ok != c.ok {
			t.Errorf("%s: got (%q, %v, %v), want (%q, %v, %v)", c.method, cap, optional, ok, c.capability, c.optional, c.ok)
		}
	}
}

func TestRegistryReturnsACopy(t *testing.T) {
	r := Registry()
	r[0].Name = "mutated"
	r[0].Methods[0] = "mutated"
	if c, _ := LookupCapability("library"); c.Name != "library" || c.Methods[0] != "library.list" {
		t.Fatalf("the registry was changed through a returned copy: %+v", c)
	}
}

func TestValidCapabilityName(t *testing.T) {
	for _, ok := range []string{"library", "cloud-saves", "install-options", "a1", "x2-y3"} {
		if !ValidCapabilityName(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "Library", "cloud_saves", "-x", "x-", "x--y", "1x", "a b", "versions@1", "../x",
		"averyveryveryveryveryveryveryveryveryverylongname"} {
		if ValidCapabilityName(bad) {
			t.Errorf("%q should be malformed", bad)
		}
	}
}
