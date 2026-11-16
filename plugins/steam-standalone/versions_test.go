package steam_test

// Versions (protocol 1.3) for the Steam plugin, through the real plugin
// path against the fake CM and CDN: branch listing, password unlock, and
// installs/updates honouring the target. No Valve network.

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
)

// versionsAppinfo is a PICS-shaped appinfo with three branches over the
// fixture depot: public and old open, beta behind a password. Like live,
// the locked branch carries no plain manifest gid: its gids only come
// from the private-beta section (privateBetaSection), unlocked with the
// branch key CheckBetaPassword returns.
func versionsAppinfo(gid uint64) string {
	return `"depots"
{
"branches"
{
"public" { "buildid" "100" "timeupdated" "1700000000" "description" "Default" }
"beta" { "buildid" "200" "timeupdated" "1700001000" "description" "Beta" "pwdrequired" "1" }
"old" { "buildid" "50" "description" "Old" }
}
"11"
{
"config" { "oslist" "linux" }
"manifests" { "public" { "gid" "` + fmt.Sprint(gid) + `" } "old" { "gid" "` + fmt.Sprint(gid) + `" } }
}
}
`
}

// privateBetaSection is the fake's 8907 answer for the locked branch: the
// depot section naming beta's manifest gid per depot plus the branch
// metadata, wrapped in a privatedepots node exactly like live Steam.
func privateBetaSection(gid uint64) string {
	return `"privatedepots"
{
"11"
{
"manifests" { "beta" { "gid" "` + fmt.Sprint(gid) + `" } }
}
"branches"
{
"beta" { "buildid" "200" "timeupdated" "1700001000" "description" "Beta" "pwdrequired" "1" }
}
}
`
}

// hiddenBranchAppinfo is the live shape for a password branch: PICS lists
// only the open branches, so the locked one must resolve through its
// private-beta section (privateBetaSection) alone.
func hiddenBranchAppinfo(gid uint64) string {
	return `"depots"
{
"branches"
{
"public" { "buildid" "100" "timeupdated" "1700000000" "description" "Default" }
"old" { "buildid" "50" "description" "Old" }
}
"11"
{
"config" { "oslist" "linux" }
"manifests" { "public" { "gid" "` + fmt.Sprint(gid) + `" } "old" { "gid" "` + fmt.Sprint(gid) + `" } }
}
}
`
}

// A branch hidden from PICS unlocks with its password and installs from
// its private-beta section: the live Valheim public-test shape.
func TestSteamVersionsInstallHiddenBranch(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", hiddenBranchAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))
	e.setCMState("beta_branches", []string{"beta"})

	// Unlock first: PICS never lists the branch, but the password opens
	// it, so unlock succeeds and the password is stored.
	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if unlocked["unlocked"] != true {
		t.Fatalf("unlock: %+v", unlocked)
	}
	// The list shows the hidden branch now that a password is stored:
	// unlocked, with the private section's build, not current.
	list := e.versionsList(map[string]any{"id": "4321"})
	beta := channelByID(t, list, "beta")
	if beta["requires"] != "password" || beta["unlocked"] != true {
		t.Fatalf("beta = %+v, want an unlocked password channel", beta)
	}
	if beta["buildId"] != "200" || beta["current"] != false {
		t.Fatalf("beta = %+v, want build 200, not current", beta)
	}
	// Install by channel: the target resolves through the private
	// section (build 200 comes from its branch metadata) and the
	// manifest downloads under that gid.
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "versions-hidden", "installDir": e.folder,
		"target": map[string]any{"channel": "beta"},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	entry := e.waitInstalled("4321", "200")
	if entry["channel"] != "beta" {
		t.Fatalf("installed = %+v", entry)
	}
	arch := entry["archiveRef"].(map[string]any)["depots"].(map[string]any)
	if arch["11"] != fmt.Sprint(e.fix.GID) {
		t.Fatalf("archiveRef = %+v, want depot 11 at the fixture gid", arch)
	}
	if log := e.waitCMLog("private_beta app=4321 branch=beta"); !strings.Contains(log, "private_beta app=4321 branch=beta") {
		t.Fatalf("no private-beta section fetch in CM log:\n%s", log)
	}
	// Installed on the hidden branch: it shows current, and no update
	// is offered against its own build.
	list = e.versionsList(map[string]any{"id": "4321"})
	if beta := channelByID(t, list, "beta"); beta["current"] != true {
		t.Fatalf("beta not current after install: %+v", beta)
	}
	if list["updateAvailable"] != false {
		t.Fatalf("updateAvailable on the hidden branch tip: %+v", beta)
	}
}

