// Shader pre-caching download (issues #205, #211): per-GPU/driver Fossilize
// buckets from the Shader service, committed under the request's shader
// dir (the steamapps/shadercache/<appid> mirror next to the game install).
// Content-system only: each bucket manifest resolves to depot == appid,
// like the client fetches it — no PICS, no branch, no DLC filtering.
// The helper downloads the buckets and runs nothing: the plugin replays
// .foz files afterwards, and the core points drivers at the tree at
// launch (see docs/research/steam-shader-cache.md).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"platypus/internal/depot"
	"platypus/internal/steamcm"
)

// swarmBucket is the shared precompiled bucket every client fetches: it
// holds the bulk of the cache (gigabytes for big games), while the
// per-GPU buckets carry small deltas.
var swarmBucket = [2]string{"SteamSwarm", "G7:VulkanPipelinesV6"}

// noMatchEResult is Steam's EResult 42: the Shader service's answer for a
// (gpu, driver) pair it never built a bucket for.
const noMatchEResult = 42

// shaderPairs lists the (gpu, driver) buckets to query: the shared swarm
// bucket first, then the positional CurrentBucketGPU/Driver pairs from
// every Steam client config on the machine (the client's own bucket keys,
// readable once the client has run here). A machine whose client never
// ran still gets the swarm bucket.
func shaderPairs() [][2]string {
	out := [][2]string{swarmBucket}
	seen := map[string]bool{pairKey(swarmBucket): true}
	reGPU := regexp.MustCompile(`"CurrentBucketGPU"\s+"([^"]*)"`)
	reDrv := regexp.MustCompile(`"CurrentBucketDriver"\s+"([^"]*)"`)
	for _, cfg := range steamClientConfigs() {
		raw, err := os.ReadFile(cfg)
		if err != nil {
			continue
		}
		mg, md := reGPU.FindSubmatch(raw), reDrv.FindSubmatch(raw)
		if mg == nil || md == nil {
			continue
		}
		gpus := strings.Split(string(mg[1]), ";")
		drvs := strings.Split(string(md[1]), ";")
		for i := 0; i < len(gpus) && i < len(drvs); i++ {
			g, d := strings.TrimSpace(gpus[i]), strings.TrimSpace(drvs[i])
			if g == "" || d == "" {
				continue
			}
			if k := pairKey([2]string{g, d}); !seen[k] {
				seen[k] = true
				out = append(out, [2]string{g, d})
			}
		}
	}
	return out
}

func pairKey(p [2]string) string { return p[0] + "\x00" + p[1] }

// bucketManifest asks the Shader service for one pair's bucket: the
// bucket id, the content manifest to download (zero: no bucket published
// for this app+pair, a soft skip) and its byte size. An unknown pair is
// EResult 42 NoMatch, also a soft skip; anything else is an error.
func bucketManifest(cm *steamcm.Session, appID uint32, pair [2]string) (bucket, manifest, size uint64, skip bool, err error) {
	e := &depot.Enc{}
	e.Varint(1, uint64(appID))
	e.Str(2, pair[0])
	e.Str(3, pair[1])
	raw, err := cm.Call("Shader.GetBucketManifest#1", e.Bytes(), 60*time.Second)
	if err != nil {
		if ce, ok := err.(*steamcm.CMError); ok && ce.EResult == noMatchEResult {
			return 0, 0, 0, true, nil
		}
		return 0, 0, 0, false, callErr("shader bucket", err)
	}
	m, err := depot.Decode(raw)
	if err != nil {
		return 0, 0, 0, false, callErr("shader bucket", err)
	}
	return m.Int(3), m.Int(1), m.Int(2), m.Int(1) == 0, nil
}

// shaderState records the downloaded bucket manifest gid per pair, so a
// later run refetches only changed buckets. It lives outside manifests/
// (whose depot namespace the game installs own) as one JSON file per app.
func shaderStatePath(dataDir string, appID uint32) string {
	return filepath.Join(dataDir, "shader", fmt.Sprint(appID)+".json")
}

func loadShaderState(dataDir string, appID uint32) map[string]uint64 {
	raw, err := os.ReadFile(shaderStatePath(dataDir, appID))
	if err != nil {
		return map[string]uint64{}
	}
	var st map[string]uint64
	if err := json.Unmarshal(raw, &st); err != nil || st == nil {
		return map[string]uint64{}
	}
	return st
}

func saveShaderState(dataDir string, appID uint32, st map[string]uint64) error {
	dir := filepath.Join(dataDir, "shader")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, _ := json.Marshal(st)
	tmp := filepath.Join(dir, fmt.Sprint(appID)+".json.new")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, shaderStatePath(dataDir, appID))
}

// shaderBucket is one resolved bucket: the pair that asked for it, the
// bucket id, the manifest to download and its parsed body.
type shaderBucket struct {
	pair   [2]string
	bucket uint64
	gid    uint64
	man    *depot.Manifest
}

func runShader(ctx context.Context, req request) error {
	if req.ShaderDir == "" {
		return fmt.Errorf("shader_dir is missing")
	}
	if err := os.MkdirAll(req.ShaderDir, 0o755); err != nil {
		return err
	}
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return err
	}
	defer cm.Close()
	ss := &session{cm: cm, dataDir: dataDir(), req: req}
	servers, err := cm.ServersForSteamPipe(cellID(), 32, 0)
	if err != nil {
		return callErr("server list", err)
	}
	ss.hosts = depotHosts(servers, req.AppID)
	if len(ss.hosts) == 0 {
		return fmt.Errorf("Steam returned no download hosts for this app")
	}
	key, err := ss.cm.DepotKey(req.AppID, req.AppID, 0)
	if err != nil {
		return callErrf("shader depot %d key", req.AppID, err)
	}
	// Phase 1: resolve every bucket manifest first, so phase 2 reports
	// one grand total instead of restarting the bar per bucket.
	state := loadShaderState(ss.dataDir, req.AppID)
	var buckets []shaderBucket
	for _, p := range shaderPairs() {
		if err := ctx.Err(); err != nil {
			return err
		}
		bucket, gid, _, skip, err := bucketManifest(cm, req.AppID, p)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		man, err := downloadShaderManifest(ctx, ss, req.AppID, gid, key)
		if err != nil {
			return err
		}
		buckets = append(buckets, shaderBucket{pair: p, bucket: bucket, gid: gid, man: man})
	}
	// Phase 2: download. Buckets whose stored gid matches and whose
	// on-disk sizes still match are skipped without hashing (the
	// post-download verify below still hashes everything it fetches).
	var total, base uint64
	var pending []shaderBucket
	for _, b := range buckets {
		if state[pairKey(b.pair)] == b.gid && shaderSizesMatch(b.man, req.ShaderDir) {
			continue
		}
		total += shaderFileBytes(b.man)
		pending = append(pending, b)
	}
	onProg, _, _ := progress(total)
	got := map[string]string{}
	var foz []string
	var n uint64
	for _, b := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		if m, err := downloadBucket(ctx, ss, b, withBase(onProg, base)); err != nil {
			return err
		} else {
			n += m
		}
		base += shaderFileBytes(b.man)
		got[pairKey(b.pair)] = fmt.Sprint(b.gid)
		state[pairKey(b.pair)] = b.gid
		// The state saves per bucket, right after its download: a
		// cancel between buckets keeps the finished ones, and the
		// next run refetches only the missing ones.
		if err := saveShaderState(ss.dataDir, req.AppID, state); err != nil {
			return err
		}
		foz = append(foz, shaderFozFiles(b.man)...)
	}
	emit(map[string]any{"type": "result", "bytes": n, "pairs": got, "foz": foz})
	return nil
}

