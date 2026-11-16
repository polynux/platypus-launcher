package runner

import (
	"errors"
	"path/filepath"
	"testing"
)

// Issue #81: Windows picks an available interpreter (cmd.exe for .bat/.cmd,
// PowerShell for .ps1) and a script no Windows interpreter can run is a clear
// error, not a raw exec failure. The choice is pure, so Linux tests it.
func TestWindowsScriptCommand(t *testing.T) {
	t.Parallel()
	lookAll := func(name string) (string, error) { return `C:\Windows\` + name + ".exe", nil }
	lookNone := func(string) (string, error) { return "", errors.New("not found") }

	bat := filepath.Join("C:", "games", "run.bat")
	target, isScript, err := windowsScriptCommand(bat, false, lookNone)
	if err != nil || !isScript || target.Exe != "cmd.exe" || !target.Native {
		t.Fatalf("bat: %+v, %v, %v", target, isScript, err)
	}
	if len(target.Args) != 2 || target.Args[0] != "/c" || target.Args[1] != bat {
		t.Fatalf("bat args = %q", target.Args)
	}
	cmd := filepath.Join("C:", "games", "run.CMD")
	if target, _, _ := windowsScriptCommand(cmd, false, lookNone); target.Exe != "cmd.exe" {
		t.Fatalf("cmd: %+v", target)
	}

	ps1 := filepath.Join("C:", "games", "run.ps1")
	if target, _, err := windowsScriptCommand(ps1, false, lookAll); err != nil ||
		target.Exe != `C:\Windows\powershell.exe` || target.Args[len(target.Args)-1] != ps1 {
		t.Fatalf("ps1: %+v, %v", target, err)
	}
	if _, isScript, err := windowsScriptCommand(ps1, false, lookNone); !isScript || err == nil {
		t.Fatalf("ps1 without PowerShell: isScript = %v, err = %v", isScript, err)
	}

	if _, isScript, err := windowsScriptCommand(filepath.Join("C:", "g", "run.sh"), false, lookAll); !isScript || err == nil {
		t.Fatalf("sh on Windows: isScript = %v, err = %v", isScript, err)
	}
	if _, isScript, err := windowsScriptCommand(filepath.Join("C:", "g", "start"), true, lookAll); !isScript || err == nil {
		t.Fatalf("shebang on Windows: isScript = %v, err = %v", isScript, err)
	}
	if _, isScript, err := windowsScriptCommand(filepath.Join("C:", "g", "game.exe"), false, lookAll); isScript || err != nil {
		t.Fatalf("exe is not a script: isScript = %v, err = %v", isScript, err)
	}
}
