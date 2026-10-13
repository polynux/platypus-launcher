package savebackup

import (
	"path/filepath"
	"testing"

	"platypus/internal/settings"
)

func newTestSettings(t *testing.T, home string) *settings.Store {
	t.Helper()
	st, err := settings.OpenWithHome(filepath.Join(t.TempDir(), "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	return st
}
