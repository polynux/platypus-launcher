// Shader pre-caching (issues #205, #211): the settings toggle and the
// helper's `shader` subcommand against the fake CM/CDN, plus the local
// fossilize_replay step. The fake answers Shader.GetBucketManifest from
// shader_buckets state (unknown pairs get NoMatch 42, like live) and serves
// the bucket manifest as depot == app id (see fakesteam.BuildShader).
package steam_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/plugins/steam-standalone/fakesteam"
)

// shaderEnv points the fake CM/CDN at a shader bucket for the test game
// (4321) and fakes a Steam client config carrying two extra pairs: one
// with no published bucket (skip) and one unknown (NoMatch skip). It also
// plants a no-op fossilize_replay in the fake client root, so replay tests
// never touch the real client binary; tests overwrite it per case.
func shaderEnv(t *testing.T, e *env) *fakesteam.Fixture {
	t.Helper()
	steamRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(steamRoot, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	vdf := `"ShaderCacheManager"
{
"CurrentBucketGPU"		"TESTGPU;BOGUSGPU"
"CurrentBucketDriver"		"L9:testdriver;G9:bogus"
}
`
	if err := os.WriteFile(filepath.Join(steamRoot, "config", "config.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	replayDir := filepath.Join(steamRoot, "ubuntu12_64")
	if err := os.MkdirAll(replayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeReplay := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(replayDir, "fossilize_replay"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeReplay("#!/bin/sh\nexit 0\n")
	t.Setenv("PLATYPUS_STEAM_ROOT", steamRoot)
	t.Setenv("PLATYPUS_PLUGIN_DATA", e.pluginData())
	shaderFix := fakesteam.BuildShader(4321, 424243)
	cdn := fakesteam.StartCDN(t, e.fix, shaderFix)
	e.cdn = cdn
	fakesteam.WriteCMState(t, e.cmState, cdn, e.fix)
	fakesteam.WriteCMShaderState(t, e.cmState, map[string]fakesteam.ShaderBucket{
		"SteamSwarm\tG7:VulkanPipelinesV6": {Bucket: 7, Manifest: shaderFix.GID, Size: shaderFix.DiskBytes},
		"TESTGPU\tL9:testdriver":           {Bucket: 9, Manifest: 0, Size: 0},
	})
	return shaderFix
}

// The toggle on: after an install the Fossilize bucket and the
// driver-native tree land in shadercache/<appid> next to the game, and
// unknown/empty pairs are skipped without failing the install.
func TestSteamShaderCacheDownloadsAfterInstall(t *testing.T) {
	e := setup(t)
	shaderFix := shaderEnv(t, e)
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	foz := filepath.Join(e.folder, "shadercache", "4321", "fozpipelinesv6", "steam_pipeline_cache.foz")
	body, err := os.ReadFile(foz)
	if err != nil {
		t.Fatalf("shader cache missing: %v", err)
	}
	if string(body) != "fake fossilize bucket" {
		t.Fatalf("shader cache content: %q", body)
	}
	bin := filepath.Join(e.folder, "shadercache", "4321", "nvidiav1", "GLCache",
		"fakedriverhash", "fakegpuhash", "steam_shader_cache.bin")
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("driver-native tree missing: %v", err)
	}
	log := e.cmLog()
	if !strings.Contains(log, "shader_bucket app=4321 gpu=SteamSwarm driver=G7:VulkanPipelinesV6") {
		t.Fatalf("swarm bucket not queried: %s", log)
	}
	if !strings.Contains(log, "gpu=BOGUSGPU") {
		t.Fatalf("config pairs not queried: %s", log)
	}
	raw, err := os.ReadFile(filepath.Join(e.pluginData(), "shader", "4321.json"))
	if err != nil || !strings.Contains(string(raw), "424243") {
		t.Fatalf("shader state not recorded: %s %v", raw, err)
	}
	if shaderFix.DiskBytes == 0 {
		t.Fatal("fixture has no bytes")
	}
}

// Two buckets merge into one client-mirror tree: the swarm bucket's .foz
// plus the per-GPU bucket's driver-native tree, under one grand progress
// total and with per-bucket state. The CDN keys manifests by depot+gid, so
// same-depot buckets don't shadow each other.
func TestSteamShaderTwoBucketsMerge(t *testing.T) {
	e := setup(t)
	steamRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(steamRoot, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	vdf := `"ShaderCacheManager"
{
"CurrentBucketGPU"		"TESTGPU"
"CurrentBucketDriver"		"L9:testdriver"
}
`
	if err := os.WriteFile(filepath.Join(steamRoot, "config", "config.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLATYPUS_STEAM_ROOT", steamRoot)
	t.Setenv("PLATYPUS_PLUGIN_DATA", e.pluginData())
	swarm := fakesteam.BuildShaderSwarm(4321, 111)
	gpu := fakesteam.BuildShaderGPU(4321, 222)
	cdn := fakesteam.StartCDN(t, e.fix, swarm, gpu)
	e.cdn = cdn
	fakesteam.WriteCMState(t, e.cmState, cdn, e.fix)
	fakesteam.WriteCMShaderState(t, e.cmState, map[string]fakesteam.ShaderBucket{
		"SteamSwarm\tG7:VulkanPipelinesV6": {Bucket: 7, Manifest: swarm.GID, Size: swarm.DiskBytes},
		"TESTGPU\tL9:testdriver":           {Bucket: 9, Manifest: gpu.GID, Size: gpu.DiskBytes},
	})
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	// Merged tree: one .foz plus the driver-native tree.
	if body, err := os.ReadFile(filepath.Join(e.folder, "shadercache", "4321",
		"fozpipelinesv6", "steam_pipeline_cache.foz")); err != nil || string(body) != "fake fossilize bucket" {
		t.Fatalf("swarm bucket missing: %v %q", err, body)
	}
	if body, err := os.ReadFile(filepath.Join(e.folder, "shadercache", "4321",
		"nvidiav1", "GLCache", "fakedriverhash", "fakegpuhash", "steam_shader_cache.bin")); err != nil || string(body) != "fake driver cache" {
		t.Fatalf("gpu bucket missing: %v %q", err, body)
	}
	// Per-bucket state: both pairs recorded under their own gids.
	raw, err := os.ReadFile(filepath.Join(e.pluginData(), "shader", "4321.json"))
	if err != nil || !strings.Contains(string(raw), "111") || !strings.Contains(string(raw), "222") {
		t.Fatalf("per-bucket state not recorded: %s %v", raw, err)
	}
	// One grand progress total across both buckets.
	want := swarm.DiskBytes + gpu.DiskBytes
	var total uint64
	for _, p := range e.notes.forMethod("job.progress") {
		var msg map[string]any
		if json.Unmarshal(p, &msg) != nil {
			continue
		}
		if msg["jobId"] != j.ID || msg["phase"] != "shaders" {
			continue
		}
		if n, _ := msg["total"].(float64); uint64(n) > total {
			total = uint64(n)
		}
	}
	if total != want {
		t.Fatalf("shader progress total = %d, want %d", total, want)
	}
}

// The toggle off (the default): installs fetch no shader buckets.
func TestSteamShaderCacheDefaultOff(t *testing.T) {
	e := setup(t)
	t.Setenv("PLATYPUS_PLUGIN_DATA", e.pluginData())
	e.login()
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(e.folder, "shadercache")); !os.IsNotExist(err) {
		t.Fatal("shader cache fetched without the toggle")
	}
	if log := e.cmLog(); strings.Contains(log, "shader_bucket") {
		t.Fatalf("shader bucket queried without the toggle: %s", log)
	}
}

// The helper skips buckets it already downloaded: the second run refetches
// the (cheap) manifests but downloads no chunks.
func TestSteamShaderCacheSkipsCurrentBuckets(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	e.login()
	tok, err := os.ReadFile(filepath.Join(e.pluginData(), "refresh.token"))
	if err != nil {
		t.Fatalf("no refresh token: %v", err)
	}
	shaderDir := filepath.Join(e.folder, "shadercache", "4321")
	run := func() map[string]any {
		t.Helper()
		req, _ := json.Marshal(map[string]any{
			"refresh_token": strings.TrimSpace(string(tok)),
			"app_id":        4321,
			"shader_dir":    shaderDir,
			"job_id":        "test-shader",
		})
		cmd := exec.Command(prebuiltDL, "shader")
		cmd.Stdin = bytes.NewReader(req)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("shader run: %v\n%s", err, stderr.String())
		}
		var result map[string]any
		for _, line := range strings.Split(stdout.String(), "\n") {
			var msg map[string]any
			if json.Unmarshal([]byte(line), &msg) == nil && msg["type"] == "result" {
				result = msg
			}
		}
		if result == nil {
			t.Fatalf("no result line:\n%s\n%s", stdout.String(), stderr.String())
		}
		return result
	}
	first := run()
	if n, _ := first["bytes"].(float64); n <= 0 {
		t.Fatalf("first run downloaded nothing: %v", first)
	}
	if foz, _ := first["foz"].([]any); len(foz) == 0 {
		t.Fatalf("first run reports no .foz for replay: %v", first)
	}
	foz := filepath.Join(shaderDir, "fozpipelinesv6", "steam_pipeline_cache.foz")
	before, err := os.Stat(foz)
	if err != nil {
		t.Fatalf("shader cache missing: %v", err)
	}
	second := run()
	if n, _ := second["bytes"].(float64); n != 0 {
		t.Fatalf("second run re-downloaded: %v", second)
	}
	after, err := os.Stat(foz)
	if err != nil {
		t.Fatalf("shader cache missing: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("second run rewrote the cache instead of skipping it")
	}
}

// A cancel during the shader phase keeps the finished install: the record
// lands before the shaders run, so discarding the canceled job must not
// delete the game, and the phase still ends with job.done carrying the
// install (so the core refreshes the library cache). The fake replay
// writes a marker first, so the cancel reliably lands inside the replay
// rather than the download.
func TestSteamShaderCancelDuringReplayKeepsInstall(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	t.Setenv("PLATYPUS_SHADER_REPLAY_TIMEOUT", "30")
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	steamRoot := os.Getenv("PLATYPUS_STEAM_ROOT")
	marker := filepath.Join(t.TempDir(), "replaying")
	script := "#!/bin/sh\necho started > \"" + marker + "\"\nsleep 60\n"
	if err := os.WriteFile(filepath.Join(steamRoot, "ubuntu12_64", "fossilize_replay"),
		[]byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("replay never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := e.call("job.cancel", map[string]any{"jobId": j.ID}, nil); err != nil {
		t.Fatalf("job.cancel: %v", err)
	}
	// The cancel still ends with job.done carrying the finished install.
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("canceled shader phase: %+v, want done", got)
	}
	if err := e.call("job.discard", map[string]any{
		"jobId": j.ID, "id": "4321", "kind": "install", "installDir": e.folder}, nil); err != nil {
		t.Fatalf("job.discard: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
	if err != nil || !strings.Contains(string(raw), "4321") {
		t.Fatalf("install record lost on shader-phase cancel: %s %v", raw, err)
	}
	// Non-vacuous: the recorded game executable itself survived, not just
	// a non-empty folder.
	var installed map[string]map[string]any
	if err := json.Unmarshal(raw, &installed); err != nil {
		t.Fatal(err)
	}
	exe, _ := installed["4321"]["executable"].(string)
	if exe == "" {
		t.Fatalf("no recorded executable: %s", raw)
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("recorded executable gone on shader-phase cancel: %v", err)
	}
}

// The replay runs the client's fossilize_replay with the launch's cache
// environment and never fails the recorded install.
func TestSteamShaderReplaySuccess(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	log := filepath.Join(t.TempDir(), "replay.log")
	t.Setenv("FOSSILIZE_LOG", log)
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	steamRoot := os.Getenv("PLATYPUS_STEAM_ROOT")
	script := "#!/bin/sh\necho \"ARGS: $@\" >> \"$FOSSILIZE_LOG\"\n" +
		"echo \"MESA=$MESA_SHADER_CACHE_DIR COMPAT=$STEAM_COMPAT_SHADER_PATH\" >> \"$FOSSILIZE_LOG\"\n"
	if err := os.WriteFile(filepath.Join(steamRoot, "ubuntu12_64", "fossilize_replay"),
		[]byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	raw, err := os.ReadFile(log)
	if err != nil || !strings.Contains(string(raw), "steam_pipeline_cache.foz") {
		t.Fatalf("replay never ran on the .foz: %s %v", raw, err)
	}
	want := filepath.Join(e.folder, "shadercache", "4321")
	if !strings.Contains(string(raw), "MESA="+want) || !strings.Contains(string(raw), "COMPAT="+want) {
		t.Fatalf("replay env wrong: %s", raw)
	}
}

// A failing replay is a warning, not a failed install.
func TestSteamShaderReplayFailure(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	steamRoot := os.Getenv("PLATYPUS_STEAM_ROOT")
	if err := os.WriteFile(filepath.Join(steamRoot, "ubuntu12_64", "fossilize_replay"),
		[]byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	// `log` notifications land in the plugin's log file, not the notify
	// stream (see notifyToLog).
	raw, err := os.ReadFile(filepath.Join(e.root, "logs", "plugin-"+id+".log"))
	if err != nil || !strings.Contains(string(raw), "shader replay failed") {
		t.Fatalf("no replay-failure warning logged: %s %v", raw, err)
	}
}

// A replay that never finishes is killed at the bound and the recorded
// install still completes.
func TestSteamShaderReplayTimeout(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	t.Setenv("PLATYPUS_SHADER_REPLAY_TIMEOUT", "2")
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	steamRoot := os.Getenv("PLATYPUS_STEAM_ROOT")
	if err := os.WriteFile(filepath.Join(steamRoot, "ubuntu12_64", "fossilize_replay"),
		[]byte("#!/bin/sh\nsleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	raw, err := os.ReadFile(filepath.Join(e.root, "logs", "plugin-"+id+".log"))
	if err != nil || !strings.Contains(string(raw), "timed out") {
		t.Fatalf("no replay-timeout warning logged: %s %v", raw, err)
	}
}

// Uninstall removes the game folder, the shader tree and the helper's
// per-bucket state.
func TestSteamShaderUninstallCleansCache(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	shaderDir := filepath.Join(e.folder, "shadercache", "4321")
	if _, err := os.Stat(shaderDir); err != nil {
		t.Fatalf("no shader cache to clean: %v", err)
	}
	u, err := e.q.Add(jobs.KindUninstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.waitJob(u.ID); got.State != jobs.StateDone {
		t.Fatalf("uninstall: %+v", got)
	}
	if _, err := os.Stat(shaderDir); !os.IsNotExist(err) {
		t.Fatal("shader cache survived uninstall")
	}
	if _, err := os.Stat(filepath.Join(e.pluginData(), "shader", "4321.json")); !os.IsNotExist(err) {
		t.Fatal("shader state survived uninstall")
	}
}

// A canceled replay kills the whole process group: the fake replay spawns
// a child, and the child is gone after the cancel (killing just the parent
// would orphan it).
func TestSteamShaderReplayKillsProcessGroup(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	t.Setenv("PLATYPUS_SHADER_REPLAY_TIMEOUT", "60")
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	steamRoot := os.Getenv("PLATYPUS_STEAM_ROOT")
	dir := t.TempDir()
	marker, pidFile := filepath.Join(dir, "replaying"), filepath.Join(dir, "child.pid")
	script := "#!/bin/sh\necho started > \"" + marker + "\"\nsleep 60 &\necho $! > \"" + pidFile + "\"\nwait\n"
	if err := os.WriteFile(filepath.Join(steamRoot, "ubuntu12_64", "fossilize_replay"),
		[]byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("replay never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := e.call("job.cancel", map[string]any{"jobId": j.ID}, nil); err != nil {
		t.Fatalf("job.cancel: %v", err)
	}
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("canceled shader phase: %+v, want done", got)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("no child pid from the replay: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		t.Fatalf("bad child pid %q: %v", raw, err)
	}
	deadline = time.Now().Add(10 * time.Second)
	for !childGone(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("replay child %d survived the cancel", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// childGone reports whether pid runs no more: no /proc entry, or a zombie
// awaiting reaping. Either way it computes nothing.
func childGone(pid int) bool {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	s := string(raw)
	i := strings.LastIndexByte(s, ')')
	return i >= 0 && i+2 < len(s) && s[i+2] == 'Z'
}

// Uninstall drops the helper's leftover shader staging (jobs/shader-<app>-*)
// alongside the tree and the per-bucket state.
func TestSteamShaderUninstallCleansStaging(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	// Leftover staging from a canceled run (a clean run clears its own).
	jobsDir := filepath.Join(e.pluginData(), "jobs")
	if err := os.MkdirAll(filepath.Join(jobsDir, "shader-4321-7.chunks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobsDir, "shader-4321-7.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobsDir, "other-job.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	u, err := e.q.Add(jobs.KindUninstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.waitJob(u.ID); got.State != jobs.StateDone {
		t.Fatalf("uninstall: %+v", got)
	}
	for _, n := range []string{"shader-4321-7.json", "shader-4321-7.chunks"} {
		if _, err := os.Stat(filepath.Join(jobsDir, n)); !os.IsNotExist(err) {
			t.Fatalf("shader staging survived uninstall: %s", n)
		}
	}
	if _, err := os.Stat(filepath.Join(jobsDir, "other-job.json")); err != nil {
		t.Fatalf("unrelated job file deleted: %v", err)
	}
}

// Discarding a canceled job drops the helper's shader staging too: it is
// named by app (jobs/shader-<app>-*), not by the plugin job id whose own
// files the discard already removes.
func TestSteamShaderDiscardCleansStaging(t *testing.T) {
	e := setup(t)
	jobsDir := filepath.Join(e.pluginData(), "jobs")
	if err := os.MkdirAll(filepath.Join(jobsDir, "shader-4321-7.chunks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobsDir, "shader-4321-7.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobsDir, "other-job.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.call("job.discard", map[string]any{
		"jobId": "test-shader-discard", "id": "4321", "kind": "install", "installDir": e.folder}, nil); err != nil {
		t.Fatalf("job.discard: %v", err)
	}
	for _, n := range []string{"shader-4321-7.json", "shader-4321-7.chunks"} {
		if _, err := os.Stat(filepath.Join(jobsDir, n)); !os.IsNotExist(err) {
			t.Fatalf("shader staging survived discard: %s", n)
		}
	}
	if _, err := os.Stat(filepath.Join(jobsDir, "other-job.json")); err != nil {
		t.Fatalf("unrelated job file deleted: %v", err)
	}
}

// Relocating the game moves the sibling shadercache tree with it.
func TestSteamShaderRelocationMovesCache(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	raw, err := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var installed map[string]map[string]any
	if err := json.Unmarshal(raw, &installed); err != nil {
		t.Fatal(err)
	}
	old, _ := installed["4321"]["path"].(string)
	if old == "" {
		t.Fatalf("no recorded path: %s", raw)
	}
	dst := filepath.Join(e.gamesFolder, "relocated", filepath.Base(old))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(old, dst); err != nil {
		t.Fatal(err)
	}
	var ok map[string]any
	if err := e.call("game.relocated", map[string]any{"id": "4321", "from": old, "to": dst}, &ok); err != nil {
		t.Fatalf("game.relocated: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(dst), "shadercache", "4321",
		"fozpipelinesv6", "steam_pipeline_cache.foz"))
	if err != nil || string(body) != "fake fossilize bucket" {
		t.Fatalf("shader cache did not move: %v %q", err, body)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(old), "shadercache", "4321")); !os.IsNotExist(err) {
		t.Fatal("old shader cache survived relocation")
	}
}

// Client-mode games skip the shader download: the official client manages
// its own cache.
func TestSteamShaderSkipsClientModeGames(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	e.login()
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	e.setLaunchMode("4321", "client")
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	mark := len(e.cmLog())
	up, err := e.q.Add(jobs.KindUpdate, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.waitJob(up.ID); got.State != jobs.StateDone {
		t.Fatalf("update: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(e.folder, "shadercache")); !os.IsNotExist(err) {
		t.Fatal("shader cache fetched for a client-mode game")
	}
	if strings.Contains(e.cmLog()[mark:], "shader_bucket") {
		t.Fatal("shader buckets queried for a client-mode game")
	}
}

// Without a replay binary the install still completes, and the job carries
// a note saying the caches stayed download-only.
func TestSteamShaderNoReplayBinaryNote(t *testing.T) {
	e := setup(t)
	shaderEnv(t, e)
	// The real Steam client on this machine ships fossilize_replay: hide
	// the home-dir candidates so the removed fake means "no binary".
	t.Setenv("HOME", t.TempDir())
	e.login()
	if err := e.call("settings.apply", map[string]any{"values": map[string]any{"region": "auto", "shader": true}}, nil); err != nil {
		t.Fatalf("settings.apply: %v", err)
	}
	steamRoot := os.Getenv("PLATYPUS_STEAM_ROOT")
	if err := os.Remove(filepath.Join(steamRoot, "ubuntu12_64", "fossilize_replay")); err != nil {
		t.Fatal(err)
	}
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	found := false
	for _, p := range e.notes.forMethod("job.done") {
		var msg map[string]any
		if json.Unmarshal(p, &msg) != nil || msg["jobId"] != j.ID {
			continue
		}
		if note, _ := msg["note"].(string); strings.Contains(note, "fossilize_replay") {
			found = true
		}
	}
	if !found {
		t.Fatal("job.done carries no replay-skipped note")
	}
}
