package app

import (
	"path/filepath"
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
	l, err := playlog.Open(filepath.Join(dir, "play-sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	a.playlog = l
	return a, dir
}

func TestAddedAtSticksAcrossScans(t *testing.T) {
	t.Parallel()
	a, _ := activityTestApp(t)
	games := []types.Game{{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"}}
	a.noteLibraryGames(games)
	first := games[0].AddedAt
	if first <= 0 {
		t.Fatalf("first sighting has no added_at: %+v", games[0])
	}
	// A rescan keeps the first time, even for a renamed title.
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
	if second[1].AddedAt <= first {
		t.Fatalf("new game added_at %d is not after %d", second[1].AddedAt, first)
	}
	if second[0].AddedAt != first {
		t.Fatalf("old game moved from %d to %d", first, second[0].AddedAt)
	}
}

func TestAddedAtSurvivesRestart(t *testing.T) {
	t.Parallel()
	a, dir := activityTestApp(t)
	games := []types.Game{{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"}}
	a.noteLibraryGames(games)
	first := games[0].AddedAt

	b := &App{host: stderrHost{}}
	b.openActivity(dir)
	b.openAddedAt(dir)
	again := []types.Game{{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"}}
	b.noteLibraryGames(again)
	if again[0].AddedAt != first {
		t.Fatalf("after restart added_at %d, want %d", again[0].AddedAt, first)
	}
	// The add row persisted too.
	found := false
	for _, e := range b.ListActivity(0, 0) {
		if e.Kind == "add" && e.GameID == "echo_1" {
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
	a.noteLibraryGames([]types.Game{{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"}})
	time.Sleep(2 * time.Millisecond)
	a.noteLibraryGames([]types.Game{{ID: "echo_1", Source: "echo", Store: "echo", Title: "One"}, {ID: "echo_2", Source: "echo", Store: "echo", Title: "Two"}})
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
	a.noteAchievementUnlocks(g, p)
	a.noteAchievementUnlocks(g, p)
	feed := a.ListActivity(0, 0)
	n := 0
	for _, e := range feed {
		if e.Kind == "achievement" && e.GameID == "echo_1" {
			n++
			if e.TS != 1700000000*1000 {
				t.Fatalf("achievement ts %d, want unlock time", e.TS)
			}
			if e.Args["achievement"] != "first" {
				t.Fatalf("achievement row misses id: %+v", e.Args)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d achievement rows, want 1", n)
	}
}
