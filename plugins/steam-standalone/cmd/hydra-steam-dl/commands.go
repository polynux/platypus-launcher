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
	// Streaming install (STEAM-1): chunks go straight to final files via
	// WriteAt as they land. Only chunk IDs + staging paths stay in RAM;
	// no whole-game map, no full staging copy.
	jobID := req.JobID
	if jobID == "" {
		jobID = fmt.Sprintf("%d", req.AppID)
	}
	total := countChunks(depots)
	sched := depot.NewScheduler(depot.NewClient(), depot.SchedulerConfig{})
	onProg, _, _ := progress(totalBytes(depots))
	state := &depot.JobState{AppID: req.AppID, ManifestID: manifestGID(depots), Done: map[string]bool{}, Total: total}
	for id := range stagingIDs(ss.dataDir, jobID) {
		state.Done[id] = true
	}
	flush := 0
	for _, d := range depots {
		w, err := depot.Preallocate(d.man, req.InstallDir)
		if err != nil {
			depot.SaveJob(ss.dataDir, jobID, state)
			return err
		}
		want := chunkInfo(d.man)
		skip := map[string]bool{}
		// Resume: replay staged chunks one at a time (bounded), then
		// skip them in the download.
		if err := restoreStaging(ss.dataDir, jobID, want, w, skip, state); err != nil {
			w.Close()
			depot.SaveJob(ss.dataDir, jobID, state)
			return err
		}
		sink := func(id string, data []byte) error {
			if err := w.Write(id, data); err != nil {
				return err
			}
			saveStagedChunk(ss.dataDir, jobID, id, data)
			state.Done[hexChunk([]byte(id))] = true
			flush++
			if flush >= 50 {
				flush = 0
				depot.SaveJob(ss.dataDir, jobID, state)
			}
			return nil
		}
		dlErr := sched.DownloadStream(ctx, d.man, d.ref.DepotID, d.hosts, d.key, skip, sink, onProg)
		cErr := w.Close()
		if dlErr != nil {
			depot.SaveJob(ss.dataDir, jobID, state)
			return dlErr
		}
		if cErr != nil {
			depot.SaveJob(ss.dataDir, jobID, state)
			return cErr
		}
		depot.SaveJob(ss.dataDir, jobID, state)
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
		// Gone files: delete after collecting across all saved branch
		// manifests (disk may match any of them).
		var gone []string
		seenGone := map[string]bool{}
		for _, old := range oldManifests {
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
		// Streaming update (STEAM-1): preallocate, copy reusable chunks
		// from disk one at a time (bounded), fetch only the rest.
		w, err := depot.Preallocate(d.man, req.InstallDir)
		if err != nil {
			return err
		}
		skip := reuseFromDisk(req.InstallDir, oldManifests, d.man, w)
		dlErr := sched.DownloadStream(ctx, d.man, d.ref.DepotID, d.hosts, d.key, skip, func(id string, data []byte) error {
			return w.Write(id, data)
		}, func(p depot.Progress) { onProg(p) })
		cErr := w.Close()
		if dlErr != nil {
			return dlErr
		}
		if cErr != nil {
			return cErr
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
// jobs/<job>.chunks/<hex> as it arrives (fsynced), so an interrupted
// install replays them one at a time and fetches only what's missing.
// Only chunk IDs stay in RAM; bytes are never all loaded at once.
func stagingDir(dataDir, jobID string) string {
	return filepath.Join(dataDir, "jobs", jobID+".chunks")
}

func saveStagedChunk(dataDir, jobID, id string, data []byte) {
	dir := stagingDir(dataDir, jobID)
	os.MkdirAll(dir, 0o755)
	p := filepath.Join(dir, hexChunk([]byte(id)))
	os.WriteFile(p, data, 0o644)
	if fh, err := os.OpenFile(p, os.O_RDWR, 0o644); err == nil {
		fh.Sync()
		fh.Close()
	}
}

// stagingIDs lists staged chunk hex IDs without loading any bytes.
func stagingIDs(dataDir, jobID string) map[string]bool {
	out := map[string]bool{}
	entries, err := os.ReadDir(stagingDir(dataDir, jobID))
	if err != nil {
		return out
	}
	for _, e := range entries {
		if len(e.Name()) == 40 {
			out[e.Name()] = true
		}
	}
	return out
}

// chunkInfo maps raw chunk id -> chunk for checksum/length checks.
func chunkInfo(m *depot.Manifest) map[string]depot.Chunk {
	out := map[string]depot.Chunk{}
	for _, c := range m.UniqueChunks() {
		out[string(c.ID)] = c
	}
	return out
}

// restoreStaging replays staged chunks into the writer one at a time,
// verifying checksum/length and skipping bad entries for re-download.
func restoreStaging(dataDir, jobID string, want map[string]depot.Chunk, w *depot.InstallWriter, skip map[string]bool, state *depot.JobState) error {
	dir := stagingDir(dataDir, jobID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		id, err := hex.DecodeString(e.Name())
		if err != nil {
			continue
		}
		key := string(id)
		c, ok := want[key]
		if !ok || uint32(len(raw)) != c.UncompressedLength || depot.Adler0(raw) != c.Checksum {
			os.Remove(filepath.Join(dir, e.Name()))
			continue
		}
		if err := w.Write(key, raw); err != nil {
			return err
		}
		skip[key] = true
		state.Done[e.Name()] = true
		raw = nil
	}
	return nil
}

// reuseFromDisk copies unchanged chunks from installed files into the
// writer one chunk at a time (bounded to one chunk in RAM), returning the
// set of chunk IDs to skip downloading. Bytes are Adler-checked, so
// foreign layouts miss and re-download.
func reuseFromDisk(installDir string, olds []*depot.Manifest, newM *depot.Manifest, w *depot.InstallWriter) map[string]bool {
	skip := map[string]bool{}
	want := chunkInfo(newM)
	// chunk id -> old locations holding it.
	type loc struct {
		path   string
		offset uint64
	}
	holders := map[string][]loc{}
	for _, old := range olds {
		for _, f := range old.Files {
			if f.IsDir() || f.IsSymlink() {
				continue
			}
			p := filepath.Join(installDir, filepath.FromSlash(f.Name))
			for _, c := range f.Chunks {
				if _, ok := want[string(c.ID)]; !ok {
					continue
				}
				holders[string(c.ID)] = append(holders[string(c.ID)], loc{path: p, offset: c.Offset})
			}
		}
	}
	handles := map[string]*os.File{}
	defer func() {
		for _, fh := range handles {
			fh.Close()
		}
	}()
	for id, c := range want {
		for _, h := range holders[id] {
			fh, ok := handles[h.path]
			if !ok {
				f, err := os.Open(h.path)
				if err != nil {
					continue
				}
				handles[h.path] = f
				fh = f
			}
			buf := make([]byte, c.UncompressedLength)
			n, err := fh.ReadAt(buf, int64(h.offset))
			if err != nil && n != len(buf) {
				continue
			}
			if uint32(len(buf)) != c.UncompressedLength || depot.Adler0(buf) != c.Checksum {
				continue
			}
			if err := w.Write(id, buf); err != nil {
				break
			}
			skip[id] = true
			break
		}
	}
	return skip
}

func clearStaging(dataDir, jobID string) {
	os.RemoveAll(stagingDir(dataDir, jobID))
}
