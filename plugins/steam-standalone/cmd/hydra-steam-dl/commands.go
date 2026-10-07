// Subcommand implementations: size, install, update and verify.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"hydra-launcher/internal/depot"
)

func runSize(ctx context.Context, req request) error {
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return err
	}
	defer cm.Close()
	ss := &session{cm: cm, dataDir: dataDir(), req: req}
	depots, _, err := fetchDepots(ctx, ss)
	if err != nil {
		return err
	}
	var disk, compressed uint64
	seen := map[string]bool{}
	var depotIDs []uint32
	for _, d := range depots {
		depotIDs = append(depotIDs, d.ref.DepotID)
		for _, f := range d.man.Files {
			disk += f.Size
			for _, c := range f.Chunks {
				if seen[string(c.ID)] {
					continue
				}
				seen[string(c.ID)] = true
				compressed += uint64(c.CompressedLength)
			}
		}
	}
	emit(map[string]any{"type": "result", "download_bytes": compressed, "disk_bytes": disk,
		"depots": len(depots), "depot_ids": depotIDs})
	return nil
}

func runInstall(ctx context.Context, req request) error {
	if req.InstallDir == "" {
		return fmt.Errorf("install_dir is missing")
	}
	if err := os.MkdirAll(req.InstallDir, 0o755); err != nil {
		return err
	}
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return err
	}
	defer cm.Close()
	ss := &session{cm: cm, dataDir: dataDir(), req: req}
	depots, osUsed, err := fetchDepots(ctx, ss)
	if err != nil {
		return err
	}
	// Resume: chunks staged by an interrupted run come back through
	// merged, re-checked by checksum inside Download.
	jobID := req.JobID
	if jobID == "" {
		jobID = fmt.Sprintf("%d", req.AppID)
	}
	total := countChunks(depots)
	sched := depot.NewScheduler(depot.NewClient(), depot.SchedulerConfig{})
	onProg, _, _ := progress(totalBytes(depots))
	// Resume: chunks staged by an interrupted run come back through
	// have, re-checked by checksum inside Download.
	merged := loadStaging(ss.dataDir, jobID)
	state := &depot.JobState{AppID: req.AppID, ManifestID: manifestGID(depots), Done: map[string]bool{}, Total: total}
	for id := range merged {
		state.Done[hexChunk([]byte(id))] = true
	}
	flush := 0
	wrapped := func(p depot.Progress) {
		onProg(p)
		if p.ChunkID != "" && len(p.Data) > 0 {
			merged[p.ChunkID] = p.Data
			state.Done[hexChunk([]byte(p.ChunkID))] = true
			saveStagedChunk(ss.dataDir, jobID, p.ChunkID, p.Data)
			flush++
			if flush >= 50 {
				flush = 0
				depot.SaveJob(ss.dataDir, jobID, state)
			}
		}
	}
	for _, d := range depots {
		data, err := sched.Download(ctx, d.man, d.ref.DepotID, d.hosts, d.key, merged, wrapped)
		if err != nil {
			depot.SaveJob(ss.dataDir, jobID, state)
			return err
		}
		for id, plain := range data {
			merged[id] = plain
			state.Done[hexChunk([]byte(id))] = true
		}
		if err := depot.WriteChunks(d.man, req.InstallDir, data); err != nil {
			depot.SaveJob(ss.dataDir, jobID, state)
			return err
		}
		if err := depot.SaveManifest(ss.dataDir, req.AppID, d.ref.DepotID, d.ref.GID, d.raw); err != nil {
			return err
		}
	}
	depot.RemoveJob(ss.dataDir, jobID)
	clearStaging(ss.dataDir, jobID)
	bad, err := depot.Verify(installedManifests(depots), req.InstallDir)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return fmt.Errorf("verify failed after install: %v", bad)
	}
	emit(map[string]any{"type": "result", "os_used": osUsed, "bytes": totalBytes(depots)})
	return nil
}

