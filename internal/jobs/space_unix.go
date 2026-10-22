//go:build !windows

package jobs

import (
	"errors"
	"path/filepath"
	"syscall"
)

func spaceStats(path string) (uint64, uint64, error) {
	for {
		var st syscall.Statfs_t
		err := syscall.Statfs(path, &st)
		if err == nil {
			return st.Blocks * uint64(st.Bsize), st.Bavail * uint64(st.Bsize), nil
		}
		parent := filepath.Dir(path)
		if !errors.Is(err, syscall.ENOENT) || parent == path {
			return 0, 0, err
		}
		path = parent
	}
}
func freeSpace(path string) (uint64, error) {
	_, free, err := spaceStats(path)
	return free, err
}
