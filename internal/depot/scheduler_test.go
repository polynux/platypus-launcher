// Scheduler tests against an httptest fake CDN with several hosts: a
// slow host, a failing host and bad chunks. Plus the multi-host vs
// single-host benchmark on throttled fake hosts.
package depot

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeHost struct {
	srv      *httptest.Server
	url      string
	requests *int64
	delay    time.Duration
	failAll  bool
	badChunk string // hex id served corruptly on this host only
	chunks   map[string][]byte
	manifest []byte
}

func (f *fakeHost) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(f.requests, 1)
		if f.delay > 0 {
			time.Sleep(f.delay)
		}
		if f.failAll {
			http.Error(w, "boom", 500)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 && parts[0] == "depot" && parts[2] == "manifest" {
			w.Write(f.manifest)
			return
		}
		if len(parts) == 4 && parts[0] == "depot" && parts[2] == "chunk" {
			hex := parts[3]
			enc, ok := f.chunks[hex]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if hex == f.badChunk {
				enc = append([]byte{}, enc...)
				enc[len(enc)-1] ^= 1 // corrupt: checksum fails, retries elsewhere
			}
			w.Write(enc)
			return
		}
		http.NotFound(w, r)
	})
}

func hexOf(id []byte) string { return fmt.Sprintf("%x", id) }

// buildFixtureDepot creates a small depot: files, encrypted chunks and the
// manifest bytes, all from a test key.
func buildFixtureDepot(t *testing.T, key []byte, nfiles, chunksPerFile int) (*Manifest, map[string][]byte) {
	t.Helper()
	var files []File
	encMap := map[string][]byte{}
	var total uint64
	var gid uint64 = 777
	for fi := 0; fi < nfiles; fi++ {
		name := fmt.Sprintf("dir/file%02d.dat", fi)
		var chunks []Chunk
		var content []byte
		var off uint64
		for ci := 0; ci < chunksPerFile; ci++ {
			plain := bytes.Repeat([]byte{byte(fi*16 + ci + 1)}, 4096)
			plain = append(plain, []byte(fmt.Sprintf("-%d-%d-", fi, ci))...)
			container := ZstdCompress(plain)
			iv := make([]byte, 16)
			rand.Read(iv)
			enc, err := EncryptChunk(key, iv, container)
			if err != nil {
				t.Fatal(err)
			}
			id := sha1.Sum(plain)
			c := Chunk{ID: id[:], Checksum: Adler0(plain), Offset: off,
				CompressedLength: uint32(len(enc)), UncompressedLength: uint32(len(plain))}
			chunks = append(chunks, c)
			encMap[hexOf(id[:])] = enc
			content = append(content, plain...)
			off += uint64(len(plain))
		}
		h := sha1.Sum(content)
		files = append(files, File{Name: name, Size: off, FileHash: h[:], Chunks: chunks})
		total += off
	}
	_ = total
	meta := Metadata{DepotID: 11, GID: gid, DiskOriginal: total, UniqueChunks: uint32(len(encMap))}
	raw, err := BuildManifest(meta, files, key, false)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return m, encMap
}

func startHosts(t *testing.T, m *Manifest, encMap map[string][]byte, delays []time.Duration, failIdx int, badIdx int, badChunk string) ([]Host, []*fakeHost) {
	t.Helper()
	raw := func() []byte {
		meta := Metadata{DepotID: 11, GID: 777}
		var files []File
		for _, f := range m.Files {
			files = append(files, f)
		}
		b, err := BuildManifest(meta, files, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}()
	_ = raw
	var hosts []Host
	var fakes []*fakeHost
	manifestRaw := mustBuild(t, m)
	for i := range delays {
		fh := &fakeHost{requests: new(int64), delay: delays[i], chunks: encMap, manifest: manifestRaw}
		if i == failIdx {
			fh.failAll = true
		}
		if i == badIdx {
			fh.badChunk = badChunk
		}
		fh.srv = httptest.NewServer(fh.handler())
		t.Cleanup(fh.srv.Close)
		fh.url = fh.srv.URL
		fakes = append(fakes, fh)
		hosts = append(hosts, Host{Name: fmt.Sprintf("host%d", i), BaseURL: fh.url})
	}
	return hosts, fakes
}

func mustBuild(t *testing.T, m *Manifest) []byte {
	t.Helper()
	raw, err := BuildManifest(m.Meta, m.Files, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSchedulerDownloadsAcrossHosts(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 4, 4)
	hosts, fakes := startHosts(t, m, encMap,
		[]time.Duration{0, 0, 0, 0}, -1, -1, "")
	s := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 8, MaxConns: 8, MaxPerHost: 4})
	data, err := s.Download(context.Background(), m, 11, hosts, key, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(encMap) {
		t.Fatalf("got %d chunks, want %d", len(data), len(encMap))
	}
	// Spread: every host served something.
	for i, f := range fakes {
		if atomic.LoadInt64(f.requests) == 0 {
			t.Fatalf("host %d served nothing", i)
		}
	}
	// Write and verify.
	dir := t.TempDir()
	if err := WriteChunks(m, dir, data); err != nil {
		t.Fatal(err)
	}
	if bad, _ := Verify(m, dir); len(bad) != 0 {
		t.Fatalf("verify failed: %v", bad)
	}
}

func TestSchedulerSlowAndFailingHosts(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 3, 3)
	var firstID string
	for id := range encMap {
		firstID = id
		break
	}
	hosts, _ := startHosts(t, m, encMap,
		[]time.Duration{80 * time.Millisecond, 0, 0}, 2, 1, firstID)
	s := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 6, MaxConns: 6, MaxPerHost: 4})
	data, err := s.Download(context.Background(), m, 11, hosts, key, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(encMap) {
		t.Fatalf("got %d chunks, want %d", len(data), len(encMap))
	}
	dir := t.TempDir()
	if err := WriteChunks(m, dir, data); err != nil {
		t.Fatal(err)
	}
	if bad, _ := Verify(m, dir); len(bad) != 0 {
		t.Fatalf("verify failed: %v", bad)
	}
}

