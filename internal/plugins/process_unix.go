//go:build !windows

package plugins

import (
	"os/exec"
	"syscall"
)

func configurePluginProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}

func killPluginProcess(cmd *exec.Cmd) { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }

// afterPluginStart is Windows-only: the job object can only be assigned once
// the process exists. Unix sets everything it needs before Start.
func afterPluginStart(*exec.Cmd) {}