// A build with no channel resolves to its branch, locked or hidden: the
// private section supplies the branch the build number alone can't name.
func TestSteamVersionsInstallBuild(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", hiddenBranchAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))
	e.setCMState("beta_branches", []string{"beta"})

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	var size plugins.InstallSize
	if err := e.call("game.installSize", map[string]any{
		"id": "4321", "target": map[string]any{"build": "200"},
	}, &size); err != nil {
		t.Fatalf("installSize for build 200: %v", err)
	}
	if size.DiskBytes != int64(e.fix.DiskBytes) {
		t.Fatalf("size = %+v, want disk=%d", size, e.fix.DiskBytes)
	}
	if log := e.waitCMLog("private_beta app=4321 branch=beta"); !strings.Contains(log, "private_beta app=4321 branch=beta") {
		t.Fatalf("no private-beta section fetch in CM log:\n%s", log)
	}
}

// A build on a listed locked branch resolves through its private section
// too: the PICS listing names the branch but carries no plain gid.
func TestSteamVersionsInstallBuildListed(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	var size plugins.InstallSize
	if err := e.call("game.installSize", map[string]any{
		"id": "4321", "target": map[string]any{"build": "200"},
	}, &size); err != nil {
		t.Fatalf("installSize for build 200 on the listed locked branch: %v", err)
	}
	if size.DiskBytes != int64(e.fix.DiskBytes) {
		t.Fatalf("size = %+v, want disk=%d", size, e.fix.DiskBytes)
	}
}

// A branch key the section doesn't honour fails the install without
// dropping the stored password: only a refused password drops it.
func TestSteamVersionsKeyMismatchKeepsPassword(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", hiddenBranchAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))
	e.setCMState("beta_branches", []string{"beta"})
	e.setCMState("beta_key_beta", strings.Repeat("00", 20))

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	var size plugins.InstallSize
	if err := e.call("game.installSize", map[string]any{
		"id": "4321", "target": map[string]any{"channel": "beta"},
	}, &size); err == nil || !strings.Contains(err.Error(), "refused the beta section") {
		t.Fatalf("key mismatch err = %v, want the refused-section error", err)
	}
	raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "beta_passwords.json"))
	if !strings.Contains(string(raw), "beta-pw-7") {
		t.Fatalf("stored password dropped without a refusal: %s", raw)
	}
}

// A dropped CM connection is a transport error, not a refusal: the stored
// password survives it.
func TestSteamVersionsTransportFailureKeepsPassword(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", hiddenBranchAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))
	e.setCMState("beta_branches", []string{"beta"})

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	e.setCMState("drop_beta", true)
	var size plugins.InstallSize
	err := e.call("game.installSize", map[string]any{
		"id": "4321", "target": map[string]any{"channel": "beta"},
	}, &size)
	if err == nil || strings.Contains(err.Error(), "wrong branch password") {
		t.Fatalf("transport err = %v, want a non-password failure", err)
	}
	raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "beta_passwords.json"))
	if !strings.Contains(string(raw), "beta-pw-7") {
		t.Fatalf("stored password dropped on a transport error: %s", raw)
	}
}

