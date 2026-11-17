//go:build windows

package platform

import "os"

// VolumeRoot is the root of the volume holding path: the drive ("C:\") or
// the share (`\\server\share\`).
func VolumeRoot(path string) string { return windowsVolumeRoot(path) }

// IsRoot says whether path is a drive or share root.
func IsRoot(path string) bool { return windowsIsRoot(path) }

// IsProtected says whether path is a folder Windows owns or inside one.
func IsProtected(path string) bool {
	return windowsIsProtected(path, windowsProtectedDirs(os.Getenv))
}
