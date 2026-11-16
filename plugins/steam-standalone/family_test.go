// Family (Steam Families) through the plugin's real JSON-RPC surface:
// members, shared/owned/multi-copy library entries, excluded apps,
// setPreferredCopy validation and request shape, the family-failure
// fallback and the logged-out paths. No Valve network: the FamilyGroups
// Web API answers from PLATYPUS_STEAM_API_FILE routes and member names
// from the fake CM.
package steam_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// installFakeExe records an installed game with a real (empty) executable,
// enough for game.launchSpec to resolve without launching anything.
func (e *env) installFakeExe(t *testing.T, appID string) {
	t.Helper()
	exe := filepath.Join(e.root, "exe-"+appID)
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := e.pluginData()
	var installed map[string]map[string]any
	if data, err := os.ReadFile(filepath.Join(dir, "installed.json")); err == nil {
		json.Unmarshal(data, &installed)
	}
	if installed == nil {
		installed = map[string]map[string]any{}
	}
	installed[appID] = map[string]any{"path": e.root, "executable": exe, "mode": "none"}
	data, _ := json.Marshal(installed)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "installed.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// familyFixtures rewrites the Web API hook file: the owned games stay the
// fixture's, and _routes answer the FamilyGroups endpoints. personas maps
// fake member steamids to display names for the fake CM.
func (e *env) familyFixtures(t *testing.T, group, apps, preferred any) {
	t.Helper()
	api := os.Getenv("PLATYPUS_STEAM_API_FILE")
	data, err := os.ReadFile(api)
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	file["_routes"] = map[string]any{
		"FamilyGroupsService/GetFamilyGroupForUser/v1": group,
		"FamilyGroupsService/GetSharedLibraryApps/v1":  apps,
		"FamilyGroupsService/GetPreferredLenders/v1":   preferred,
		"FamilyGroupsService/SetPreferredLender/v1":    map[string]any{"response": map[string]any{}},
	}
	out, _ := json.Marshal(file)
	if err := os.WriteFile(api, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// familyFixturesData is the shared-apps shape the tests read: the user
// 76561198 plus members 76561199 (adult) and 76561200 (child), one
// single-owner game, one multi-owner game with 76561199 preferred, one
// withheld game, one non-game, and owned 4321 also lent by a member.
func familyFixturesData() (group, apps, preferred any) {
	group = map[string]any{"response": map[string]any{
		"family_groupid":             float64(424242),
		"is_not_member_of_any_group": false,
		"family_group": map[string]any{
			"name": "TestFam",
			"members": []any{
				map[string]any{"steamid": "76561198", "role": float64(1)},
				map[string]any{"steamid": "76561199", "role": float64(1)},
				map[string]any{"steamid": "76561200", "role": float64(2)},
			},
		},
	}}
	apps = map[string]any{"response": map[string]any{"apps": []any{
		map[string]any{"appid": float64(5001), "name": "Shared Single",
			"owner_steamids": []any{"76561199"}, "exclude_reason": float64(0), "app_type": float64(1)},
		map[string]any{"appid": float64(5002), "name": "Shared Both",
			"owner_steamids": []any{"76561199", "76561200"}, "exclude_reason": float64(0), "app_type": float64(1)},
		map[string]any{"appid": float64(5003), "name": "Shared Withheld",
			"owner_steamids": []any{"76561200"}, "exclude_reason": float64(1), "app_type": float64(1)},
		map[string]any{"appid": float64(5004), "name": "Shared Tool",
			"owner_steamids": []any{"76561199"}, "exclude_reason": float64(0), "app_type": float64(4)},
		map[string]any{"appid": float64(4321), "name": "Standalone Game",
			"owner_steamids": []any{"76561200"}, "exclude_reason": float64(0), "app_type": float64(1)},
	}}}
	preferred = map[string]any{"response": map[string]any{"members": []any{
		map[string]any{"steamid": "76561199", "preferred_appids": []any{float64(5002)}},
	}}}
	return group, apps, preferred
}

// familyEnv is a logged-in env with the family fixtures in place.
func familyEnv(t *testing.T) *env {
	t.Helper()
	e := setup(t)
	group, apps, preferred := familyFixturesData()
	e.familyFixtures(t, group, apps, preferred)
	e.login()
	// After the login: the login's own persona check expects the fake CM
	// default, while the family lookups below resolve every member name
	// through these per-id personas.
	e.setCMState("personas", map[string]any{
		"76561198": "SelfMember",
		"76561199": "MemberA",
		"76561200": "MemberB",
	})
	return e
}

func (e *env) rawCall(method string, params any) json.RawMessage {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var raw json.RawMessage
	if err := e.m.Call(ctx, id, method, params, &raw); err != nil {
		e.t.Fatalf("%s: %v", method, err)
	}
	return raw
}

func (e *env) rawCallErr(method string, params any) error {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var raw json.RawMessage
	return e.m.Call(ctx, id, method, params, &raw)
}

func TestSteamFamilyMembers(t *testing.T) {
	e := setup(t)
	if err := e.rawCallErr("family.members", map[string]any{}); e.rpcCode(err) != -32001 {
		t.Fatalf("logged out family.members = %v, want -32001", err)
	}
	e = familyEnv(t)
	var members []map[string]any
	if err := json.Unmarshal(e.rawCall("family.members", map[string]any{}), &members); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, m := range members {
		byID[m["id"].(string)] = m
	}
	for id, want := range map[string][2]string{
		"76561198": {"SelfMember", "adult"},
		"76561199": {"MemberA", "adult"},
		"76561200": {"MemberB", "child"},
	} {
		m, ok := byID[id]
		if !ok {
			t.Fatalf("no member %s in %v", id, members)
		}
		if m["name"] != want[0] || m["role"] != want[1] {
			t.Fatalf("member %s = %v, want name %q role %q", id, m, want[0], want[1])
		}
	}
}

func TestSteamFamilyNoGroup(t *testing.T) {
	e := setup(t)
	e.familyFixtures(t,
		map[string]any{"response": map[string]any{
			"family_groupid":             float64(0),
			"is_not_member_of_any_group": true,
		}},
		map[string]any{"response": map[string]any{"apps": []any{}}},
		map[string]any{"response": map[string]any{"members": []any{}}})
	e.login()
	var members []map[string]any
	if err := json.Unmarshal(e.rawCall("family.members", map[string]any{}), &members); err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 {
		t.Fatalf("members = %v without a family, want []", members)
	}
	var lib struct {
		Games []map[string]any `json:"games"`
	}
	if err := json.Unmarshal(e.rawCall("library.list", map[string]any{}), &lib); err != nil {
		t.Fatal(err)
	}
	if len(lib.Games) != 2 {
		t.Fatalf("library = %d games without a family, want the 2 owned", len(lib.Games))
	}
}

func libraryByID(t *testing.T, e *env) map[string]map[string]any {
	t.Helper()
	var lib struct {
		Games []map[string]any `json:"games"`
	}
	if err := json.Unmarshal(e.rawCall("library.list", map[string]any{}), &lib); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, g := range lib.Games {
		byID[g["id"].(string)] = g
	}
	return byID
}

func TestSteamFamilyLibraryEntries(t *testing.T) {
	e := familyEnv(t)
	byID := libraryByID(t, e)

	owned := byID["4321"]
	if _, ok := owned["sharedBy"]; ok {
		t.Fatalf("owned 4321 carries sharedBy: %v", owned)
	}
	if _, ok := owned["copies"]; ok {
		t.Fatalf("owned 4321 carries copies: %v", owned)
	}

	single := byID["5001"]
	if single == nil {
		t.Fatal("no shared game 5001 in the library")
	}
	if shared, _ := single["sharedBy"].(map[string]any); shared["id"] != "76561199" || shared["name"] != "MemberA" {
		t.Fatalf("5001 sharedBy = %v, want MemberA/76561199", single["sharedBy"])
	}
	if _, ok := single["copies"]; ok {
		t.Fatalf("single-owner 5001 carries copies: %v", single)
	}

	both := byID["5002"]
	if both == nil {
		t.Fatal("no shared game 5002 in the library")
	}
	if shared, _ := both["sharedBy"].(map[string]any); shared["id"] != "76561199" {
		t.Fatalf("5002 sharedBy = %v, want the preferred lender 76561199", both["sharedBy"])
	}
	copies, _ := both["copies"].([]any)
	if len(copies) != 2 {
		t.Fatalf("5002 copies = %v, want one per owner", both["copies"])
	}
	for _, c := range copies {
		cm := c.(map[string]any)
		if cm["ownerId"] == nil || cm["available"] != true || cm["inUse"] != false {
			t.Fatalf("5002 copy = %v, want ownerId, available true, inUse false", cm)
		}
		if _, ok := cm["exclude"]; ok {
			t.Fatalf("included 5002 copy carries exclude: %v", cm)
		}
	}
	if both["preferredCopy"] != "76561199" {
		t.Fatalf("5002 preferredCopy = %v, want 76561199", both["preferredCopy"])
	}

	withheld := byID["5003"]
	if withheld == nil {
		t.Fatal("no shared game 5003 in the library")
	}
	if withheld["installable"] != false {
		t.Fatalf("excluded 5003 installable = %v, want false", withheld["installable"])
	}
	if withheld["note"] != "The publisher doesn't allow family sharing" {
		t.Fatalf("5003 note = %q, want the publisher exclusion text", withheld["note"])
	}
	wcopies, _ := withheld["copies"].([]any)
	if len(wcopies) != 1 {
		t.Fatalf("5003 copies = %v, want the withheld copy listed", withheld["copies"])
	}
	wc := wcopies[0].(map[string]any)
	if wc["available"] != false || wc["exclude"] == nil || wc["exclude"] == "" {
		t.Fatalf("5003 copy = %v, want available false with an exclude reason", wc)
	}

	tool := byID["5004"]
	if tool == nil {
		t.Fatal("no shared game 5004 in the library")
	}
	if tool["installable"] != false {
		t.Fatalf("shared non-game 5004 installable = %v, want false", tool["installable"])
	}
}

func TestSteamFamilySetPreferredCopy(t *testing.T) {
	e := setup(t)
	if err := e.rawCallErr("family.setPreferredCopy",
		map[string]any{"id": "5002", "ownerId": "76561199"}); e.rpcCode(err) != -32001 {
		t.Fatalf("logged out setPreferredCopy = %v, want -32001", err)
	}
	e = familyEnv(t)
	for name, tc := range map[string]struct {
		params map[string]any
		code   int
	}{
		"empty owner":   {map[string]any{"id": "5002", "ownerId": ""}, -32602},
		"unknown app":   {map[string]any{"id": "9999", "ownerId": "76561199"}, -32602},
		"unknown owner": {map[string]any{"id": "5002", "ownerId": "76561198"}, -32602},
		"owned game":    {map[string]any{"id": "4321", "ownerId": "76561200"}, -32602},
		"withheld copy": {map[string]any{"id": "5003", "ownerId": "76561200"}, -32002},
	} {
		if err := e.rawCallErr("family.setPreferredCopy", tc.params); e.rpcCode(err) != tc.code {
			t.Fatalf("%s: = %v, want %d", name, err, tc.code)
		}
	}

	os.Remove(os.Getenv("PLATYPUS_STEAM_API_FILE") + ".posted")
	e.rawCall("family.setPreferredCopy", map[string]any{"id": "5002", "ownerId": "76561200"})
	posted, err := os.ReadFile(os.Getenv("PLATYPUS_STEAM_API_FILE") + ".posted")
	if err != nil {
		t.Fatal(err)
	}
	var calls []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(posted)), "\n") {
		var c map[string]any
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, c)
	}
	if len(calls) != 1 {
		t.Fatalf("posted = %q, want the one SetPreferredLender call", posted)
	}
	call := calls[0]
	if !strings.Contains(call["url"].(string), "FamilyGroupsService/SetPreferredLender/v1") {
		t.Fatalf("posted url = %v, want SetPreferredLender", call["url"])
	}
	params, _ := call["params"].(map[string]any)
	if params["family_groupid"] != "424242" || params["appid"] != "5002" || params["lender_steamid"] != "76561200" {
		t.Fatalf("posted params = %v, want group 424242 app 5002 lender 76561200", params)
	}

	// The pick is stored server-side: the next listing reads it back.
	group, apps, _ := familyFixturesData()
	e.familyFixtures(t, group, apps,
		map[string]any{"response": map[string]any{"members": []any{
			map[string]any{"steamid": "76561200", "preferred_appids": []any{float64(5002)}},
		}}})
	byID := libraryByID(t, e)
	if byID["5002"]["preferredCopy"] != "76561200" {
		t.Fatalf("5002 preferredCopy = %v after the pick, want 76561200", byID["5002"]["preferredCopy"])
	}
	if shared, _ := byID["5002"]["sharedBy"].(map[string]any); shared["id"] != "76561200" {
		t.Fatalf("5002 sharedBy = %v after the pick, want lender 76561200", byID["5002"]["sharedBy"])
	}
}

