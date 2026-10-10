// Package fakesteam builds the fake Steam content the steam-standalone
// tests download: fixture depots (encrypted with a test key), an httptest
// fake CDN serving their manifests and chunks, and the fake-CM state that
// points platypus-steam-dl at them. Both the plugin's own tests and the
// conformance suite use it, so the two harnesses can't drift apart.
//
// Variants ride the FAKE_DD_STATE file (kept for historical reasons) with
// knobs: bare_elf, unity_decoy, windows_only, plus FAKE_DD_STEAMWORKS for
// the Steamworks library that marks a game "optional".
package fakesteam

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/depot"
)

// Fixture is one fake depot: manifest, encrypted chunks and expected sizes.
type Fixture struct {
	Key             []byte
	DepotID         uint32
	GID             uint64
	OSUsed          string // "linux" or "windows"
	Manifest        *depot.Manifest
	Raw             []byte // manifest file bytes
	Chunks          map[string][]byte
	DiskBytes       uint64
	CompressedBytes uint64
}

// Knobs are the fixture behaviour switches (see above).
type Knobs struct {
	BareELF     bool
	UnityDecoy  bool
	WindowsOnly bool
	Steamworks  string // e.g. "steam_api64.dll" from FAKE_DD_STEAMWORKS
	// ExtraFiles adds that many small files (one chunk each), and
	// ChunkDelayMs serves chunks one at a time with that delay, so a download
	// has several progress steps and can be canceled halfway
	// (extra_files / chunk_delay_ms in FAKE_DD_STATE).
	ExtraFiles   int
	ChunkDelayMs int
}

// ReadKnobs loads the variant from the FAKE_DD_STATE file and env.
func ReadKnobs() Knobs {
	var k Knobs
	if state := os.Getenv("FAKE_DD_STATE"); state != "" {
		if raw, err := os.ReadFile(state); err == nil {
			var st map[string]any
			if json.Unmarshal(raw, &st) == nil {
				_, k.BareELF = st["bare_elf"]
				_, k.UnityDecoy = st["unity_decoy"]
				_, k.WindowsOnly = st["windows_only"]
				if v, ok := st["bare_elf"].(bool); ok {
					k.BareELF = v
				}
				if v, ok := st["unity_decoy"].(bool); ok {
					k.UnityDecoy = v
				}
				if v, ok := st["windows_only"].(bool); ok {
					k.WindowsOnly = v
				}
				if v, ok := st["extra_files"].(float64); ok {
					k.ExtraFiles = int(v)
				}
				if v, ok := st["chunk_delay_ms"].(float64); ok {
					k.ChunkDelayMs = int(v)
				}
			}
		}
	}
	k.Steamworks = os.Getenv("FAKE_DD_STEAMWORKS")
	return k
}

// TestKey is the fixed depot key fixtures encrypt with.
func TestKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i*3 + 7)
	}
	return k
}

type fileSpec struct {
	name  string
	body  []byte
	flags uint32
}

