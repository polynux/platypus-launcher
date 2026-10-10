package compat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCachePersistsAcrossOpens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compat-cache.json")
	c, err := OpenCache(path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	if err := c.SetProtonDB("620", ProtonEntry{Found: true, FetchedAt: at, Tier: "platinum", Total: 343}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetLocal("/games/x", LocalEntry{FetchedAt: at, Hits: []LocalHit{{Kind: "eos", Label: "Epic Online Services", Name: "EOSBootstrapper.exe"}}}); err != nil {
		t.Fatal(err)
	}
	again, err := OpenCache(path)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := again.ProtonDB("620")
	if !ok || e.Tier != "platinum" || !e.FetchedAt.Equal(at) {
		t.Fatalf("reopened entry %+v ok %v", e, ok)
	}
	l, ok := again.Local("/games/x")
	if !ok || len(l.Hits) != 1 || l.Hits[0].Name != "EOSBootstrapper.exe" {
		t.Fatalf("reopened scan %+v ok %v", l, ok)
	}
}

func TestCacheCorruptFileIsMovedAsideNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compat-cache.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := OpenCache(path)
	if err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("err %v, want a corrupt-cache error", err)
	}
	if _, ok := c.ProtonDB("620"); ok {
		t.Fatal("corrupt cache served data")
	}
	matches, _ := filepath.Glob(path + ".bad-*")
	if len(matches) != 1 {
		t.Fatalf("bad copy not kept: %v", matches)
	}
}

func TestCacheMissingFileIsEmpty(t *testing.T) {
	c, err := OpenCache(filepath.Join(t.TempDir(), "none.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.ProtonDB("1"); ok {
		t.Fatal("entry in an empty cache")
	}
}
