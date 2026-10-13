package savebackup

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/testenv"
)

// The full-size index path (issue #42): a manifest the size of the real one
// (~17 MB, thousands of games) splits on column-0 keys and builds without
// decoding the whole YAML at once. Synthetic data keeps it hermetic: no
// network, only PLATYPUS_TEST_SLOW=1.
func TestFullSizeIndexBuild(t *testing.T) {
	testenv.RequireSlow(t)
	var b strings.Builder
	b.WriteString("# synthetic full-size manifest\n")
	const games = 4000
	for i := 0; i < games; i++ {
		fmt.Fprintf(&b, "Synthetic Game %d:\n", i)
		b.WriteString("  aliases:\n    - SGOTY\n")
		fmt.Fprintf(&b, "  steam:\n    id: %d\n", 100000+i)
		b.WriteString("  files:\n    <winDocuments>/My Games/Synth/save*.sav:\n      tags: [save]\n")
		b.WriteString("    <winAppData>/Synth/settings.ini:\n      tags: [config]\n")
	}
	data := []byte(b.String())
	t.Logf("synthetic manifest: %.1f MB", float64(len(data))/(1<<20))
	chunks := SplitManifest(data)
	if len(chunks) != games {
		t.Fatalf("chunks = %d, want %d", len(chunks), games)
	}
	cache := filepath.Join(t.TempDir(), "cache")
	idx, err := BuildIndex(data, `"slow"`, cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Games) != games {
		t.Fatalf("index games = %d, want %d", len(idx.Games), games)
	}
	loaded := LoadIndex(cache)
	if loaded == nil || len(loaded.Games) != games {
		t.Fatalf("reloaded games = %d", len(loaded.GetGames()))
	}
	if m := Match(loaded, "synthetic game 3999", "", ""); m == nil || m.SteamID != "103999" {
		t.Error("title match in the full-size index failed")
	}
	if m := Match(loaded, "whatever", "100042", ""); m == nil {
		t.Error("steam-id match in the full-size index failed")
	}
}

func (x *Index) GetGames() []GameEntry {
	if x == nil {
		return nil
	}
	return x.Games
}