// A private section without branch metadata still records the channel and
// the manifest pins, so a later plain update stays on the branch instead
// of falling back to public.
func TestSteamVersionsInstallNoBranchMeta(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", hiddenBranchAppinfo(e.fix.GID))
	e.setCMState("private_beta", `"privatedepots" { "11" { "manifests" { "beta" { "gid" "`+fmt.Sprint(e.fix.GID)+`" } } } }`)
	e.setCMState("beta_branches", []string{"beta"})

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "versions-nometa", "installDir": e.folder,
		"target": map[string]any{"channel": "beta"},
	}, nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
		var installed map[string]map[string]any
		if _ = json.Unmarshal(raw, &installed); installed["4321"]["channel"] == "beta" {
			if arch, _ := installed["4321"]["archiveRef"].(map[string]any); arch != nil {
				if depots, _ := arch["depots"].(map[string]any); depots["11"] == fmt.Sprint(e.fix.GID) {
					// The unknown tip compares to nothing: no update,
					// and never a fallback to public.
					list := e.versionsList(map[string]any{"id": "4321"})
					if list["updateAvailable"] != false {
						t.Fatalf("updateAvailable without a tip: %+v", list["updateAvailable"])
					}
					return
				}
			}
			t.Fatalf("installed without manifest pins: %+v", installed["4321"])
		}
		if time.Now().After(deadline) {
			t.Fatalf("no beta install recorded: %s", raw)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// A refused stored password is dropped by the list itself: the branch
// comes back locked, with no build id and no update against public.
func TestSteamVersionsListRefusedPassword(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", hiddenBranchAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))
	e.setCMState("beta_branches", []string{"beta"})

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	e.setCMState("beta_password_eresult", 5)
	list := e.versionsList(map[string]any{"id": "4321"})
	beta := channelByID(t, list, "beta")
	if beta["requires"] != "password" || beta["unlocked"] != false {
		t.Fatalf("beta = %+v, want a locked skeleton", beta)
	}
	if beta["buildId"] != "" {
		t.Fatalf("beta = %+v, want no build id", beta)
	}
	raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "beta_passwords.json"))
	if strings.Contains(string(raw), "beta-pw-7") {
		t.Fatalf("refused password kept by list: %s", raw)
	}
}

// A transport failure keeps the hidden branch listed as unlocked with no
// build id; an installed game on it reports no update, never public.
func TestSteamVersionsListTransportKeepsBranch(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", hiddenBranchAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))
	e.setCMState("beta_branches", []string{"beta"})

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "versions-transport-list", "installDir": e.folder,
		"target": map[string]any{"channel": "beta"},
	}, nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	e.waitInstalled("4321", "200")
	e.setCMState("drop_beta", true)
	list := e.versionsList(map[string]any{"id": "4321"})
	beta := channelByID(t, list, "beta")
	if beta["unlocked"] != true || beta["buildId"] != "" {
		t.Fatalf("beta = %+v, want unlocked with no build id", beta)
	}
	if list["updateAvailable"] != false {
		t.Fatalf("updateAvailable over a dead CM: %+v", list["updateAvailable"])
	}
	raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "beta_passwords.json"))
	if !strings.Contains(string(raw), "beta-pw-7") {
		t.Fatalf("stored password dropped on a transport error: %s", raw)
	}
}

func rpcCode(t *testing.T, err error) int {
	t.Helper()
	var re *jsonrpc.Error
	if !errors.As(err, &re) {
		t.Fatalf("not an RPC error: %v", err)
	}
	return re.Code
}

func (e *env) versionsList(params map[string]any) map[string]any {
	e.t.Helper()
	var out map[string]any
	if err := e.call("versions.list", params, &out); err != nil {
		e.t.Fatalf("versions.list: %v", err)
	}
	return out
}

func channelByID(t *testing.T, list map[string]any, id string) map[string]any {
	t.Helper()
	for _, c := range list["channels"].([]any) {
		if c.(map[string]any)["id"] == id {
			return c.(map[string]any)
		}
	}
	t.Fatalf("no channel %q in %+v", id, list["channels"])
	return nil
}

