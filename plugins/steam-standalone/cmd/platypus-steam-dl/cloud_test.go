// Cloud saves, read + pull side: mapper, compare and transfer unit tests.
// No Valve network: listings are synthetic, downloads ride httptest (loopback
// only, like the repo's fast set demands).
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"platypus/internal/steam"
)

func mustParseKV(t *testing.T, src string) *steam.KV {
	t.Helper()
	kv, err := steam.ParseKV(src)
	if err != nil {
		t.Fatal(err)
	}
	return kv
}

// The SUPERHOT ufs block, as recorded in docs/research/steam-cloud-social.md
// §1.4: two savefiles rules and the Mac/Linux root overrides.
const superhotUFS = `"ufs"
{
	"quota"		"524288"
	"maxnumfiles"		"5"
	"savefiles"
	{
		"0" { "root" "WinAppDataLocalLow" "path" "SUPERHOT_Team/SUPERHOT" "pattern" "*.hot" }
		"1" { "root" "WinAppDataLocalLow" "path" "SUPERHOT_Team/SUPERHOT" "pattern" "*.cfg" }
	}
	"rootoverrides"
	{
		"0" { "os" "MacOS" "oscompare" "=" "root" "WinAppDataLocalLow" "useinstead" "MacAppSupport"
			"pathtransforms" { "0" { "find" "SUPERHOT_Team/SUPERHOT" "replace" "unity.SUPERHOT_Team.SUPERHOT" } } }
		"1" { "os" "Linux" "oscompare" "=" "root" "WinAppDataLocalLow" "useinstead" "LinuxHome"
			"pathtransforms" { "0" { "find" "SUPERHOT_Team/SUPERHOT" "replace" ".config/unity3d/SUPERHOT_Team/SUPERHOT" } } }
	}
}`

func TestParseCloudRulesSUPERHOT(t *testing.T) {
	ufs := parseCloudRules(mustParseKV(t, superhotUFS))
	if !ufs.HasUFS || !ufs.HasRules {
		t.Fatalf("SUPERHOT ufs must parse as rules: %+v", ufs)
	}
	if len(ufs.Rules) != 2 || len(ufs.Overrides) != 2 {
		t.Fatalf("want 2 rules and 2 overrides, got %+v", ufs)
	}
	if ufs.Rules[0].Root != "WinAppDataLocalLow" || ufs.Rules[0].Pattern != "*.hot" {
		t.Fatalf("first rule: %+v", ufs.Rules[0])
	}
	ov := ufs.Overrides[1]
	if ov.OS != "Linux" || ov.UseInstead != "LinuxHome" || len(ov.Find) != 1 || ov.Replace[0] != ".config/unity3d/SUPERHOT_Team/SUPERHOT" {
		t.Fatalf("linux override: %+v", ov)
	}
}

func TestParseCloudRulesAPIOnly(t *testing.T) {
	// Terraria (105600): quota only, no savefiles — the Steamworks API
	// storage the plugin can't sync from outside the game.
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "quota" "100000" "maxnumfiles" "10" }`))
	if !ufs.HasUFS || ufs.HasRules {
		t.Fatalf("quota-only ufs is API storage, not rules: %+v", ufs)
	}
	if parseCloudRules(mustParseKV(t, `"depots" {}`)).HasUFS {
		t.Fatal("no ufs section must parse as absent")
	}
}

// appinfo spells the platforms block both ways: indexed (`"1" "Windows"`,
// measured on Fallout 4 and Taiji) and name-keyed (`"Windows" "1"`). The OS
// names are what matter; an empty block means all platforms.
func TestParseCloudRulesPlatforms(t *testing.T) {
	for _, tc := range []struct {
		name  string
		block string
		want  map[string]bool
	}{
		{"indexed", `"platforms" { "1" "Windows" }`, map[string]bool{"windows": true}},
		{"name-keyed", `"platforms" { "Windows" "1" }`, map[string]bool{"windows": true}},
		{"mixed", `"platforms" { "0" "Linux" "Windows" "1" }`, map[string]bool{"linux": true, "windows": true}},
		{"all value", `"platforms" { "0" "all" }`, nil},
		{"all key", `"platforms" { "All" "0" }`, nil},
		{"empty", `"platforms" { }`, nil},
	} {
		ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
			"0" { "root" "WinMyDocuments" "path" "/My Games/Fallout4/Saves" "pattern" "*.fos"
			      `+tc.block+` } } }`))
		if len(ufs.Rules) != 1 {
			t.Fatalf("%s: rules = %+v", tc.name, ufs.Rules)
		}
		p := ufs.Rules[0].Platforms
		if len(p) != len(tc.want) {
			t.Fatalf("%s: platforms = %+v, want %+v", tc.name, p, tc.want)
		}
		for osName := range tc.want {
			if !p[osName] {
				t.Fatalf("%s: platforms = %+v, want %+v", tc.name, p, tc.want)
			}
		}
	}
}

func superhotCtx(home string) cloudContext {
	return cloudContext{OS: "linux", Home: home, InstallDir: filepath.Join(home, "install")}
}

func TestMapCloudNameNative(t *testing.T) {
	ufs := parseCloudRules(mustParseKV(t, superhotUFS))
	got, rel, ok := mapCloudName("%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/super.hot", ufs, superhotCtx("/home/u"), 0)
	if !ok {
		t.Fatal("SUPERHOT cloud name must map on native linux")
	}
	if want := "/home/u/.config/unity3d/SUPERHOT_Team/SUPERHOT/super.hot"; got != want {
		t.Fatalf("local = %q, want %q", got, want)
	}
	if rel != "SUPERHOT_Team/SUPERHOT/super.hot" {
		t.Fatalf("rel = %q", rel)
	}
}

func TestMapCloudNameProton(t *testing.T) {
	ufs := parseCloudRules(mustParseKV(t, superhotUFS))
	ctx := cloudContext{OS: "windows", Home: "/home/u", InstallDir: "/games/x",
		PrefixDir: "/prefixes/game", WinUser: "steamuser"}
	got, _, ok := mapCloudName("%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/super.hot", ufs, ctx, 0)
	if !ok {
		t.Fatal("SUPERHOT cloud name must map under Proton")
	}
	if want := "/prefixes/game/drive_c/users/steamuser/AppData/LocalLow/SUPERHOT_Team/SUPERHOT/super.hot"; got != want {
		t.Fatalf("local = %q, want %q", got, want)
	}
}

func TestMapCloudNameTraversalRefused(t *testing.T) {
	ufs := parseCloudRules(mustParseKV(t, superhotUFS))
	ctx := superhotCtx("/home/u")
	for _, name := range []string{
		"%WinAppDataLocalLow%../../evil",
		"%WinAppDataLocalLow%SUPERHOT_Team/../evil",
		"%WinAppDataLocalLow%/absolute",
		"%WinAppDataLocalLow%C:/evil",
		"%WinAppDataLocalLow%" + strings.Repeat("a/", 33) + "x",
		"no-root-at-all",
		"%Unclosed",
	} {
		if _, _, ok := mapCloudName(name, ufs, ctx, 0); ok {
			t.Fatalf("%q must be refused", name)
		}
	}
}

