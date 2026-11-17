package platform

import "testing"

// Issue #83: a Windows path names its volume by drive or share, whatever the
// separator or the case.
func TestWindowsVolumeRoot(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]string{
		`C:\Games`:             `C:\`,
		`c:/games/steam`:       `C:\`,
		`D:\`:                  `D:\`,
		`\\server\share\games`: `\\server\share\`,
		`\\server\share`:       `\\server\share\`,
		`\\server`:             ``,
		`Games\local`:          ``,
		`relative/games`:       ``,
		``:                     ``,
	} {
		if got := windowsVolumeRoot(path); got != want {
			t.Errorf("windowsVolumeRoot(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestWindowsIsRoot(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]bool{
		`C:\`:                  true,
		`c:/`:                  true,
		`D:\`:                  true,
		`\\server\share\`:      true,
		`\\server\share`:       true,
		`C:\Windows`:           false,
		`C:\Games\x`:           false,
		`\\server\share\games`: false,
		`Games`:                false,
		`/`:                    false,
	} {
		if got := windowsIsRoot(path); got != want {
			t.Errorf("windowsIsRoot(%q) = %v, want %v", path, got, want)
		}
	}
}

// The protected folders come from the environment; a machine without it
// still protects the defaults on its system drive.
func TestWindowsProtectedDirs(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		"SystemDrive":       "D:",
		"SystemRoot":        `D:\WinNT`,
		"windir":            `D:\WinNT`,
		"ProgramFiles":      `D:\Program Files`,
		"ProgramFiles(x86)": `D:\Program Files (x86)`,
		"ProgramData":       `D:\ProgramData`,
	}
	got := windowsProtectedDirs(func(k string) string { return env[k] })
	want := map[string]bool{
		`D:\WinNT`:               false,
		`D:\Program Files`:       false,
		`D:\Program Files (x86)`: false,
		`D:\ProgramData`:         false,
		// Always protected, whatever the environment names the folder.
		`D:\Windows`: false,
	}
	if len(got) != len(want) {
		t.Fatalf("dirs = %v, want each of %v once", got, want)
	}
	for _, dir := range got {
		if _, ok := want[dir]; !ok {
			t.Fatalf("dirs = %v, want each of %v once", got, want)
		}
		delete(want, dir)
	}

	defaults := windowsProtectedDirs(func(string) string { return "" })
	if len(defaults) != 4 {
		t.Fatalf("dirs without an environment = %v, want the four defaults", defaults)
	}
	for _, dir := range defaults {
		if len(dir) < 3 || dir[:3] != `C:\` {
			t.Fatalf("dirs without an environment = %v, want them on C:", defaults)
		}
	}
}

func TestWindowsIsProtected(t *testing.T) {
	t.Parallel()
	dirs := []string{`C:\Windows`, `C:\Program Files`, `C:\ProgramData`}
	for path, want := range map[string]bool{
		`C:\Windows`:                  true,
		`c:\windows`:                  true,
		`C:/Windows/System32`:         true,
		`C:\WindowsApps`:              false,
		`C:\Program Files\Game`:       true,
		`C:\Program Files (x86)\Game`: false,
		`C:\ProgramData\Platypus`:     true,
		`C:\Games`:                    false,
		`C:\Games\Windows`:            false,
		`D:\Windows`:                  false,
	} {
		if got := windowsIsProtected(path, dirs); got != want {
			t.Errorf("windowsIsProtected(%q) = %v, want %v", path, got, want)
		}
	}
}

// A folder that doesn't exist yet has no filesystem: the query runs on its
// nearest existing parent.
func TestSpacePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := spacePath(dir); got != dir {
		t.Fatalf("spacePath(%q) = %q, want it as it is", dir, got)
	}
	if got := spacePath(dir + "/missing/child"); got != dir {
		t.Fatalf("spacePath of a missing child = %q, want %q", got, dir)
	}
}
