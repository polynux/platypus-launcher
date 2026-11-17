// Workshop subscriptions (protocol 1.22): the CM listing (fake CM), the local
// content placement rule, and the sync request's validation. Real content
// downloads ride platypus-steam-dl's workshop subcommand (see dl_build_test.go
// and the live check).
package steam_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/jobs"

	"platypus/internal/jsonrpc"
)

type workshopItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	PreviewURL    string `json:"previewUrl"`
	SizeBytes     int64  `json:"sizeBytes"`
	UpdatedAt     int64  `json:"updatedAt"`
	Installed     bool   `json:"installed"`
	NeedsUpdate   bool   `json:"needsUpdate"`
	PlacementKind string `json:"placementKind"`
	PlacementPath string `json:"placementPath"`
}

// Content placement: a game in a Steam library uses the client's
// steamapps/workshop/content/<appid>, a Platypus-installed game its own
// workshop folder.
func TestSteamWorkshopRoot(t *testing.T) {
	got := pyRun(t, `
print(hs.workshop_root("4321", "/mnt/Steam/steamapps/common/Some Game"))
print(hs.workshop_root("4321", "/home/u/Games/steam/Some_Game_4321"))
`)
	want := "/mnt/Steam/steamapps/workshop/content/4321\n/home/u/Games/steam/Some_Game_4321/workshop"
	if got != want {
		t.Fatalf("roots = %q, want %q", got, want)
	}
}

// Legacy file_url items: only https on Steam's own content hosts, redirects
// stay on that list, and the bytes never exceed the reported size.
func TestSteamWorkshopHTTPAllowlist(t *testing.T) {
	got := pyRun(t, `
for u in ("file:///etc/passwd", "http://cloud-3.steamusercontent.com/ugc/x",
          "https://cloud-3.steamusercontent.com/ugc/x",
          "https://media.steampowered.com/ugc/x",
          "https://cdn.example.com/x", "https://evil.steamusercontent.com.example/x",
          "https://x.akamaihd.net/y"):
    print(u, hs.workshop_http_allowed(u))
`)
	want := `file:///etc/passwd False
http://cloud-3.steamusercontent.com/ugc/x False
https://cloud-3.steamusercontent.com/ugc/x True
https://media.steampowered.com/ugc/x True
https://cdn.example.com/x False
https://evil.steamusercontent.com.example/x False
https://x.akamaihd.net/y True`
	if got != want {
		t.Fatalf("allowlist = %q, want %q", got, want)
	}
}

func TestSteamWorkshopHTTPItemWritesAndCleansThePartFile(t *testing.T) {
	got := pyRun(t, `
import io, os, tempfile
root = tempfile.mkdtemp()
class Fake:
    def open(self, url, timeout=0):
        return io.BytesIO(b"addon")
hs.urllib.request.build_opener = lambda *a, **k: Fake()
hs.notify = lambda *a, **k: None
plan = {"id": "9200", "filename": "addon.gma", "size": 5,
        "file_url": "https://cloud-1.steamusercontent.com/ugc/addon"}
hs.workshop_http_item(root, plan, "j1", 0, 5)
print(open(os.path.join(root, "9200", "addon.gma")).read())
print(sorted(os.listdir(os.path.join(root, "9200"))))
`)
	want := "addon\n['addon.gma']"
	if got != want {
		t.Fatalf("http item = %q, want %q", got, want)
	}
}

// A canceled job stops before it reports done or records state; the next
// Retry starts from an idle job id.
func TestSteamWorkshopCancelStopsBeforeState(t *testing.T) {
	got := pyRun(t, `
notified = []
hs.notify = lambda *a, **k: notified.append(a)
hs.workshop_items = lambda app: []
hs.workshop_job_canceled = lambda job_id: True
hs.jobs["j1"] = {"canceled": True, "id": "4321"}
hs.follow_workshop("j1", "4321", "/tmp/workshop-cancel-test")
print("jobs:", sorted(hs.jobs))
print("notified:", len(notified))
`)
	want := "jobs: []\nnotified: 0"
	if got != want {
		t.Fatalf("cancel = %q, want %q", got, want)
	}
}