func TestSteamFamilyFailureListsOwned(t *testing.T) {
	e := setup(t)
	// GetFamilyGroupForUser answers a list: the lookup fails, so the
	// listing must still return the user's own games, without sharedBy.
	e.familyFixtures(t,
		[]any{float64(1)},
		map[string]any{"response": map[string]any{"apps": []any{}}},
		map[string]any{"response": map[string]any{"members": []any{}}})
	e.login()
	byID := libraryByID(t, e)
	if len(byID) != 2 || byID["4321"] == nil || byID["4322"] == nil {
		t.Fatalf("library = %v on family failure, want the 2 owned games", byID)
	}
	for id, g := range byID {
		if _, ok := g["sharedBy"]; ok {
			t.Fatalf("game %s carries sharedBy on family failure: %v", id, g)
		}
	}
}

func TestSteamFamilyHangingCM(t *testing.T) {
	// The CM accepts the names lookup and never answers (hang_link_details):
	// library.list must still return within the family budget with the
	// shared games, whose sharedBy carries the lender id and no name. (The
	// plugin is a persistent stdio process, so the CM endpoint its
	// environment names is fixed at setup; the fake CM hangs instead.)
	e := familyEnv(t)
	e.setCMState("hang_link_details", true)
	start := time.Now()
	byID := libraryByID(t, e)
	if el := time.Since(start); el >= 15*time.Second {
		t.Fatalf("library.list with a silent CM took %v, want < 15s", el)
	}
	for _, id := range []string{"5001", "5002", "5003"} {
		g := byID[id]
		if g == nil {
			t.Fatalf("no shared game %s with a silent CM", id)
		}
		shared, _ := g["sharedBy"].(map[string]any)
		if shared == nil || shared["id"] == nil || shared["id"] == "" {
			t.Fatalf("game %s sharedBy = %v, want the lender id", id, g["sharedBy"])
		}
		if _, ok := shared["name"]; ok {
			t.Fatalf("game %s sharedBy carries a name with a silent CM: %v", id, shared)
		}
	}
}

