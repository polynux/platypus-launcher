//go:build windows

package plugtest

import "syscall"

var (
	kernel32TestKill = syscall.NewLazyDLL("kernel32.dll")
	procTestOpenProc = kernel32TestKill.NewProc("OpenProcess")
	procTestTermProc = kernel32TestKill.NewProc("TerminateProcess")
	procTestCloseHan = kernel32TestKill.NewProc("CloseHandle")
)

const testProcessTerminate = 0x0001

// killPid ends a plugin process abruptly, the way a crash does.
func killPid(pid int) error {
	h, _, _ := procTestOpenProc.Call(uintptr(testProcessTerminate), 0, uintptr(pid))
	if h == 0 {
		return syscall.Errno(5) // access denied: the pid is already gone
	}
	defer procTestCloseHan.Call(h)
	ok, _, _ := procTestTermProc.Call(h, 1)
	if ok == 0 {
		return syscall.Errno(5)
	}
	return nil
}
