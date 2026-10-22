//go:build windows

package jobs

import "errors"

// Windows volume free-space reporting is not implemented yet.
func spaceStats(string) (uint64, uint64, error) {
	return 0, 0, errors.New("filesystem space reporting is unavailable on Windows")
}
func freeSpace(string) (uint64, error) {
	return 0, errors.New("filesystem space reporting is unavailable on Windows")
}
