package app

import (
	"testing"
)

// The embedded store view (issue #17) lists the plugins that declare a
// storefront: the manifest's URL and domains must reach the UI.
func TestListStorePluginsExposesStorefront(t *testing.T) {
	t.Parallel()
	a, _ := socialEnv(t)
	var found bool
	for _, p := range a.ListStorePlugins() {
		if p.ID != "echo" {
			continue
		}
		found = true
		if p.StorefrontURL != "http://127.0.0.1:18427/store/" {
			t.Fatalf("storefront url: %q", p.StorefrontURL)
		}
		if len(p.StorefrontDomains) != 1 || p.StorefrontDomains[0] != "127.0.0.1" {
			t.Fatalf("storefront domains: %q", p.StorefrontDomains)
		}
	}
	if !found {
		t.Fatal("echo not listed")
	}
}

// Closing the store view drops the source's cached listing (issue #17), so
// what was bought appears. InvalidateLibrary is the UI's call for that.
func TestInvalidateLibraryDropsTheCache(t *testing.T) {
	t.Parallel()
	a, _ := socialEnv(t)
	a.library.set("echo", nil, nil)
	if _, _, ok := a.library.fresh("echo"); !ok {
		t.Fatal("setup: echo listing not cached")
	}
	a.InvalidateLibrary("echo")
	if _, _, ok := a.library.fresh("echo"); ok {
		t.Fatal("echo listing still cached after invalidate")
	}
}