func TestMapCloudNameUnmappedRoots(t *testing.T) {
	// Hades: WinMyDocuments with only a MacOS override — native Linux has no
	// mapping, so the cloud file counts as unmapped instead of landing in a
	// guessed folder.
	ufs := parseCloudRules(mustParseKV(t, `"ufs" {
		"savefiles" { "0" { "root" "WinMyDocuments" "path" "Saved Games/Hades" "pattern" "Profile*.sav" } }
		"rootoverrides" { "0" { "os" "MacOS" "root" "WinMyDocuments" "useinstead" "MacAppSupport" } }
	}`))
	if _, _, ok := mapCloudName("%WinMyDocuments%Saved Games/Hades/Profile0.sav", ufs, superhotCtx("/home/u"), 0); ok {
		t.Fatal("WinMyDocuments without a Linux override must not map on native linux")
	}
}

func TestMapCloudNameSteamIDSubstitution(t *testing.T) {
	ufs := parseCloudRules(mustParseKV(t, `"ufs" {
		"savefiles" { "0" { "root" "GameInstall" "path" "saves/{64BitSteamID}" "pattern" "*.sav" } }
	}`))
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: "/games/x"}
	got, rel, ok := mapCloudName("%GameInstall%saves/{64BitSteamID}/a.sav", ufs, ctx, 76561198000000000)
	if !ok {
		t.Fatal("steamid paths must map")
	}
	if want := "/games/x/saves/76561198000000000/a.sav"; got != want {
		t.Fatalf("local = %q, want %q", got, want)
	}
	if !strings.Contains(rel, "76561198000000000") {
		t.Fatalf("rel = %q", rel)
	}
}

func gameInstallUFS() string {
	return `"ufs" { "savefiles" { "0" { "root" "GameInstall" "path" "saves" "pattern" "*.sav" } } }`
}

// rulePathParts normalises the path spellings appinfo really uses.
func TestRulePathParts(t *testing.T) {
	for _, tc := range []struct {
		rel  string
		want []string
	}{
		{"/My Games/Fallout4/Saves", []string{"My Games", "Fallout4", "Saves"}},
		{"./my games/borderlands/savedata", []string{"my games", "borderlands", "savedata"}},
		{"Steam Save/", []string{"Steam Save"}},
		{"./", nil},
		{"", nil},
		{".", nil},
		{"saves", []string{"saves"}},
	} {
		got := rulePathParts(tc.rel)
		if len(got) != len(tc.want) {
			t.Errorf("rulePathParts(%q) = %v, want %v", tc.rel, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("rulePathParts(%q) = %v, want %v", tc.rel, got, tc.want)
				break
			}
		}
	}
}

// Coverage is the pull filter's rule: same root, under the rule path,
// matching the pattern. Platforms don't matter (one cloud name serves every
// OS), and rootoverrides don't either: they only say where a file lands.
func TestCloudNameCovered(t *testing.T) {
	ufs := parseCloudRules(mustParseKV(t, superhotUFS))
	native := superhotCtx("/home/u")
	proton := cloudContext{OS: "windows", Home: "/home/u", InstallDir: "/games/x",
		PrefixDir: "/prefixes/game", WinUser: "steamuser"}
	for _, tc := range []struct {
		name    string
		covered bool
	}{
		{"%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/super.hot", true},
		{"%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/settings.cfg", true},
		// Right root, outside the rule path: the official client's own
		// files or another game's folder.
		{"%WinAppDataLocalLow%SUPERHOT_Team/Other/super.hot", false},
		{"%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/super.txt", false},
		{"%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOTs/super.hot", false},
		// Another root, even if a local path could be guessed.
		{"%GameInstall%SUPERHOT_Team/SUPERHOT/super.hot", false},
		{"no-root-at-all", false},
		{"%WinAppDataLocalLow%", false},
	} {
		if got := cloudNameCovered(tc.name, ufs, native, 0); got != tc.covered {
			t.Errorf("native %q covered = %v, want %v", tc.name, got, tc.covered)
		}
		if got := cloudNameCovered(tc.name, ufs, proton, 0); got != tc.covered {
			t.Errorf("proton %q covered = %v, want %v", tc.name, got, tc.covered)
		}
	}
	// Hades: WinMyDocuments with only a MacOS override. The rule owns the
	// name on every OS; placement is mapCloudName's job and fails natively.
	hades := parseCloudRules(mustParseKV(t, `"ufs" {
		"savefiles" { "0" { "root" "WinMyDocuments" "path" "Saved Games/Hades" "pattern" "Profile*.sav" } }
		"rootoverrides" { "0" { "os" "MacOS" "root" "WinMyDocuments" "useinstead" "MacAppSupport" } }
	}`))
	name := "%WinMyDocuments%Saved Games/Hades/Profile0.sav"
	if !cloudNameCovered(name, hades, proton, 0) || !cloudNameCovered(name, hades, native, 0) {
		t.Fatal("Hades' rule must cover its save whatever the OS override says")
	}
}

// A rule without `recursive` owns the folder's own files only, like the scan
// it feeds; a recursive one owns its subdirectories.
func TestCloudNameCoveredDepth(t *testing.T) {
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: "/games/x"}
	plain := parseCloudRules(mustParseKV(t, gameInstallUFS()))
	if !cloudNameCovered("%GameInstall%saves/a.sav", plain, ctx, 0) {
		t.Fatal("a non-recursive rule must cover its folder's own files")
	}
	if cloudNameCovered("%GameInstall%saves/deep/a.sav", plain, ctx, 0) {
		t.Fatal("a non-recursive rule must not cover subdirectories")
	}
	recursive := parseCloudRules(mustParseKV(t, `"ufs" {
		"savefiles" { "0" { "root" "GameInstall" "path" "saves" "pattern" "*.sav" "recursive" "1" } }
	}`))
	if !cloudNameCovered("%GameInstall%saves/deep/a.sav", recursive, ctx, 0) {
		t.Fatal("a recursive rule owns its subdirectories")
	}
}

// Proton's drive_c is case-insensitive, so a rule's spelling covers the
// cloud's other casing; a native Linux path does not.
func TestCloudNameCoveredCaseFolding(t *testing.T) {
	ufs := parseCloudRules(mustParseKV(t, `"ufs" {
		"savefiles" { "0" { "root" "WinMyDocuments" "path" "My Games/Game" "pattern" "Save*.sav" } }
	}`))
	name := "%WinMyDocuments%my games/game/save01.sav"
	proton := cloudContext{OS: "windows", Home: "/home/u", InstallDir: "/games/x",
		PrefixDir: "/prefixes/game", WinUser: "steamuser"}
	native := cloudContext{OS: "linux", Home: "/home/u", InstallDir: "/games/x"}
	if !cloudNameCovered(name, ufs, proton, 0) {
		t.Fatal("Proton must cover another casing of the rule path")
	}
	if cloudNameCovered(name, ufs, native, 0) {
		t.Fatal("native Linux paths are case-sensitive")
	}
}

// rootoverrides only place a file: a pathtransforms find string that also
// appears in the file name can't make the rule miss its own save.
func TestCloudNameCoveredOverrideInFileName(t *testing.T) {
	ufs := parseCloudRules(mustParseKV(t, `"ufs" {
		"savefiles" { "0" { "root" "WinAppDataLocalLow" "path" "Foo/Saves" "pattern" "Foo*.sav" } }
		"rootoverrides" { "0" { "os" "Linux" "root" "WinAppDataLocalLow" "useinstead" "LinuxHome"
			"pathtransforms" { "0" { "find" "Foo" "replace" "foo" } } } }
	}`))
	name := "%WinAppDataLocalLow%Foo/Saves/FooSave.sav"
	native := superhotCtx("/home/u")
	proton := cloudContext{OS: "windows", Home: "/home/u", InstallDir: "/games/x",
		PrefixDir: "/prefixes/game", WinUser: "steamuser"}
	if !cloudNameCovered(name, ufs, native, 0) || !cloudNameCovered(name, ufs, proton, 0) {
		t.Fatal("the override must not decide whether the rule covers the name")
	}
	// Placement still follows the override.
	got, _, ok := mapCloudName(name, ufs, native, 0)
	if !ok || got != "/home/u/foo/Saves/fooSave.sav" {
		t.Fatalf("local = %q, ok = %v, want the transformed path", got, ok)
	}
}

