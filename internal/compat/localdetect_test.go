package compat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func labels(hits []LocalHit) map[string]bool {
	out := map[string]bool{}
	for _, h := range hits {
		out[h.Label] = true
	}
	return out
}

func TestScanFindsKnownMarkersOnly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Game.exe")
	writeFile(t, root, "steam_api64.dll") // every Steam game has it: not a marker
	writeFile(t, root, "EasyAntiCheat/EasyAntiCheat_EOS_Setup.exe")
	writeFile(t, root, "BattleEye/BEClient_x64.dll")
	writeFile(t, root, "redist/EpicOnlineServicesInstaller.exe")

	hits, truncated := ScanInstall(root)
	if truncated {
		t.Fatal("a small folder was truncated")
	}
	got := labels(hits)
	for _, want := range []string{"Easy Anti-Cheat", "BattlEye", "Epic Online Services"} {
		if !got[want] {
			t.Errorf("no %s in %+v", want, hits)
		}
	}
	if len(got) != 3 {
		t.Errorf("extra hits: %+v", hits)
	}
	for _, h := range hits {
		if strings.Contains(strings.ToLower(h.Name), "steam_api") || strings.Contains(strings.ToLower(h.Label), "denuvo") {
			t.Errorf("guessed marker %+v", h)
		}
	}
}

func TestScanStopsAtDepthLimit(t *testing.T) {
	root := t.TempDir()
	deep := root
	for i := 0; i < scanMaxDepth+2; i++ {
		deep = filepath.Join(deep, "d")
	}
	writeFile(t, deep, "BEClient.dll")
	if hits, _ := ScanInstall(root); len(hits) != 0 {
		t.Fatalf("found a marker below the depth limit: %+v", hits)
	}
}

func TestScanMissingFolderIsNothing(t *testing.T) {
	hits, truncated := ScanInstall(filepath.Join(t.TempDir(), "gone"))
	if len(hits) != 0 || truncated {
		t.Fatalf("hits %+v truncated %v", hits, truncated)
	}
}