func TestSteamVersionsList(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))

	list := e.versionsList(map[string]any{
		"id":     "4321",
		"ledger": []any{map[string]any{"store": id, "game": "g", "build": "999", "date": "2024-05-01T00:00:00Z"}},
		"pinned": map[string]any{"kind": "channel", "id": "old"},
	})

	var ids []string
	for _, c := range list["channels"].([]any) {
		ids = append(ids, c.(map[string]any)["id"].(string))
	}
	if len(ids) != 3 || ids[0] != "public" {
		t.Fatalf("channels = %v, want public first of 3", ids)
	}
	public := channelByID(t, list, "public")
	if public["requires"] != "none" || public["unlocked"] != true || public["buildId"] != "100" {
		t.Fatalf("public = %+v", public)
	}
	if public["updatedAt"] != "2023-11-14" || public["description"] != "Default" {
		t.Fatalf("public metadata = %+v", public)
	}
	beta := channelByID(t, list, "beta")
	if beta["requires"] != "password" || beta["unlocked"] != false {
		t.Fatalf("beta = %+v, want a locked password channel", beta)
	}
	builds := map[string]map[string]any{}
	for _, b := range list["builds"].([]any) {
		bm := b.(map[string]any)
		builds[bm["id"].(string)] = bm
	}
	if builds["100"]["channel"] != "public" || builds["100"]["source"] != "store" {
		t.Fatalf("build 100 = %+v", builds["100"])
	}
	if builds["200"]["available"] != false {
		t.Fatalf("locked beta build must be unavailable: %+v", builds["200"])
	}
	arch, ok := builds["999"]
	if !ok || arch["source"] != "archive" || arch["available"] != true || arch["name"] != "build 999" {
		t.Fatalf("archive build = %+v", arch)
	}
	if arch["publishedAt"] != "2024-05-01" {
		t.Fatalf("archive publishedAt = %+v", arch)
	}
	manual := list["manual"].(map[string]any)
	if !strings.Contains(manual["hint"].(string), "11:") {
		t.Fatalf("manual hint doesn't explain the format: %+v", manual)
	}
	links := list["links"].([]any)
	if len(links) != 1 || !strings.HasSuffix(links[0].(map[string]any)["url"].(string), "/app/4321/depots/") {
		t.Fatalf("links = %+v, want the SteamDB depots page", links)
	}
	if !strings.Contains(links[0].(map[string]any)["url"].(string), "steamdb.info") {
		t.Fatalf("links must point at SteamDB, never fetched: %+v", links)
	}
	// Nothing installed: no current markers, no update.
	for _, c := range list["channels"].([]any) {
		if c.(map[string]any)["current"] == true {
			t.Fatalf("channel marked current with nothing installed: %+v", c)
		}
	}
	if list["updateAvailable"] == true {
		t.Fatalf("updateAvailable with nothing installed")
	}

	// Missing and bad ids are invalid params.
	if err := e.call("versions.list", map[string]any{}, nil); rpcCode(t, err) != jsonrpc.CodeInvalidParams {
		t.Fatalf("empty params err = %v", err)
	}
	if err := e.call("versions.list", map[string]any{"id": "nope"}, nil); rpcCode(t, err) != jsonrpc.CodeInvalidParams {
		t.Fatalf("non-numeric id err = %v", err)
	}
}

func TestSteamVersionsUnlock(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))

	var out map[string]any
	if err := e.call("versions.unlock", map[string]any{}, &out); rpcCode(t, err) != jsonrpc.CodeInvalidParams {
		t.Fatalf("empty params err = %v", err)
	}
	// A valid password that opens another branch names no branch here:
	// an error saying the branch wasn't recognised, not a silent false.
	// Only a listed passwordless branch refuses the password outright.
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "nope", "password": "x"}, &out); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("unknown channel: err=%v out=%+v, want the branch error", err, out)
	}
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "public", "password": "x"}, &out); err == nil {
		t.Fatal("passwordless channel took a password")
	}

	// A wrong password is {unlocked: false}, not an error.
	e.setCMState("beta_password_eresult", 5)
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "wrong"}, &out); err != nil {
		t.Fatalf("wrong password errored: %v", err)
	}
	if out["unlocked"] != false {
		t.Fatalf("wrong password: %+v", out)
	}

	e.setCMState("beta_password_eresult", 1)
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &out); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if out["unlocked"] != true {
		t.Fatalf("unlock: %+v", out)
	}
	// The password is kept in plugin-private state, 0600, never logged.
	raw, err := os.ReadFile(filepath.Join(e.pluginData(), "beta_passwords.json"))
	if err != nil {
		t.Fatalf("no unlock store: %v", err)
	}
	var stored map[string]map[string]string
	if err := json.Unmarshal(raw, &stored); err != nil || stored["4321"]["beta"] != "beta-pw-7" {
		t.Fatalf("unlock store = %s", raw)
	}
	if fi, _ := os.Stat(filepath.Join(e.pluginData(), "beta_passwords.json")); fi.Mode().Perm() != 0o600 {
		t.Fatalf("unlock store perms = %v, want 0600", fi.Mode())
	}
	logs, _ := filepath.Glob(filepath.Join(e.root, "logs", "*"))
	for _, l := range logs {
		if data, _ := os.ReadFile(l); strings.Contains(string(data), "beta-pw-7") {
			t.Fatalf("password leaked into log %s", l)
		}
	}

	list := e.versionsList(map[string]any{"id": "4321"})
	if beta := channelByID(t, list, "beta"); beta["unlocked"] != true {
		t.Fatalf("beta still locked after unlock: %+v", beta)
	}
	builds := map[string]map[string]any{}
	for _, b := range list["builds"].([]any) {
		bm := b.(map[string]any)
		builds[bm["id"].(string)] = bm
	}
	if builds["200"]["available"] != true {
		t.Fatalf("beta build still unavailable after unlock: %+v", builds["200"])
	}
}

