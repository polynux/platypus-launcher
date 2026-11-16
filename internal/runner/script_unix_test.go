//go:build !windows

package runner

import (
	"os"
	"path/filepath"
	"testing"
)

// Issue #81: a script with no execute bit runs through the POSIX shell, .sh
// or #! alike; a plain file is not a script.
func TestScriptCommandUnix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, path := range []string{
		write("start.sh", []byte("echo hi\n")),
		write("start", []byte("#!/bin/sh\necho hi\n")),
	} {
		target, isScript, err := ScriptCommand(path)
		if err != nil || !isScript {
			t.Fatalf("%s: isScript = %v, err = %v", path, isScript, err)
		}
		if target.Exe != "/bin/sh" || len(target.Args) != 1 || target.Args[0] != path || !target.Native {
			t.Fatalf("%s: target = %+v", path, target)
		}
	}
	if _, isScript, err := ScriptCommand(write("game", []byte("\x7fELF"))); err != nil || isScript {
		t.Fatalf("ELF: isScript = %v, err = %v", isScript, err)
	}
}
