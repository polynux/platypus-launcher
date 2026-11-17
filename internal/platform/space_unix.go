//go:build !windows

package platform

import "syscall"

// SpaceStats is the total and user-available bytes of the filesystem holding
// path, or its nearest existing parent: what statfs reports for the mount.
func SpaceStats(path string) (total, free uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(spacePath(path), &st); err != nil {
		return 0, 0, err
	}
	return st.Blocks * uint64(st.Bsize), st.Bavail * uint64(st.Bsize), nil
}

// FreeSpace is the bytes available where path would land: the install
// dialog uses it to say whether an install fits, and a move's check compares
// it with the game's size.
func FreeSpace(path string) (uint64, error) {
	_, free, err := SpaceStats(path)
	return free, err
}
