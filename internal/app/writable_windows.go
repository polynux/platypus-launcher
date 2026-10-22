//go:build windows

package app

import (
	"errors"
	"os"
)

// Windows ACL-aware writability checks are not implemented; this checks the directory's mode bits.
func checkDirectoryWritable(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.Mode().Perm()&0222 == 0 {
		return errors.New("directory is not writable")
	}
	return nil
}