func runUpdate(ctx context.Context, req request) error {
	if req.InstallDir == "" {
		return fmt.Errorf("install_dir is missing")
	}
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return err
	}
	defer cm.Close()
	ss := &session{cm: cm, dataDir: dataDir(), req: req}
	depots, osUsed, err := fetchDepots(ctx, ss)
	if err != nil {
		return err
	}
	sched := depot.NewScheduler(depot.NewClient(), depot.SchedulerConfig{})
	onProg, _, _ := progress(totalBytes(depots))
	for _, d := range depots {
		oldManifests := loadInstalledAll(ss.dataDir, req.AppID, d.ref.DepotID)
		// Saved manifests keep encrypted filenames; decrypt the old ones
		// with this depot's key so reuse reads the right paths (missed
		// live: every reuse lookup failed and updates re-downloaded).
		for _, old := range oldManifests {
			if old.Meta.FilenamesEncrypted {
				if err := old.DecryptFilenames(d.key); err != nil {
					return err
				}
			}
		}
		// Reuse chunks from every saved manifest: several gids can be
		// stored (one per installed branch), and the disk may match any
		// of them. Bytes are Adler-checked, so foreign layouts miss.
		var have map[string][]byte
		var gone []string
		seenGone := map[string]bool{}
		for _, old := range oldManifests {
			for id, plain := range readOldChunks(req.InstallDir, old) {
				if have == nil {
					have = map[string][]byte{}
				}
				have[id] = plain
			}
			_, g := depot.Diff(old, d.man)
			for _, name := range g {
				if !seenGone[name] {
					seenGone[name] = true
					gone = append(gone, name)
				}
			}
		}
		for _, name := range gone {
			os.Remove(filepath.Join(req.InstallDir, filepath.FromSlash(name)))
		}
		data, err := sched.Download(ctx, d.man, d.ref.DepotID, d.hosts, d.key, have, func(p depot.Progress) { onProg(p) })
		if err != nil {
			return err
		}
		// Merge reused chunks for the write.
		for id, plain := range have {
			if _, ok := data[id]; !ok {
				data[id] = plain
			}
		}
		if err := depot.WriteChunks(d.man, req.InstallDir, data); err != nil {
			return err
		}
		if err := depot.SaveManifest(ss.dataDir, req.AppID, d.ref.DepotID, d.ref.GID, d.raw); err != nil {
			return err
		}
	}
	emit(map[string]any{"type": "result", "os_used": osUsed, "bytes": totalBytes(depots)})
	return nil
}

func runVerify(ctx context.Context, req request) error {
	if req.InstallDir == "" {
		return fmt.Errorf("install_dir is missing")
	}
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return err
	}
	defer cm.Close()
	ss := &session{cm: cm, dataDir: dataDir(), req: req}
	depots, _, err := fetchDepots(ctx, ss)
	if err != nil {
		return err
	}
	combined := installedManifests(depots)
	bad, err := depot.Verify(combined, req.InstallDir)
	if err != nil {
		return err
	}
	if len(bad) == 0 {
		emit(map[string]any{"type": "result", "bad": 0, "fixed": 0})
		return nil
	}
	// Re-fetch the chunks of mismatched files only.
	sched := depot.NewScheduler(depot.NewClient(), depot.SchedulerConfig{})
	fixed := 0
	for _, d := range depots {
		var want []string
		for _, f := range d.man.Files {
			for _, b := range bad {
				if f.Name == b {
					want = append(want, f.Name)
				}
			}
		}
		if len(want) == 0 {
			continue
		}
		sub := filterFiles(d.man, want)
		data, err := sched.Download(ctx, sub, d.ref.DepotID, ss.hosts, d.key, nil, nil)
		if err != nil {
			return err
		}
		if err := depot.WriteChunks(sub, req.InstallDir, data); err != nil {
			return err
		}
		fixed += len(want)
	}
	emit(map[string]any{"type": "result", "bad": len(bad), "fixed": fixed})
	return nil
}