// downloadShaderManifest fetches and decrypts one bucket manifest (depot
// == appid, like the client) without downloading any chunks.
func downloadShaderManifest(ctx context.Context, ss *session, appID uint32, gid uint64, key []byte) (*depot.Manifest, error) {
	code, err := ss.cm.ManifestRequestCode(appID, appID, gid, "public", "", 30*time.Second)
	if err != nil {
		return nil, callErr("shader manifest request code", err)
	}
	client := depot.NewClient()
	var raw []byte
	for _, h := range ss.hostsForDepot(ctx, DepotRef{DepotID: appID}) {
		raw, err = client.DownloadManifest(ctx, h, appID, gid, code)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	man, err := depot.ParseManifest(raw)
	if err != nil {
		return nil, depErrf(appID, err)
	}
	if man.Meta.FilenamesEncrypted {
		if err := man.DecryptFilenames(key); err != nil {
			return nil, depErrf(appID, err)
		}
	}
	return man, nil
}

// shaderFileBytes is the byte count the progress bar tracks: regular
// files only, matching what the writer lays down.
func shaderFileBytes(man *depot.Manifest) uint64 {
	var n uint64
	for _, f := range man.Files {
		if f.IsDir() || f.IsSymlink() {
			continue
		}
		n += f.Size
	}
	return n
}

// shaderSizesMatch skips the chunk download when every regular file is
// a fresh download verifies below, and updates carry a new gid.
func shaderSizesMatch(man *depot.Manifest, dir string) bool {
	for _, f := range man.Files {
		if f.IsDir() || f.IsSymlink() {
			continue
		}
		st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f.Name)))
		if err != nil || !st.Mode().IsRegular() || uint64(st.Size()) != f.Size {
			return false
		}
	}
	return true
}

// shaderFozFiles lists the Fossilize databases in a bucket manifest, as
// shader-dir-relative paths for the plugin's replay step. Driver-native
// trees (nvidiav1/…) carry none and need no replay.
func shaderFozFiles(man *depot.Manifest) []string {
	var out []string
	for _, f := range man.Files {
		if f.IsDir() || f.IsSymlink() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name), ".foz") {
			out = append(out, f.Name)
		}
	}
	return out
}

// downloadBucket streams one bucket manifest's files into the shader dir.
// Buckets merge into one client-mirror tree (per-GPU buckets already sit
// in their driver-native nvidiav1/ layout, which the driver reads in
// place — per-bucket folders would break that and buy nothing: the state
// above already tracks buckets separately). Staging resumes per bucket
// under shader-<app>-<bucket>, like the game installs' job staging.
func downloadBucket(ctx context.Context, ss *session, b shaderBucket, onProg func(depot.Progress)) (uint64, error) {
	appID := ss.req.AppID
	man := b.man
	key, err := ss.cm.DepotKey(appID, appID, 0)
	if err != nil {
		return 0, callErrf("shader depot %d key", appID, err)
	}
	hosts := ss.hostsForDepot(ctx, DepotRef{DepotID: appID})
	jobID := fmt.Sprintf("shader-%d-%d", appID, b.bucket)
	state := &depot.JobState{AppID: appID, DepotID: appID, ManifestID: b.gid, Done: map[string]bool{}, Total: len(man.UniqueChunks())}
	for id := range stagingIDs(ss.dataDir, jobID) {
		state.Done[id] = true
	}
	flusher := newJobFlusher(ss.dataDir, jobID, state, 2*time.Second)
	defer flusher.stop()
	w, err := depot.Preallocate(man, ss.req.ShaderDir)
	if err != nil {
		flusher.saveNow()
		return 0, err
	}
	want := chunkInfo(man)
	skip := map[string]bool{}
	if err := restoreStaging(ss.dataDir, jobID, want, w, skip, flusher); err != nil {
		w.Close()
		flusher.saveNow()
		return 0, err
	}
	sink := func(id string, data []byte) error {
		if err := w.Write(id, data); err != nil {
			return err
		}
		saveStagedChunk(ss.dataDir, jobID, id, data)
		flusher.mark(hexChunk([]byte(id)))
		return nil
	}
	client := depot.NewClient()
	sched := depot.NewScheduler(client, depot.SchedulerConfig{InitialConns: seededInitialConns(len(hosts), 4, len(man.UniqueChunks()))})
	dlErr := sched.DownloadStream(ctx, man, appID, hosts, key, skip, sink, onProg)
	cErr := w.Close()
	if dlErr != nil {
		flusher.saveNow()
		return 0, dlErr
	}
	if cErr != nil {
		flusher.saveNow()
		return 0, cErr
	}
	flusher.stop()
	depot.RemoveJob(ss.dataDir, jobID)
	clearStaging(ss.dataDir, jobID)
	if bad, err := depot.Verify(man, ss.req.ShaderDir); err != nil {
		return 0, err
	} else if len(bad) > 0 {
		return 0, fmt.Errorf("verify failed after shader download: %v", bad)
	}
	return shaderFileBytes(man), nil
}
