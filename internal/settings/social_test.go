package settings

import (
	"path/filepath"
	"testing"
)

// The friends switch is on for every plugin until the user turns one off, and
// the choice survives a restart.
func TestSocialEnabledDefaultsOnAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SocialEnabled("echo") {
		t.Fatal("friends should be on by default")
	}
	if err := s.SetSocialEnabled("echo", false); err != nil {
		t.Fatal(err)
	}
	if s.SocialEnabled("echo") {
		t.Fatal("echo still on after switching off")
	}
	if !s.SocialEnabled("gog") {
		t.Fatal("switching echo off switched gog off too")
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.SocialEnabled("echo") {
		t.Fatal("the choice was lost on restart")
	}
	if err := reopened.SetSocialEnabled("echo", true); err != nil {
		t.Fatal(err)
	}
	if !reopened.SocialEnabled("echo") {
		t.Fatal("echo still off after switching on")
	}
}