func TestSchedulerResumeAndVerify(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 2, 3)
	hosts, _ := startHosts(t, m, encMap, []time.Duration{0, 0}, -1, -1, "")
	s := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 4, MaxConns: 4})
	full, err := s.Download(context.Background(), m, 11, hosts, key, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Resume with half the chunks already present (one corrupted).
	have := map[string][]byte{}
	i := 0
	for id, plain := range full {
		if i%2 == 0 {
			have[id] = plain
		}
		i++
	}
	for id := range have {
		bad := append([]byte{}, have[id]...)
		bad[0] ^= 1
		have[id] = bad
		break // one corrupt entry must be re-fetched
	}
	resumed, err := s.Download(context.Background(), m, 11, hosts, key, have, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed) != len(encMap) {
		t.Fatalf("resumed %d chunks, want %d", len(resumed), len(encMap))
	}
	dir := t.TempDir()
	if err := WriteChunks(m, dir, resumed); err != nil {
		t.Fatal(err)
	}
	if bad, _ := Verify(m, dir); len(bad) != 0 {
		t.Fatalf("verify after resume: %v", bad)
	}
	// Corrupt a file on disk; verify must catch it.
	f0 := m.Files[0]
	p := dir + "/" + f0.Name
	raw := mustRead(t, p)
	raw[0] ^= 1
	mustWrite(t, p, raw)
	if bad, _ := Verify(m, dir); len(bad) != 1 {
		t.Fatalf("verify should flag one file, got %v", bad)
	}
}

func TestSchedulerMultiBeatsSingle(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 8, 8) // 64 chunks
	throttle := []time.Duration{15 * time.Millisecond, 15 * time.Millisecond, 15 * time.Millisecond, 15 * time.Millisecond}
	multiHosts, _ := startHosts(t, m, encMap, throttle, -1, -1, "")
	singleHosts, _ := startHosts(t, m, encMap, []time.Duration{15 * time.Millisecond}, -1, -1, "")
	cfg := SchedulerConfig{InitialConns: 8, MaxConns: 8, MaxPerHost: 4, GrowEvery: time.Hour}
	ms := NewScheduler(NewClient(), cfg)
	start := time.Now()
	if _, err := ms.Download(context.Background(), m, 11, multiHosts, key, nil, nil); err != nil {
		t.Fatal(err)
	}
	multi := time.Since(start)
	ss := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 2, MaxConns: 2, MaxPerHost: 2, GrowEvery: time.Hour})
	start = time.Now()
	if _, err := ss.Download(context.Background(), m, 11, singleHosts, key, nil, nil); err != nil {
		t.Fatal(err)
	}
	single := time.Since(start)
	t.Logf("multi-host %v vs single-host %v", multi, single)
	if multi*2 >= single {
		t.Fatalf("multi-host should clearly beat single-host: %v vs %v", multi, single)
	}
}

func BenchmarkSingleHost(b *testing.B) {
	key := testKey()
	m, encMap := buildFixtureDepotBench(b, key, 8, 8)
	hosts, _ := startHostsBench(b, m, encMap, []time.Duration{5 * time.Millisecond})
	s := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 2, MaxConns: 2, MaxPerHost: 2, GrowEvery: time.Hour})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Download(context.Background(), m, 11, hosts, key, nil, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMultiHost(b *testing.B) {
	key := testKey()
	m, encMap := buildFixtureDepotBench(b, key, 8, 8)
	hosts, _ := startHostsBench(b, m, encMap, []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond})
	s := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 8, MaxConns: 8, MaxPerHost: 4, GrowEvery: time.Hour})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Download(context.Background(), m, 11, hosts, key, nil, nil); err != nil {
			b.Fatal(err)
		}
	}
}