// A recursive rule owns its subdirectories, and a rule path with the
// account id only covers the expanded spelling.
func TestCloudNameCoveredRecursiveAndSteamID(t *testing.T) {
	ufs := parseCloudRules(mustParseKV(t, `"ufs" {
		"savefiles" {
			"0" { "root" "GameInstall" "path" "saves/{64BitSteamID}" "pattern" "*.sav" "recursive" "1" }
			"1" { "root" "LinuxHome" "path" ".config/game/{Steam3AccountID}" "pattern" "*.cfg" }
		} }`))
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: "/games/x"}
	const id uint64 = 76561198000000000
	if !cloudNameCovered("%GameInstall%saves/76561198000000000/deep/a.sav", ufs, ctx, id) {
		t.Fatal("a recursive rule must cover nested saves")
	}
	if cloudNameCovered("%GameInstall%saves/76561198000000000/deep/a.txt", ufs, ctx, id) {
		t.Fatal("the pattern still decides")
	}
	if cloudNameCovered("%GameInstall%saves/123/a.sav", ufs, ctx, id) {
		t.Fatal("another account's folder is not this rule's")
	}
	// {Steam3AccountID} is the low 32 bits of the same id.
	low := id & 0xffffffff
	if !cloudNameCovered("%LinuxHome%.config/game/"+strconv.FormatUint(low, 10)+"/a.cfg", ufs, ctx, id) {
		t.Fatal("{Steam3AccountID} rules must cover their expanded spelling")
	}
	if cloudNameCovered("%LinuxHome%.config/game/1/a.cfg", ufs, ctx, id) {
		t.Fatal("another account id is not this rule's")
	}
}

