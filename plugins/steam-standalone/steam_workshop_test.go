// Workshop subscriptions (protocol 1.22): the CM listing (fake CM), the local
// content placement rule, and the sync request's validation. Real content
// downloads ride platypus-steam-dl's workshop subcommand (see dl_build_test.go
// and the live check).
package steam_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/jsonrpc"
)

type workshopItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	PreviewURL  string `json:"previewUrl"`
	SizeBytes   int64  `json:"sizeBytes"`
	UpdatedAt   int64  `json:"updatedAt"`
	Installed   bool   `json:"installed"`
	NeedsUpdate bool   `json:"needsUpdate"`
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

func TestSteamWorkshopList(t *testing.T) {
	e := setup(t)
	e.login()
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
}
