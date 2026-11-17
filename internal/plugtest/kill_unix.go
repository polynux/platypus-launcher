//go:build !windows

package plugtest

import "syscall"

// killPid ends a plugin process abruptly, the way a crash does.
func killPid(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }
