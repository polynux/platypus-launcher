package jobs

import (
	"os"
	"path/filepath"
	"testing"
)

// CORE-12: persistence must be durable (Sync before rename, mirroring
// settings) and state-save failures must be visible instead of swallowed.
func TestSaveRoundTrip(t *testing.T) {
	b := &fakeBackend{}
	q, path := newQueue(t, b)
	if _, err := q.Add(KindInstall, "echo", "game-1", "tiny", "Tiny", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("jobs file missing after add: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("jobs file empty after add")
	}
	reopened, err := Open(path, b, nil)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	list := reopened.List()
	if len(list) != 1 || list[0].GameID != "game-1" {
		t.Fatalf("progress lost across reopen: %+v", list)
	}
}

func TestStateSaveErrorsSurfaced(t *testing.T) {
	blocker, err := os.CreateTemp(t.TempDir(), "blocker")
	if err != nil {
		t.Fatal(err)
	}
	blocker.Close()
	bad := filepath.Join(blocker.Name(), "jobs.json") // parent is a file: no save can succeed
	q := &Queue{path: bad}
	if err := q.save(); err == nil {
		t.Fatal("save to an impossible path succeeded")
	}
	q.mu.Lock()
	q.changed(true)
	q.mu.Unlock()
	if q.SaveErrors() == 0 {
		t.Fatal("state-save failure was swallowed: SaveErrors() == 0")
	}
	if q.LastSaveError() == "" {
		t.Fatal("LastSaveError() empty after failed save")
	}
}
