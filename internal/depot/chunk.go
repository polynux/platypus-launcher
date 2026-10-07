// Chunk decrypt, decompress and checksum. The wire format follows
// SteamKit2's DepotChunk: the first 16 bytes are the IV encrypted with
// AES-256-ECB under the depot key, the rest is AES-256-CBC with PKCS7.
// Decompression is by magic: VSZa = zstd, VZa = LZMA in a VZip container,
// PK\x03\x04 = zip. Checksums are Adler-32 seeded with 0, not the usual 1.
package depot

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"fmt"
	"hash/adler32"
	"hash/crc32"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz/lzma"
)

// VZip container constants (VZipUtil.cs): 'V' 'Z' magic, version 'a',
// then a CRC-or-timestamp word, LZMA properties, dictionary size, the
// LZMA stream and a 10-byte footer (CRC, decompressed size, 'z' 'v').
const (
	vzipMagic  = 0x5A56
	vzipFooter = 0x767A
	vzipVer    = 'a'
)

// Adler0 hashes with a zero seed, the way Steam checksums chunks.
func Adler0(data []byte) uint32 {
	h := adler32.New()
	// adler32 starts at 1; reset to 0 by hashing with a custom state.
	// The stdlib has no seed parameter, so compute directly.
	var a, b uint32
	const mod = 65521
	for _, c := range data {
		a = (a + uint32(c)) % mod
		b = (b + a) % mod
	}
	_ = h
	return b<<16 | a
}

// DecryptChunk removes the AES layer, returning the still-compressed
// payload (a VSZa/VZa/zip container). Key must be 32 bytes.
func DecryptChunk(key, data []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("depot key must be 32 bytes")
	}
	if len(data) < 32 {
		return nil, fmt.Errorf("chunk too short (%d bytes)", len(data))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, 16)
	block.Decrypt(iv, data[:16])
	body := data[16:]
	if len(body)%16 != 0 {
		return nil, fmt.Errorf("encrypted chunk is not a whole number of blocks")
	}
	plain := make([]byte, len(body))
	prev := iv
	var tmp [16]byte
	for i := 0; i < len(body); i += 16 {
		block.Decrypt(tmp[:], body[i:i+16])
		for j := range tmp {
			plain[i+j] = tmp[j] ^ prev[j]
		}
		prev = body[i : i+16]
	}
	// PKCS7 unpad.
	pad := int(plain[len(plain)-1])
	if pad == 0 || pad > 16 || pad > len(plain) {
		return nil, fmt.Errorf("bad chunk padding")
	}
	for _, c := range plain[len(plain)-pad:] {
		if int(c) != pad {
			return nil, fmt.Errorf("bad chunk padding")
		}
	}
	return plain[:len(plain)-pad], nil
}

