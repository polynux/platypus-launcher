//go:build windows

package session

import (
	"os"
	"os/exec"
)

// Windows currently stops only the tracked process; descendant/job-object support is missing.
func prepareProcess(*exec.Cmd) {}
func killProcesses(pid int, _ bool, _ string, _ bool) {
	// Windows process groups and descendant enumeration are not implemented yet.
	_ = (&os.Process{Pid: pid}).Kill()
}
