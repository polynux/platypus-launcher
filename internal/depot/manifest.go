// Manifest file parsing and filename decryption. A manifest file is a
// zip with one entry; the entry holds <magic><length><protobuf>
// sections (see manifest_types.go). File names are encrypted when the
// metadata says so: base64, then the same AES ECB-IV + CBC scheme as
// chunks (DepotManifest.cs DecryptFilenames, reimplemented here).
package depot

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"strings"
)

// ParseManifest parses a manifest file's bytes (the whole .manifest or
// the CDN's manifest response body).
func ParseManifest(data []byte) (*Manifest, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("manifest zip: %w", err)
	}
	if len(zr.File) != 1 {
		return nil, fmt.Errorf("manifest zip holds %d files, want 1", len(zr.File))
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var entry bytes.Buffer
	if _, err := entry.ReadFrom(rc); err != nil {
		return nil, err
	}
	payload, metadata, err := splitSections(entry.Bytes())
	if err != nil {
		return nil, err
	}
	m := &Manifest{}
	if err := parsePayload(m, payload); err != nil {
		return nil, err
	}
	if err := parseMetadata(m, metadata); err != nil {
		return nil, err
	}
	return m, nil
}

func parsePayload(m *Manifest, payload []byte) error {
	msg, err := Decode(payload)
	if err != nil {
		return fmt.Errorf("payload protobuf: %w", err)
	}
	for _, fm := range msg.AllBytes(1) { // mappings
		fmsg, err := Decode(fm)
		if err != nil {
			return err
		}
		f := File{
			Name:       string(fmsg.Bytes(1)), // filename
			Size:       fmsg.Int(2),           // size
			Flags:      uint32(fmsg.Int(3)),   // flags
			NameHash:   append([]byte{}, fmsg.Bytes(4)...),
			FileHash:   append([]byte{}, fmsg.Bytes(5)...),
			LinkTarget: string(fmsg.Bytes(7)), // linktarget
		}
		for _, ch := range fmsg.AllBytes(6) { // chunks
			cmsg, err := Decode(ch)
			if err != nil {
				return err
			}
			f.Chunks = append(f.Chunks, Chunk{
				ID:                 append([]byte{}, cmsg.Bytes(1)...),
				Checksum:           uint32(cmsg.Int(2)),
				Offset:             cmsg.Int(3),
				CompressedLength:   uint32(cmsg.Int(5)),
				UncompressedLength: uint32(cmsg.Int(4)),
			})
		}
		m.Files = append(m.Files, f)
	}
	return nil
}

func parseMetadata(m *Manifest, metadata []byte) error {
	if len(metadata) == 0 {
		return nil
	}
	msg, err := Decode(metadata)
	if err != nil {
		return fmt.Errorf("metadata protobuf: %w", err)
	}
	m.Meta.DepotID = uint32(msg.Int(1))
	m.Meta.GID = msg.Int(2)
	m.Meta.CreationTime = uint32(msg.Int(3))
	m.Meta.FilenamesEncrypted = msg.Int(4) != 0
	m.Meta.DiskOriginal = msg.Int(5)
	m.Meta.DiskCompressed = msg.Int(6)
	m.Meta.UniqueChunks = uint32(msg.Int(7))
	m.DepotID = m.Meta.DepotID
	m.GID = m.Meta.GID
	return nil
}

// DecryptFilenames decrypts every file name (and link target) in place.
// It mirrors what DepotManifest does: base64-decode, ECB-decrypt the
// first 16 bytes into the IV, CBC-decrypt the rest with PKCS7, strip one
// trailing NUL the encrypter may leave, and swap the alt separator.
func (m *Manifest) DecryptFilenames(key []byte) error {
	if !m.Meta.FilenamesEncrypted {
		return nil
	}
	if len(key) != 32 {
		return fmt.Errorf("depot key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	for i, f := range m.Files {
		name, err := decryptName(block, f.Name)
		if err != nil {
			return fmt.Errorf("file %d: %w", i, err)
		}
		m.Files[i].Name = name
		if f.LinkTarget != "" {
			link, err := decryptName(block, f.LinkTarget)
			if err != nil {
				return fmt.Errorf("file %d link: %w", i, err)
			}
			m.Files[i].LinkTarget = link
		}
	}
	m.Meta.FilenamesEncrypted = false
	return nil
}

func decryptName(block cipher.Block, enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(enc)
		if err != nil {
			return "", fmt.Errorf("base64: %w", err)
		}
	}
	if len(raw) < 32 || len(raw)%16 != 0 {
		return "", fmt.Errorf("encrypted name has odd length %d", len(raw))
	}
	iv := make([]byte, 16)
	block.Decrypt(iv, raw[:16])
	plain := make([]byte, len(raw)-16)
	prev := iv
	var tmp [16]byte
	for i := 16; i < len(raw); i += 16 {
		block.Decrypt(tmp[:], raw[i:i+16])
		for j := range tmp {
			plain[i-16+j] = tmp[j] ^ prev[j]
		}
		prev = raw[i : i+16]
	}
	pad := int(plain[len(plain)-1])
	if pad <= 0 || pad > 16 {
		return "", fmt.Errorf("bad name padding")
	}
	plain = plain[:len(plain)-pad]
	// The encrypter leaves one NUL terminator; drop it.
	if len(plain) > 0 && plain[len(plain)-1] == 0 {
		plain = plain[:len(plain)-1]
	}
	s := string(plain)
	s = strings.ReplaceAll(s, "\\", "/")
	return s, nil
}