// EncryptChunk is the inverse of DecryptChunk, for test fixtures: CBC
// with PKCS7 under the ECB-encrypted IV.
func EncryptChunk(key, iv, plain []byte) ([]byte, error) {
	if len(key) != 32 || len(iv) != 16 {
		return nil, fmt.Errorf("key must be 32 bytes and iv 16 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	pad := 16 - len(plain)%16
	padded := append(append([]byte{}, plain...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	encIV := make([]byte, 16)
	block.Encrypt(encIV, iv)
	out := append(encIV, make([]byte, len(padded))...)
	prev := iv
	var tmp [16]byte
	for i := 0; i < len(padded); i += 16 {
		for j := range tmp {
			tmp[j] = padded[i+j] ^ prev[j]
		}
		block.Encrypt(out[16+i:16+i+16], tmp[:])
		prev = out[16+i : 16+i+16]
	}
	return out, nil
}

// DecompressChunk expands one decrypted container into dst, which must
// hold the chunk's uncompressed bytes.
func DecompressChunk(container, dst []byte) ([]byte, error) {
	if len(container) < 4 {
		return nil, fmt.Errorf("chunk container too short")
	}
	switch {
	case bytes.HasPrefix(container, []byte("VSZa")):
		return zstdDecompress(container, dst)
	case len(container) >= 3 && container[0] == 'V' && container[1] == 'Z' && container[2] == 'a':
		return vzipDecompress(container, dst)
	case bytes.HasPrefix(container, []byte("PK\x03\x04")):
		return zipDecompress(container, dst)
	default:
		return nil, fmt.Errorf("unknown chunk compression %02x %02x %02x %02x",
			container[0], container[1], container[2], container[3])
	}
}

// VZstd container layout (VZstdUtil.cs): an 8-byte header (uint32 magic
// 0x615A5356 "VSZa" LE, then uint32 CRC32), the zstd frame, and a 15-byte
// footer (uint32 CRC32, uint32 decompressed size, 4 bytes SteamKit ignores,
// then "zsv"). Both CRCs cover the decompressed data.
const (
	vzstdMagic  = 0x615A5356
	vzstdFooter = "zsv"
)

func zstdDecompress(container, dst []byte) ([]byte, error) {
	if len(container) < 8+15 {
		return nil, fmt.Errorf("vszd too short")
	}
	if binary.LittleEndian.Uint32(container) != vzstdMagic {
		return nil, fmt.Errorf("bad vszd header")
	}
	headCRC := binary.LittleEndian.Uint32(container[4:])
	footCRC := binary.LittleEndian.Uint32(container[len(container)-15:])
	wantSize := int(binary.LittleEndian.Uint32(container[len(container)-11:]))
	if headCRC != footCRC {
		return nil, fmt.Errorf("vszd header and footer CRCs differ")
	}
	if string(container[len(container)-3:]) != vzstdFooter {
		return nil, fmt.Errorf("bad vszd footer")
	}
	if uint64(wantSize) > uint64(len(dst)) {
		return nil, fmt.Errorf("vszd output overflows the chunk buffer")
	}
	dec, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	got, err := dec.DecodeAll(container[8:len(container)-15], dst[:0])
	if err != nil {
		return nil, fmt.Errorf("zstd: %w", err)
	}
	if len(got) != wantSize {
		return nil, fmt.Errorf("vszd size mismatch: got %d, want %d", len(got), wantSize)
	}
	if crc32.ChecksumIEEE(got) != footCRC {
		return nil, fmt.Errorf("vszd crc mismatch")
	}
	return got, nil
}

// ZstdCompress builds a VSZa container for fixtures, in the real layout.
func ZstdCompress(plain []byte) []byte {
	enc, _ := zstd.NewWriter(nil)
	frame := enc.EncodeAll(plain, nil)
	enc.Close()
	crc := crc32.ChecksumIEEE(plain)
	out := make([]byte, 0, 8+len(frame)+15)
	var h [8]byte
	binary.LittleEndian.PutUint32(h[:], vzstdMagic)
	binary.LittleEndian.PutUint32(h[4:], crc)
	out = append(out, h[:]...)
	out = append(out, frame...)
	var f [15]byte
	binary.LittleEndian.PutUint32(f[:], crc)
	binary.LittleEndian.PutUint32(f[4:], uint32(len(plain)))
	// f[8:12] is 4 bytes SteamKit never reads; leave them zero.
	copy(f[12:], vzstdFooter)
	out = append(out, f[:]...)
	return out
}

func vzipDecompress(container, dst []byte) ([]byte, error) {
	if len(container) < 7+10 {
		return nil, fmt.Errorf("vzip too short")
	}
	if binary.LittleEndian.Uint16(container) != vzipMagic || container[2] != vzipVer {
		return nil, fmt.Errorf("bad vzip header")
	}
	props := container[7]
	dictSize := binary.LittleEndian.Uint32(container[8:12])
	compStart := 12
	compEnd := len(container) - 10
	if compEnd < compStart {
		return nil, fmt.Errorf("bad vzip bounds")
	}
	wantCRC := binary.LittleEndian.Uint32(container[compEnd:])
	wantSize := int32(binary.LittleEndian.Uint32(container[compEnd+4:]))
	if binary.LittleEndian.Uint16(container[compEnd+8:]) != vzipFooter {
		return nil, fmt.Errorf("bad vzip footer")
	}
	// ulikunitz/xz wants the classic 13-byte LZMA header
	// (props + dict size + 8-byte size); VZip keeps the size in the
	// footer, so synthesize the header around the raw stream.
	hdr := make([]byte, 13)
	hdr[0] = props
	copy(hdr[1:5], container[8:12])
	for i := 5; i < 13; i++ {
		hdr[i] = 0xFF // unknown size marker is handled below
	}
	var sizeBytes [8]byte
	if wantSize >= 0 {
		binary.LittleEndian.PutUint64(sizeBytes[:], uint64(wantSize))
		copy(hdr[5:], sizeBytes[:])
	} else {
		for i := 5; i < 13; i++ {
			hdr[i] = 0xFF
		}
	}
	r, err := lzma.NewReader(io.MultiReader(bytes.NewReader(hdr), bytes.NewReader(container[compStart:compEnd])))
	if err != nil {
		return nil, fmt.Errorf("lzma: %w", err)
	}
	_ = dictSize
	got, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("lzma: %w", err)
	}
	if wantSize >= 0 && len(got) != int(wantSize) {
		return nil, fmt.Errorf("vzip size mismatch: got %d, want %d", len(got), wantSize)
	}
	if uint32(len(got)) > uint32(len(dst)) {
		return nil, fmt.Errorf("vzip output overflows the chunk buffer")
	}
	copy(dst, got)
	if crc32.ChecksumIEEE(got) != wantCRC {
		return nil, fmt.Errorf("vzip crc mismatch")
	}
	return dst[:len(got)], nil
}

// VzipCompress builds a VZip container for fixtures.
func VzipCompress(plain []byte) []byte {
	var comp bytes.Buffer
	w, _ := lzma.NewWriter(&comp)
	w.Write(plain)
	w.Close()
	raw := comp.Bytes()
	// Strip the classic 13-byte header; VZip stores props + dict size
	// in its own header and the size in the footer.
	var props byte
	var dict uint32 = 1 << 23
	body := raw
	if len(raw) >= 13 {
		props = raw[0]
		dict = binary.LittleEndian.Uint32(raw[1:5])
		body = raw[13:]
	}
	out := make([]byte, 0, 11+len(body)+10)
	out = append(out, byte(vzipMagic&0xFF), byte(vzipMagic>>8), vzipVer)
	var crcWord [4]byte
	binary.LittleEndian.PutUint32(crcWord[:], crc32.ChecksumIEEE(plain))
	out = append(out, crcWord[:]...)
	out = append(out, props)
	var db [4]byte
	binary.LittleEndian.PutUint32(db[:], dict)
	out = append(out, db[:]...)
	out = append(out, body...)
	var foot [10]byte
	binary.LittleEndian.PutUint32(foot[:], crc32.ChecksumIEEE(plain))
	binary.LittleEndian.PutUint32(foot[4:], uint32(len(plain)))
	binary.LittleEndian.PutUint16(foot[8:], vzipFooter)
	out = append(out, foot[:]...)
	return out
}

func zipDecompress(container, dst []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(container), int64(len(container)))
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
	}
	if len(zr.File) != 1 {
		return nil, fmt.Errorf("zip chunk holds %d files, want 1", len(zr.File))
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	if len(got) > len(dst) {
		return nil, fmt.Errorf("zip output overflows the chunk buffer")
	}
	copy(dst, got)
	return dst[:len(got)], nil
}

// ZipCompress builds a one-file zip container for fixtures.
func ZipCompress(name string, plain []byte) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create(name)
	f.Write(plain)
	w.Close()
	return buf.Bytes()
}

// ProcessChunk decrypts, decompresses and checks one downloaded chunk.
// The checksum is Adler-32 seeded with 0 over the uncompressed bytes.
func ProcessChunk(key, encrypted []byte, info Chunk) ([]byte, error) {
	plain, err := DecryptChunk(key, encrypted)
	if err != nil {
		return nil, err
	}
	dst := make([]byte, info.UncompressedLength)
	got, err := DecompressChunk(plain, dst)
	if err != nil {
		return nil, err
	}
	if uint32(len(got)) != info.UncompressedLength {
		return nil, fmt.Errorf("chunk %x decompressed to %d bytes, want %d",
			info.ID, len(got), info.UncompressedLength)
	}
	if Adler0(got) != info.Checksum {
		return nil, fmt.Errorf("chunk %x checksum mismatch", info.ID)
	}
	return got, nil
}
