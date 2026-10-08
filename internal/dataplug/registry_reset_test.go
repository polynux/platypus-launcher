package dataplug

import "testing"

// CORE-5: Rescan must drop fixes from removed data plugins without a
// restart. The registry needs a Reset so the app can rebuild it.
func TestRegistryResetDropsRemovedPluginFixes(t *testing.T) {
	r := NewRegistry()
	r.Add(Builtin())
	r.Add(&Data{Fixes: []Fix{{Title: "Community Game", Source: "community", Tier: "gold", Description: "fix"}}})
	if f := r.Fix("Community Game", ""); f == nil {
		t.Fatal("community fix missing before reset")
	}
	r.Reset()
	if f := r.Fix("Community Game", ""); f != nil {
		t.Fatalf("removed plugin fix survived reset: %+v", f)
	}
	// Rebuilding restores the built-in data.
	r.Add(Builtin())
	if f := r.Fix("Fallout: London", ""); f == nil {
		t.Fatal("built-in fix missing after rebuild")
	}
}