// Build assembles the fixture for the knobs.
func Build(k Knobs) *Fixture {
	key := TestKey()
	var specs []fileSpec
	osUsed := "linux"
	if k.WindowsOnly {
		osUsed = "windows"
		specs = append(specs, fileSpec{name: "game.exe", body: []byte("MZ fake game executable")})
		if k.Steamworks != "" {
			specs = append(specs, fileSpec{name: "libs/" + k.Steamworks, body: []byte("fake steamworks")})
		}
	} else if k.BareELF {
		body := append([]byte("\x7fELF fake binary"), bytes.Repeat([]byte(" "), 4096)...)
		if k.UnityDecoy {
			// A bigger ELF launcher next to a Unity player + data dir:
			// the player is the game.
			specs = append(specs,
				fileSpec{name: "Game.x86_64", body: []byte("\x7fELF unity player"), flags: 0},
				fileSpec{name: "Game_Data/placeholder", body: []byte("data")},
				fileSpec{name: "BareGame", body: append([]byte("\x7fELF fake binary"), bytes.Repeat([]byte(" "), 8192)...)})
		} else {
			specs = append(specs, fileSpec{name: "BareGame", body: body})
		}
	} else {
		specs = append(specs, fileSpec{name: "game.sh",
			body:  []byte("#!/bin/sh\necho 'steam standalone game'\n"),
			flags: depot.FlagExecutable})
		if k.Steamworks != "" {
			specs = append(specs, fileSpec{name: "libs/" + k.Steamworks, body: []byte("fake steamworks")})
		}
	}
	for i := 0; i < k.ExtraFiles; i++ {
		specs = append(specs, fileSpec{name: fmt.Sprintf("data/extra%02d.bin", i),
			body: bytes.Repeat([]byte{byte(i + 1)}, 2048)})
	}
	fix := &Fixture{Key: key, DepotID: 11, GID: 999, OSUsed: osUsed, Chunks: map[string][]byte{}}
	var files []depot.File
	for _, sp := range specs {
		plain := sp.body
		container := depot.ZstdCompress(plain)
		iv := make([]byte, 16)
		rand.Read(iv)
		enc, err := depot.EncryptChunk(key, iv, container)
		if err != nil {
			panic(err)
		}
		id := sha1.Sum(plain)
		c := depot.Chunk{ID: id[:], Checksum: depot.Adler0(plain),
			CompressedLength: uint32(len(enc)), UncompressedLength: uint32(len(plain))}
		fix.Chunks[fmt.Sprintf("%x", id[:])] = enc
		fix.CompressedBytes += uint64(len(enc))
		h := sha1.Sum(plain)
		files = append(files, depot.File{Name: sp.name, Size: uint64(len(plain)),
			Flags: sp.flags, FileHash: h[:], Chunks: []depot.Chunk{c}})
		fix.DiskBytes += uint64(len(plain))
	}
	meta := depot.Metadata{DepotID: fix.DepotID, GID: fix.GID, DiskOriginal: fix.DiskBytes,
		DiskCompressed: fix.CompressedBytes, UniqueChunks: uint32(len(fix.Chunks))}
	raw, err := depot.BuildManifest(meta, files, key, false)
	if err != nil {
		panic(err)
	}
	m, err := depot.ParseManifest(raw)
	if err != nil {
		panic(err)
	}
	fix.Manifest = m
	fix.Raw = raw
	return fix
}

// CDN is a fake content server farm serving one fixture.
type CDN struct {
	t       *testing.T
	servers []*httptest.Server
	// Hosts are "127.0.0.1:port" entries for the fake CM state.
	Hosts []string
	Hits  *int64
	fix   *Fixture
}

// StartCDN serves fix from two httptest hosts (so multi-host spread is
// exercised even in plugin tests).
func StartCDN(t *testing.T, fix *Fixture) *CDN {
	t.Helper()
	c := &CDN{t: t, Hits: new(int64), fix: fix}
	delay := time.Duration(ReadKnobs().ChunkDelayMs) * time.Millisecond
	var chunkMu sync.Mutex // serves chunks one at a time when a delay is set
	for i := 0; i < 2; i++ {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt64(c.Hits, 1)
			path := strings.TrimPrefix(r.URL.Path, "/")
			parts := strings.Split(path, "/")
			if len(parts) >= 4 && parts[0] == "depot" && parts[2] == "manifest" {
				w.Write(fix.Raw)
				return
			}
			if len(parts) == 4 && parts[0] == "depot" && parts[2] == "chunk" {
				if enc, ok := fix.Chunks[parts[3]]; ok {
					if delay > 0 {
						chunkMu.Lock()
						defer chunkMu.Unlock()
						time.Sleep(delay)
					}
					w.Write(enc)
					return
				}
				http.NotFound(w, r)
				return
			}
			http.NotFound(w, r)
		}))
		t.Cleanup(srv.Close)
		c.servers = append(c.servers, srv)
		c.Hosts = append(c.Hosts, strings.TrimPrefix(srv.URL, "http://"))
	}
	return c
}

// WriteCMState points the fake CM at the fixture: depot key, hosts, gid
// and oslist. It preserves the other knobs already in the state file.
func WriteCMState(t *testing.T, cmState string, cdn *CDN, fix *Fixture) {
	t.Helper()
	var st map[string]any
	if raw, err := os.ReadFile(cmState); err == nil {
		json.Unmarshal(raw, &st)
	}
	if st == nil {
		st = map[string]any{}
	}
	st["depot_key"] = hex.EncodeToString(fix.Key)
	st["cdn_hosts"] = cdn.Hosts
	st["depot_id"] = fix.DepotID
	st["manifest_gid"] = fix.GID
	st["oslist"] = fix.OSUsed
	st["request_code"] = 4242
	raw, _ := json.Marshal(st)
	if err := os.WriteFile(cmState, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