func TestSteamFamilyFailureServesLastGood(t *testing.T) {
	// List once, store a pick (which invalidates the snapshot), then break
	// the group lookup: the listing must still serve the shared games from
	// the last good snapshot instead of pruning them.
	e := familyEnv(t)
	if byID := libraryByID(t, e); byID["5001"] == nil || byID["5002"] == nil || byID["5003"] == nil {
		t.Fatalf("no shared games in the first listing: %v", byID)
	}
	e.rawCall("family.setPreferredCopy", map[string]any{"id": "5002", "ownerId": "76561200"})
	_, apps, preferred := familyFixturesData()
	e.familyFixtures(t,
		[]any{float64(1)},
		apps,
		preferred)
	byID := libraryByID(t, e)
	for _, id := range []string{"5001", "5002", "5003"} {
		if byID[id] == nil {
			t.Fatalf("no shared game %s after the family fetch failed", id)
		}
	}
}

// Borrowed copies someone else is playing carry inUse: the helper's CM
// read is real, with the fake answering GetFamilyGroup and pushing
// NotifyRunningApps after the reply (the real ordering). Two pairs share
// one appid, riding one push entry.
func TestSteamFamilyInUse(t *testing.T) {
	e := familyEnv(t)
	e.setCMState("family_group_id", 424242)
	e.setCMState("family_push_after_reply", true)
	// MemberB plays shared 5002 from MemberA's copy (the preferred one)
	// while MemberA plays it from MemberB's; MemberB also plays
	// single-owner 5001 from MemberA's only copy; the user plays
	// withheld 5003 themselves, which is not "in use".
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5002", "player": "76561200", "owner": "76561199"},
		map[string]any{"appid": "5002", "player": "76561199", "owner": "76561200"},
		map[string]any{"appid": "5001", "player": "76561200", "owner": "76561199"},
		map[string]any{"appid": "5003", "player": "76561198", "owner": "76561200"},
	})
	byID := libraryByID(t, e)
	// The running-apps read went over the CM: the group lookup and the
	// push the helper drains for. The plugin passes its known group id,
	// so the helper skips its own ForUser lookup.
	e.waitCMLog("service FamilyGroups.GetFamilyGroup#1")
	e.waitCMLog("push 146 FamilyGroupsClient.NotifyRunningApps#1")
	if strings.Contains(e.cmLog(), "GetFamilyGroupForUser") {
		t.Fatal("the helper looked up the group id the plugin already knew")
	}

	both := byID["5002"]
	if both == nil {
		t.Fatal("no shared game 5002 in the library")
	}
	copies, _ := both["copies"].([]any)
	if len(copies) != 2 {
		t.Fatalf("5002 copies = %v, want one per owner", both["copies"])
	}
	byOwner := map[string]map[string]any{}
	for _, c := range copies {
		cm := c.(map[string]any)
		byOwner[cm["ownerId"].(string)] = cm
	}
	if byOwner["76561199"]["inUse"] != true {
		t.Fatalf("preferred copy 76561199 = %v, want inUse true", byOwner["76561199"])
	}
	if byOwner["76561199"]["ownerName"] != "MemberA" {
		t.Fatalf("preferred copy 76561199 = %v, want ownerName MemberA", byOwner["76561199"])
	}
	if byOwner["76561200"]["inUse"] != true {
		t.Fatalf("second copy 76561200 = %v, want inUse true: MemberA plays from it", byOwner["76561200"])
	}

	single := byID["5001"]
	if single == nil {
		t.Fatal("no shared game 5001 in the library")
	}
	scopies, _ := single["copies"].([]any)
	if len(scopies) != 1 {
		t.Fatalf("5001 copies = %v, want the one in-use copy listed", single["copies"])
	}
	sc := scopies[0].(map[string]any)
	if sc["ownerId"] != "76561199" || sc["inUse"] != true || sc["available"] != true {
		t.Fatalf("5001 copy = %v, want MemberA's copy inUse", sc)
	}
	if _, ok := single["preferredCopy"]; ok {
		t.Fatalf("5001 preferredCopy = %v, want none without a stored pick", single["preferredCopy"])
	}

	withheld := byID["5003"]
	wcopies, _ := withheld["copies"].([]any)
	if len(wcopies) != 1 {
		t.Fatalf("5003 copies = %v, want the withheld copy listed", withheld["copies"])
	}
	if wcopies[0].(map[string]any)["inUse"] != false {
		t.Fatalf("5003 copy = %v, want inUse false: the user is the player", wcopies[0])
	}
}

// Steam pushes nothing when nothing is running: copies stay inUse false
// and no copies appear for single-owner games. The group lookup still
// goes over the CM (only the push is missing).
func TestSteamFamilyNoPushLeavesInUseFalse(t *testing.T) {
	e := familyEnv(t)
	e.setCMState("family_group_id", 424242)
	byID := libraryByID(t, e)
	e.waitCMLog("service FamilyGroups.GetFamilyGroup#1")

	both := byID["5002"]
	copies, _ := both["copies"].([]any)
	if len(copies) != 2 {
		t.Fatalf("5002 copies = %v, want one per owner", both["copies"])
	}
	for _, c := range copies {
		if c.(map[string]any)["inUse"] != false {
			t.Fatalf("5002 copy = %v, want inUse false with no push", c)
		}
	}
	if _, ok := byID["5001"]["copies"]; ok {
		t.Fatalf("single-owner 5001 carries copies with no push: %v", byID["5001"])
	}
}

// game.launchSpec refuses with -32002 naming the player only when every
// copy of a borrowed game is in use; a free copy launches. The refusal
// reads the cached snapshot and borrowed set the listing merged — never
// its own fetch — so the listing runs first to warm the cache.
func TestSteamFamilyLaunchSpecRefusal(t *testing.T) {
	e := familyEnv(t)
	e.setCMState("family_group_id", 424242)
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5002", "player": "76561200", "owner": "76561199"},
		map[string]any{"appid": "5001", "player": "76561200", "owner": "76561199"},
		map[string]any{"appid": "4321", "player": "76561200", "owner": "76561200"},
	})
	for _, app := range []string{"5002", "5001", "4321"} {
		e.installFakeExe(t, app)
	}
	libraryByID(t, e)
	calls := func() int {
		return strings.Count(e.cmLog(), "service FamilyGroups.GetFamilyGroup#1")
	}
	e.waitCMLog("service FamilyGroups.GetFamilyGroup#1")
	if n := calls(); n != 1 {
		t.Fatalf("GetFamilyGroup calls after listing = %d, want 1", n)
	}
	var spec map[string]any
	// 5002's preferred copy is busy, but MemberB's own copy is free.
	if err := e.call("game.launchSpec", map[string]any{"id": "5002"}, &spec); err != nil {
		t.Fatalf("launchSpec for 5002 with a free copy: %v", err)
	}
	// 5001's only copy is busy: refusal naming the player, decided from
	// the fresh cache with no re-read.
	err := e.rawCallErr("game.launchSpec", map[string]any{"id": "5001"})
	if e.rpcCode(err) != -32002 {
		t.Fatalf("launchSpec for in-use 5001 = %v, want -32002", err)
	}
	if !strings.Contains(err.Error(), "MemberB") {
		t.Fatalf("launchSpec for in-use 5001 = %v, want the player named", err)
	}
	// Owned 4321 is also lent by the playing member, but it isn't
	// borrowed: no refusal.
	if err := e.call("game.launchSpec", map[string]any{"id": "4321"}, &spec); err != nil {
		t.Fatalf("launchSpec for owned 4321: %v", err)
	}
	time.Sleep(time.Second) // the CM log lands async: no re-read means none
	if n := calls(); n != 1 {
		t.Fatalf("GetFamilyGroup calls after fresh launches = %d, want 1", n)
	}
}

