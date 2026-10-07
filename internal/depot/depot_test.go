package depot

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"testing"
)

func testKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i*7 + 1)
	}
	return k
}

func chunkFixture(t *testing.T, key, plain []byte, compress func([]byte) []byte) (Chunk, []byte) {
	t.Helper()
	container := compress(plain)
	iv := make([]byte, 16)
	rand.Read(iv)
	enc, err := EncryptChunk(key, iv, container)
	if err != nil {
		t.Fatal(err)
	}
	id := sha1.Sum(plain)
	return Chunk{
		ID:                 id[:],
		Checksum:           Adler0(plain),
		Offset:             0,
		CompressedLength:   uint32(len(enc)),
		UncompressedLength: uint32(len(plain)),
	}, enc
}

func TestChunkRoundTripZstd(t *testing.T) {
	key := testKey()
	plain := bytes.Repeat([]byte("hello steam chunk, "), 2000)
	info, enc := chunkFixture(t, key, plain, ZstdCompress)
	got, err := ProcessChunk(key, enc, info)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("zstd round trip mismatch")
	}
}

func TestChunkRoundTripVZip(t *testing.T) {
	key := testKey()
	plain := bytes.Repeat([]byte("vzip lzma payload 0123456789 "), 1500)
	info, enc := chunkFixture(t, key, plain, VzipCompress)
	got, err := ProcessChunk(key, enc, info)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("vzip round trip mismatch")
	}
}

func TestChunkRoundTripZip(t *testing.T) {
	key := testKey()
	plain := []byte("a small zip chunk")
	info, enc := chunkFixture(t, key, plain, func(p []byte) []byte { return ZipCompress("c", p) })
	got, err := ProcessChunk(key, enc, info)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("zip round trip mismatch")
	}
}

func TestChunkChecksumMismatch(t *testing.T) {
	key := testKey()
	plain := []byte("checksum me")
	info, enc := chunkFixture(t, key, plain, ZstdCompress)
	info.Checksum ^= 1
	if _, err := ProcessChunk(key, enc, info); err == nil {
		t.Fatal("bad checksum accepted")
	}
}

func TestChunkWrongKey(t *testing.T) {
	key := testKey()
	plain := []byte("wrong key chunk with enough bytes to pad well 12345678")
	_, enc := chunkFixture(t, key, plain, ZstdCompress)
	other := testKey()
	other[0] ^= 1
	info := Chunk{ID: []byte("x"), Checksum: Adler0(plain), UncompressedLength: uint32(len(plain))}
	if _, err := ProcessChunk(other, enc, info); err == nil {
		t.Fatal("wrong key accepted")
	}
}

func TestAdlerSeedZero(t *testing.T) {
	// "Wikipedia" is the reference vector: zlib's 1-seeded Adler is
	// 0x11e60398, so seed 0 must differ by exactly the seed offset.
	if got := Adler0([]byte("Wikipedia")); got == 0x11e60398 {
		t.Fatal("Adler0 looks seed-1, not seed-0")
	}
	if got, want := Adler0(nil), uint32(0); got != want {
		t.Fatalf("empty Adler0 = %08x, want %08x", got, want)
	}
}

func TestManifestRoundTrip(t *testing.T) {
	key := testKey()
	plain1 := bytes.Repeat([]byte("file one chunk "), 500)
	plain2 := bytes.Repeat([]byte("second file, two chunks!! "), 400)
	c1, _ := chunkFixture(t, key, plain1, ZstdCompress)
	c2a, _ := chunkFixture(t, key, plain2[:1000], VzipCompress)
	c2b, _ := chunkFixture(t, key, plain2[1000:], ZstdCompress)
	c2a.Offset = 0
	c2b.Offset = 1000
	// A chunk shared by two files is stored once on the wire.
	h1 := sha1.Sum(plain1)
	files := []File{
		{Name: "dir/game.dat", Size: uint64(len(plain1)), FileHash: h1[:], Chunks: []Chunk{c1}},
		{Name: "readme.txt", Size: uint64(len(plain2)), Flags: FlagExecutable, Chunks: []Chunk{c2a, c2b}},
		{Name: "emptydir", Flags: FlagDirectory},
		{Name: "link", Flags: FlagSymlink, LinkTarget: "readme.txt"},
	}
	meta := Metadata{DepotID: 9, GID: 1234, DiskOriginal: 1 << 20, UniqueChunks: 3}
	raw, err := BuildManifest(meta, files, key, true)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Meta.FilenamesEncrypted {
		t.Fatal("fixture should mark names encrypted")
	}
	if err := m.DecryptFilenames(key); err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 4 || m.Files[0].Name != "dir/game.dat" {
		t.Fatalf("names: %+v", m.Files)
	}
	if m.Files[3].LinkTarget != "readme.txt" {
		t.Fatalf("link: %+v", m.Files[3])
	}
	if got := len(m.UniqueChunks()); got != 3 {
		t.Fatalf("unique chunks = %d, want 3", got)
	}
	// Shared chunk: same id in two files is returned once.
	dup := m.Files[1].Chunks[0]
	m.Files = append(m.Files, File{Name: "copy.txt", Size: 1000, Chunks: []Chunk{dup}})
	if got := len(m.UniqueChunks()); got != 3 {
		t.Fatalf("shared chunk fetched twice: %d uniques", got)
	}
	if m.TotalSize() == 0 {
		t.Fatal("total size is zero")
	}
}
