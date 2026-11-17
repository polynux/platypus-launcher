//go:build !windows

package plugins

import "syscall"

// killTestProcess ends a process abruptly, the way a crash does.
func killTestProcess(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }

// testProcessAlive reports whether a process exists (signal zero).
func testProcessAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }
