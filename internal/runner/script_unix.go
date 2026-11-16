//go:build !windows

package runner

import (
	"path/filepath"
	"strings"
)

// ScriptCommand returns the command that runs the script at path on this
// host: /bin/sh when it is a .sh file or starts with "#!". isScript is false
// for anything else.
func ScriptCommand(path string) (Target, bool, error) {
	if strings.EqualFold(filepath.Ext(path), ".sh") || hasShebang(path) {
		return Target{Exe: "/bin/sh", Args: []string{path}, Native: true}, true, nil
	}
	return Target{}, false, nil
}
