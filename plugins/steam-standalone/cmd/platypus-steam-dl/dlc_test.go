package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/depot"
	"platypus/internal/steam"
)

// dlcappid rides on the depot entry (DepotDownloader reads it the same
// way): selected depots carry it, config-less and garbage ones read zero.
func TestSelectDepotsReadsDLCAppID(t *testing.T) {
	src := `"depots" {` +
		`"11" { "config" { "oslist" "linux" } "dlcappid" "100" "manifests" { "public" { "gid" "111" } } }` +
		`"12" { "config" { "oslist" "linux" } "manifests" { "public" { "gid" "112" } } }` +
		`"13" { "config" { "oslist" "linux" } "dlcappid" "junk" "manifests" { "public" { "gid" "113" } } }` +
		`"14" { "config" { "oslist" "linux" } "dlcappid" "200" "depotfromapp" "543" }` +
		`}`
	kv, err := steam.ParseKV(src)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := selectDepots(kv, 4321, "public", "linux")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[uint32]DepotRef{}
	for _, r := range refs {
		byID[r.DepotID] = r
	}
	if byID[11].DLCAppID != 100 {
		t.Fatalf("depot 11 dlcappid = %d, want 100", byID[11].DLCAppID)
	}
	if byID[12].DLCAppID != 0 {
		t.Fatalf("depot 12 without dlcappid = %d, want base 0", byID[12].DLCAppID)
	}
	if byID[13].DLCAppID != 0 {
		t.Fatalf("depot 13 with garbage dlcappid = %d, want base 0", byID[13].DLCAppID)
	}
	if byID[14].DLCAppID != 200 || byID[14].FromApp != 543 {
		t.Fatalf("shared depot 14 = %+v, want dlc 200 from 543", byID[14])
	}
}

func TestListDLCParsesListOfDLC(t *testing.T) {
	kv, err := steam.ParseKV(`"extended" { "listofdlc" "100, 200,100" }`)
	if err != nil {
		t.Fatal(err)
	}
	if got := listDLC(kv); len(got) != 2 || got[0] != 100 || got[1] != 200 {
		t.Fatalf("listofdlc = %v, want [100 200] sorted and deduped", got)
	}
	for _, src := range []string{`"common" {}`, `"extended" { "listofdlc" "junk,,0" }`} {
		kv, err := steam.ParseKV(src)
		if err != nil {
			t.Fatal(err)
		}
		if got := listDLC(kv); len(got) != 0 {
			t.Fatalf("%s: listofdlc = %v, want empty", src, got)
		}
	}
}

func TestFilterDepotsByDLC(t *testing.T) {
	refs := []DepotRef{{DepotID: 11}, {DepotID: 12, DLCAppID: 100}, {DepotID: 13, DLCAppID: 200}}
	kept, dropped := filterDepotsByDLC(refs, nil)
	if len(kept) != 3 || len(dropped) != 0 {
		t.Fatalf("nil keep must install everything: kept %v dropped %v", kept, dropped)
	}
	kept, dropped = filterDepotsByDLC(refs, map[uint32]bool{100: true})
	if len(kept) != 2 || len(dropped) != 1 || dropped[0].DepotID != 13 {
		t.Fatalf("selecting 100 keeps base + 100: kept %v dropped %v", kept, dropped)
	}
	kept, dropped = filterDepotsByDLC(refs, map[uint32]bool{})
	if len(kept) != 1 || kept[0].DepotID != 11 || len(dropped) != 2 {
		t.Fatalf("empty selection keeps only the base: kept %v dropped %v", kept, dropped)
	}
	if keep, err := dlcKeepSet(nil); keep != nil || err != nil {
		t.Fatalf("nil request = no filtering: %v, %v", keep, err)
	}
	if keep, err := dlcKeepSet([]string{"100"}); err != nil || !keep[100] || len(keep) != 1 {
		t.Fatalf("keep {100} = %v, %v", keep, err)
	}
	if _, err := dlcKeepSet([]string{"junk"}); err == nil {
		t.Fatal("garbage dlc id must error, not filter silently")
	}
}