// The CM rejects the running read: the listing still serves the shared
// games, with every copy inUse false.
func TestSteamFamilyRunningReadFailure(t *testing.T) {
	e := familyEnv(t)
	// Fail every service reply: the link-details names and the helper's
	// GetFamilyGroup read go down together (the login's cached access
	// token still opens the Web API).
	e.setCMState("service_eresult", 2)
	byID := libraryByID(t, e)
	if byID["5001"] == nil || byID["5002"] == nil {
		t.Fatalf("no shared games when the running read fails: %v", byID)
	}
	if _, ok := byID["5001"]["copies"]; ok {
		t.Fatalf("single-owner 5001 carries copies on a failed read: %v", byID["5001"])
	}
	for _, c := range byID["5002"]["copies"].([]any) {
		if c.(map[string]any)["inUse"] != false {
			t.Fatalf("5002 copy = %v, want inUse false on a failed read", c)
		}
	}
}

// The helper binary is missing: the listing still serves the shared games,
// with every copy inUse false and no FamilyGroups call on the wire.
func TestSteamFamilyHelperMissing(t *testing.T) {
	e := familyEnv(t)
	bin := filepath.Join(e.root, "plugins", id, "bin", "platypus-steam-dl")
	if err := os.Rename(bin, bin+".bak"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", e.root) // nothing resolvable: no PATH fallback either
	byID := libraryByID(t, e)
	if byID["5001"] == nil || byID["5002"] == nil {
		t.Fatalf("no shared games with the helper missing: %v", byID)
	}
	for _, c := range byID["5002"]["copies"].([]any) {
		if c.(map[string]any)["inUse"] != false {
			t.Fatalf("5002 copy = %v, want inUse false with no helper", c)
		}
	}
	if strings.Contains(e.cmLog(), "FamilyGroups.GetFamilyGroup") {
		t.Fatal("the missing helper still reached the CM")
	}
}

// The user playing their only borrowed copy launches: their own play is
// neither in use nor a refusal.
func TestSteamFamilyOwnPlayLaunches(t *testing.T) {
	e := familyEnv(t)
	e.setCMState("family_group_id", 424242)
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5001", "player": "76561198", "owner": "76561199"},
	})
	e.installFakeExe(t, "5001")
	byID := libraryByID(t, e)
	if _, ok := byID["5001"]["copies"]; ok {
		t.Fatalf("5001 carries copies for the user's own play: %v", byID["5001"])
	}
	var spec map[string]any
	if err := e.call("game.launchSpec", map[string]any{"id": "5001"}, &spec); err != nil {
		t.Fatalf("launchSpec for the user's own play: %v", err)
	}
}

