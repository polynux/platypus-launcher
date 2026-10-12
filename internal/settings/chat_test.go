package settings

import (
	"path/filepath"
	"testing"
)

// Message alerts are on for every plugin until the user turns one off, and the
// choice survives a restart.
func TestChatAlertsDefaultOnAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ChatAlerts("echo") {
		t.Fatal("chat alerts should be on by default")
	}
	if err := s.SetChatAlerts("echo", false); err != nil {
		t.Fatal(err)
	}
	if s.ChatAlerts("echo") || !s.ChatAlerts("gog") {
		t.Fatal("switching echo's alerts off changed another store, or didn't switch")
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ChatAlerts("echo") {
		t.Fatal("the choice was lost on restart")
	}
}

// Desktop notifications for chat are on until the user switches them off, and
// the choice survives a restart.
func TestChatDesktopNotifyDefaultOnAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ChatDesktopNotify() {
		t.Fatal("desktop notifications should be on by default")
	}
	if err := s.SetChatDesktopNotify(false); err != nil {
		t.Fatal(err)
	}
	if s.ChatDesktopNotify() {
		t.Fatal("desktop notifications still on after switching off")
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ChatDesktopNotify() {
		t.Fatal("the choice was lost on restart")
	}
	if err := reopened.SetChatDesktopNotify(true); err != nil {
		t.Fatal(err)
	}
	if !reopened.ChatDesktopNotify() {
		t.Fatal("switching back on didn't take")
	}
}
