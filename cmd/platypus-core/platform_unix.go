//go:build !windows

package main

import (
	"os"
	"strconv"
	"syscall"
)

// protocolFile keeps a close-on-exec duplicate of stdout for the protocol and
// points fd 1 at stderr.
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

// closeInheritedFds closes every fd but stdin/stdout/stderr: Chromium leaves
// files and sockets without close-on-exec and libuv does not close them at
// spawn, so the core would otherwise hold the GPU cache, Local Storage, even
// the remote debugger's socket (and its own plugins and games would inherit
// them in turn).
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