// Stale running data triggers one bounded re-read per launch: with every
// copy still busy the refusal stands, and once the borrower quits the
// re-read finds a free copy and the launch goes. FRESH=0 ages every read
// instantly; it is set before setup so the plugin process inherits it.
func TestSteamFamilyStaleRunningRefetches(t *testing.T) {
	t.Setenv("PLATYPUS_STEAM_RUNNING_FRESH", "0")
	e := familyEnv(t)
	e.setCMState("family_group_id", 424242)
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5001", "player": "76561200", "owner": "76561199"},
	})
	e.installFakeExe(t, "5001")
	libraryByID(t, e)
	calls := func() int {
		return strings.Count(e.cmLog(), "service FamilyGroups.GetFamilyGroup#1")
	}
	e.waitCMLog("service FamilyGroups.GetFamilyGroup#1")
	if n := calls(); n != 1 {
		t.Fatalf("GetFamilyGroup calls after listing = %d, want 1", n)
	}
	waitCalls := func(want int) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			if calls() >= want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("GetFamilyGroup calls = %d, want %d", calls(), want)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	// Every copy still busy: the re-read confirms, refusal stands.
	err := e.rawCallErr("game.launchSpec", map[string]any{"id": "5001"})
	if e.rpcCode(err) != -32002 {
		t.Fatalf("launchSpec for in-use 5001 = %v, want -32002", err)
	}
	waitCalls(2)
	// The borrower quit: the re-read finds a free copy, launch goes.
	e.setCMState("family_running", []any{})
	var spec map[string]any
	if err := e.call("game.launchSpec", map[string]any{"id": "5001"}, &spec); err != nil {
		t.Fatalf("launchSpec after the borrower quit: %v", err)
	}
	waitCalls(3)
}

