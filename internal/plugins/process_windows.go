//go:build windows

package plugins

import "os/exec"

// Windows job objects and parent-death cleanup are not implemented yet.
func configurePluginProcess(*exec.Cmd) {}
func killPluginProcess(cmd *exec.Cmd)  { _ = cmd.Process.Kill() }