// A rule path the scan refuses names no directory, so it can't cover a cloud
// file either: the name is unmapped instead of downloaded somewhere guessed.
func TestCloudNameCoveredRejectsUnscannablePath(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: dir}
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
		"0" { "root" "GameInstall" "path" "../evil" "pattern" "*.sav" }
		"1" { "root" "GameInstall" "path" "C:/saves" "pattern" "*.sav" } } }`))
	for _, r := range ufs.Rules {
		if _, ok := ruleSearchDir(r, ufs, ctx, 0); ok {
			t.Fatalf("rule path %q must be refused by the scan", r.Path)
		}
	}
	for _, name := range []string{"%GameInstall%../evil/a.sav", "%GameInstall%C:/saves/a.sav"} {
		if cloudNameCovered(name, ufs, ctx, 0) {
			t.Fatalf("%q must not be covered", name)
		}
	}
	list := []cloudFile{listed("%GameInstall%../evil/a.sav", "v1", 100),
		listed("%GameInstall%C:/saves/b.sav", "v2", 100)}
	files, unmapped := compareCloud(list, nil, ufs, ctx, 0, &cloudBaseline{Files: map[string]cloudBaselineFile{}})
	if unmapped != 2 || len(files) != 0 {
		t.Fatalf("unscannable rules must leave their names unmapped: unmapped=%d files=%+v", unmapped, files)
	}
}

func writeLocal(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func shaHex(b []byte) string {
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:])
}

func listed(name string, content string, mtime int64) cloudFile {
	return cloudFile{CloudName: name, SHA: mustSHA(content), Size: uint32(len(content)), MTime: mtime}
}

func mustSHA(s string) []byte {
	sum := sha1.Sum([]byte(s))
	return sum[:]
}

// Two entries for one local path (issue #6 L2: Megabonk lists 4 entries
// across two machine/prefix variants) compare once, latest winning; the
// rest of the matrix covers every compare state. Baseline entries carry
// the local path they were recorded for.
func TestCompareCloudStates(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: dir}
	ufs := parseCloudRules(mustParseKV(t, gameInstallUFS()))
	writeLocal(t, filepath.Join(dir, "saves", "same.sav"), "same")
	writeLocal(t, filepath.Join(dir, "saves", "conflict.sav"), "local-version")
	writeLocal(t, filepath.Join(dir, "saves", "upload.sav"), "new-local")
	writeLocal(t, filepath.Join(dir, "saves", "localonly.sav"), "brand-new")
	writeLocal(t, filepath.Join(dir, "saves", "stale.sav"), "same")
	at := func(name string) string { return filepath.Join(dir, "saves", name) }
	base := &cloudBaseline{Files: map[string]cloudBaselineFile{
		"saves/same.sav":     {SHA: shaHex([]byte("same")), Local: at("same.sav")},
		"saves/upload.sav":   {SHA: shaHex([]byte("same")), Local: at("upload.sav")}, // cloud == baseline
		"saves/download.sav": {SHA: shaHex([]byte("same")), Local: at("download.sav")},
		"saves/deleted.sav":  {SHA: shaHex([]byte("same")), Local: at("deleted.sav")},
		"saves/stale.sav":    {SHA: shaHex([]byte("same")), Local: at("elsewhere.sav")}, // other side's record
	}}
	list := []cloudFile{
		listed("%GameInstall%saves/same.sav", "same", 100),
		listed("%GameInstall%saves/upload.sav", "same", 100), // cloud == baseline, local changed
		listed("%GameInstall%saves/download.sav", "cloud-new", 100),
		listed("%GameInstall%saves/deleted.sav", "same", 100),
		listed("%GameInstall%saves/stale.sav", "same", 100),
		listed("%GameInstall%saves/conflict.sav", "cloud-version", 100),
		{CloudName: "%GameInstall%saves/dup.sav", SHA: mustSHA("old"), Size: 3, MTime: 50},
		{CloudName: "%GameInstall%saves/dup.sav", SHA: mustSHA("new"), Size: 3, MTime: 150},
		{CloudName: "%WinMyDocuments%elsewhere/x.sav", SHA: mustSHA("x"), Size: 1},
	}
	_ = os.Remove(at("download.sav"))
	_ = os.Remove(at("deleted.sav"))
	local := scanLocal(ufs, ctx, 0)
	files, unmapped := compareCloud(list, local, ufs, ctx, 0, base)
	if unmapped != 1 {
		t.Fatalf("unmapped = %d, want 1", unmapped)
	}
	want := map[string]string{
		"saves/same.sav":      "same",
		"saves/conflict.sav":  "conflict", // no baseline record: ask
		"saves/upload.sav":    "upload",
		"saves/download.sav":  "download", // missing locally: nothing to lose
		"saves/deleted.sav":   "download", // deleted here, unchanged there: restore
		"saves/stale.sav":     "same",     // local matches cloud; other side's record ignored
		"saves/dup.sav":       "download", // latest entry wins; no baseline
		"saves/localonly.sav": "upload",
	}
	if len(files) != len(want) {
		t.Fatalf("files = %+v, want %d entries", files, len(want))
	}
	for _, f := range files {
		if want[f.Rel] != f.Want {
			t.Errorf("%s = %s, want %s", f.Rel, f.Want, want[f.Rel])
		}
	}
}

// A native<->Proton switch never misreads the other side's record: the
// baseline entry for the old local path doesn't turn the new side's
// differing file into an upload.
func TestCompareCloudPlatformSwitch(t *testing.T) {
	dir := t.TempDir()
	native := cloudContext{OS: "linux", Home: filepath.Join(dir, "home"), InstallDir: filepath.Join(dir, "install")}
	prefix := filepath.Join(dir, "prefix")
	proton := cloudContext{OS: "windows", Home: filepath.Join(dir, "home"), InstallDir: filepath.Join(dir, "install"),
		PrefixDir: prefix, WinUser: "steamuser"}
	ufs := parseCloudRules(mustParseKV(t, superhotUFS))
	cloudName := "%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/super.hot"
	nativeLocal, _, ok := mapCloudName(cloudName, ufs, native, 0)
	if !ok {
		t.Fatal("SUPERHOT must map natively")
	}
	protonLocal, _, ok := mapCloudName(cloudName, ufs, proton, 0)
	if !ok {
		t.Fatal("SUPERHOT must map under Proton")
	}
	if nativeLocal == protonLocal {
		t.Fatal("the switch must move the file")
	}
	writeLocal(t, protonLocal, "proton-save")
	old := &cloudBaseline{Files: map[string]cloudBaselineFile{
		"SUPERHOT_Team/SUPERHOT/super.hot": {SHA: shaHex([]byte("native-save")), Local: nativeLocal},
	}}
	list := []cloudFile{{CloudName: cloudName, SHA: mustSHA("native-save"), Size: 11, MTime: 100}}
	local := scanLocal(ufs, proton, 0)
	files, _ := compareCloud(list, local, ufs, proton, 0, old)
	if len(files) != 1 || files[0].Want != "conflict" {
		t.Fatalf("the other side's record must not decide: %+v", files)
	}
}

// All same, then the cloud changes: the new listing downloads instead of
// conflicting, because the baseline proves the local side is unchanged.
func TestCompareCloudSameThenChanged(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: dir}
	ufs := parseCloudRules(mustParseKV(t, gameInstallUFS()))
	writeLocal(t, filepath.Join(dir, "saves", "a.sav"), "v1")
	at := filepath.Join(dir, "saves", "a.sav")
	base := &cloudBaseline{Files: map[string]cloudBaselineFile{
		"saves/a.sav": {SHA: shaHex([]byte("v1")), Local: at},
	}}
	list := []cloudFile{listed("%GameInstall%saves/a.sav", "v2", 200)}
	local := scanLocal(ufs, ctx, 0)
	files, _ := compareCloud(list, local, ufs, ctx, 0, base)
	if len(files) != 1 || files[0].Want != "download" {
		t.Fatalf("changed only in the cloud must download: %+v", files)
	}
}

// A cloud name no rule covers is unmapped and never becomes a plan file,
// even though its root maps: the untouched local file stays untouched.
func TestCompareCloudDropsUncoveredNames(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: dir}
	ufs := parseCloudRules(mustParseKV(t, gameInstallUFS()))
	writeLocal(t, filepath.Join(dir, "config", "settings.ini"), "local")
	list := []cloudFile{
		listed("%GameInstall%saves/a.sav", "a", 100),
		listed("%GameInstall%saves/notes.txt", "b", 100),
		listed("%GameInstall%config/settings.ini", "c", 100),
	}
	base := &cloudBaseline{Files: map[string]cloudBaselineFile{}}
	files, unmapped := compareCloud(list, scanLocal(ufs, ctx, 0), ufs, ctx, 0, base)
	if unmapped != 2 {
		t.Fatalf("unmapped = %d, want 2", unmapped)
	}
	if len(files) != 1 || files[0].Rel != "saves/a.sav" || files[0].Want != "download" {
		t.Fatalf("files = %+v", files)
	}
}

func TestCompareCloudDeletedInCloud(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: dir}
	ufs := parseCloudRules(mustParseKV(t, gameInstallUFS()))
	writeLocal(t, filepath.Join(dir, "saves", "gone.sav"), "still-here")
	base := &cloudBaseline{Files: map[string]cloudBaselineFile{}}
	list := []cloudFile{{CloudName: "%GameInstall%saves/gone.sav", SHA: mustSHA("x"), Size: 1, Persist: 2}}
	local := scanLocal(ufs, ctx, 0)
	files, _ := compareCloud(list, local, ufs, ctx, 0, base)
	if len(files) != 1 || files[0].Want != "conflict" {
		t.Fatalf("deleted in cloud but present here must ask: %+v", files)
	}
}

func TestBuildStatusShapes(t *testing.T) {
	files := []cloudPlanFile{{Rel: "a", Want: "same"}, {Rel: "b", Want: "download"}}
	st := buildStatus(1, files, 0, "x")
	if st["state"] != "pending" {
		t.Fatalf("state = %v", st["state"])
	}
	counts := st["counts"].(cloudStatusCounts)
	if counts.Same != 1 || counts.Download != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	st = buildStatus(1, []cloudPlanFile{{Rel: "a", Want: "conflict", Remote: cloudFile{MTime: 5}}}, 0, "x")
	if st["state"] != "conflict" {
		t.Fatalf("state = %v", st["state"])
	}
	if conflicts := st["conflicts"].([]cloudStatusConflict); len(conflicts) != 1 || conflicts[0].Path != "a" {
		t.Fatalf("conflicts = %+v", st["conflicts"])
	}
	st = buildStatus(1, []cloudPlanFile{{Rel: "a", Want: "same"}}, 0, "ok")
	if st["state"] != "synced" || st["conflicts"].([]cloudStatusConflict) == nil {
		t.Fatalf("synced status must carry an (empty) conflicts list: %+v", st)
	}
}

func zipBody(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func serveBytes(body []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
}

func TestFetchCloudBytesPlain(t *testing.T) {
	srv := serveBytes([]byte("save-data"))
	defer srv.Close()
	dl := &downloadInfo{Host: strings.TrimPrefix(srv.URL, "http://"),
		Path: "/f", SHA: mustSHA("save-data"), RawSize: 9}
	raw, err := fetchCloudBytes(context.Background(), dl)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "save-data" {
		t.Fatalf("raw = %q", raw)
	}
}

func TestFetchCloudBytesZip(t *testing.T) {
	inner := "zipped-save"
	body := zipBody(t, "save.sav", inner)
	srv := serveBytes(body)
	defer srv.Close()
	dl := &downloadInfo{Host: strings.TrimPrefix(srv.URL, "http://"),
		Path: "/f", SHA: mustSHA(inner), RawSize: uint32(len(inner))}
	raw, err := fetchCloudBytes(context.Background(), dl)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != inner {
		t.Fatalf("raw = %q", raw)
	}
}

func TestFetchCloudBytesBadSHAWritesNothing(t *testing.T) {
	srv := serveBytes([]byte("tampered"))
	defer srv.Close()
	dl := &downloadInfo{Host: strings.TrimPrefix(srv.URL, "http://"),
		Path: "/f", SHA: mustSHA("original"), RawSize: 8}
	if _, err := fetchCloudBytes(context.Background(), dl); err == nil {
		t.Fatal("a SHA mismatch must fail before anything is written")
	}
}

func TestFetchCloudBytesEncryptedRefused(t *testing.T) {
	dl := &downloadInfo{Encrypted: true, Host: "x", Path: "/f"}
	if _, err := fetchCloudBytes(context.Background(), dl); err == nil {
		t.Fatal("encrypted files must be refused")
	}
}

func TestWriteCloudFileBackupAndMtime(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	env := &cloudEnv{appID: 4321, jobID: "job-1", base: loadBaseline(4321)}
	local := filepath.Join(t.TempDir(), "saves", "a.sav")
	writeLocal(t, local, "old")
	before, _ := os.Stat(local)
	backedUp, err := env.writeCloudFile(local, "saves/a.sav", []byte("new"), 1700000000)
	if err != nil {
		t.Fatal(err)
	}
	if !backedUp {
		t.Fatal("replacing a local file must back it up")
	}
	if got, _ := os.ReadFile(local); string(got) != "new" {
		t.Fatalf("local = %q", got)
	}
	st, _ := os.Stat(local)
	if st.ModTime().Unix() != 1700000000 {
		t.Fatalf("mtime = %v", st.ModTime())
	}
	backupRoot := filepath.Join(os.Getenv("PLATYPUS_PLUGIN_DATA"), "cloud-backups", "4321")
	entries, err := os.ReadDir(backupRoot)
	if err != nil || len(entries) != 1 {
		t.Fatalf("backups = %v, %v", entries, err)
	}
	// One run leaves one backup folder, named with the timestamp and job.
	if !strings.Contains(entries[0].Name(), "job-1") {
		t.Fatalf("backup dir = %q", entries[0].Name())
	}
	got, err := os.ReadFile(filepath.Join(backupRoot, entries[0].Name(), "saves", "a.sav"))
	if err != nil || string(got) != "old" {
		t.Fatalf("backup = %q, %v", got, err)
	}
	// The backup keeps the original mtime.
	bst, _ := os.Stat(filepath.Join(backupRoot, entries[0].Name(), "saves", "a.sav"))
	if !bst.ModTime().Equal(before.ModTime()) {
		t.Fatalf("backup mtime = %v, want %v", bst.ModTime(), before.ModTime())
	}
	// A fresh file (nothing to replace) writes without a backup.
	fresh := filepath.Join(t.TempDir(), "saves", "b.sav")
	backedUp, err = env.writeCloudFile(fresh, "saves/b.sav", []byte("b"), 1700000000)
	if err != nil || backedUp {
		t.Fatalf("fresh write: backedUp=%v err=%v", backedUp, err)
	}
}

func TestApplyChoices(t *testing.T) {
	env := &cloudEnv{appID: 1, base: &cloudBaseline{Files: map[string]cloudBaselineFile{}}}
	files := []cloudPlanFile{
		{Rel: "a.sav", Want: "conflict", Local: "/x/a.sav", LocalInfo: cloudLocal{Found: true, Size: 1}, Remote: cloudFile{SHA: mustSHA("r")}},
	}
	remote, err := env.applyChoices(files, map[string]string{"a.sav": "remote"})
	if err != nil {
		t.Fatal(err)
	}
	if !remote["a.sav"] || files[0].Want != "download" {
		t.Fatalf("remote choice must download: %+v %v", files, remote)
	}
	files[0].Want = "conflict"
	if _, err := env.applyChoices(files, map[string]string{"a.sav": "local"}); err != nil {
		t.Fatal(err)
	}
	if files[0].Want != "upload" || env.base.Choices["a.sav"].Side != "local" {
		t.Fatalf("local choice must defer to the push phase: %+v", files)
	}
	if _, err := env.applyChoices(files, map[string]string{"nope.sav": "remote"}); err == nil {
		t.Fatal("a choice for an unlisted path must be refused")
	}
	if _, err := env.applyChoices(files, map[string]string{"a.sav": "sideways"}); err == nil {
		t.Fatal("a bad choice value must be refused")
	}
}

func TestJobResultPath(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	saveJobResult(1, "conf-pull", json.RawMessage(`{"a":1}`))
	raw, ok := loadJobResult(1, "conf-pull")
	if !ok || string(raw) != `{"a":1}` {
		t.Fatalf("job cache round trip: %q %v", raw, ok)
	}
	if _, ok := loadJobResult(1, "nope"); ok {
		t.Fatal("a missing job must miss")
	}
	// Unsafe ids hash instead of escaping the jobs dir.
	saveJobResult(1, "../../evil", json.RawMessage(`{}`))
	matches, _ := filepath.Glob(filepath.Join(cloudJobsDir(), "*.json"))
	for _, m := range matches {
		if rel, _ := filepath.Rel(cloudJobsDir(), m); strings.Contains(rel, "..") || strings.ContainsRune(rel, '/') {
			t.Fatalf("job cache escaped: %q", m)
		}
	}
}

func TestScanLocalRecursiveAndPlatforms(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: dir}
	ufs := parseCloudRules(mustParseKV(t, `"ufs" {
		"savefiles" {
			"0" { "root" "GameInstall" "path" "saves" "pattern" "*.sav" "recursive" "1" }
			"1" { "root" "GameInstall" "path" "winsaves" "pattern" "*.sav"
			       "platforms" { "0" "Windows" } }
		} }`))
	writeLocal(t, filepath.Join(dir, "saves", "deep", "a.sav"), "a")
	writeLocal(t, filepath.Join(dir, "saves", "b.txt"), "b")
	writeLocal(t, filepath.Join(dir, "winsaves", "c.sav"), "c")
	got := scanLocal(ufs, ctx, 0)
	if len(got) != 1 {
		t.Fatalf("recursive scan must find only the matching save: %v", got)
	}
	for lp, sc := range got {
		if !strings.HasSuffix(lp, filepath.Join("saves", "deep", "a.sav")) || sc.Rel != "saves/deep/a.sav" {
			t.Fatalf("scan = %q -> %+v", lp, sc)
		}
	}
}

// The rule paths appinfo really carries are spelled "/My Games/…", "./saves/"
// or "Steam Save/"; the scan must find those files and name them the way the
// mapper spells them, or the upload name is rejected and re-uploaded forever
// (#187).
func TestScanLocalNormalisesRulePaths(t *testing.T) {
	dir := t.TempDir()
	prefix := filepath.Join(dir, "prefix")
	docs := filepath.Join(prefix, "drive_c", "users", "steamuser", "Documents")
	ctx := cloudContext{OS: "windows", Home: filepath.Join(dir, "home"), InstallDir: filepath.Join(dir, "install"),
		PrefixDir: prefix, WinUser: "steamuser"}
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
		"0" { "root" "WinMyDocuments" "path" "/My Games/Fallout4/Saves" "pattern" "*.fos"
		      "platforms" { "1" "Windows" } }
		"1" { "root" "GameInstall" "path" "./saves/" "pattern" "*.sav" }
		"2" { "root" "GameInstall" "path" "Steam Save/" "pattern" "*.ss" }
	} }`))
	writeLocal(t, filepath.Join(docs, "My Games", "Fallout4", "Saves", "SAVE1.FOS"), "fos")
	writeLocal(t, filepath.Join(ctx.InstallDir, "saves", "a.sav"), "a")
	writeLocal(t, filepath.Join(ctx.InstallDir, "Steam Save", "b.ss"), "b")
	got := scanLocal(ufs, ctx, 0)
	want := map[string]string{ // local path -> normalised cloud-relative path
		filepath.Join(docs, "My Games", "Fallout4", "Saves", "SAVE1.FOS"): "My Games/Fallout4/Saves/SAVE1.FOS",
		filepath.Join(ctx.InstallDir, "saves", "a.sav"):                   "saves/a.sav",
		filepath.Join(ctx.InstallDir, "Steam Save", "b.ss"):               "Steam Save/b.ss",
	}
	if len(got) != len(want) {
		t.Fatalf("scan = %v, want %d files", got, len(want))
	}
	for lp, sc := range got {
		if want[lp] != sc.Rel {
			t.Errorf("%q -> rel %q, want %q", lp, sc.Rel, want[lp])
		}
		if !cloudNameCovered("%"+sc.Root+"%"+sc.Rel, ufs, ctx, 0) {
			t.Errorf("scanned name %q is not covered", sc.Rel)
		}
	}
}

// Super Cable Boy's appinfo spells a rule root "gameinstall" while the cloud
// names use %GameInstall%: root tokens match case-insensitively, so the scan
// finds the folder and a new save's name maps back to it.
func TestScanLocalRootCase(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "windows", Home: filepath.Join(dir, "home"), InstallDir: filepath.Join(dir, "install"),
		PrefixDir: filepath.Join(dir, "prefix"), WinUser: "steamuser"}
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
		"0" { "root" "gameinstall" "path" "./" "pattern" "*.sav" } } }`))
	save := filepath.Join(ctx.InstallDir, "save1.sav")
	writeLocal(t, save, "v1")
	if lp, rel, ok := mapCloudName("%GameInstall%save1.sav", ufs, ctx, 0); !ok || lp != save || rel != "save1.sav" {
		t.Fatalf("map = %q, %q, %v", lp, rel, ok)
	}
	sc, found := scanLocal(ufs, ctx, 0)[save]
	if !found || sc.Rel != "save1.sav" {
		t.Fatalf("scan = %v", scanLocal(ufs, ctx, 0))
	}
	// With no listing to copy from, the documented spelling names the file.
	files, _ := compareCloud(nil, scanLocal(ufs, ctx, 0), ufs, ctx, 0,
		&cloudBaseline{Files: map[string]cloudBaselineFile{}})
	if len(files) != 1 || files[0].CloudName != "%GameInstall%save1.sav" {
		t.Fatalf("new save = %+v, want the documented root spelling", files)
	}
	if lp, _, ok := mapCloudName(files[0].CloudName, ufs, ctx, 0); !ok || lp != save {
		t.Fatalf("upload name %q must map back, got %q, %v", files[0].CloudName, lp, ok)
	}
}

// New upload names follow the cloud's own spellings: a listed entry's root
// casing wins over the documented one, and a listed file in the same folder
// lends its folder casing (the Windows client writes "my games/fallout4/saves/").
func TestUploadCloudName(t *testing.T) {
	listed := func(name string) []cloudFile { return []cloudFile{{CloudName: name}} }
	for _, tc := range []struct {
		name      string
		sc        cloudScanned
		list      []cloudFile
		root, rel string
	}{
		{"documented root", cloudScanned{Rel: "save1.sav", Root: "gameinstall"}, nil,
			"GameInstall", "save1.sav"},
		{"listed root casing", cloudScanned{Rel: "save1.sav", Root: "gameinstall"},
			listed("%gameInstall%old.sav"), "gameInstall", "save1.sav"},
		{"listed folder casing", cloudScanned{Rel: "My Games/Fallout4/Saves/x.fos", Root: "WinMyDocuments"},
			listed("%WinMyDocuments%my games/fallout4/saves/save1.fos"),
			"WinMyDocuments", "my games/fallout4/saves/x.fos"},
		{"other folder untouched", cloudScanned{Rel: "My Games/Fallout4/Saves/x.fos", Root: "WinMyDocuments"},
			listed("%WinMyDocuments%Other/Save/save1.fos"),
			"WinMyDocuments", "My Games/Fallout4/Saves/x.fos"},
		// S1: another root's folder is not this rule's, even at the same rel.
		{"cross-root folder not borrowed", cloudScanned{Rel: "install/saves/x.sav", Root: "GameInstall"},
			listed("%LinuxHome%Install/Saves/old.sav"), "GameInstall", "install/saves/x.sav"},
		// N1: the canonical spelling wins over a folder neighbour's, in any order.
		{"canonical root beats folder casing", cloudScanned{Rel: "Saves/x.sav", Root: "WinMyDocuments"},
			[]cloudFile{{CloudName: "%winmydocuments%Saves/old.sav"}, {CloudName: "%WinMyDocuments%Other/old.sav"}},
			"WinMyDocuments", "Saves/x.sav"},
		{"canonical root beats folder casing reversed", cloudScanned{Rel: "Saves/x.sav", Root: "WinMyDocuments"},
			[]cloudFile{{CloudName: "%WinMyDocuments%Other/old.sav"}, {CloudName: "%winmydocuments%Saves/old.sav"}},
			"WinMyDocuments", "Saves/x.sav"},
		{"folder root when no canonical", cloudScanned{Rel: "My Games/G/x.fos", Root: "WinMyDocuments"},
			[]cloudFile{{CloudName: "%winmydocuments%Other/old.sav"}, {CloudName: "%winmydocuments%my games/g/save.fos"}},
			"winmydocuments", "my games/g/x.fos"},
		{"first listed root when nothing else matches", cloudScanned{Rel: "Saves/x.sav", Root: "WinMyDocuments"},
			[]cloudFile{{CloudName: "%winmydocuments%Other/a.sav"}, {CloudName: "%WINMYDOCUMENTS%Third/b.sav"}},
			"winmydocuments", "Saves/x.sav"},
	} {
		root, rel := uploadCloudName(tc.sc, tc.list)
		if root != tc.root || rel != tc.rel {
			t.Errorf("%s: got root %q rel %q, want root %q rel %q", tc.name, root, rel, tc.root, tc.rel)
		}
	}
}

// Fallout 4: the cloud's folder spelling differs in case and the rule path
// has a leading slash. After the pull the status is "same" with no upload
// candidate; a new save uploads under the mapper's name and the next status
// is "same" again, so the push doesn't repeat.
func TestCompareCloudFalloutRoundTrip(t *testing.T) {
	dir := t.TempDir()
	prefix := filepath.Join(dir, "prefix")
	docs := filepath.Join(prefix, "drive_c", "users", "steamuser", "Documents")
	ctx := cloudContext{OS: "windows", Home: filepath.Join(dir, "home"), InstallDir: filepath.Join(dir, "install"),
		PrefixDir: prefix, WinUser: "steamuser"}
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
		"0" { "root" "WinMyDocuments" "path" "/My Games/Fallout4/Saves" "pattern" "*.fos" } } }`))
	save := filepath.Join(docs, "My Games", "Fallout4", "Saves", "save1.fos")
	writeLocal(t, save, "v1")
	cloudName := "%WinMyDocuments%my games/fallout4/saves/save1.fos" // the Windows client's casing
	base := &cloudBaseline{Files: map[string]cloudBaselineFile{}}
	files, unmapped := compareCloud([]cloudFile{listed(cloudName, "v1", 100)}, scanLocal(ufs, ctx, 0), ufs, ctx, 0, base)
	if unmapped != 0 || len(files) != 1 || files[0].Want != "same" || files[0].Local != save {
		t.Fatalf("after a pull the sides agree: unmapped=%d files=%+v", unmapped, files)
	}
	writeLocal(t, filepath.Join(docs, "My Games", "Fallout4", "Saves", "x.fos"), "new")
	files, unmapped = compareCloud([]cloudFile{listed(cloudName, "v1", 100)}, scanLocal(ufs, ctx, 0), ufs, ctx, 0, base)
	var up *cloudPlanFile
	for i := range files {
		if files[i].Want == "upload" {
			up = &files[i]
		}
	}
	const wantName = "%WinMyDocuments%my games/fallout4/saves/x.fos" // the listed folder's casing
	if unmapped != 0 || up == nil || up.CloudName != wantName {
		t.Fatalf("new save = %+v (unmapped %d), want upload %q", files, unmapped, wantName)
	}
	if !cloudNameCovered(up.CloudName, ufs, ctx, 0) {
		t.Fatal("every upload candidate must be a name a rule covers")
	}
	// The push records what it uploaded; the next status agrees.
	base.Files[up.Rel] = cloudBaselineFile{SHA: shaHex([]byte("new")), Size: 3, Local: up.Local}
	next := []cloudFile{listed(cloudName, "v1", 100), listed(up.CloudName, "new", 101)}
	files, unmapped = compareCloud(next, scanLocal(ufs, ctx, 0), ufs, ctx, 0, base)
	if unmapped != 0 || len(files) != 2 {
		t.Fatalf("next status: unmapped=%d files=%+v", unmapped, files)
	}
	for _, f := range files {
		if f.Want != "same" {
			t.Fatalf("re-upload loop: %+v", files)
		}
	}
}

// A file the cloud already lists uploads under the listing's exact name: the
// server keys it by that spelling, not by the rule's.
func TestCompareCloudKeepsListedName(t *testing.T) {
	dir := t.TempDir()
	prefix := filepath.Join(dir, "prefix")
	docs := filepath.Join(prefix, "drive_c", "users", "steamuser", "Documents")
	ctx := cloudContext{OS: "windows", Home: filepath.Join(dir, "home"), InstallDir: filepath.Join(dir, "install"),
		PrefixDir: prefix, WinUser: "steamuser"}
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
		"0" { "root" "WinMyDocuments" "path" "/My Games/Fallout4/Saves" "pattern" "*.fos" } } }`))
	save := filepath.Join(docs, "My Games", "Fallout4", "Saves", "Save1.FOS")
	writeLocal(t, save, "edited")
	cloudName := "%WinMyDocuments%my games/fallout4/saves/Save1.FOS"
	base := &cloudBaseline{Files: map[string]cloudBaselineFile{
		"my games/fallout4/saves/Save1.FOS": {SHA: shaHex([]byte("v1")), Local: save},
	}}
	files, unmapped := compareCloud([]cloudFile{listed(cloudName, "v1", 100)}, scanLocal(ufs, ctx, 0), ufs, ctx, 0, base)
	if unmapped != 0 || len(files) != 1 || files[0].Want != "upload" || files[0].CloudName != cloudName {
		t.Fatalf("files = %+v (unmapped %d), want the listing's name", files, unmapped)
	}
}

// The name a new file uploads under is the baseline key, so a later edit
// compares as an upload of that file instead of a conflict with it.
func TestCompareCloudNewFileBaselineAfterUpload(t *testing.T) {
	dir := t.TempDir()
	prefix := filepath.Join(dir, "prefix")
	docs := filepath.Join(prefix, "drive_c", "users", "steamuser", "Documents")
	ctx := cloudContext{OS: "windows", Home: filepath.Join(dir, "home"), InstallDir: filepath.Join(dir, "install"),
		PrefixDir: prefix, WinUser: "steamuser"}
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
		"0" { "root" "WinMyDocuments" "path" "/My Games/Fallout4/Saves" "pattern" "*.fos" } } }`))
	save := filepath.Join(docs, "My Games", "Fallout4", "Saves", "save1.fos")
	writeLocal(t, save, "v1")
	listedName := "%WinMyDocuments%my games/fallout4/saves/save1.fos"
	base := &cloudBaseline{Files: map[string]cloudBaselineFile{}}
	files, _ := compareCloud([]cloudFile{listed(listedName, "v1", 100)}, scanLocal(ufs, ctx, 0), ufs, ctx, 0, base)
	if len(files) != 1 || files[0].Want != "same" {
		t.Fatalf("pulled state = %+v", files)
	}
	base.Files[files[0].Rel] = cloudBaselineFile{SHA: shaHex([]byte("v1")), Size: 2, Local: save}
	newSave := filepath.Join(docs, "My Games", "Fallout4", "Saves", "x.fos")
	writeLocal(t, newSave, "new")
	files, _ = compareCloud([]cloudFile{listed(listedName, "v1", 100)}, scanLocal(ufs, ctx, 0), ufs, ctx, 0, base)
	var up *cloudPlanFile
	for i := range files {
		if files[i].Want == "upload" {
			up = &files[i]
		}
	}
	if up == nil || up.Rel != "my games/fallout4/saves/x.fos" {
		t.Fatalf("new save = %+v", files)
	}
	sent := up.CloudName
	base.Files[up.Rel] = cloudBaselineFile{SHA: shaHex([]byte("new")), Size: 3, Local: up.Local} // pushFiles' record
	writeLocal(t, newSave, "new2")
	files, unmapped := compareCloud([]cloudFile{listed(listedName, "v1", 100), listed(sent, "new", 101)},
		scanLocal(ufs, ctx, 0), ufs, ctx, 0, base)
	if unmapped != 0 || len(files) != 2 {
		t.Fatalf("later status: unmapped=%d files=%+v", unmapped, files)
	}
	for _, f := range files {
		if f.Rel == "my games/fallout4/saves/x.fos" && f.Want != "upload" {
			t.Fatalf("a clean edit must upload, not conflict: %+v", f)
		}
	}
}

// A rule path carrying a backslash names the same subdirectories: the scan
// normalises it, coverage follows, and the name maps back.
func TestScanLocalBackslashRulePath(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: dir}
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
		"0" { "root" "GameInstall" "path" "a\\b/saves" "pattern" "*.sav" } } }`))
	save := filepath.Join(dir, "a", "b", "saves", "x.sav")
	writeLocal(t, save, "v1")
	sc, found := scanLocal(ufs, ctx, 0)[save]
	if !found || sc.Rel != "a/b/saves/x.sav" {
		t.Fatalf("scan = %v", scanLocal(ufs, ctx, 0))
	}
	name := "%" + sc.Root + "%" + sc.Rel
	if !cloudNameCovered(name, ufs, ctx, 0) {
		t.Fatalf("%q must be covered", name)
	}
	if lp, _, ok := mapCloudName(name, ufs, ctx, 0); !ok || lp != save {
		t.Fatalf("map %q = %q, %v", name, lp, ok)
	}
}

// A Windows-tagged rule doesn't describe native Linux's device files, but the
// cloud name it owns still downloads: coverage ignores platforms, the scan
// respects them.
func TestScanLocalSkipsWindowsRuleOnLinux(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: dir}
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
		"0" { "root" "GameInstall" "path" "saves" "pattern" "*.sav"
		      "platforms" { "0" "Windows" } } } }`))
	writeLocal(t, filepath.Join(dir, "saves", "local.sav"), "local")
	if got := scanLocal(ufs, ctx, 0); len(got) != 0 {
		t.Fatalf("a Windows rule must not scan on linux: %v", got)
	}
	cloudName := "%GameInstall%saves/cloud.sav"
	files, unmapped := compareCloud([]cloudFile{listed(cloudName, "remote", 100)}, scanLocal(ufs, ctx, 0),
		ufs, ctx, 0, &cloudBaseline{Files: map[string]cloudBaselineFile{}})
	if unmapped != 0 || len(files) != 1 || files[0].Want != "download" || files[0].CloudName != cloudName {
		t.Fatalf("the cloud name must still download: unmapped=%d files=%+v", unmapped, files)
	}
}

