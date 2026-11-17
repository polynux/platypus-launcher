//go:build windows

package platform

import (
	"syscall"
	"unsafe"
)

var getDiskFreeSpaceExW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

// SpaceStats is the total and user-available bytes of the volume holding
// path, or its nearest existing parent. GetDiskFreeSpaceExW reports the
// selected volume's real capacity, which the Windows build had no answer for
// before (issue #83).
func SpaceStats(path string) (total, free uint64, err error) {
	name, err := syscall.UTF16PtrFromString(spacePath(path))
	if err != nil {
		return 0, 0, err
	}
	var available, bytes, totalFree uint64
	ok, _, callErr := getDiskFreeSpaceExW.Call(uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&bytes)), uintptr(unsafe.Pointer(&totalFree)))
	if ok == 0 {
		return 0, 0, callErr
	}
	return bytes, available, nil
}

// FreeSpace is the bytes available where path would land: the install
// dialog uses it to say whether an install fits, and a move's check compares
// it with the game's size.
func FreeSpace(path string) (uint64, error) {
	_, free, err := SpaceStats(path)
	return free, err
}
