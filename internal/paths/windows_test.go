package paths

import (
	"path/filepath"
	"testing"
)

// The Windows defaults are pure functions so they are checked here, on Linux
// (issue #82): data under %LOCALAPPDATA%, config under %APPDATA%, cache under
// data.

func TestWindowsAppDirs(t *testing.T) {
	home := filepath.Join("C:", "Users", "u")
	local := filepath.Join(home, "AppData", "Local")
	roaming := filepath.Join(home, "AppData", "Roaming")
	if got := windowsAppDir(local, home, filepath.Join("AppData", "Local")); got != filepath.Join(local, AppName) {
		t.Fatalf("data with LOCALAPPDATA = %q", got)
	}
	if got := windowsAppDir("", home, filepath.Join("AppData", "Local")); got != filepath.Join(local, AppName) {
		t.Fatalf("data fallback = %q", got)
	}
	if got := windowsAppDir(roaming, home, filepath.Join("AppData", "Roaming")); got != filepath.Join(roaming, AppName) {
		t.Fatalf("config with APPDATA = %q", got)
	}
	if got := windowsAppDir("", home, filepath.Join("AppData", "Roaming")); got != filepath.Join(roaming, AppName) {
		t.Fatalf("config fallback = %q", got)
	}
	if got := windowsCacheDir(local, home); got != filepath.Join(local, AppName, "cache") {
		t.Fatalf("cache = %q", got)
	}
	if got := windowsCacheDir("", home); got != filepath.Join(local, AppName, "cache") {
		t.Fatalf("cache fallback = %q", got)
	}
}

func TestWindowsSaveDirs(t *testing.T) {
	home := filepath.Join("C:", "Users", "u")
	local := filepath.Join(home, "AppData", "Local")
	dirs := windowsSaveDirs(home, "", "", "", "", "")
	want := map[string]string{
		"home":            home,
		"documents":       filepath.Join(home, "Documents"),
		"appData":         filepath.Join(home, "AppData", "Roaming"),
		"localAppData":    local,
		"localAppDataLow": filepath.Join(home, "AppData", "LocalLow"),
	}
	for label, path := range want {
		if dirs[label] != path {
			t.Errorf("%s = %q, want %q", label, dirs[label], path)
		}
	}
	for _, label := range []string{"public", "programData", "windir"} {
		if _, ok := dirs[label]; ok {
			t.Errorf("%s = %q, want omitted when the host does not name it", label, dirs[label])
		}
	}

	system := filepath.Join("D:", "System")
	dirs = windowsSaveDirs(home, system, "", system, system, system)
	if dirs["appData"] != system || dirs["public"] != system || dirs["programData"] != system || dirs["windir"] != system {
		t.Fatalf("system folders = %v", dirs)
	}
	if dirs["localAppData"] != local {
		t.Fatalf("localAppData = %q, want profile fallback", dirs["localAppData"])
	}
}
