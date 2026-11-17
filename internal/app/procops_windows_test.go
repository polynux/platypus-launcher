//go:build windows

package app

import "syscall"

var (
	kernel32TestProc = syscall.NewLazyDLL("kernel32.dll")
	procTestOpen     = kernel32TestProc.NewProc("OpenProcess")
	procTestTerm     = kernel32TestProc.NewProc("TerminateProcess")
	procTestClose    = kernel32TestProc.NewProc("CloseHandle")
)

const testProcTerminate = 0x0001

// killTestProcess ends a process abruptly, the way a crash does.
func killTestProcess(pid int) error {
	h, _, _ := procTestOpen.Call(uintptr(testProcTerminate), 0, uintptr(pid))
	if h == 0 {
		return syscall.Errno(5)
	}
	defer procTestClose.Call(h)
	if ok, _, _ := procTestTerm.Call(h, 1); ok == 0 {
		return syscall.Errno(5)
	}
	return nil
}
