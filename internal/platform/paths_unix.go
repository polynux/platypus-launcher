//go:build !windows

package platform

import "strings"

// unixSystemDirs are the folders the OS owns: no Platypus action deletes,
// installs into or allows games from one.
var unixSystemDirs = []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib32", "/lib64", "/proc", "/run", "/sbin", "/sys", "/usr", "/var"}

// VolumeRoot is the root of the volume holding path: Unix has one filesystem
// root, whatever the path.
func VolumeRoot(string) string { return "/" }

// IsRoot says whether path is the filesystem root.
func IsRoot(path string) bool { return path == "/" }

// IsProtected says whether path is one of the OS's own folders or inside one.
func IsProtected(path string) bool {
	for _, dir := range unixSystemDirs {
		if path == dir || strings.HasPrefix(path, dir+"/") {
			return true
		}
	}
	return false
}
