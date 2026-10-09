package jobs

import (
	"path/filepath"
	"testing"

	"hydra-launcher/internal/types"
)

// Empty choices are no choices: a job that sent `options: {}` in memory would
// send nothing after a restart, where the JSON round trip drops the empty map.
func TestEmptyChoicesAreNoChoices(t *testing.T) {
	q, _ := newQueue(t, &fakeBackend{})
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	j, err := q.AddWithOptions(KindInstall, "echo", "g", "tiny", "Tiny", dir, nil, types.InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if j.Options != nil {
		t.Fatalf("empty choices queued as %#v, want nil", j.Options)
	}
}
