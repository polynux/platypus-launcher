//go:build windows

package app

import (
	"os/exec"
)

// detachProcess is a no-op on Windows: installers run natively and are
// stopped with Process.Kill (see killProcessTree).
func detachProcess(cmd *exec.Cmd) {}

// killProcessTree kills the installer process. Descendants are best effort:
// the wineserver stop that follows a cancel ends what runs in the prefix.
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