func TestSteamWorkshopHTTPItemAbortsOversizeAndCleansUp(t *testing.T) {
	got := pyRun(t, `
import io, os, tempfile
root = tempfile.mkdtemp()
class Fake:
    def open(self, url, timeout=0):
        return io.BytesIO(b"x" * (2 << 20))
hs.urllib.request.build_opener = lambda *a, **k: Fake()
hs.notify = lambda *a, **k: None
plan = {"id": "9200", "filename": "big.bin", "size": 1 << 20,
        "file_url": "https://cloud-1.steamusercontent.com/ugc/big"}
try:
    hs.workshop_http_item(root, plan, "j1", 0, 0)
    print("no error")
except hs.RPCError as e:
    print("error:", e)
print("left:", sorted(os.listdir(os.path.join(root, "9200"))))
`)
	want := "error: workshop item 9200 is larger than the store reported (1048576 bytes)\nleft: []"
	if got != want {
		t.Fatalf("oversize = %q, want %q", got, want)
	}
}

func TestSteamWorkshopList(t *testing.T) {
	e := setup(t)
	e.login()
	gameDir := t.TempDir()
	installed, _ := json.Marshal(map[string]any{"4321": map[string]any{"path": gameDir, "title": "Standalone Game"}})
	if err := os.WriteFile(filepath.Join(e.m.DataDirOf(id), "installed.json"), installed, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{"9001", "9002"} {
		if err := os.MkdirAll(filepath.Join(gameDir, "workshop", item), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	e.setCMState("workshop_items", []map[string]any{
		{"published_file_id": uint64(9001), "appid": 4321, "hcontent": uint64(5001), "size": 2048, "subscribed": 100, "updated": 1790000000, "depot_content": true},
		{"published_file_id": uint64(9002), "appid": 4321, "hcontent": uint64(5002), "size": 4096, "subscribed": 200, "updated": 1791000000, "depot_content": true},
		{"published_file_id": uint64(9003), "appid": 9999, "hcontent": uint64(5003), "size": 1, "subscribed": 300},
	})
	e.setCMState("workshop_details", []map[string]any{
		{"id": uint64(9001), "appid": 4321, "title": "First Map", "preview_url": "https://cdn.example/9001.jpg", "hcontent": uint64(5001), "size": 2048, "updated": 1790000000, "file_type": 1},
		{"id": uint64(9002), "appid": 4321, "title": "Second Map", "hcontent": uint64(5002), "size": 4096, "updated": 1791000000, "file_type": 1},
	})
	// State from an earlier sync: 9001 is current, 9002 is stale.
	state := map[string]map[string]any{"4321": {"9001": uint64(5001), "9002": uint64(4999)}}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(e.m.DataDirOf(id), "workshop.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	var items []workshopItem
	if err := e.call("workshop.list", map[string]any{"id": "4321"}, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v (another app's item must not leak in)", items)
	}
	second, first := items[0], items[1]
	if second.ID != "9002" || !second.Installed || !second.NeedsUpdate || second.Title != "Second Map" || second.SizeBytes != 4096 || second.UpdatedAt != 1791000000 {
		t.Fatalf("newest item = %+v", second)
	}
	if first.ID != "9001" || !first.Installed || first.NeedsUpdate || first.PreviewURL != "https://cdn.example/9001.jpg" {
		t.Fatalf("oldest item = %+v", first)
	}
	// 4321 has no placement rule: no placement fields ride along (optional
	// fields, protocol unchanged for unlisted apps).
	for _, it := range items {
		if it.PlacementKind != "" || it.PlacementPath != "" {
			t.Fatalf("unlisted app carries placement: %+v", it)
		}
	}
	// A second page read within the minute is served from the cache: no new
	// CM session, so the fake sees no second subscription call.
	beforeLog := strings.Count(e.cmLog(), "workshop subscribed app=4321")
	items = nil
	if err := e.call("workshop.list", map[string]any{"id": "4321"}, &items); err != nil {
		t.Fatal(err)
	}
	if afterLog := strings.Count(e.cmLog(), "workshop subscribed app=4321"); afterLog != beforeLog {
		t.Fatalf("the cached listing went back to the CM (%d -> %d)", beforeLog, afterLog)
	}
}

// The CM pages its subscription list; the plugin follows start_index instead
// of repeating the first page.
func TestSteamWorkshopSubscriptionPages(t *testing.T) {
	e := setup(t)
	e.login()
	var subs []map[string]any
	for i := 0; i < 3; i++ {
		subs = append(subs, map[string]any{
			"published_file_id": uint64(9400 + i), "appid": 4321, "hcontent": uint64(5400 + i),
			"size": 1, "subscribed": 10 + i, "updated": 1791000000,
		})
	}
	e.setCMState("workshop_items", subs)
	e.setCMState("workshop_page", 2)
	e.setCMState("workshop_details", []map[string]any{})
	var items []workshopItem
	if err := e.call("workshop.list", map[string]any{"id": "4321"}, &items); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if strings.Join(ids, ",") != "9402,9401,9400" {
		t.Fatalf("paged ids = %v", ids)
	}
}

// A subscription Steam no longer details still lists, under a readable name.
func TestSteamWorkshopListItemWithoutDetails(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("workshop_items", []map[string]any{
		{"published_file_id": uint64(9100), "appid": 4321, "hcontent": uint64(5100), "size": 12, "subscribed": 10},
	})
	e.setCMState("workshop_details", []map[string]any{})
	var items []workshopItem
	if err := e.call("workshop.list", map[string]any{"id": "4321"}, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Workshop item 9100" || items[0].Installed {
		t.Fatalf("items = %+v", items)
	}
}

func TestSteamWorkshopRejectsBadParams(t *testing.T) {
	e := setup(t)
	e.login()
	var out json.RawMessage
	err := e.call("workshop.list", map[string]any{}, &out)
	if code := e.rpcCode(err); code != jsonrpc.CodeInvalidParams {
		t.Fatalf("workshop.list without id: %d (%v)", code, err)
	}
	err = e.call("workshop.sync", map[string]any{"id": "4321"}, &out)
	if code := e.rpcCode(err); code != jsonrpc.CodeInvalidParams {
		t.Fatalf("workshop.sync without jobId: %d (%v)", code, err)
	}
	// 4322 is not installed in this fixture: a readable refusal, not a job.
	err = e.call("workshop.sync", map[string]any{"id": "4322", "jobId": "j1", "installDir": "/tmp/x"}, &out)
	if code := e.rpcCode(err); code == jsonrpc.CodeInvalidParams || code == jsonrpc.CodeMethodNotFound {
		t.Fatalf("workshop.sync for an uninstalled game: %d (%v)", code, err)
	}
	// An installed game only syncs into its own recorded folder.
	installed, _ := json.Marshal(map[string]any{"4321": map[string]any{"path": "/tmp/real-game"}})
	if err := os.WriteFile(filepath.Join(e.m.DataDirOf(id), "installed.json"), installed, 0o600); err != nil {
		t.Fatal(err)
	}
	err = e.call("workshop.sync", map[string]any{"id": "4321", "jobId": "j2", "installDir": "/tmp/other"}, &out)
	if code := e.rpcCode(err); code != jsonrpc.CodeInvalidParams {
		t.Fatalf("workshop.sync for another folder: %d (%v)", code, err)
	}
}

// One subscribed item's content downloads through the plugin's workshop.sync
// job and lands in <install folder>/workshop/<item>, the Platypus-owned
// layout for a game installed without the Steam client.
func TestSteamWorkshopSyncDownloadsIntoTheGameFolder(t *testing.T) {
	e := setup(t)
	e.login()
	gameDir := t.TempDir()
	dataDir := e.m.DataDirOf(id)
	installed, _ := json.Marshal(map[string]any{"4321": map[string]any{"path": gameDir, "title": "Standalone Game"}})
	if err := os.WriteFile(filepath.Join(dataDir, "installed.json"), installed, 0o600); err != nil {
		t.Fatal(err)
	}
	e.setCMState("workshop_items", []map[string]any{
		{"published_file_id": uint64(9200), "appid": 4321, "hcontent": e.fix.GID, "size": e.fix.CompressedBytes, "subscribed": 10, "updated": 1791000000, "depot_content": true},
	})
	e.setCMState("workshop_details", []map[string]any{
		{"id": uint64(9200), "appid": 4321, "title": "Echo Addon", "hcontent": e.fix.GID, "size": e.fix.CompressedBytes, "updated": 1791000000, "file_type": 1},
	})

	// A listing before the sync populates the cache; the sync must drop it.
	var before []workshopItem
	if err := e.call("workshop.list", map[string]any{"id": "4321"}, &before); err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].Installed {
		t.Fatalf("pre-sync listing = %+v", before)
	}
	j, err := e.q.AddWorkshop(id, "p.steam.4321", "4321", "Standalone Game", gameDir)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(j.ID); done.State != "done" {
		t.Fatalf("workshop job = %+v", done)
	}
	itemDir := filepath.Join(gameDir, "workshop", "9200")
	for _, f := range e.fix.Manifest.Files {
		if f.IsDir() || f.IsSymlink() {
			continue
		}
		path := filepath.Join(itemDir, filepath.FromSlash(f.Name))
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s didn't land: %v", f.Name, err)
		}
		if uint64(fi.Size()) != f.Size {
			t.Fatalf("%s is %d bytes, want %d", f.Name, fi.Size(), f.Size)
		}
	}
	// The plugin remembers the content handle it wrote, so the next listing
	// shows the item installed and up to date.
	state, err := os.ReadFile(filepath.Join(dataDir, "workshop.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]uint64
	if err := json.Unmarshal(state, &got); err != nil {
		t.Fatal(err)
	}
	if got["4321"]["9200"] != e.fix.GID {
		t.Fatalf("workshop.json = %s, want item 9200 at gid %d", state, e.fix.GID)
	}
	// A folder the user deleted is not installed any more, whatever the
	// state file says.
	if err := os.RemoveAll(filepath.Join(gameDir, "workshop", "9200")); err != nil {
		t.Fatal(err)
	}
	var items []workshopItem
	if err := e.call("workshop.list", map[string]any{"id": "4321"}, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Installed {
		t.Fatalf("listing after the folder vanished = %+v", items)
	}
}

// A subscribed collection has no content of its own: its children download,
// and its installed/needsUpdate state follows theirs.
func TestSteamWorkshopCollectionSync(t *testing.T) {
	t.Setenv("PLATYPUS_WORKSHOP_CACHE_TTL", "0")
	e := setup(t)
	e.login()
	gameDir := t.TempDir()
	dataDir := e.m.DataDirOf(id)
	installed, _ := json.Marshal(map[string]any{"4321": map[string]any{"path": gameDir, "title": "Standalone Game"}})
	if err := os.WriteFile(filepath.Join(dataDir, "installed.json"), installed, 0o600); err != nil {
		t.Fatal(err)
	}
	e.setCMState("workshop_items", []map[string]any{
		{"published_file_id": uint64(9300), "appid": 4321, "hcontent": uint64(0), "subscribed": 10, "updated": 1791000000},
	})
	e.setCMState("workshop_details", []map[string]any{
		{"id": uint64(9300), "appid": 4321, "title": "Echo Collection", "updated": 1791000000, "file_type": 3, "children": []string{"9301"}},
		{"id": uint64(9301), "appid": 4321, "title": "Echo Child", "hcontent": e.fix.GID, "size": e.fix.CompressedBytes, "updated": 1791000000, "file_type": 1},
	})
	j, err := e.q.AddWorkshop(id, "p.steam.4321", "4321", "Standalone Game", gameDir)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(j.ID); done.State != "done" {
		t.Fatalf("collection job = %+v", done)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "workshop", "9301")); err != nil {
		t.Fatalf("the child's content didn't land: %v", err)
	}
	var items []workshopItem
	if err := e.call("workshop.list", map[string]any{"id": "4321"}, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].Installed || items[0].NeedsUpdate {
		t.Fatalf("collection listing = %+v", items)
	}
	// A child update makes the collection outdated.
	e.setCMState("workshop_details", []map[string]any{
		{"id": uint64(9300), "appid": 4321, "title": "Echo Collection", "file_type": 3, "children": []string{"9301"}},
		{"id": uint64(9301), "appid": 4321, "title": "Echo Child", "hcontent": e.fix.GID + 1, "size": 1, "file_type": 1},
	})
	items = nil
	if err := e.call("workshop.list", map[string]any{"id": "4321"}, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].NeedsUpdate {
		t.Fatalf("collection after a child update = %+v", items)
	}
}

// A collection child Steam no longer details fails the sync with a readable
// count instead of silently finishing.
func TestSteamWorkshopCollectionMissingChildFails(t *testing.T) {
	e := setup(t)
	e.login()
	gameDir := t.TempDir()
	dataDir := e.m.DataDirOf(id)
	installed, _ := json.Marshal(map[string]any{"4321": map[string]any{"path": gameDir, "title": "Standalone Game"}})
	if err := os.WriteFile(filepath.Join(dataDir, "installed.json"), installed, 0o600); err != nil {
		t.Fatal(err)
	}
	e.setCMState("workshop_items", []map[string]any{
		{"published_file_id": uint64(9310), "appid": 4321, "subscribed": 10},
	})
	e.setCMState("workshop_details", []map[string]any{
		{"id": uint64(9310), "appid": 4321, "title": "Gone Collection", "file_type": 3, "children": []string{"9311", "9312"}},
	})
	j, err := e.q.AddWorkshop(id, "p.steam.4321", "4321", "Standalone Game", gameDir)
	if err != nil {
		t.Fatal(err)
	}
	done := e.waitJob(j.ID)
	if done.State != "failed" || !strings.Contains(done.Error, "2 items in your subscribed collections") {
		t.Fatalf("missing children job = %+v", done)
	}
}

// Uninstalling the game drops its Workshop record and the root Platypus
// wrote for it.
func TestSteamWorkshopUninstallCleansUp(t *testing.T) {
	e := setup(t)
	e.login()
	gameDir := t.TempDir()
	dataDir := e.m.DataDirOf(id)
	installed, _ := json.Marshal(map[string]any{"4321": map[string]any{"path": gameDir, "title": "Standalone Game"}})
	if err := os.WriteFile(filepath.Join(dataDir, "installed.json"), installed, 0o600); err != nil {
		t.Fatal(err)
	}
	e.setCMState("workshop_items", []map[string]any{
		{"published_file_id": uint64(9200), "appid": 4321, "hcontent": e.fix.GID, "size": e.fix.CompressedBytes, "subscribed": 10},
	})
	e.setCMState("workshop_details", []map[string]any{
		{"id": uint64(9200), "appid": 4321, "title": "Echo Addon", "hcontent": e.fix.GID, "size": e.fix.CompressedBytes, "file_type": 1},
	})
	j, err := e.q.AddWorkshop(id, "p.steam.4321", "4321", "Standalone Game", gameDir)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(j.ID); done.State != "done" {
		t.Fatalf("sync job = %+v", done)
	}
	// A folder from an item unsubscribed afterwards is pruned by the next
	// sync (the fake still lists only 9200).
	stale := filepath.Join(gameDir, "workshop", "9999")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	again, err := e.q.AddWorkshop(id, "p.steam.4321", "4321", "Standalone Game", gameDir)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(again.ID); done.State != "done" {
		t.Fatalf("second sync = %+v", done)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("an unsubscribed item folder stayed: %v", err)
	}
	u, err := e.q.Add(jobs.KindUninstall, id, "p.steam.4321", "4321", "Standalone Game", gameDir)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(u.ID); done.State != "done" {
		t.Fatalf("uninstall = %+v", done)
	}
	if _, err := os.Stat(gameDir); !os.IsNotExist(err) {
		t.Fatalf("game folder stayed: %v", err)
	}
	for _, name := range []string{"workshop.json", "workshop-roots.json"} {
		data, err := os.ReadFile(filepath.Join(dataDir, name))
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("%s still holds %v", name, got)
		}
	}
}

// A game whose folder sits in a Steam library gets the client's
// steamapps/workshop/content layout; the root is recorded because Platypus
// wrote it, so uninstalling the game removes it (and only it).
func TestSteamWorkshopLibraryRootLeavesWithTheGame(t *testing.T) {
	e := setup(t)
	e.login()
	lib := t.TempDir()
	gameDir := filepath.Join(lib, "steamapps", "common", "Standalone Game")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dataDir := e.m.DataDirOf(id)
	installed, _ := json.Marshal(map[string]any{"4321": map[string]any{"path": gameDir, "title": "Standalone Game"}})
	if err := os.WriteFile(filepath.Join(dataDir, "installed.json"), installed, 0o600); err != nil {
		t.Fatal(err)
	}
	e.setCMState("workshop_items", []map[string]any{
		{"published_file_id": uint64(9200), "appid": 4321, "hcontent": e.fix.GID, "size": e.fix.CompressedBytes, "subscribed": 10},
	})
	e.setCMState("workshop_details", []map[string]any{
		{"id": uint64(9200), "appid": 4321, "title": "Echo Addon", "hcontent": e.fix.GID, "size": e.fix.CompressedBytes, "file_type": 1},
	})
	j, err := e.q.AddWorkshop(id, "p.steam.4321", "4321", "Standalone Game", gameDir)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(j.ID); done.State != "done" {
		t.Fatalf("sync job = %+v", done)
	}
	root := filepath.Join(lib, "steamapps", "workshop", "content", "4321")
	if _, err := os.Stat(filepath.Join(root, "9200")); err != nil {
		t.Fatalf("content isn't in the Steam layout: %v", err)
	}
	u, err := e.q.Add(jobs.KindUninstall, id, "p.steam.4321", "4321", "Standalone Game", gameDir)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(u.ID); done.State != "done" {
		t.Fatalf("uninstall = %+v", done)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("the recorded workshop root stayed: %v", err)
	}
}

// Placement rules (issue #214): ruled apps resolve to their own layout, an
// unlisted app keeps today's behaviour (the Steam layout in a library,
// workshop/ under a standalone folder).
func TestSteamWorkshopPlacementRoots(t *testing.T) {
	got := pyRun(t, `
lib = "/mnt/Steam/steamapps/common/Some Game"
own = "/home/u/Games/steam/Some_Game_4321"
print(hs.workshop_root("620", lib))
print(hs.workshop_root("620", own))
print(hs.workshop_root("730", lib))
print(hs.workshop_root("4000", lib))
print(hs.workshop_root("252950", lib))
print(hs.workshop_root("431960", lib))
print(hs.workshop_root("4321", lib))
print(hs.workshop_root("4321", own))
print(hs.workshop_placement_display("620", lib))
print(hs.workshop_placement_display("730", lib))
print(hs.workshop_placement_display("4321", lib))
print(hs.workshop_placement_display("620", ""))
`)
	want := `/mnt/Steam/steamapps/common/Some Game/portal2/maps/workshop
/home/u/Games/steam/Some_Game_4321/portal2/maps/workshop
/mnt/Steam/steamapps/workshop/content/730
/mnt/Steam/steamapps/workshop/content/4000
/mnt/Steam/steamapps/workshop/content/252950
/mnt/Steam/steamapps/workshop/content/431960
/mnt/Steam/steamapps/workshop/content/4321
/home/u/Games/steam/Some_Game_4321/workshop
('install-subfolder', 'portal2/maps/workshop')
('client-only', 'steamapps/workshop/content/730')
('', '')
('', '')`
	if got != want {
		t.Fatalf("roots = %q, want %q", got, want)
	}
}

// A placement entry that escapes the game folder (.., absolute) or passes
// through a symlink is refused; the sync fails instead of writing elsewhere.
func TestSteamWorkshopPlacementConfinement(t *testing.T) {
	got := pyRun(t, `
import os, tempfile
base = tempfile.mkdtemp()
os.mkdir(os.path.join(base, "real"))
os.symlink(os.path.join(base, "real"), os.path.join(base, "link"))
hs.WORKSHOP_PLACEMENT["9901"] = ("install-subfolder", "../escape")
hs.WORKSHOP_PLACEMENT["9902"] = ("install-subfolder", "/abs/path")
hs.WORKSHOP_PLACEMENT["9903"] = ("install-subfolder", "link/mods")
hs.WORKSHOP_PLACEMENT["9904"] = ("install-subfolder", "real/mods")
for app in ("9901", "9902", "9903"):
    try:
        hs.workshop_rule_root(app, base)
        print(app, "no error")
    except hs.RPCError as e:
        print(app, "error:", e)
print("9904", hs.workshop_rule_root("9904", base))
`)
	want := "9901 error: workshop placement for 9901 is not a clean relative path\n" +
		"9902 error: workshop placement for 9902 is not a clean relative path\n" +
		"9903 error: workshop placement for 9903 passes through a link\n9904 "
	if !strings.HasPrefix(got, want) || !strings.HasSuffix(got, string(os.PathSeparator)+"real"+string(os.PathSeparator)+"mods") {
		t.Fatalf("confinement = %q", got)
	}
}

// workshop.list carries the placement for a ruled app (optional fields only).
func TestSteamWorkshopListCarriesPlacement(t *testing.T) {
	e := setup(t)
	e.login()
	gameDir := t.TempDir()
	installed, _ := json.Marshal(map[string]any{"620": map[string]any{"path": gameDir, "title": "Standalone Game"}})
	if err := os.WriteFile(filepath.Join(e.m.DataDirOf(id), "installed.json"), installed, 0o600); err != nil {
		t.Fatal(err)
	}
	e.setCMState("workshop_items", []map[string]any{
		{"published_file_id": uint64(9501), "appid": 620, "hcontent": uint64(5501), "size": 1024, "subscribed": 10, "updated": 1791000000, "depot_content": true},
	})
	e.setCMState("workshop_details", []map[string]any{
		{"id": uint64(9501), "appid": 620, "title": "Ruled Chamber", "hcontent": uint64(5501), "size": 1024, "updated": 1791000000, "file_type": 1},
	})
	var items []map[string]any
	if err := e.call("workshop.list", map[string]any{"id": "620"}, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["placementKind"] != "install-subfolder" || items[0]["placementPath"] != "portal2/maps/workshop" {
		t.Fatalf("ruled items = %+v", items)
	}
}
