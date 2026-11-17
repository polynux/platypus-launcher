// Shader pre-caching download (issue #205): per-GPU/driver Fossilize
// buckets from the Shader service, committed under the request's shader
// dir (the steamapps/shadercache/<appid> mirror next to the game install).
// Content-system only: each bucket manifest resolves to depot == appid,
// like the client fetches it — no PICS, no branch, no DLC filtering.
// The helper downloads the .foz files and runs nothing: the Steam client
// replays them into driver caches on its next run (see
// docs/research/steam-shader-cache.md).
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
			if k := g + "\x00" + d; !seen[k] {
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
	pairs := shaderPairs()
	servers, err := cm.ServersForSteamPipe(cellID(), 32, 0)
	if err != nil {
		return callErr("server list", err)
	}
	hosts := depotHosts(servers, req.AppID)
	if len(hosts) == 0 {
		return fmt.Errorf("Steam returned no download hosts for this app")
	}
	ss.hosts = hosts
	state := loadShaderState(ss.dataDir, req.AppID)
	got := map[string]string{}
	var total, base uint64
	for _, p := range pairs {
		bucket, gid, size, skip, err := bucketManifest(cm, req.AppID, p)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		n, err := downloadBucket(ctx, ss, p, bucket, gid, size, base, state[pairKey(p)] == gid)
		if err != nil {
			return err
		}
		got[pairKey(p)] = fmt.Sprint(gid)
		state[pairKey(p)] = gid
		total += n
		base += size
	}
	if err := saveShaderState(ss.dataDir, req.AppID, state); err != nil {
		return err
	}
	emit(map[string]any{"type": "result", "bytes": total, "pairs": got})
	return nil
}

// downloadBucket fetches one bucket manifest (depot == appid, like the
// client) and streams its files into the shader dir. current skips the
// chunk download when the manifest's files already verify clean on disk.
func downloadBucket(ctx context.Context, ss *session, pair [2]string, bucket, gid, size, base uint64, current bool) (uint64, error) {
	appID := ss.req.AppID
	code, err := ss.cm.ManifestRequestCode(appID, appID, gid, "public", "", 30*time.Second)
	if err != nil {
		return 0, callErr("shader manifest request code", err)
	}
	key, err := ss.cm.DepotKey(appID, appID, 0)
	if err != nil {
		return 0, callErrf("shader depot %d key", appID, err)
	}
	client := depot.NewClient()
	var raw []byte
	for _, h := range withShaderTokens(ctx, ss, appID) {
		raw, err = client.DownloadManifest(ctx, h, appID, gid, code)
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0, err
	}
	man, err := depot.ParseManifest(raw)
	if err != nil {
		return 0, depErrf(appID, err)
	}
	if man.Meta.FilenamesEncrypted {
		if err := man.DecryptFilenames(key); err != nil {
			return 0, depErrf(appID, err)
		}
	}
	if current {
		if bad, err := depot.Verify(man, ss.req.ShaderDir); err == nil && len(bad) == 0 {
			return 0, nil
		}
	}
	jobID := fmt.Sprintf("shader-%d-%d", appID, bucket)
	total := len(man.UniqueChunks())
	onProg, _, _ := progress(man.TotalSize())
	onProg = withBase(onProg, base)
	state := &depot.JobState{AppID: appID, DepotID: appID, ManifestID: gid, Done: map[string]bool{}, Total: total}
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
	sched := depot.NewScheduler(client, depot.SchedulerConfig{InitialConns: seededInitialConns(len(ss.hosts), 4, len(man.UniqueChunks()))})
	dlErr := sched.DownloadStream(ctx, man, appID, ss.hosts, key, skip, sink, onProg)
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
	return man.TotalSize(), nil
}

// withShaderTokens copies the session hosts, attaching that depot's CDN
// auth tokens (tokens are per depot and host), like withTokens but for
// the single shader depot.
func withShaderTokens(ctx context.Context, ss *session, appID uint32) []depot.Host {
	out := make([]depot.Host, len(ss.hosts))
	copy(out, ss.hosts)
	for i, h := range out {
		tok, _, err := ss.cm.CDNAuthToken(appID, h.Name, appID, 0)
		if err == nil && tok != "" {
			out[i].Token = tok
		}
	}
	return out
}