// A listed name that differs from a new local file's only in case is that
// file's cloud copy on a case-sensitive OS: uploading the other casing would
// add a duplicate save, and the two spellings would download+upload each
// other forever. Proton resolves both spellings to one file, which compares
// normally.
func TestCompareCloudNoCaseOnlySiblingUploads(t *testing.T) {
	const recursiveUFS = `"ufs" { "savefiles" {
		"0" { "root" "GameInstall" "path" "saves" "pattern" "*.sav" "recursive" "1" } } }`
	const plainUFS = `"ufs" { "savefiles" {
		"0" { "root" "GameInstall" "path" "Saves" "pattern" "*.sav" } } }`
	for _, tc := range []struct {
		name     string
		os       string
		rules    string
		rel      string // local save under the install dir
		cloud    string
		unmapped int
	}{
		// (a) a recursive rule, cloud subfolder spelled lowercase. The listed
		// name is covered and downloads; the local case twin is unmapped.
		{"recursive linux", "linux", recursiveUFS, "saves/Slot1/x.sav", "%GameInstall%saves/slot1/x.sav", 1},
		{"recursive windows", "windows", recursiveUFS, "saves/Slot1/x.sav", "%GameInstall%saves/slot1/x.sav", 0},
		// (b) a plain rule whose folder itself is the differing case: on
		// native Linux the listed name isn't the rule's either.
		{"plain linux", "linux", plainUFS, "Saves/x.sav", "%GameInstall%saves/x.sav", 2},
		{"plain windows", "windows", plainUFS, "Saves/x.sav", "%GameInstall%saves/x.sav", 0},
	} {
		dir := t.TempDir()
		ctx := cloudContext{OS: tc.os, Home: filepath.Join(dir, "home"), InstallDir: filepath.Join(dir, "install"),
			PrefixDir: filepath.Join(dir, "prefix"), WinUser: "steamuser"}
		ufs := parseCloudRules(mustParseKV(t, tc.rules))
		writeLocal(t, filepath.Join(ctx.InstallDir, filepath.FromSlash(tc.rel)), "v1")
		list := []cloudFile{listed(tc.cloud, "v1", 100)}
		files, unmapped := compareCloud(list, scanLocal(ufs, ctx, 0), ufs, ctx, 0,
			&cloudBaseline{Files: map[string]cloudBaselineFile{}})
		for _, f := range files {
			if f.Want == "upload" || f.CloudName != tc.cloud && !strings.EqualFold(f.CloudName, tc.cloud) {
				t.Fatalf("%s: a case-only sibling must not round-trip: files=%+v unmapped=%d", tc.name, files, unmapped)
			}
		}
		if unmapped != tc.unmapped {
			t.Fatalf("%s: unmapped = %d, want %d: files=%+v", tc.name, unmapped, tc.unmapped, files)
		}
		if tc.os == "windows" {
			// Proton reads the two spellings as one file: the listing is
			// this local file already.
			if len(files) != 1 || files[0].Want != "same" {
				t.Fatalf("%s: the listing must compare as this file: files=%+v", tc.name, files)
			}
		}
	}
}

// The upload invariant holds at the compare boundary too: a scanned file
// whose cloud name no rule covers is unmapped, never an upload candidate.
func TestCompareCloudSkipsUncoveredLocalCandidates(t *testing.T) {
	dir := t.TempDir()
	ctx := cloudContext{OS: "linux", Home: "/home/u", InstallDir: dir}
	ufs := parseCloudRules(mustParseKV(t, gameInstallUFS()))
	outside := filepath.Join(dir, "config", "settings.ini")
	writeLocal(t, outside, "local")
	local := map[string]cloudScanned{outside: {Rel: "config/settings.ini", Root: "GameInstall"}}
	files, unmapped := compareCloud(nil, local, ufs, ctx, 0, &cloudBaseline{Files: map[string]cloudBaselineFile{}})
	if unmapped != 1 || len(files) != 0 {
		t.Fatalf("an uncovered upload candidate must be unmapped: files=%+v unmapped=%d", files, unmapped)
	}
}

func TestDetectSteamClientAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if detectSteamClient() != nil {
		t.Fatal("no pid file must mean no client")
	}
}

func TestPruneBackupsKeepsLatest(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	root := filepath.Join(os.Getenv("PLATYPUS_PLUGIN_DATA"), "cloud-backups", "7")
	for _, d := range []string{"20240101-000000", "20240102-000000", "20240103-000000"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	pruneBackups(7, 2)
	entries, _ := os.ReadDir(root)
	if len(entries) != 2 || entries[0].Name() != "20240102-000000" {
		t.Fatalf("entries = %v", entries)
	}
}

// A truncated body matches neither the listing SHA nor a zip: nothing is
// written and the pull fails.
func TestFetchCloudBytesTruncated(t *testing.T) {
	srv := serveBytes([]byte("short"))
	defer srv.Close()
	dl := &downloadInfo{Host: strings.TrimPrefix(srv.URL, "http://"),
		Path: "/f", SHA: mustSHA("the whole save"), RawSize: 100}
	if _, err := fetchCloudBytes(context.Background(), dl); err == nil {
		t.Fatal("a truncated body must fail before anything is written")
	}
}

// noteSame records agreed files with the resolved local path; saveIfDirty
// writes once and stays quiet afterwards.
func TestNoteSameAndSaveIfDirty(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	env := &cloudEnv{appID: 4321, base: loadBaseline(4321)}
	local := filepath.Join(t.TempDir(), "saves", "a.sav")
	writeLocal(t, local, "v1")
	files := []cloudPlanFile{{Rel: "saves/a.sav", Local: local, Want: "same",
		HasRemote: true, Remote: cloudFile{SHA: mustSHA("v1"), Size: 2}}}
	env.noteSame(files)
	if !env.dirty {
		t.Fatal("recording a same file must dirty the baseline")
	}
	if err := env.saveIfDirty(); err != nil {
		t.Fatal(err)
	}
	if env.dirty {
		t.Fatal("a save must clear the dirty flag")
	}
	reloaded := loadBaseline(4321)
	got := reloaded.Files["saves/a.sav"]
	if got.SHA != shaHex([]byte("v1")) || got.Local != local {
		t.Fatalf("baseline = %+v", got)
	}
	// Nothing new learned: no write.
	env.noteSame(files)
	if env.dirty {
		t.Fatal("an already recorded file must not dirty the baseline")
	}
}

func TestJoinCloudRel(t *testing.T) {
	if got := joinCloudRel("", "a.sav"); got != "a.sav" {
		t.Fatalf("empty rule path: %q", got)
	}
	if got := joinCloudRel("saves", "a.sav"); got != "saves/a.sav" {
		t.Fatalf("join: %q", got)
	}
}

func TestParseChoices(t *testing.T) {
	if _, err := parseChoices(nil); err == nil {
		t.Fatal("missing choices must be refused")
	}
	if _, err := parseChoices(json.RawMessage(`"x"`)); err == nil {
		t.Fatal("non-object choices must be refused")
	}
	if _, err := parseChoices(json.RawMessage(`{"a": "sideways"}`)); err == nil {
		t.Fatal("a bad choice value must be refused")
	}
	got, err := parseChoices(json.RawMessage(`{"a": "remote", "b": "local"}`))
	if err != nil || got["a"] != "remote" || got["b"] != "local" {
		t.Fatalf("choices = %+v, %v", got, err)
	}
}
