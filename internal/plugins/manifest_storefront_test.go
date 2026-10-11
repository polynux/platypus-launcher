package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePlug writes a manifest plus a runnable exec stub into plug (a plugin
// folder), so LoadManifest can validate it like Discover would.
func writePlug(t *testing.T, plug, manifest string) error {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(plug, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plug, "bin/run"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return os.WriteFile(filepath.Join(plug, ManifestFile), []byte(manifest), 0o644)
}

// storefrontManifest parses a manifest with the given storefront block,
// through the same path Discover uses (folder name "echo" is a test plugin,
// so the http loopback exception applies; "steam-standalone" is not).
func storefrontManifest(t *testing.T, id, storefront string) error {
	t.Helper()
	dir := t.TempDir()
	plug := dir + "/" + id
	manifest := `{"id":` + quote(id) + `,"name":"T","version":"1.0.0","store":"t",` +
		`"protocol":{"versions":["1.0"],"transport":"jsonrpc-stdio"},` +
		`"exec":"bin/run","capabilities":["library"],` +
		`"storefront":` + storefront + `}`
	if err := writePlug(t, plug, manifest); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(plug)
	return err
}

func quote(s string) string { return `"` + s + `"` }

func TestStorefrontValid(t *testing.T) {
	err := storefrontManifest(t, "steam-standalone",
		`{"url":"https://store.steampowered.com/","domains":["steampowered.com","steamcommunity.com","steamstatic.com"]}`)
	if err != nil {
		t.Fatalf("valid storefront rejected: %v", err)
	}
}

func TestStorefrontCases(t *testing.T) {
	cases := map[string]struct {
		id         string
		storefront string
		want       string // "" means valid
	}{
		"subdomain of a domain": {"steam-standalone",
			`{"url":"https://store.steampowered.com/app/620/","domains":["steampowered.com"]}`, ""},
		"http refused for production": {"steam-standalone",
			`{"url":"http://store.steampowered.com/","domains":["steampowered.com"]}`, "must be https"},
		"non-web scheme refused": {"steam-standalone",
			`{"url":"steam://store/","domains":["steampowered.com"]}`, "must be https"},
		"host outside domains": {"steam-standalone",
			`{"url":"https://evil.example/","domains":["steampowered.com"]}`, "not inside"},
		"no domains": {"steam-standalone",
			`{"url":"https://store.steampowered.com/","domains":[]}`, "domains is required"},
		"wildcard domain": {"steam-standalone",
			`{"url":"https://store.steampowered.com/","domains":["*.steampowered.com"]}`, "plain hostname"},
		"domain with path": {"steam-standalone",
			`{"url":"https://store.steampowered.com/","domains":["steampowered.com/store"]}`, "plain hostname"},
		"domain with port": {"steam-standalone",
			`{"url":"https://store.steampowered.com/","domains":["steampowered.com:443"]}`, "plain hostname"},
		"IP domain refused for production": {"steam-standalone",
			`{"url":"https://93.184.216.34/","domains":["93.184.216.34"]}`, "not an IP"},
		"duplicate domain": {"steam-standalone",
			`{"url":"https://store.steampowered.com/","domains":["steampowered.com","SteamPowered.com"]}`, "twice"},
		"not a url": {"steam-standalone",
			`{"url":"not a url","domains":["steampowered.com"]}`, "not a URL"},
		"missing url": {"steam-standalone",
			`{"url":"","domains":["steampowered.com"]}`, "not a URL"},
		"test plugin http loopback": {"echo",
			`{"url":"http://127.0.0.1:18427/store/","domains":["127.0.0.1"]}`, ""},
		"test plugin http elsewhere refused": {"echo",
			`{"url":"http://store.steampowered.com/","domains":["steampowered.com"]}`, "must be https"},
		"production http loopback refused": {"steam-standalone",
			`{"url":"http://127.0.0.1:18427/store/","domains":["127.0.0.1"]}`, "must be https"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := storefrontManifest(t, c.id, c.storefront)
			if c.want == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestStorefrontHTTPOptIn(t *testing.T) {
	t.Setenv("PLATYPUS_ALLOW_HTTP_STOREFRONT", "1")
	err := storefrontManifest(t, "steam-standalone",
		`{"url":"http://127.0.0.1:18427/store/","domains":["127.0.0.1"]}`)
	if err != nil {
		t.Fatalf("opted-in http loopback rejected: %v", err)
	}
}

func TestStorefrontDataPluginHasNone(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	plug := dir + "/data-plug"
	manifest := `{"id":"data-plug","name":"D","version":"1.0.0","kind":"data","data":["x.json"],` +
		`"storefront":{"url":"https://example.com/","domains":["example.com"]}}`
	if err := writePlug(t, plug, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(plug); err == nil || !strings.Contains(err.Error(), "data plugin has no storefront") {
		t.Fatalf("want data-plugin storefront error, got %v", err)
	}
}