func countChunks(depots []fetchedDepot) int {
	seen := map[string]bool{}
	for _, d := range depots {
		for _, c := range d.man.UniqueChunks() {
			seen[string(c.ID)] = true
		}
	}
	return len(seen)
}

func totalBytes(depots []fetchedDepot) uint64 {
	var n uint64
	for _, d := range depots {
		n += d.man.TotalSize()
	}
	return n
}

func manifestGID(depots []fetchedDepot) uint64 {
	if len(depots) > 0 {
		return depots[0].man.GID
	}
	return 0
}

func installedManifests(depots []fetchedDepot) *depot.Manifest {
	combined := &depot.Manifest{}
	for _, d := range depots {
		combined.Files = append(combined.Files, d.man.Files...)
	}
	return combined
}

func loadInstalled(dataDir string, appID, depotID uint32) *depot.Manifest {
	all := loadInstalledAll(dataDir, appID, depotID)
	if len(all) == 0 {
		return nil
	}
	return all[0]
}

// loadInstalledAll reads every saved manifest for a depot (one per
// installed branch gid), oldest first by file name.
func loadInstalledAll(dataDir string, appID, depotID uint32) []*depot.Manifest {
	dir := filepath.Join(dataDir, "manifests", fmt.Sprint(appID))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	prefix := fmt.Sprintf("%d_", depotID)
	var out []*depot.Manifest
	for _, e := range entries {
		if len(e.Name()) > len(prefix) && e.Name()[:len(prefix)] == prefix {
			raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			if m, err := depot.ParseManifest(raw); err == nil {
				out = append(out, m)
			}
		}
	}
	return out
}

// readOldChunks reuses unchanged chunks by copying them from the installed
// files, checked by checksum like a resume.
func readOldChunks(dir string, old *depot.Manifest) map[string][]byte {
	out := map[string][]byte{}
	for _, f := range old.Files {
		if f.IsDir() || f.IsSymlink() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Name)))
		if err != nil {
			continue
		}
		for _, c := range f.Chunks {
			if uint64(len(raw)) < c.Offset+uint64(c.UncompressedLength) {
				continue
			}
			plain := append([]byte{}, raw[c.Offset:c.Offset+uint64(c.UncompressedLength)]...)
			if depot.Adler0(plain) != c.Checksum {
				continue
			}
			out[string(c.ID)] = plain
		}
	}
	return out
}

func filterFiles(m *depot.Manifest, names []string) *depot.Manifest {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	sub := &depot.Manifest{GID: m.GID, DepotID: m.DepotID, Meta: m.Meta}
	for _, f := range m.Files {
		if want[f.Name] {
			sub.Files = append(sub.Files, f)
		}
	}
	return sub
}

// Chunk staging for resume: each landed chunk is written to
// jobs/<job>.chunks/<hex> as it arrives, so an interrupted install
// re-checks them by checksum and fetches only what's missing.
func stagingDir(dataDir, jobID string) string {
	return filepath.Join(dataDir, "jobs", jobID+".chunks")
}

func saveStagedChunk(dataDir, jobID, id string, data []byte) {
	dir := stagingDir(dataDir, jobID)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, hexChunk([]byte(id))), data, 0o644)
}

func loadStaging(dataDir, jobID string) map[string][]byte {
	out := map[string][]byte{}
	entries, err := os.ReadDir(stagingDir(dataDir, jobID))
	if err != nil {
		return out
	}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(stagingDir(dataDir, jobID), e.Name()))
		if err != nil {
			continue
		}
		id, err := hex.DecodeString(e.Name())
		if err != nil {
			continue
		}
		out[string(id)] = raw
	}
	return out
}

func clearStaging(dataDir, jobID string) {
	os.RemoveAll(stagingDir(dataDir, jobID))
}
