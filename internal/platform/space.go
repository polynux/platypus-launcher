// Package platform answers what the core asks about the host filesystem:
// the room a volume has, which volume a path belongs to, what a filesystem
// root is and which folders the OS owns (issue #83). Each answer comes from a
// build-tagged implementation; the Windows path rules are pure functions, so
// the Linux test run covers them too.
package platform

import (
	"errors"
	"os"
	"path/filepath"
)

// spacePath is the folder a space query runs on: path itself when it exists,
// else its nearest existing parent. A folder that isn't created yet has no
// filesystem of its own, and Windows fails the query outright on it.
func spacePath(path string) string {
	for {
		_, err := os.Stat(path)
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}