// Installing from a locked branch sends the branch and the password-derived
// hash with the manifest request code; the install records channel, build
// and per-depot gids for the ledger.
func TestSteamVersionsInstallBranch(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "versions-branch", "installDir": e.folder,
		"target": map[string]any{"channel": "beta"},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	entry := e.waitInstalled("4321", "200")
	if entry["channel"] != "beta" {
		t.Fatalf("installed = %+v", entry)
	}
	arch := entry["archiveRef"].(map[string]any)["depots"].(map[string]any)
	if arch["11"] != fmt.Sprint(e.fix.GID) {
		t.Fatalf("archiveRef = %+v, want depot 11 at the fixture gid", arch)
	}
	log, _ := os.ReadFile(e.cmState + ".log")
	sum := sha1.Sum([]byte("beta-pw-7"))
	if want := fmt.Sprintf("request_code depot=11 branch=beta hash=%x", sum); !strings.Contains(string(log), want) {
		t.Fatalf("no password-derived request code in CM log:\n%s", log)
	}
	if !strings.Contains(string(log), "beta_password app=4321") {
		t.Fatalf("no CheckBetaPassword in CM log:\n%s", log)
	}
	// The locked gid comes from the private-beta section, unlocked with
	// the branch key — never from plain manifests (live shape).
	if !strings.Contains(string(log), "private_beta app=4321 branch=beta key=20B") {
		t.Fatalf("no private-beta section fetch in CM log:\n%s", log)
	}

	list := e.versionsList(map[string]any{"id": "4321"})
	if beta := channelByID(t, list, "beta"); beta["current"] != true {
		t.Fatalf("beta not current after install: %+v", beta)
	}
	// An unpinned install follows its channel: at the beta tip there is
	// no update, even though public sits at another build.
	if list["updateAvailable"] != false {
		t.Fatalf("updateAvailable at the beta tip: %+v", list)
	}

	// M3: a plain update (no target) stays on the installed channel,
	// never silently back to public.
	if err := os.WriteFile(e.cmState+".log", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "versions-stay", "installDir": e.folder,
	}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	e.waitCMLog("request_code depot=11 branch=beta")
	entry = e.waitInstalled("4321", "200")
	if entry["channel"] != "beta" {
		t.Fatalf("plain update left beta: %+v", entry)
	}
	list = e.versionsList(map[string]any{"id": "4321"})
	if list["updateAvailable"] != false {
		t.Fatalf("updateAvailable after staying on beta: %+v", list)
	}

	// Back to public: the same target road, no password needed.
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "versions-public", "installDir": e.folder,
		"target": map[string]any{"channel": "public"},
	}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	entry = e.waitInstalled("4321", "100")
	if entry["channel"] != "public" {
		t.Fatalf("installed = %+v", entry)
	}
	list = e.versionsList(map[string]any{"id": "4321", "pinned": map[string]any{"kind": "channel", "id": "public"}})
	if list["updateAvailable"] == true {
		t.Fatal("updateAvailable pinned at the installed channel")
	}
}

// A plain update of a locked-channel install without a stored password
// fails with the locked error instead of switching to public.
func TestSteamVersionsUpdateLockedWithoutPassword(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "versions-locked-branch", "installDir": e.folder,
		"target": map[string]any{"channel": "beta"},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	if entry := e.waitInstalled("4321", "200"); entry["channel"] != "beta" {
		t.Fatalf("installed = %+v", entry)
	}
	// Forget the password: the next plain update has nothing to unlock
	// with. The downloader refuses before any request code goes out, so
	// no request_code line may appear and the install stays on beta.
	if err := os.Remove(filepath.Join(e.pluginData(), "beta_passwords.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.cmState+".log", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "versions-locked-update", "installDir": e.folder,
	}, nil); err != nil {
		t.Fatalf("update start: %v", err)
	}
	e.waitCMLog("pics_info 4321")
	time.Sleep(5 * time.Second)
	raw, _ := os.ReadFile(e.cmState + ".log")
	if strings.Contains(string(raw), "request_code") {
		t.Fatalf("locked update downloaded instead of refusing:\n%s", raw)
	}
	var installed map[string]map[string]any
	raw, _ = os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
	if _ = json.Unmarshal(raw, &installed); installed["4321"]["channel"] != "beta" {
		t.Fatalf("locked update switched channels: %+v", installed["4321"])
	}
}

// A stored password Steam refuses fails the install distinctly and is
// dropped, so the unlock form comes back instead of failing every later
// install behind a stale secret.
func TestSteamVersionsRefusedPasswordDropped(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	// Steam now refuses the stored password (changed or revoked): even a
	// size check fails with the wrong-password error, not a transport one.
	e.setCMState("beta_password_eresult", 5)
	var size plugins.InstallSize
	if err := e.call("game.installSize", map[string]any{
		"id": "4321", "target": map[string]any{"channel": "beta"},
	}, &size); err == nil || !strings.Contains(err.Error(), "wrong branch password") {
		t.Fatalf("refused size err = %v, want the wrong-password error", err)
	}
	// The size check drops the refused secret on its own: read the store
	// right here, before any install runs.
	if raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "beta_passwords.json")); strings.Contains(string(raw), "beta-pw-7") {
		t.Fatalf("refused password kept by installSize: %s", raw)
	}
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "versions-refused", "installDir": e.folder,
		"target": map[string]any{"channel": "beta"},
	}, nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	// The failed job drops the refused password: poll the plugin store,
	// the way the unlock form reads it back.
	deadline := time.Now().Add(30 * time.Second)
	for {
		raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "beta_passwords.json"))
		if !strings.Contains(string(raw), "beta-pw-7") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("refused password kept: %s", raw)
		}
		time.Sleep(100 * time.Millisecond)
	}
	raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
	if strings.Contains(string(raw), `"channel":"beta"`) {
		t.Fatal("refused install recorded a beta install")
	}
	list := e.versionsList(map[string]any{"id": "4321"})
	if beta := channelByID(t, list, "beta"); beta["unlocked"] != false {
		t.Fatalf("beta still unlocked after drop: %+v", beta)
	}
}

// Shared depots (depotfromapp) resolve their manifest from the source app.
// The source here only carries public, so a beta install falls back to the
// source's public gid; a pinned manifest for the shared depot skips the
// source lookup entirely.
func sharedVersionsAppinfo(gid uint64) string {
	return `"depots"
{
"branches"
{
"public" { "buildid" "100" "description" "Default" }
"beta" { "buildid" "200" "description" "Beta" "pwdrequired" "1" }
}
"11"
{
"config" { "oslist" "linux" }
"manifests" { "public" { "gid" "` + fmt.Sprint(gid) + `" } }
}
"12"
{
"config" { "oslist" "linux" }
"depotfromapp" "999"
}
}
`
}

func sharedSourceAppinfo(gid uint64) string {
	return `"depots"
{
"12"
{
"config" { "oslist" "linux" }
"manifests" { "public" { "gid" "` + fmt.Sprint(gid) + `" } }
}
}
`
}

