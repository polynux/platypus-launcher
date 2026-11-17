//go:build !windows

package app

import "syscall"

// killTestProcess ends a process abruptly, the way a crash does.
func killTestProcess(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }
