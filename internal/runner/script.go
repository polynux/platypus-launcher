package runner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Script launches (issue #81): a script with no execute bit runs through a
// host interpreter. Which interpreter is available is per-OS, so each host's
// choice lives behind a build tag (script_unix.go, script_windows.go); the
// Windows choice itself stays a pure function here, testable on any host.

// hasShebang says whether the file starts with "#!", so a shell can run it
// whatever its name.
func hasShebang(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 2)
	n, _ := io.ReadFull(f, buf)
	return n == 2 && buf[0] == '#' && buf[1] == '!'
}

// windowsScriptCommand is the Windows interpreter choice: .bat/.cmd run
// through cmd.exe /c, .ps1 through PowerShell when one is installed. A .sh or
// #! script has no Windows interpreter, so it returns a clear error instead
// of running an unrelated program. look reports whether a program is on PATH.
func windowsScriptCommand(path string, shebang bool, look func(string) (string, error)) (Target, bool, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".bat", ".cmd":
		return Target{Exe: "cmd.exe", Args: []string{"/c", path}, Native: true}, true, nil
	case ".ps1":
		for _, name := range []string{"powershell", "pwsh"} {
			if exe, err := look(name); err == nil {
				return Target{Exe: exe, Args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path}, Native: true}, true, nil
			}
		}
		return Target{}, true, fmt.Errorf("%s is a PowerShell script, but no PowerShell was found", path)
	}
	if ext == ".sh" || shebang {
		return Target{}, true, fmt.Errorf("%s is a Unix shell script, which this Windows host cannot run", path)
	}
	return Target{}, false, nil
}