func TestSteamVersionsSharedDepot(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", sharedVersionsAppinfo(e.fix.GID))
	e.setCMState("appinfo_999", sharedSourceAppinfo(e.fix.GID))
	e.setCMState("private_beta", privateBetaSection(e.fix.GID))

	var unlocked map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "beta-pw-7"}, &unlocked); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "versions-shared", "installDir": e.folder,
		"target": map[string]any{"channel": "beta"},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	entry := e.waitInstalled("4321", "200")
	if entry["channel"] != "beta" {
		t.Fatalf("installed = %+v", entry)
	}
	arch := entry["archiveRef"].(map[string]any)["depots"].(map[string]any)
	if arch["11"] != fmt.Sprint(e.fix.GID) || arch["12"] != fmt.Sprint(e.fix.GID) {
		t.Fatalf("shared depot missed the source public gid: %+v", arch)
	}
	if log := e.waitCMLog("pics_info 999"); !strings.Contains(log, "pics_info 999") {
		t.Fatalf("no source-app lookup in CM log:\n%s", log)
	}

	// A pinned manifest for the shared depot skips the source app: point
	// the source at nothing and the update still lands.
	e.setCMState("appinfo_999", `"depots" { }`)
	if err := os.WriteFile(e.cmState+".log", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.call("game.update", map[string]any{
		"id": "4321", "jobId": "versions-shared-pinned", "installDir": e.folder,
		"target": map[string]any{"manual": map[string]string{"input": "11: " + fmt.Sprint(e.fix.GID) + "\n12: " + fmt.Sprint(e.fix.GID)}},
	}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	e.waitCMLog("request_code depot=12")
	log := e.cmLog()
	if strings.Contains(log, "pics_info 999") {
		t.Fatalf("pinned update asked the source app:\n%s", log)
	}
}

func TestSteamVersionsInstallPinned(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))

	var size plugins.InstallSize
	if err := e.call("game.installSize", map[string]any{
		"id": "4321", "target": map[string]any{"manual": map[string]string{"input": "11: " + fmt.Sprint(e.fix.GID)}},
	}, &size); err != nil {
		t.Fatalf("installSize with a pinned manifest: %v", err)
	}
	if size.DiskBytes != int64(e.fix.DiskBytes) {
		t.Fatalf("size = %+v, want disk=%d", size, e.fix.DiskBytes)
	}

	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "versions-pinned", "installDir": e.folder,
		"target": map[string]any{"manual": map[string]string{"input": "11: " + fmt.Sprint(e.fix.GID)}},
	}, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	entry := e.waitInstalledPrefix("4321", "manual-")
	arch := entry["archiveRef"].(map[string]any)["depots"].(map[string]any)
	if arch["11"] != fmt.Sprint(e.fix.GID) {
		t.Fatalf("archiveRef = %+v", arch)
	}
	// Core always passes the install history: the pinned build comes back
	// as an archive build, marked current.
	list := e.versionsList(map[string]any{"id": "4321", "ledger": []any{
		map[string]any{"store": id, "game": "g", "build": entry["build"],
			"channel": "public", "date": "2026-11-15T00:00:00Z"},
	}})
	found := false
	for _, b := range list["builds"].([]any) {
		if b.(map[string]any)["id"] == entry["build"] && b.(map[string]any)["current"] == true {
			found = true
		}
	}
	if !found {
		t.Fatalf("pinned build not current: %+v", list["builds"])
	}
}

// Bad targets fail synchronously on installSize (the same resolution the
// installs run): locked without a password, unknown branch, a stale build,
// a mistyped manifest, a depot outside the branch.
func TestSteamVersionsTargetErrors(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))

	cases := map[string]map[string]any{
		"locked without a password": {"channel": "beta"},
		"unknown branch":            {"channel": "nope"},
		"stale build":               {"channel": "old", "build": "51"},
		"unknown build":             {"build": "999"},
		"mistyped manifest":         {"manual": map[string]string{"input": "abc"}},
		"depot outside the branch":  {"manual": map[string]string{"input": "99: 123"}},
	}
	for name, target := range cases {
		var size plugins.InstallSize
		if err := e.call("game.installSize", map[string]any{"id": "4321", "target": target}, &size); err == nil {
			t.Fatalf("%s: installSize succeeded, want an error", name)
		}
	}
	// ... and a locked-channel install fails instead of installing public.
	if err := e.call("game.install", map[string]any{
		"id": "4321", "jobId": "versions-locked", "installDir": e.folder,
		"target": map[string]any{"channel": "beta"},
	}, nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(5 * time.Second)
	raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
	if strings.Contains(string(raw), `"channel":"beta"`) {
		t.Fatal("locked install went through as public")
	}
}

