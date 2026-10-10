package settings

import (
	"path/filepath"
	"testing"
)

func TestCloudModesPersistAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	if s.CloudPluginMode("echo") != "" || s.CloudGameMode("p.echo.tiny") != "" {
		t.Fatal("modes should start unset")
	}
	if err := s.SetCloudPluginMode("echo", "auto"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloudGameMode("p.echo.tiny", "off"); err != nil {
		t.Fatal(err)
	}
	again, _ := Open(path)
	if again.CloudPluginMode("echo") != "auto" || again.CloudGameMode("p.echo.tiny") != "off" {
		t.Fatalf("not persisted: %q %q", again.CloudPluginMode("echo"), again.CloudGameMode("p.echo.tiny"))
	}
	pm, gm := again.CloudModes()
	if len(pm) != 1 || len(gm) != 1 {
		t.Fatalf("%v %v", pm, gm)
	}
	if err := again.SetCloudGameMode("p.echo.tiny", ""); err != nil {
		t.Fatal(err)
	}
	if again.CloudGameMode("p.echo.tiny") != "" {
		t.Fatal("clearing a game's mode left it set")
	}
}

func TestCloudModeRejectsNonsense(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err := s.SetCloudPluginMode("echo", "always"); err == nil {
		t.Fatal("accepted an unknown mode")
	}
	if err := s.SetCloudGameMode("", "off"); err == nil {
		t.Fatal("accepted an empty game id")
	}
	if s.CloudPluginMode("echo") != "" {
		t.Fatal("a rejected mode was stored")
	}
}
