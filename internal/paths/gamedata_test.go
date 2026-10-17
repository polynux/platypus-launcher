package paths

import (
	"path/filepath"
	"testing"
)

func TestGameDataDirOverrideWins(t *testing.T) {
	t.Parallel()
	if got := GameDataDir("g1", "/mine", "/root"); got != "/mine" {
		t.Fatalf("override = %q, want /mine", got)
	}
	if got := GameDataDir("g1", "", "/root"); got != filepath.Join("/root", "g1") {
		t.Fatalf("default = %q, want /root/g1", got)
	}
}