func TestSteamVersionsLoggedOut(t *testing.T) {
	e := setup(t)
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))
	if err := e.call("versions.list", map[string]any{"id": "4321"}, nil); rpcCode(t, err) != plugins.CodeAuthRequired {
		t.Fatalf("versions.list logged out err = %v", err)
	}
	var out map[string]any
	if err := e.call("versions.unlock", map[string]any{"id": "4321", "channel": "beta", "password": "x"}, &out); rpcCode(t, err) != plugins.CodeAuthRequired {
		t.Fatalf("versions.unlock logged out err = %v", err)
	}
}

// The target and the stored beta passwords reach platypus-steam-dl's stdin
func TestSteamVersionsTargetInRequest(t *testing.T) {
	got := pyRun(t, `
hs.refresh_token = lambda: "tok"
hs.beta_passwords = lambda: {"4321": {"beta": "pw"}}
reqs = []
hs.dl_run = lambda sub, req, *a, **k: reqs.append((sub, dict(req))) or {"os_used": "linux", "disk_bytes": 8, "download_bytes": 4}
hs.install_size({"id": "4321", "target": {"channel": "beta"}})
hs.install_size({"id": "4321", "branch": "old", "branchPassword": "pw2"})
print([(s, r.get("branch"), r.get("target"), r.get("branch_passwords")) for s, r in reqs])
`)
	want := `[('size', 'public', {'channel': 'beta'}, {'beta': 'pw'}), ('size', 'old', None, None)]`
	if got != want {
		t.Fatalf("requests = %s, want %s", got, want)
	}
}

// version_result names the ledger entry; job.done carries it but never the
// password.
func TestSteamVersionsInstalledEntry(t *testing.T) {
	got := pyRun(t, `
hs.refresh_token = lambda: "tok"
hs.dl_run = lambda *a, **k: {"os_used": "linux", "branch": "beta", "build": "200",
                             "version_name": "beta", "manifests": {"11": 112}}
hs.record_install = lambda app, d, o, v=None: v
hs.find_executable = lambda *a, **k: "game.sh"
hs.detect_mode = lambda d: "none"
hs.store_app = lambda app: {"name": "G"}
sent = []
hs.notify = lambda m, p: sent.append((m, p))
hs.follow_install("j1", "4321", "/tmp/g", "install", {"target": {"channel": "beta"}})
print(sent)
print(hs.load_state("beta_passwords.json", "missing"))
`)
	if !strings.Contains(got, `'build': '200'`) || !strings.Contains(got, `'channel': 'beta'`) {
		t.Fatalf("job.done missed the ledger entry: %s", got)
	}
	if !strings.Contains(got, `'archiveRef': {'depots': {'11': '112'}}`) {
		t.Fatalf("archiveRef missed the manifest gids: %s", got)
	}
	if strings.Contains(got, "pw") {
		t.Fatalf("password leaked: %s", got)
	}
}

func TestSteamVersionsCMFailureErrors(t *testing.T) {
	e := setup(t)
	e.login()
	// The fake CM exits when its state file disappears: the next CM read
	// must error, never answer an empty/default list. (The plugin process
	// keeps the endpoint it started with, so repointing the env can't do
	// this.)
	if err := os.Remove(e.cmState); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if err := e.call("versions.list", map[string]any{"id": "4321"}, nil); err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("versions.list succeeded with no CM")
		}
		time.Sleep(200 * time.Millisecond)
	}
	var size plugins.InstallSize
	if err := e.call("game.installSize", map[string]any{
		"id": "4321", "target": map[string]any{"channel": "beta"},
	}, &size); err == nil {
		t.Fatal("installSize succeeded with no CM")
	}
}

// waitInstalled polls the plugin's installed.json until the app records the
// build (game.install answers once started; the download finishes async).
func (e *env) waitInstalled(app, build string) map[string]any {
	e.t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
		var installed map[string]map[string]any
		if _ = json.Unmarshal(raw, &installed); installed[app]["build"] == build {
			return installed[app]
		}
		time.Sleep(100 * time.Millisecond)
	}
	e.t.Fatalf("app %s never recorded build %s", app, build)
	return nil
}

func (e *env) waitInstalledPrefix(app, prefix string) map[string]any {
	e.t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
		var installed map[string]map[string]any
		if _ = json.Unmarshal(raw, &installed); installed[app] != nil {
			if b, _ := installed[app]["build"].(string); strings.HasPrefix(b, prefix) {
				return installed[app]
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	e.t.Fatalf("app %s never recorded a %q build", app, prefix)
	return nil
}
