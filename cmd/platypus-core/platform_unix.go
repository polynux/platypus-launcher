//go:build !windows

package main

import (
	"os"
	"strconv"
	"syscall"
)

func protocolFile() (*os.File, error) {
	protoFd, err := syscall.Dup(1)
	if err != nil {
		return nil, err
	}
	syscall.CloseOnExec(protoFd)
	if err := syscall.Dup3(2, 1, 0); err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(protoFd), "protocol"), nil
}
func closeInheritedFds() {
	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	for _, fd := range fds {
		name := fd.Name()
		if name == "0" || name == "1" || name == "2" {
			continue
		}
		if n, err := strconv.Atoi(name); err == nil {
			syscall.Close(n)
		}
	}
}
func shutdownSignals() []os.Signal { return []os.Signal{syscall.SIGTERM, syscall.SIGINT} }
