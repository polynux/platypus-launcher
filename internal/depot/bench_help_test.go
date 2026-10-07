package depot

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func buildFixtureDepotBench(b *testing.B, key []byte, nfiles, chunksPerFile int) (*Manifest, map[string][]byte) {
	b.Helper()
	var files []File
	encMap := map[string][]byte{}
	var total uint64
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
				b.Fatal(err)
			}
			id := sha1.Sum(plain)
			c := Chunk{ID: id[:], Checksum: Adler0(plain), Offset: off,
				CompressedLength: uint32(len(enc)), UncompressedLength: uint32(len(plain))}
			chunks = append(chunks, c)
			encMap[fmt.Sprintf("%x", id[:])] = enc
			content = append(content, plain...)
			off += uint64(len(plain))
		}
		h := sha1.Sum(content)
		files = append(files, File{Name: name, Size: off, FileHash: h[:], Chunks: chunks})
		total += off
	}
	meta := Metadata{DepotID: 11, GID: 777, DiskOriginal: total}
	raw, err := BuildManifest(meta, files, key, false)
	if err != nil {
		b.Fatal(err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		b.Fatal(err)
	}
	return m, encMap
}

func startHostsBench(b *testing.B, m *Manifest, encMap map[string][]byte, delays []time.Duration) ([]Host, []*fakeHost) {
	b.Helper()
	manifestRaw, err := BuildManifest(m.Meta, m.Files, nil, false)
	if err != nil {
		b.Fatal(err)
	}
	var hosts []Host
	var fakes []*fakeHost
	for i := range delays {
		fh := &fakeHost{requests: new(int64), delay: delays[i], chunks: encMap, manifest: manifestRaw}
		fh.srv = httptest.NewServer(fh.handler())
		b.Cleanup(fh.srv.Close)
		fh.url = fh.srv.URL
		fakes = append(fakes, fh)
		hosts = append(hosts, Host{Name: fmt.Sprintf("host%d", i), BaseURL: fh.url})
	}
	return hosts, fakes
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
