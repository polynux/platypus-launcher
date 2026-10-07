// Package depot holds the offline core of the Steam depot downloader:
// the chunk decrypt/decompress/verify path, the manifest parser with
// filename decryption, and the fixtures the tests build from. No network
// here, so it can be tested on its own with a fake CDN later.
package depot

import (
	"encoding/binary"
	"fmt"
)

// Section magics of a manifest file: a zip with one entry holding a
// sequence of <uint32 magic><uint32 length><protobuf> sections
// (steam/content_manifest.proto, DepotManifest.cs).
const (
	magicPayload   = 0x71F617D0
	magicMetadata  = 0x1F4812BE
	magicSignature = 0x1B81B817
	magicEnd       = 0x32C415AB
)

// ManifestVersion is the only manifest version Steam serves today.
const ManifestVersion = 5

// File flags (EDepotFileFlag in SteamKit's
// Resources/SteamLanguage/enums.steamd).
const (
	FlagUserConfig          = 1
	FlagVersionedUserConfig = 2
	FlagEncrypted           = 4
	FlagReadOnly            = 8
	FlagHidden              = 16
	FlagExecutable          = 32
	FlagDirectory           = 64
	FlagCustomExecutable    = 128
	FlagInstallScript       = 256
	FlagSymlink             = 512
)

// Chunk is one content chunk of a file: ChunkID is the SHA-1 from
// ContentManifestPayload.FileMapping.ChunkData.sha, Checksum the Adler-32
// (seed 0) of the uncompressed data.
type Chunk struct {
	ID                 []byte
	Checksum           uint32
	Offset             uint64
	CompressedLength   uint32
	UncompressedLength uint32
}

// File is one file mapping of a manifest.
type File struct {
	Name       string
	Size       uint64
	Flags      uint32
	FileHash   []byte // SHA-1 of the file content (sha_content)
	NameHash   []byte // SHA-1 of the name (sha_filename)
	LinkTarget string
	Chunks     []Chunk
}

// IsDir reports whether the entry is a directory placeholder.
func (f File) IsDir() bool { return f.Flags&FlagDirectory != 0 }

// IsExec reports whether the file wants its exec bit set.
func (f File) IsExec() bool { return f.Flags&FlagExecutable != 0 }

// IsSymlink reports whether the entry is a symlink.
func (f File) IsSymlink() bool { return f.Flags&FlagSymlink != 0 || f.LinkTarget != "" }

// Metadata mirrors ContentManifestMetadata.
type Metadata struct {
	DepotID            uint32
	GID                uint64
	CreationTime       uint32
	FilenamesEncrypted bool
	DiskOriginal       uint64
	DiskCompressed     uint64
	UniqueChunks       uint32
}

// Manifest is a parsed depot manifest.
type Manifest struct {
	Files []File
	Meta  Metadata
	// GID is the manifest id (Meta.GID), kept at the top for callers.
	GID     uint64
	DepotID uint32
}

// TotalSize sums the files' sizes.
func (m *Manifest) TotalSize() uint64 {
	var n uint64
	for _, f := range m.Files {
		n += f.Size
	}
	return n
}

// UniqueChunks returns each chunk id once, in first-seen order. A chunk
// id can appear in several files; the downloader fetches it once.
func (m *Manifest) UniqueChunks() []Chunk {
	seen := map[string]bool{}
	var out []Chunk
	for _, f := range m.Files {
		for _, c := range f.Chunks {
			k := string(c.ID)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, c)
		}
	}
	return out
}

func u32le(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

func splitSections(entry []byte) (payload, metadata []byte, err error) {
	pos := 0
	for pos+8 <= len(entry) {
		magic := u32le(entry[pos:])
		length := u32le(entry[pos+4:])
		pos += 8
		if length == 0 && magic == magicEnd {
			break
		}
		if pos+int(length) > len(entry) {
			return nil, nil, fmt.Errorf("section magic %08x runs past the end", magic)
		}
		body := entry[pos : pos+int(length)]
		pos += int(length)
		switch magic {
		case magicPayload:
			payload = body
		case magicMetadata:
			metadata = body
		case magicSignature:
			// Signatures are not verified; Steam serves them but
			// the content hashes already authenticate every byte.
		case magicEnd:
			pos = len(entry)
		default:
			return nil, nil, fmt.Errorf("unknown manifest section %08x", magic)
		}
	}
	if payload == nil {
		return nil, nil, fmt.Errorf("manifest has no payload section")
	}
	return payload, metadata, nil
}
