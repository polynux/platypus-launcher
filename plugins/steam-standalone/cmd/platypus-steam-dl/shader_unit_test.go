// Shader helper unit tests (issue #211): the skip/state/pair helpers, the
// per-bucket dedupe, the disk-space guard and the speed-cap wiring. The
// download itself is covered by the plugin's fake-CM tests.
package main

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/depot"
)

func TestShaderPairKeyDistinct(t *testing.T) {
	a := pairKey([2]string{"SteamSwarm", "G7:VulkanPipelinesV6"})
	b := pairKey([2]string{"SteamSwarm", "G7:bogus"})
	if a == b {
		t.Fatal("pairKey collides across drivers")
	}
}

func TestShaderPairsFromClientConfig(t *testing.T) {
	root := t.TempDir()
	vdf := `"ShaderCacheManager"
{
"CurrentBucketGPU"		"GPU1;GPU2;GPU1"
"CurrentBucketDriver"		"D1;D2;D1"
}
`
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "config.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLATYPUS_STEAM_ROOT", root)
	pairs := shaderPairs()
	if len(pairs) == 0 || pairs[0] != swarmBucket {
		t.Fatalf("swarm bucket is not first: %v", pairs)
	}
	// GPU1/D1 repeats: parsed once, counted once.
	seen := map[string]int{}
	for _, p := range pairs {
		seen[pairKey(p)]++
	}
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("pair %q appears %d times in %v", k, n, pairs)
		}
	}
	if len(pairs) != 3 {
		t.Fatalf("pairs = %v, want swarm + 2 config pairs", pairs)
	}
}

func TestShaderStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if got := loadShaderState(dir, 4321); len(got) != 0 {
		t.Fatalf("empty state = %v", got)
	}
	want := map[string]uint64{"a\x00b": 424243}
	if err := saveShaderState(dir, 4321, want); err != nil {
		t.Fatal(err)
	}
	got := loadShaderState(dir, 4321)
	if got["a\x00b"] != 424243 {
		t.Fatalf("state round trip = %v", got)
	}
}

func TestShaderSizesMatch(t *testing.T) {
	dir := t.TempDir()
	man := &depot.Manifest{Files: []depot.File{
		{Name: "fozpipelinesv6/steam_pipeline_cache.foz", Size: 12},
	}}
	if shaderSizesMatch(man, dir) {
		t.Fatal("sizes match with nothing on disk")
	}
	if err := os.MkdirAll(filepath.Join(dir, "fozpipelinesv6"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fozpipelinesv6", "steam_pipeline_cache.foz"), make([]byte, 11), 0o644); err != nil {
		t.Fatal(err)
	}
	if shaderSizesMatch(man, dir) {
		t.Fatal("sizes match with a short file")
	}
	if err := os.WriteFile(filepath.Join(dir, "fozpipelinesv6", "steam_pipeline_cache.foz"), make([]byte, 12), 0o644); err != nil {
		t.Fatal(err)
	}
	if !shaderSizesMatch(man, dir) {
		t.Fatal("sizes do not match with the full file")
	}
}

func TestShaderFozFiles(t *testing.T) {
	man := &depot.Manifest{Files: []depot.File{
		{Name: "fozpipelinesv6/steam_pipeline_cache.foz"},
		{Name: "nvidiav1/GLCache/h/g/steam_shader_cache.bin"},
		{Name: "somedir", Flags: depot.FlagDirectory},
	}}
	foz := shaderFozFiles(man)
	if len(foz) != 1 || foz[0] != "fozpipelinesv6/steam_pipeline_cache.foz" {
		t.Fatalf("foz = %v", foz)
	}
}

func TestDedupeShaderBuckets(t *testing.T) {
	in := []shaderResolved{
		{pair: [2]string{"SteamSwarm", "G7:VulkanPipelinesV6"}, bucket: 7, gid: 1, size: 100},
		{pair: [2]string{"GPU", "D"}, bucket: 7, gid: 1, size: 100},
		{pair: [2]string{"GPU2", "D2"}, bucket: 9, gid: 2, size: 50},
	}
	out := dedupeShaderBuckets(in)
	if len(out) != 2 || out[0].bucket != 7 || out[1].bucket != 9 {
		t.Fatalf("dedupe = %+v", out)
	}
	var need uint64
	for _, r := range out {
		need += r.size
	}
	if need != 150 {
		t.Fatalf("deduped total = %d, want 150", need)
	}
}

func TestShaderSpaceOK(t *testing.T) {
	dir := t.TempDir()
	stage := t.TempDir()
	if ok, _ := shaderSpaceOK(dir, stage, 0); !ok {
		t.Fatal("zero need must always fit")
	}
	if ok, _ := shaderSpaceOK(dir, stage, 1); !ok {
		t.Fatal("one byte must fit in a temp dir")
	}
	if ok, why := shaderSpaceOK(dir, stage, 1<<62); !ok && why == "" {
		t.Fatal("skip without a warning")
	} else if ok {
		t.Skip("disk holds 8 EB free; cannot exercise the skip")
	} else if got := why; len(got) < 10 || !contains(got, "install is unaffected") {
		t.Fatalf("warning = %q", got)
	}
	if ok, _ := shaderSpaceOK(filepath.Join(dir, "no-such-dir"), stage, 1<<62); ok {
		t.Fatal("missing target must not pass")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestShaderLimiterWiring(t *testing.T) {
	// No cap (the default): nil limiter, like the install path.
	if lim := shaderLimiter(request{}); lim != nil {
		t.Fatalf("unlimited run has a limiter: %+v", lim)
	}
	// Capped: the request's limit reaches the scheduler the bucket
	// download builds with.
	req := request{DownloadLimit: 1024}
	lim := shaderLimiter(req)
	if lim == nil {
		t.Fatal("capped run has no limiter")
	}
	if sched := newScheduler(depot.NewClient(), lim, 1, 1); sched == nil {
		t.Fatal("no scheduler for the shader bucket")
	}
}