// EncryptName is the inverse for fixtures.
func EncryptName(key []byte, name string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("depot key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	plain := append([]byte(name), 0)
	pad := 16 - len(plain)%16
	plain = append(plain, bytes.Repeat([]byte{byte(pad)}, pad)...)
	// Fixed IV keeps fixtures deterministic; real manifests use random ones.
	iv := bytes.Repeat([]byte{0x42}, 16)
	encIV := make([]byte, 16)
	block.Encrypt(encIV, iv)
	out := append(encIV, make([]byte, len(plain))...)
	prev := iv
	var tmp [16]byte
	for i := 0; i < len(plain); i += 16 {
		for j := range tmp {
			tmp[j] = plain[i+j] ^ prev[j]
		}
		block.Encrypt(out[16+i:16+i+16], tmp[:])
		prev = out[16+i : 16+i+16]
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

// BuildManifest assembles a manifest file's bytes from Go structs, for
// fixtures. Names are encrypted when encryptNames is set.
func BuildManifest(meta Metadata, files []File, key []byte, encryptNames bool) ([]byte, error) {
	p := &Enc{}
	for _, f := range files {
		name := f.Name
		link := f.LinkTarget
		if encryptNames {
			var err error
			if name, err = EncryptName(key, f.Name); err != nil {
				return nil, err
			}
			if link != "" {
				if link, err = EncryptName(key, f.LinkTarget); err != nil {
					return nil, err
				}
			}
		}
		fm := &Enc{}
		fm.Str(1, name)
		fm.Varint(2, f.Size)
		fm.Varint(3, uint64(f.Flags))
		if len(f.NameHash) > 0 {
			fm.BytesField(4, f.NameHash)
		}
		if len(f.FileHash) > 0 {
			fm.BytesField(5, f.FileHash)
		}
		for _, c := range f.Chunks {
			ch := &Enc{}
			ch.BytesField(1, c.ID)
			ch.Varint(2, uint64(c.Checksum))
			ch.Varint(3, c.Offset)
			ch.Varint(4, uint64(c.UncompressedLength))
			ch.Varint(5, uint64(c.CompressedLength))
			fm.MsgField(6, ch)
		}
		if link != "" {
			fm.Str(7, link)
		}
		p.MsgField(1, fm)
	}
	md := &Enc{}
	md.Varint(1, uint64(meta.DepotID))
	md.Varint(2, meta.GID)
	md.Varint(3, uint64(meta.CreationTime))
	if encryptNames {
		md.Varint(4, 1)
	}
	md.Varint(5, meta.DiskOriginal)
	md.Varint(6, meta.DiskCompressed)
	md.Varint(7, uint64(meta.UniqueChunks))
	var entry bytes.Buffer
	writeSection(&entry, magicPayload, p.Bytes())
	writeSection(&entry, magicMetadata, md.Bytes())
	writeSection(&entry, magicEnd, nil)
	var file bytes.Buffer
	w := zip.NewWriter(&file)
	f, _ := w.Create("manifest.bin")
	f.Write(entry.Bytes())
	w.Close()
	return file.Bytes(), nil
}

func writeSection(buf *bytes.Buffer, magic uint32, body []byte) {
	var h [8]byte
	binaryLittle(h[:], magic)
	binaryLittle(h[4:], uint32(len(body)))
	buf.Write(h[:])
	buf.Write(body)
}

func binaryLittle(b []byte, v uint32) {
	b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}
