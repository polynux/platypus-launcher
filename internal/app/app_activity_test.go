package app

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"platypus/internal/playlog"
	"platypus/internal/types"
)

// activityTestApp is a minimal App with an activity feed, first-seen map and
// session log in a temp data dir. The host drops events; tests that need
// them replace a.host.
func activityTestApp(t *testing.T) (*App, string) {
	t.Helper()
	a := &App{host: stderrHost{}}
	dir := t.TempDir()
	a.openActivity(dir)
	a.openAddedAt(dir)
	a.openAchSeen(dir)
	l, err := playlog.Open(filepath.Join(dir, "play-sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	a.playlog = l
	return a, dir
}

func TestFirstScanSeedsSilently(t *testing.T) {
	t.Parallel()
	a, _ := activityTestApp(t)
	mk := func(id string) types.Game {
		return types.Game{ID: id, Source: "echo", Store: "echo", Title: id}
	}
	games := []types.Game{mk("echo_1"), mk("echo_2"), mk("echo_3")}
	a.noteLibraryGames(games)
	for _, g := range games {
		if g.AddedAt != 0 {
			t.Fatalf("first sighting should seed added_at 0, got %+v", g)
		}
	}
	for _, e := range a.ListActivity(0, 0) {
		if e.Kind == "add" {
			t.Fatalf("first scan flooded the feed: %+v", e)
		}
	}
	// A game that appears on the next scan is genuinely new: it gets a time
	// and exactly one row.
	time.Sleep(2 * time.Millisecond)
	next := []types.Game{mk("echo_1"), {ID: "echo_9", Source: "echo", Store: "echo", Title: "Nine"}}
	a.noteLibraryGames(next)
	if next[1].AddedAt <= 0 {
		t.Fatalf("new game has no added_at: %+v", next[1])
	}
	if next[0].AddedAt != 0 {
		t.Fatalf("seeded game moved from 0 to %d", next[0].AddedAt)
	}
	adds := 0
	for _, e := range a.ListActivity(0, 0) {
		if e.Kind == "add" {
			adds++
			if e.GameID != "echo_9" {
				t.Fatalf("unexpected add row: %+v", e)
			}
		}
	}
	if adds != 1 {
		t.Fatalf("want exactly 1 add row, got %d", adds)
	}
}

func TestNewSourceSeedsSilently(t *testing.T) {
	t.Parallel()
	a, _ := activityTestApp(t)
	a.noteLibraryGames([]types.Game{{ID: "a_1", Source: "a", Store: "a", Title: "A"}})
	// A store's first login (a source never seen before) seeds silently.
	a.noteLibraryGames([]types.Game{
		{ID: "a_1", Source: "a", Store: "a", Title: "A"},
		{ID: "b_1", Source: "b", Store: "b", Title: "B"},
		{ID: "b_2", Source: "b", Store: "b", Title: "B2"},
	})
	for _, e := range a.ListActivity(0, 0) {
		if e.Kind == "add" {
			t.Fatalf("a new source flooded the feed: %+v", e)
		}
	}
}

func TestAddedAtSticksAcrossScans(t *testing.T) {
	t.Parallel()
	a, _ := activityTestApp(t)
	games := []types.Game{{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"}}
	a.noteLibraryGames(games)
	// A rescan keeps the seeded time, even for a renamed title.
	first := games[0].AddedAt
	time.Sleep(2 * time.Millisecond)
	again := []types.Game{{ID: "echo_1", Source: "echo", Store: "echo", Title: "One Renamed"}}
	a.noteLibraryGames(again)
	if again[0].AddedAt != first {
		t.Fatalf("rescan moved added_at %d to %d", first, again[0].AddedAt)
	}
	// A new game is newer.
	time.Sleep(2 * time.Millisecond)
	second := []types.Game{{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"}, {ID: "echo_2", Source: "echo", Store: "echo", Title: "Two"}}
	a.noteLibraryGames(second)
	if second[1].AddedAt <= 0 {
		t.Fatalf("new game has no added_at: %+v", second[1])
	}
	if second[0].AddedAt != first {
		t.Fatalf("old game moved from %d to %d", first, second[0].AddedAt)
	}
}

func TestAddedAtSurvivesRestart(t *testing.T) {
	t.Parallel()
	a, dir := activityTestApp(t)
	a.noteLibraryGames([]types.Game{{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"}})
	time.Sleep(2 * time.Millisecond)
	a.noteLibraryGames([]types.Game{
		{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"},
		{ID: "echo_2", Source: "echo", Store: "echo", Title: "Two"},
	})
	if a.addedAt["echo_2"] <= 0 {
		t.Fatal("new game has no added_at")
	}

	b := &App{host: stderrHost{}}
	b.openActivity(dir)
	b.openAddedAt(dir)
	b.openAchSeen(dir)
	again := []types.Game{
		{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"},
		{ID: "echo_2", Source: "echo", Store: "echo", Title: "Two"},
	}
	b.noteLibraryGames(again)
	if again[1].AddedAt != a.addedAt["echo_2"] {
		t.Fatalf("after restart added_at %d, want %d", again[1].AddedAt, a.addedAt["echo_2"])
	}
	// The add row persisted too.
	found := false
	for _, e := range b.ListActivity(0, 0) {
		if e.Kind == "add" && e.GameID == "echo_2" {
			found = true
		}
	}
	if !found {
		t.Fatal("the add event didn't survive the restart")
	}
}

func TestLibraryAddsFeedRecentlyAdded(t *testing.T) {
	t.Parallel()
	a, _ := activityTestApp(t)
	a.noteLibraryGames([]types.Game{{ID: "seed", Source: "echo", Store: "echo", Title: "Seed"}})
	time.Sleep(2 * time.Millisecond)
	a.noteLibraryGames([]types.Game{
		{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"},
		{ID: "seed", Source: "echo", Store: "echo", Title: "Seed"},
	})
	time.Sleep(2 * time.Millisecond)
	a.noteLibraryGames([]types.Game{
		{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"},
		{ID: "echo_2", Source: "echo", Store: "echo", Title: "Two"},
		{ID: "seed", Source: "echo", Store: "echo", Title: "Seed"},
	})
	adds := map[string]int64{}
	for _, e := range a.ListActivity(0, 0) {
		if e.Kind == "add" {
			adds[e.GameID] = e.TS
		}
	}
	if len(adds) != 2 {
		t.Fatalf("add events = %v, want one per game", adds)
	}
	if adds["echo_2"] <= adds["echo_1"] {
		t.Fatalf("newer game isn't newer in the feed: %v", adds)
	}
}

func TestSessionActivitySurvivesRestart(t *testing.T) {
	t.Parallel()
	a, dir := activityTestApp(t)
	start := time.Now().Add(-26 * time.Hour).Truncate(time.Second)
	end := start.Add(30 * time.Minute)
	a.playlog.StartAt("echo_1", start)
	a.playlog.FinishAt("echo_1", end)
	yesterday := end.UnixMilli()

	b := &App{host: stderrHost{}}
	b.openActivity(dir)
	b.openAddedAt(dir)
	l, err := playlog.Open(filepath.Join(dir, "play-sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	b.playlog = l
	feed := b.ListActivity(0, 0)
	found := false
	for _, e := range feed {
		if e.Kind == "session" && e.GameID == "echo_1" && e.TS == yesterday {
			found = true
		}
	}
	if !found {
		t.Fatalf("yesterday's session isn't in the feed: %+v", feed)
	}
}

func TestJobActivityRecordsStoreSizeVersion(t *testing.T) {
	t.Parallel()
	a, _ := activityTestApp(t)
	j := types.Job{
		Kind: "install", Source: "echo", GameID: "echo_1", Title: "One",
		Total: 12345, Target: &types.VersionTarget{Channel: "beta"},
	}
	a.recordJobActivity(j)
	feed := a.ListActivity(0, 0)
	if len(feed) != 1 {
		t.Fatalf("feed has %d rows, want 1", len(feed))
	}
	e := feed[0]
	if e.Kind != "install" || e.GameID != "echo_1" || e.TextKey != "activity:log.jobDone" {
		t.Fatalf("bad job row: %+v", e)
	}
	if e.Args["store"] != "echo" || e.Args["size"] != "12345" || e.Args["version"] != "beta" {
		t.Fatalf("job row misses store/size/version: %+v", e.Args)
	}
	if _, ok := e.Args["action"]; ok {
		t.Fatalf("job row carries an English verb: %+v", e.Args)
	}
	// Verify and move are recorded too (issue #145).
	for _, kind := range []string{"update", "uninstall", "verify", "move"} {
		a.recordJobActivity(types.Job{Kind: kind, Source: "echo", GameID: "echo_1", Title: "One"})
	}
	kinds := map[string]bool{}
	for _, e := range a.ListActivity(0, 0) {
		kinds[e.Kind] = true
	}
	for _, kind := range []string{"install", "update", "uninstall", "verify", "move"} {
		if !kinds[kind] {
			t.Fatalf("no %s row in %+v", kind, kinds)
		}
	}
}

func TestListActivitySinceAndLimit(t *testing.T) {
	t.Parallel()
	a, _ := activityTestApp(t)
	a.recordActivityAt(1000, "install", "g1", "activity:log.jobDone", nil)
	a.recordActivityAt(2000, "update", "g1", "activity:log.jobDone", nil)
	a.recordActivityAt(3000, "add", "g2", "activity:log.added", nil)
	newest := a.ListActivity(2000, 0)
	if len(newest) != 1 || newest[0].TS != 3000 {
		t.Fatalf("since=2000 gives %+v, want only ts=3000", newest)
	}
	capped := a.ListActivity(0, 2)
	if len(capped) != 2 || capped[0].TS != 3000 || capped[1].TS != 2000 {
		t.Fatalf("limit=2 gives %+v, want newest first", capped)
	}
}

func TestAchievementUnlocksLogOnce(t *testing.T) {
	t.Parallel()
	a, _ := activityTestApp(t)
	g := types.Game{ID: "echo_1", Source: "echo", Title: "One"}
	p := types.AchievementProgress{Total: 1, Unlocked: 1, List: []types.ProgressAchievement{
		{ID: "first", Name: "First", Unlocked: true, UnlockedAt: 1700000000},
	}}
	// The first poll only seeds the seen set: no rows for history.
	a.noteAchievementUnlocks(g, p)
	for _, e := range a.ListActivity(0, 0) {
		if e.Kind == "achievement" {
			t.Fatalf("first poll flooded the feed: %+v", e)
		}
	}
	// A genuinely new unlock logs once, even under concurrent polls.
	p.List = append(p.List, types.ProgressAchievement{ID: "second", Name: "Second", Unlocked: true, UnlockedAt: 1700000100})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); a.noteAchievementUnlocks(g, p) }()
	}
	wg.Wait()
	feed := a.ListActivity(0, 0)
	n := 0
	for _, e := range feed {
		if e.Kind == "achievement" && e.GameID == "echo_1" {
			n++
			if e.TS != 1700000100*1000 {
				t.Fatalf("achievement ts %d, want unlock time", e.TS)
			}
			if e.Args["achievement"] != "second" {
				t.Fatalf("achievement row misses id: %+v", e.Args)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d achievement rows, want 1", n)
	}
}

func TestCapKeepsNewestTimestamps(t *testing.T) {
	t.Parallel()
	a, _ := activityTestApp(t)
	for i := 0; i < maxActivityEvents; i++ {
		a.recordActivityAt(int64(1000+i), "install", "g", "activity:log.jobDone", nil)
	}
	// A backfilled old row must not evict a fresh one.
	a.recordActivityAt(1, "install", "g", "activity:log.jobDone", nil)
	feed := a.ListActivity(0, maxActivityEvents+100)
	if len(feed) != maxActivityEvents {
		t.Fatalf("feed has %d rows, want %d", len(feed), maxActivityEvents)
	}
	for _, e := range feed {
		if e.TS < 1000 {
			t.Fatalf("the cap evicted a fresh row for a backfill: %+v", e)
		}
	}
}

func cloudTestApp(t *testing.T) *App {
	t.Helper()
	a, _ := activityTestApp(t)
	return a
}

func countKind(feed []types.ActivityEvent, kind, game string) int {
	n := 0
	for _, e := range feed {
		if e.Kind == kind && (game == "" || e.GameID == game) {
			n++
		}
	}
	return n
}

func TestCloudDownloadLoggedFromCount(t *testing.T) {
	t.Parallel()
	a := cloudTestApp(t)
	a.cloudEmit(types.CloudEvent{GameID: "g1", Kind: types.CloudEvStatus, Done: 3,
		Status: &types.CloudStatus{State: types.CloudStateSynced}})
	feed := a.ListActivity(0, 0)
	if countKind(feed, "cloud", "g1") != 1 {
		t.Fatalf("no downloaded row in %+v", feed)
	}
	if feed[0].TextKey != "activity:log.cloudDownloaded" || feed[0].Args["files"] != "3" {
		t.Fatalf("bad downloaded row: %+v", feed[0])
	}
	// A status without downloads logs nothing.
	a.cloudEmit(types.CloudEvent{GameID: "g1", Kind: types.CloudEvStatus,
		Status: &types.CloudStatus{State: types.CloudStateSynced}})
	if countKind(a.ListActivity(0, 0), "cloud", "g1") != 1 {
		t.Fatal("a clean status logged a row")
	}
}

func TestCloudUploadAndConflict(t *testing.T) {
	t.Parallel()
	a := cloudTestApp(t)
	a.cloudEmit(types.CloudEvent{GameID: "g1", Kind: types.CloudEvPushed, Done: 2})
	feed := a.ListActivity(0, 0)
	if countKind(feed, "cloud", "g1") != 1 || feed[0].TextKey != "activity:log.cloudUploaded" || feed[0].Args["files"] != "2" {
		t.Fatalf("bad uploaded row: %+v", feed)
	}
	// A push-time conflict arrives as Failed: log it once, then dedupe the
	// repeated prompt until the next push.
	failed := types.CloudEvent{GameID: "g1", Kind: types.CloudEvFailed,
		Status: &types.CloudStatus{State: types.CloudStateConflict}}
	a.cloudEmit(failed)
	a.cloudEmit(types.CloudEvent{GameID: "g1", Kind: types.CloudEvPrompt, Prompt: types.CloudPromptConflict})
	if countKind(a.ListActivity(0, 0), "cloud", "g1") != 2 {
		t.Fatalf("conflict not logged once: %+v", a.ListActivity(0, 0))
	}
	// A push clears the dedupe: the next conflict logs again.
	a.cloudEmit(types.CloudEvent{GameID: "g1", Kind: types.CloudEvPushed, Done: 1})
	a.cloudEmit(failed)
	if countKind(a.ListActivity(0, 0), "cloud", "g1") != 4 {
		t.Fatalf("conflict after push not logged: %+v", a.ListActivity(0, 0))
	}
}

func TestNewerFilesAreMovedAside(t *testing.T) {
	t.Parallel()
	_, dir := activityTestApp(t)
	_ = os.WriteFile(filepath.Join(dir, "activity.json"), []byte(`{"version":99,"next_id":1,"events":[]}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "library-added.json"), []byte(`{"version":99,"added":{}}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "achievements-seen.json"), []byte(`{"version":99,"games":{}}`), 0o644)
	b := &App{host: stderrHost{}}
	b.openActivity(dir)
	b.openAddedAt(dir)
	b.openAchSeen(dir)
	if len(b.activityEvents) != 0 || len(b.addedAt) != 0 || len(b.achSeen) != 0 {
		t.Fatal("newer files must load as empty")
	}
	// The canonical names must not still hold the newer files: they were
	// moved aside.
	for _, name := range []string{"activity.json", "library-added.json", "achievements-seen.json"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil && len(raw) >= 14 && string(raw)[:14] == `{"version":99` {
			t.Fatalf("%s was overwritten instead of moved aside", name)
		}
		matches, _ := filepath.Glob(filepath.Join(dir, name+".bad-*"))
		if len(matches) == 0 {
			t.Fatalf("%s has no moved-aside file", name)
		}
	}
}
