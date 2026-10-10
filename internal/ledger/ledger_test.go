package ledger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/types"
)

func entry(game, build, channel string) types.LedgerEntry {
	return types.LedgerEntry{Store: "echo", Game: game, Build: build, Channel: channel, VersionName: "v " + build}
}

func TestAppendPersistsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "install-history.json")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	l.now = func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) }
	e := entry("g1", "b200", "legacy")
	e.ArchiveRef = json.RawMessage(`{ "manifest": "m-200.json" }`)
	if err := l.Append(e); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(entry("g1", "b300", "latest")); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(entry("g2", "b100", "")); err != nil {
		t.Fatal(err)
	}

	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := again.Entries("g1")
	if len(got) != 2 || got[0].Build != "b200" || got[1].Build != "b300" {
		t.Fatalf("g1 history = %+v, want b200 then b300", got)
	}
	if got[0].Date != "2026-10-09T12:00:00Z" {
		t.Errorf("date = %q, want the append time in RFC 3339 UTC", got[0].Date)
	}
	var ref bytes.Buffer
	if err := json.Compact(&ref, got[0].ArchiveRef); err != nil || ref.String() != `{"manifest":"m-200.json"}` {
		t.Errorf("archiveRef = %s (%v), want the original", got[0].ArchiveRef, err)
	}
	if g2 := again.Entries("g2"); len(g2) != 1 || g2[0].Channel != "" {
		t.Errorf("g2 history = %+v", g2)
	}
	if none := again.Entries("nobody"); none == nil || len(none) != 0 {
		t.Errorf("an unknown game has an empty (non-nil) history, got %#v", none)
	}
}

func TestReinstallingTheLatestBuildRefreshesItsRow(t *testing.T) {
	l, _ := Open(filepath.Join(t.TempDir(), "h.json"))
	first := entry("g", "b300", "latest")
	first.Date = "2026-01-01T00:00:00Z"
	second := entry("g", "b300", "latest")
	second.Date = "2026-02-02T00:00:00Z"
	second.VersionName = "3.0.0 (renamed)"
	for _, e := range []types.LedgerEntry{first, entry("g", "b200", ""), second} {
		if err := l.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	got := l.Entries("g")
	if len(got) != 3 {
		t.Fatalf("b300, b200, b300 are three installs, got %+v", got)
	}
	// Same build twice in a row is one row, with the newest details.
	if err := l.Append(second); err != nil {
		t.Fatal(err)
	}
	third := second
	third.Date = "2026-03-03T00:00:00Z"
	if err := l.Append(third); err != nil {
		t.Fatal(err)
	}
	got = l.Entries("g")
	if len(got) != 3 || got[2].Date != "2026-03-03T00:00:00Z" {
		t.Fatalf("repeat of the last build should refresh it, got %+v", got)
	}
}

func TestAppendRefusesBadEntriesAndKeepsTheFileIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.json")
	l, _ := Open(path)
	if err := l.Append(entry("g", "b1", "")); err != nil {
		t.Fatal(err)
	}
	bad := map[string]types.LedgerEntry{
		"no store": {Game: "g", Build: "b2"},
		"no game":  {Store: "echo", Build: "b2"},
		"no build": {Store: "echo", Game: "g"},
		"bad ref":  {Store: "echo", Game: "g", Build: "b2", ArchiveRef: json.RawMessage(`{nope`)},
		"huge ref": {Store: "echo", Game: "g", Build: "b2", ArchiveRef: json.RawMessage(`"` + strings.Repeat("x", maxArchiveRef) + `"`)},
	}
	for name, e := range bad {
		if err := l.Append(e); err == nil {
			t.Errorf("%s: appended", name)
		}
	}
	if got := l.Entries("g"); len(got) != 1 {
		t.Fatalf("history changed by refused entries: %+v", got)
	}
}

func TestHistoryIsBoundedPerGame(t *testing.T) {
	l, _ := Open(filepath.Join(t.TempDir(), "h.json"))
	for i := 0; i < maxPerGame+10; i++ {
		if err := l.Append(entry("g", "b"+strings.Repeat("1", 1)+string(rune('a'+i%26))+string(rune('a'+i/26)), "")); err != nil {
			t.Fatal(err)
		}
	}
	got := l.Entries("g")
	if len(got) != maxPerGame {
		t.Fatalf("kept %d entries, want %d", len(got), maxPerGame)
	}
}

func TestCorruptFileIsMovedAsideNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "h.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := Open(path)
	if err == nil {
		t.Fatal("a corrupt history should be reported")
	}
	if err := l.Append(entry("g", "b1", "")); err != nil {
		t.Fatalf("the ledger is still usable: %v", err)
	}
	moved, _ := filepath.Glob(path + ".bad-*")
	if len(moved) != 1 {
		t.Fatalf("corrupt file not moved aside: %v", moved)
	}
	if raw, _ := os.ReadFile(moved[0]); string(raw) != "{broken" {
		t.Fatalf("moved file changed: %q", raw)
	}
}

func TestAnUnreadableHistoryIsNeverWrittenOver(t *testing.T) {
	// A history file that exists but can't be read (here: a directory in its place)
	// must not turn into an empty history that the next install saves over.
	path := filepath.Join(t.TempDir(), "h.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := Open(path)
	if err == nil {
		t.Fatal("an unreadable history should be reported")
	}
	if err := l.Append(entry("g", "b1", "")); err == nil {
		t.Fatal("the read-only ledger accepted an install")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("the history file was replaced: %v %v", info, err)
	}
}

func TestTheHistoryOfAllGamesIsBounded(t *testing.T) {
	l, _ := Open(filepath.Join(t.TempDir(), "h.json"))
	l.maxTotal = 3
	l.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	for i, g := range []string{"a", "b", "a", "b"} {
		e := entry(g, "b"+string(rune('1'+i)), "")
		e.Date = time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
		if err := l.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(l.Entries("a")) + len(l.Entries("b")); n != 3 {
		t.Fatalf("kept %d entries, want 3", n)
	}
	// The first install (a, b1) was the oldest of all: it is the one that went.
	if got := l.Entries("a"); len(got) != 1 || got[0].Build != "b3" {
		t.Fatalf("a = %+v", got)
	}
	again, _ := Open(l.path)
	if n := len(again.Entries("a")) + len(again.Entries("b")); n != 3 {
		t.Fatalf("the bound was not saved: %d entries", n)
	}
}
