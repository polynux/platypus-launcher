// Shader pre-caching (issue #205): the settings toggle and the helper's
// `shader` subcommand against the fake CM/CDN. The fake answers
// Shader.GetBucketManifest from shader_buckets state (unknown pairs get
// NoMatch 42, like live) and serves the bucket manifest as depot == app
// id (see fakesteam.BuildShader).
package steam_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/jobs"
	"platypus/plugins/steam-standalone/fakesteam"
)

// shaderEnv points the fake CM/CDN at a shader bucket for the test game
// (4321) and fakes a Steam client config carrying two extra pairs: one
// with no published bucket (skip) and one unknown (NoMatch skip).
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

// The toggle on: after an install the Fossilize bucket lands in
// shadercache/<appid> next to the game, and unknown/empty pairs are
// skipped without failing the install.
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
