package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Token-bearing files must end up 0600 even when they already exist as
// 0644: WriteFile keeps the old mode, so storeToken chmods after writing
// and tokenFromFile tightens on first touch. session.json carries no
// secret and stays 0644.
func TestStoreTokenFiles0600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLATYPUS_PLUGIN_DATA", dir)
	// Pre-create the token files world-readable, like the pre-fix saves.
	os.WriteFile(filepath.Join(dir, "refresh.token"), []byte("stale\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "token.json"), []byte(`{"refresh_token":"stale"}`), 0o644)
	const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiI3NjU2MTE5OCJ9." +
		"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c"
	if err := storeToken(token, "TestUser"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"refresh.token", "token.json"} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if fi.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s mode %v, want 0600", name, fi.Mode())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "session.json")); err != nil {
		t.Fatalf("session.json: %v", err)
	}
	// tokenFromFile tightens a pre-existing 0644 refresh.token on read.
	os.Chmod(filepath.Join(dir, "refresh.token"), 0o644)
	if got := tokenFromFile(); got != token {
		t.Fatalf("tokenFromFile: got %q", got[:20])
	}
	if fi, _ := os.Stat(filepath.Join(dir, "refresh.token")); fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("refresh.token mode after read %v, want 0600", fi.Mode())
	}
}