// The owner taking a borrowed game back warns once (issue #178, part 2):
// the social session's NotifyRunningApps push carries the running list,
// and when the owner of the copy the user plays starts the same app the
// plugin sends family.warning once. Repeats stay silent, unrelated starts
// don't warn, and with the social session off nothing warns. The warning
// re-arms once the owner stops, and a first push that already shows the
// take-back warns too.
func TestSteamFamilyTakebackWarning(t *testing.T) {
	e := familyEnv(t)
	e.installFakeExe(t, "5001")
	e.installFakeExe(t, "5003")
	// Fast social syncs: the friends list and personas arrive on logon,
	// so social.start never sits out waitReady.
	for knob, value := range socialKnobs() {
		e.setCMState(knob, value)
	}
	libraryByID(t, e)
	e.setCMState("family_group_id", 424242)
	pushes := func() int {
		return strings.Count(e.cmLog(), "push 146 FamilyGroupsClient.NotifyRunningApps#1")
	}
	// A restart delivers exactly one new push: wait for it, then give the
	// plugin a bounded window to act on it.
	restart := func() {
		t.Helper()
		before := pushes()
		if err := e.call("social.stop", map[string]any{}, nil); err != nil {
			t.Fatalf("social.stop: %v", err)
		}
		if err := e.call("social.start", map[string]any{}, nil); err != nil {
			t.Fatalf("social.start: %v", err)
		}
		deadline := time.Now().Add(15 * time.Second)
		for pushes() <= before {
			if time.Now().After(deadline) {
				t.Fatalf("no new running-apps push: %s", e.cmLog())
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(2 * time.Second)
	}
	warnings := func() []json.RawMessage { return e.notes.forMethod("family.warning") }
	waitWarnings := func(n int) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for len(warnings()) < n {
			if time.Now().After(deadline) {
				t.Fatalf("family.warning count = %d, want %d", len(warnings()), n)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	borrowing := []any{
		map[string]any{"appid": "5001", "player": "76561198", "owner": "76561199"},
		map[string]any{"appid": "5003", "player": "76561200", "owner": "76561200"},
	}
	e.setCMState("family_running", borrowing)
	if err := e.call("social.start", map[string]any{}, nil); err != nil {
		t.Fatalf("social.start: %v", err)
	}
	// The subscribe's wire shape: steamid as a uint64 varint, like the
	// group id (the fake answers no group on a bad shape).
	e.waitCMLog("family foruser ok")
	// Gate: the push is applied once the refreshed in-use list refuses
	// 5003, naming the player. Nothing warns: no owner plays the user's game.
	deadline := time.Now().Add(15 * time.Second)
	for {
		err := e.rawCallErr("game.launchSpec", map[string]any{"id": "5003"})
		if err != nil && e.rpcCode(err) == -32002 && strings.Contains(err.Error(), "MemberB") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("push not applied: launchSpec 5003 = %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if n := len(warnings()); n != 0 {
		t.Fatalf("borrowing alone warned: %d warnings", n)
	}
	// With the social session off, the take-back warns nothing.
	if err := e.call("social.stop", map[string]any{}, nil); err != nil {
		t.Fatalf("social.stop: %v", err)
	}
	takeback := append(append([]any{}, borrowing...),
		map[string]any{"appid": "5001", "player": "76561199", "owner": "76561199"})
	e.setCMState("family_running", takeback)
	time.Sleep(2 * time.Second)
	if n := len(warnings()); n != 0 {
		t.Fatalf("family.warning with social off = %d, want 0", n)
	}
	// The owner starts the copy the user plays: exactly one warning,
	// naming the owner, with no deadline (Steam gives none).
	if err := e.call("social.start", map[string]any{}, nil); err != nil {
		t.Fatalf("social.start again: %v", err)
	}
	waitWarnings(1)
	var w struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}
	raw := warnings()[len(warnings())-1]
	if err := json.Unmarshal(raw, &w); err != nil {
		t.Fatalf("family.warning params: %v", err)
	}
	if w.ID != "5001" || !strings.Contains(w.Message, "MemberA is taking this game back") {
		t.Fatalf("family.warning = %+v, want id 5001 taking back naming MemberA", w)
	}
	if strings.Contains(string(raw), "secondsLeft") {
		t.Fatalf("family.warning carries secondsLeft: %s", raw)
	}
	// The same push again: no repeat.
	e.setCMState("family_running", takeback)
	restart()
	if n := len(warnings()); n != 1 {
		t.Fatalf("repeated push warned again: count = %d", n)
	}
	// An unrelated start (a copy the user doesn't play) warns nothing.
	unrelated := append(append([]any{}, takeback...),
		map[string]any{"appid": "5002", "player": "76561199", "owner": "76561199"})
	e.setCMState("family_running", unrelated)
	restart()
	if got := warnings(); len(got) != 1 {
		t.Fatalf("unrelated start warned: %d warnings", len(got))
	} else if strings.Contains(string(got[0]), `"id":"5002"`) {
		t.Fatalf("warning for a game the user isn't playing: %s", got[0])
	}
	// The owner stops: the warning re-arms (applied: 5001 launches again).
	e.setCMState("family_running", borrowing)
	restart()
	if n := len(warnings()); n != 1 {
		t.Fatalf("owner stopping warned: count = %d", n)
	}
	deadline = time.Now().Add(15 * time.Second)
	for {
		var spec map[string]any
		if err := e.call("game.launchSpec", map[string]any{"id": "5001"}, &spec); err == nil {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("owner stopping not applied: launchSpec 5001 = %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	// The owner starts again: a second warning.
	e.setCMState("family_running", takeback)
	restart()
	waitWarnings(2)
	// A first push that already shows the take-back warns too: the user is
	// being kicked either way. A fresh copy keeps it independent.
	firstSeen := append(append([]any{}, takeback...),
		map[string]any{"appid": "5002", "player": "76561198", "owner": "76561200"},
		map[string]any{"appid": "5002", "player": "76561200", "owner": "76561200"})
	e.setCMState("family_running", firstSeen)
	restart()
	waitWarnings(3)
	raw = warnings()[len(warnings())-1]
	if err := json.Unmarshal(raw, &w); err != nil {
		t.Fatalf("family.warning params: %v", err)
	}
	if w.ID != "5002" {
		t.Fatalf("first-seen take-back warned %+v, want id 5002", w)
	}
	// The user's entry vanishes in the same push the owner's appears: still a
	// warning.
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5002", "player": "76561198", "owner": "76561199"},
	})
	restart()
	if n := len(warnings()); n != 3 {
		t.Fatalf("borrowing alone warned: count = %d", n)
	}
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5002", "player": "76561199", "owner": "76561199"},
	})
	restart()
	waitWarnings(4)
	// Separate pushes: the user stops, an empty list arrives, then the owner
	// starts. From the list alone that's a user who quit, so no warning.
	e.setCMState("family_running", []any{})
	restart()
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5002", "player": "76561198", "owner": "76561199"},
	})
	restart()
	e.setCMState("family_running", []any{})
	restart()
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5002", "player": "76561199", "owner": "76561199"},
	})
	restart()
	if n := len(warnings()); n != 4 {
		t.Fatalf("separate quit-then-owner pushes warned: count = %d", n)
	}
	// Another lender's copy playing warns nothing for the user's copy...
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5002", "player": "76561198", "owner": "76561199"},
		map[string]any{"appid": "5002", "player": "76561200", "owner": "76561200"},
	})
	restart()
	if n := len(warnings()); n != 4 {
		t.Fatalf("another lender's copy warned: count = %d", n)
	}
	// ...and neither does another member's borrowing nor the user's own play.
	e.setCMState("family_running", []any{
		map[string]any{"appid": "5001", "player": "76561200", "owner": "76561199"},
		map[string]any{"appid": "4321", "player": "76561198", "owner": "76561198"},
		map[string]any{"appid": "4321", "player": "76561200", "owner": "76561200"},
	})
	restart()
	if n := len(warnings()); n != 4 {
		t.Fatalf("uninvolved play warned: count = %d", n)
	}
	if err := e.call("social.stop", map[string]any{}, nil); err != nil {
		t.Fatalf("social.stop: %v", err)
	}
}
