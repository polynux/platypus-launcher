package stores

import (
	"path/filepath"
	"testing"

	"platypus/internal/plugins"
	"platypus/internal/settings"
)

// The library tells the UI per game whether Allow is possible (issue #173):
// a system folder, the home folder and the root never allow, a normal outside
// folder does, and an already-allowed folder needs no allow.
func TestToGameCanAllowFolder(t *testing.T) {
	st, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := plugins.NewManager(plugins.Options{Dir: t.TempDir(), Settings: st, Home: "/home/tester"})
	p := plugins.Plugin{Manifest: plugins.Manifest{ID: "echo", Store: "echo"}}
	s := NewPlugin(m, p, nil)
	folders := []string{"/games/echo"}
	cases := []struct {
		name       string
		installDir string
		outside    bool
		canAllow   bool
		reason     string
	}{
		{"system", "/usr/bin", true, false, "system"},
		{"system child", "/usr/bin/wine", true, false, "system"},
		{"home", "/home/tester", true, false, "home"},
		{"root", "/", true, false, "root"},
		{"normal outside", "/opt/games/escape", true, true, ""},
		{"already allowed", "/games/echo/inside", false, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := s.toGame(plugins.LibraryGame{ID: "g", Installed: true, InstallPath: c.installDir}, folders)
			if g.OutsideFolder != c.outside {
				t.Errorf("OutsideFolder = %v, want %v", g.OutsideFolder, c.outside)
			}
			if g.CanAllowFolder != c.canAllow {
				t.Errorf("CanAllowFolder = %v, want %v", g.CanAllowFolder, c.canAllow)
			}
			if g.CanAllowFolderReason != c.reason {
				t.Errorf("CanAllowFolderReason = %q, want %q", g.CanAllowFolderReason, c.reason)
			}
		})
	}
}
