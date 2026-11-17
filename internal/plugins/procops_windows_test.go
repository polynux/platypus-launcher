//go:build windows

package plugins

import (
	"syscall"
	"unsafe"
)

var (
	kernel32TestProc = syscall.NewLazyDLL("kernel32.dll")
	procTestOpen     = kernel32TestProc.NewProc("OpenProcess")
	procTestQuery    = kernel32TestProc.NewProc("GetExitCodeProcess")
	procTestTerm     = kernel32TestProc.NewProc("TerminateProcess")
	procTestClose    = kernel32TestProc.NewProc("CloseHandle")
)

const (
	testProcTerminate = 0x0001
	testProcQuery     = 0x1000
	testStillActive   = 259
)

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

// testProcessAlive reports whether a process handle opens and is still
// running.
func testProcessAlive(pid int) bool {
	h, _, _ := procTestOpen.Call(uintptr(testProcQuery), 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer procTestClose.Call(h)
	var code uintptr
	if ok, _, _ := procTestQuery.Call(h, uintptr(unsafe.Pointer(&code))); ok == 0 {
		return false
	}
	return code == testStillActive
}
