//go:build windows

package runner

import "os/exec"

// ScriptCommand returns the command that runs the script at path on Windows:
// cmd.exe for .bat/.cmd, PowerShell for .ps1 when present. isScript is false
// for anything else; a script with no Windows interpreter returns the error.
func ScriptCommand(path string) (Target, bool, error) {
	return windowsScriptCommand(path, hasShebang(path), exec.LookPath)
}
