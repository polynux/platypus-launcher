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
	"strings"
	"testing"
	"time"
)

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
