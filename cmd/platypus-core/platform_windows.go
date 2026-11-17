//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetStdHandle    = kernel32.NewProc("GetStdHandle")
	procSetStdHandle    = kernel32.NewProc("SetStdHandle")
	procDuplicateHandle = kernel32.NewProc("DuplicateHandle")
	procGetCurrentProc  = kernel32.NewProc("GetCurrentProcess")
)

const (
	stdOutputHandle = 0xFFFFFFF5 // (uint32)-11
	stdErrorHandle  = 0xFFFFFFF4 // (uint32)-12

	duplicateSameAccess = 2
)

// protocolFile keeps a non-inheritable duplicate of stdout for the protocol
// and points fd 1 at stderr, mirroring the Unix side: stray prints can't
// corrupt the JSON-RPC stream, and children the core spawns never inherit
// the protocol handle (Go's exec only passes the stdio handles set up for
// each child via PROC_THREAD_ATTRIBUTE_HANDLE_LIST).
func protocolFile() (*os.File, error) {
	stdout, _, _ := procGetStdHandle.Call(uintptr(stdOutputHandle))
	if stdout == 0 || stdout == uintptr(^uintptr(0)) {
		return os.Stdout, nil
	}
	current, _, _ := procGetCurrentProc.Call()
	var dup uintptr
	ok, _, _ := procDuplicateHandle.Call(current, stdout, current,
		uintptr(unsafe.Pointer(&dup)), 0, 0, uintptr(duplicateSameAccess))
	if ok == 0 {
		return os.Stdout, nil
	}
	stderr, _, _ := procGetStdHandle.Call(uintptr(stdErrorHandle))
	if stderr != 0 && stderr != uintptr(^uintptr(0)) {
		procSetStdHandle.Call(uintptr(stdOutputHandle), stderr)
		os.Stdout = os.Stderr
		syscall.Stdout = syscall.Stderr
	}
	return os.NewFile(uintptr(dup), "protocol"), nil
}

// closeInheritedFds has no safe Windows equivalent: unlike Unix fds, Windows
// handles are inherited only when listed explicitly (Go's exec passes just
// the child's stdio handles via PROC_THREAD_ATTRIBUTE_HANDLE_LIST), so the
// shell's leaked handles don't reach the core's children anyway. Walking our
// own handle table to close unknowns would risk closing handles the runtime
// owns.
func closeInheritedFds() {}

func shutdownSignals() []os.Signal { return []os.Signal{os.Interrupt} }