// A deselected DLC loses its depots: files no kept depot carries go, files
// a kept depot still lists stay, and the retired depot's saved manifests
// are forgotten. Unencrypted names need no CM round trip.
func TestRemoveRetiredDepotsKeepsSharedFiles(t *testing.T) {
	dataDir := t.TempDir()
	installDir := t.TempDir()
	const appID = 4321
	key := make([]byte, 32)
	mkfile := func(name string, body []byte) depot.File {
		return depot.File{Name: name, Size: uint64(len(body)), FileHash: []byte{1}}
	}
	meta := func(depotID uint32, gid uint64) depot.Metadata {
		return depot.Metadata{DepotID: depotID, GID: gid}
	}
	// Kept base depot: game.sh plus shared.txt; retired DLC depot:
	// the same shared.txt plus its own dlc.dat.
	baseFiles := []depot.File{mkfile("game.sh", []byte("base")), mkfile("shared.txt", []byte("shared"))}
	dlcFiles := []depot.File{mkfile("shared.txt", []byte("shared")), mkfile("dlc.dat", []byte("dlc"))}
	baseRaw, err := depot.BuildManifest(meta(11, 111), baseFiles, key, false)
	if err != nil {
		t.Fatal(err)
	}
	dlcRaw, err := depot.BuildManifest(meta(12, 112), dlcFiles, key, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := depot.SaveManifest(dataDir, appID, 11, 111, baseRaw); err != nil {
		t.Fatal(err)
	}
	if err := depot.SaveManifest(dataDir, appID, 12, 112, dlcRaw); err != nil {
		t.Fatal(err)
	}
	for _, f := range []depot.File{mkfile("game.sh", nil), mkfile("shared.txt", nil), mkfile("dlc.dat", nil)} {
		if err := os.WriteFile(filepath.Join(installDir, f.Name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	combined, err := depot.ParseManifest(baseRaw)
	if err != nil {
		t.Fatal(err)
	}
	s := &session{dataDir: dataDir}
	if err := removeRetiredDepots(s, appID, map[uint32]bool{11: true}, combined, installDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "dlc.dat")); !os.IsNotExist(err) {
		t.Fatalf("dlc.dat must go with its retired depot: %v", err)
	}
	for _, keep := range []string{"game.sh", "shared.txt"} {
		if _, err := os.Stat(filepath.Join(installDir, keep)); err != nil {
			t.Fatalf("%s must stay: %v", keep, err)
		}
	}
	if got := loadInstalledAll(dataDir, appID, 12); len(got) != 0 {
		t.Fatalf("retired depot manifests must be forgotten, have %d", len(got))
	}
	if got := loadInstalledAll(dataDir, appID, 11); len(got) != 1 {
		t.Fatalf("kept depot manifests must stay, have %d", len(got))
	}
	// Nothing retired: no PICS, no error, nothing touched.
	if err := removeRetiredDepots(s, appID, map[uint32]bool{11: true}, combined, installDir); err != nil {
		t.Fatal(err)
	}
}

// Only files go; directories stay unless empty, and a user file inside a
// DLC folder keeps the folder. A folder the kept depot still lists is
// never named for removal.
func TestRetireDepotKeepsDirsWithUserFiles(t *testing.T) {
	installDir := t.TempDir()
	key := make([]byte, 32)
	dir := func(name string) depot.File {
		return depot.File{Name: name, Flags: depot.FlagDirectory}
	}
	file := func(name string) depot.File {
		return depot.File{Name: name, Size: 1, FileHash: []byte{1}}
	}
	meta := depot.Metadata{DepotID: 12, GID: 112}
	// Retired DLC depot: its own folder with one shipped file, plus a
	// folder shared with the base depot. The kept base depot lists the
	// shared folder and its file.
	dlcFiles := []depot.File{dir("dlc"), file("dlc/a.dat"), dir("shared"), file("shared/s.dat")}
	baseFiles := []depot.File{dir("shared"), file("shared/s.dat")}
	dlcRaw, err := depot.BuildManifest(meta, dlcFiles, key, false)
	if err != nil {
		t.Fatal(err)
	}
	baseRaw, err := depot.BuildManifest(depot.Metadata{DepotID: 11, GID: 111}, baseFiles, key, false)
	if err != nil {
		t.Fatal(err)
	}
	dlcMan, err := depot.ParseManifest(dlcRaw)
	if err != nil {
		t.Fatal(err)
	}
	combined, err := depot.ParseManifest(baseRaw)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dlc/a.dat", "dlc/keep.txt", "shared/s.dat"} {
		p := filepath.Join(installDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := retireDepot(t.TempDir(), installDir, 4321, 12, []*depot.Manifest{dlcMan}, combined, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "dlc", "a.dat")); !os.IsNotExist(err) {
		t.Fatalf("shipped DLC file must go: %v", err)
	}
	// The user's file keeps its folder; the shared folder stays with
	// the base depot's file.
	for _, keep := range []string{"dlc/keep.txt", "shared/s.dat"} {
		if _, err := os.Stat(filepath.Join(installDir, filepath.FromSlash(keep))); err != nil {
			t.Fatalf("%s must stay: %v", keep, err)
		}
	}
	if _, err := os.Stat(filepath.Join(installDir, "dlc")); err != nil {
		t.Fatalf("non-empty DLC folder must stay: %v", err)
	}
	// An emptied DLC folder is removed.
	if err := os.Remove(filepath.Join(installDir, "dlc", "keep.txt")); err != nil {
		t.Fatal(err)
	}
	if err := retireDepot(t.TempDir(), installDir, 4321, 12, []*depot.Manifest{dlcMan}, combined, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "dlc")); !os.IsNotExist(err) {
		t.Fatalf("emptied DLC folder must go: %v", err)
	}
}

// Filenames-encrypted manifests decrypt with the depot key before the
// diff; a wrong key is an error (the caller leaves the files and forgets
// the manifests instead of failing the update).
func TestRetireDepotEncryptedNames(t *testing.T) {
	installDir := t.TempDir()
	key := bytes.Repeat([]byte{0xA5}, 32)
	files := []depot.File{{Name: "dlc.dat", Size: 3, FileHash: []byte{1}}}
	raw, err := depot.BuildManifest(depot.Metadata{DepotID: 12, GID: 112}, files, key, true)
	if err != nil {
		t.Fatal(err)
	}
	combined, err := depot.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	// The combined manifest only carries the base file: dlc.dat is gone.
	combined.Files = nil
	man, err := depot.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !man.Meta.FilenamesEncrypted {
		t.Fatal("fixture must encrypt names")
	}
	p := filepath.Join(installDir, "dlc.dat")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := retireDepot(t.TempDir(), installDir, 4321, 12, []*depot.Manifest{man}, combined, key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("dlc.dat must go with the right key: %v", err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	man, err = depot.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := retireDepot(t.TempDir(), installDir, 4321, 12, []*depot.Manifest{man}, combined, bytes.Repeat([]byte{0xB5}, 32)); err == nil {
		t.Fatal("wrong key must error")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("dlc.dat must stay with the wrong key: %v", err)
	}
}
