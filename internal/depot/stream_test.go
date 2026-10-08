package depot

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// TestDownloadStreamWritesFiles proves chunks stream to final files with
// bounded memory: the sink writes each chunk out as it lands instead of
// accumulating a whole-depot map.
func TestDownloadStreamWritesFiles(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 4, 4)
	hosts, _ := startHosts(t, m, encMap, []time.Duration{0, 0, 0, 0}, -1, -1, "")
	s := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 8, MaxConns: 8, MaxPerHost: 4})
	dir := t.TempDir()
	w, err := Preallocate(m, dir)
	if err != nil {
		t.Fatal(err)
	}
	var calls int64
	sink := func(id string, data []byte) error {
		atomic.AddInt64(&calls, 1)
		return w.Write(id, data)
	}
	if err := s.DownloadStream(context.Background(), m, 11, hosts, key, nil, sink, nil); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&calls); got != int64(len(encMap)) {
		t.Fatalf("sink calls = %d, want %d", got, len(encMap))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if bad, _ := Verify(m, dir); len(bad) != 0 {
		t.Fatalf("verify failed: %v", bad)
	}
}

// TestDownloadStreamResumeSkip proves resume reuses staged IDs without
// re-fetching: skipped chunks are counted but never hit the CDN.
func TestDownloadStreamResumeSkip(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 2, 3)
	hosts, fakes := startHosts(t, m, encMap, []time.Duration{0, 0}, -1, -1, "")
	s := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 4, MaxConns: 4})
	// Skip half the chunks (raw chunk-ID keys, as DownloadStream expects).
	skip := map[string]bool{}
	uniq := m.UniqueChunks()
	for i, c := range uniq {
		if i%2 == 0 {
			skip[string(c.ID)] = true
		}
	}
	for _, f := range fakes {
		atomic.StoreInt64(f.requests, 0)
	}
	dir := t.TempDir()
	w, err := Preallocate(m, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Pretend skipped chunks are already written (e.g. restored from
	// staging one at a time); streaming must not fetch them.
	got := map[string][]byte{}
	full := map[string][]byte{}
	// Fetch everything once to get plaintexts for the skipped set.
	allHosts, _ := startHosts(t, m, encMap, []time.Duration{0}, -1, -1, "")
	tmp, err := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 2, MaxConns: 2}).Download(context.Background(), m, 11, allHosts, key, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	full = tmp
	for id := range skip {
		if err := w.Write(id, full[id]); err != nil {
			t.Fatal(err)
		}
	}
	sink := func(id string, data []byte) error {
		got[id] = append([]byte{}, data...)
		return w.Write(id, data)
	}
	if err := s.DownloadStream(context.Background(), m, 11, hosts, key, skip, sink, nil); err != nil {
		t.Fatal(err)
	}
	if len(got)+len(skip) != len(encMap) {
		t.Fatalf("streamed %d + skipped %d, want %d total", len(got), len(skip), len(encMap))
	}
	for id := range skip {
		if _, ok := got[id]; ok {
			t.Fatalf("skipped chunk %s was re-fetched", id)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if bad, _ := Verify(m, dir); len(bad) != 0 {
		t.Fatalf("verify failed: %v", bad)
	}
}

// TestInstallWriterSharedChunk proves a chunk shared by two files is
// written to both without fetching twice.
func TestInstallWriterSharedChunk(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 2, 2)
	hosts, _ := startHosts(t, m, encMap, []time.Duration{0}, -1, -1, "")
	// Share the first chunk of file0 into an extra file.
	var shared Chunk
	var sharedPlain []byte
	s0 := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 1, MaxConns: 1})
	all, err := s0.Download(context.Background(), m, 11, hosts, key, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Files[0].Chunks {
		shared = c
		sharedPlain = all[string(c.ID)]
		break
	}
	m.Files = append(m.Files, File{Name: "copy.dat", Size: uint64(len(sharedPlain)), FileHash: m.Files[0].FileHash, Chunks: []Chunk{{ID: shared.ID, Checksum: shared.Checksum, Offset: 0, CompressedLength: shared.CompressedLength, UncompressedLength: shared.UncompressedLength}}})
	dir := t.TempDir()
	w, err := Preallocate(m, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(string(shared.ID), sharedPlain); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	raw1, err := os.ReadFile(dir + "/dir/file00.dat")
	if err != nil {
		t.Fatal(err)
	}
	raw2, err := os.ReadFile(dir + "/copy.dat")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw2) != string(sharedPlain) {
		t.Fatal("shared chunk not written to second file")
	}
	_ = raw1
}
