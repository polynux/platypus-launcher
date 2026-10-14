package app

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/types"
)

// Cache, background scans and forced scans (issue #50). Scans measure the
// tests' games without any plugin: the library is replaced, so GetLibrary
// (and its stores) is never touched. Temp dirs only.

// scanEnv is a removeEnv whose scans measure games without any plugin.
func scanEnv(t *testing.T, games ...types.Game) *removeEnv {
	t.Helper()
	e := newRemoveEnv(t)
	e.a.diskGames = func() ([]types.Game, error) { return games, nil }
	return e
}

// waitDiskIdle waits for the background pass to end and returns the snapshot.
func waitDiskIdle(t *testing.T, a *App) types.DiskUsage {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		a.diskUsage.mu.Lock()
		scanning := a.diskUsage.scanning
		snap := a.diskUsage.snap
		a.diskUsage.mu.Unlock()
		if !scanning {
			return snap
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("disk scan never ended")
	return types.DiskUsage{}
}

// diskHost records the core events of a scan for tests.
type diskHost struct {
	mu     sync.Mutex
	events []string
}

func (h *diskHost) Emit(event string, _ ...any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
}

func (h *diskHost) Infof(string, ...any)  {}
func (h *diskHost) Warnf(string, ...any)  {}
func (h *diskHost) Errorf(string, ...any) {}

func (h *diskHost) OpenFile(FileDialog) (string, error) { return "", nil }

func (h *diskHost) OpenDirectory(DirectoryDialog) (string, error) { return "", nil }

func (h *diskHost) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.events)
}

// waitDiskEvent waits for the scan's event.
func waitDiskEvent(t *testing.T, h *diskHost) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		got := len(h.events) > 0
		h.mu.Unlock()
		if got {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no diskusage event")
}

func fakeSpace(string) (uint64, uint64, error) { return 10 << 30, 5 << 30, nil }

// The RPC path never walks: the first call returns an empty snapshot marked
// scanning, and the background pass fills it, trusting the stores.
func TestDiskUsageReturnedWithoutWalking(t *testing.T) {
	t.Parallel()
	drive := evalTemp(t)
	dir := filepath.Join(drive, "Games", "A")
	writeFileOf(t, dir, 1<<20)
	game := types.Game{ID: "a", Title: "A", Source: "echo", Store: types.StoreGOG, InstallDir: dir, Executable: filepath.Join(dir, "run.sh"), IsInstalled: true, SizeBytes: 6 << 20, Native: true}
	e := scanEnv(t, game)
	got := e.a.GetDiskUsage()
	if len(got.Games) != 0 || !got.Scanning {
		t.Fatalf("first snapshot = %+v, want empty and scanning", got)
	}
	snap := waitDiskIdle(t, e.a)
	if len(snap.Games) != 1 || !snap.Games[0].FromPlugin || snap.Games[0].Measured {
		t.Fatalf("snapshot = %+v, want the store's figure", snap.Games)
	}
}

// A pass over games the stores sized walks nothing.
func TestDiskUsageScanSkipsWalkForPluginSizes(t *testing.T) {
	t.Parallel()
	drive := evalTemp(t)
	aDir := filepath.Join(drive, "Games", "A")
	writeFileOf(t, aDir, 1<<20)
	bDir := filepath.Join(drive, "Games", "B")
	writeFileOf(t, bDir, 2<<20)
	mk := func(id, dir string, size int64) types.Game {
		return types.Game{ID: id, Title: id, Source: "echo", Store: types.StoreGOG, InstallDir: dir, Executable: filepath.Join(dir, "run.sh"), IsInstalled: true, SizeBytes: size, Native: true}
	}
	e := scanEnv(t, mk("a", aDir, 6<<20), mk("b", bDir, 5<<20))
	var walks walkLog
	if again := e.a.scanDiskWith([]types.Game{mk("a", aDir, 6<<20), mk("b", bDir, 5<<20)},
		[]diskMount{{point: drive}}, fakeSpace, walks.measure); again {
		t.Fatalf("another pass is due with no forced scans")
	}
	if len(walks.paths) != 0 {
		t.Fatalf("walked %v, want no walk", walks.paths)
	}
}

// Concurrent calls while stale share one pass.
func TestDiskUsageSingleFlight(t *testing.T) {
	t.Parallel()
	e := scanEnv(t)
	var calls atomic.Int64
	e.a.diskGames = func() ([]types.Game, error) { calls.Add(1); return nil, nil }
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; e.a.GetDiskUsage() }()
	}
	close(start)
	wg.Wait()
	waitDiskIdle(t, e.a)
	if calls.Load() != 1 {
		t.Fatalf("library listed %d times, want 1", calls.Load())
	}
}

// ScanGameSize walks the forced game only: its walk outranks the store,
// every other game keeps its figures.
func TestDiskUsageScanGameWalksForcedOnly(t *testing.T) {
	t.Parallel()
	drive := evalTemp(t)
	aDir := filepath.Join(drive, "Games", "A")
	writeFileOf(t, aDir, 1<<20)
	bDir := filepath.Join(drive, "Games", "B")
	writeFileOf(t, bDir, 2<<20)
	mk := func(id, dir string, size int64) types.Game {
		return types.Game{ID: id, Title: id, Source: "echo", Store: types.StoreGOG, InstallDir: dir, Executable: filepath.Join(dir, "run.sh"), IsInstalled: true, SizeBytes: size, Native: true}
	}
	a, b := mk("a", aDir, 6<<20), mk("b", bDir, 5<<20)
	e := scanEnv(t, a, b)
	var walks walkLog
	if again := e.a.scanDiskWith([]types.Game{a, b}, []diskMount{{point: drive}}, fakeSpace, walks.measure); again {
		t.Fatalf("another pass is due with no forced scans")
	}
	e.a.diskUsage.mu.Lock()
	e.a.diskUsage.forceGames = map[string]bool{"a": true}
	e.a.diskUsage.mu.Unlock()
	if again := e.a.scanDiskWith([]types.Game{a, b}, []diskMount{{point: drive}}, fakeSpace, walks.measure); again {
		t.Fatalf("another pass is due with no new forced scans")
	}
	if !walks.walked(aDir) || walks.walked(bDir) {
		t.Fatalf("walks = %v, want only the forced game", walks.paths)
	}
	sized := map[string]types.GameSize{}
	for _, g := range e.a.diskUsage.snap.Games {
		sized[g.GameID] = g
	}
	if !sized["a"].Measured || sized["a"].FromPlugin || sized["a"].SizeBytes != 1<<20 {
		t.Fatalf("a = %+v, want the walked size", sized["a"])
	}
	if !sized["b"].FromPlugin || sized["b"].Measured {
		t.Fatalf("b = %+v, want the kept store figure", sized["b"])
	}
}

// ScanGameSize serves the forced game end to end and refuses games that
// aren't installed.
func TestDiskUsageScanGameServesForcedGame(t *testing.T) {
	t.Parallel()
	drive := evalTemp(t)
	aDir := filepath.Join(drive, "Games", "A")
	writeFileOf(t, aDir, 1<<20)
	a := types.Game{ID: "a", Title: "A", Source: "echo", Store: types.StoreGOG, InstallDir: aDir, Executable: filepath.Join(aDir, "run.sh"), IsInstalled: true, SizeBytes: 6 << 20, Native: true}
	b := types.Game{ID: "b", Title: "B", Source: "echo", Store: types.StoreGOG, InstallDir: filepath.Join(drive, "Games", "B"), IsInstalled: true, SizeBytes: 5 << 20, Native: true}
	e := scanEnv(t, a, b)
	e.a.GetDiskUsage()
	waitDiskIdle(t, e.a)
	got, err := e.a.ScanGameSize(a)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Scanning {
		t.Fatalf("snapshot = %+v, want scanning", got)
	}
	snap := waitDiskIdle(t, e.a)
	sized := map[string]types.GameSize{}
	for _, g := range snap.Games {
		sized[g.GameID] = g
	}
	if !sized["a"].Measured || sized["a"].SizeBytes != 1<<20 {
		t.Fatalf("a = %+v, want the walked size", sized["a"])
	}
	if !sized["b"].FromPlugin {
		t.Fatalf("b = %+v, want the kept store figure", sized["b"])
	}
	if _, err := e.a.ScanGameSize(types.Game{ID: "x", Title: "X"}); apperr.CodeOf(err) != "storage.notInstalled" {
		t.Fatalf("err = %v, want storage.notInstalled", err)
	}
}

// RefreshDiskUsage walks everything, overriding the stores.
func TestDiskUsageRefreshOverridesPluginSizes(t *testing.T) {
	t.Parallel()
	drive := evalTemp(t)
	dir := filepath.Join(drive, "Games", "A")
	writeFileOf(t, dir, 1<<20)
	game := types.Game{ID: "a", Title: "A", Source: "echo", Store: types.StoreGOG, InstallDir: dir, Executable: filepath.Join(dir, "run.sh"), IsInstalled: true, SizeBytes: 6 << 20, Native: true}
	e := scanEnv(t, game)
	if got := e.a.RefreshDiskUsage(); !got.Scanning {
		t.Fatalf("snapshot = %+v, want scanning", got)
	}
	snap := waitDiskIdle(t, e.a)
	if len(snap.Games) != 1 || !snap.Games[0].Measured || snap.Games[0].FromPlugin || snap.Games[0].SizeBytes != 1<<20 {
		t.Fatalf("games = %+v, want the walked size", snap.Games)
	}
}

// A finished job drops the snapshot: the bytes changed.
func TestDiskUsageInvalidatedByJob(t *testing.T) {
	t.Parallel()
	e := scanEnv(t)
	e.a.GetDiskUsage()
	waitDiskIdle(t, e.a)
	e.a.diskUsage.mu.Lock()
	fresh := e.a.diskUsage.fresh()
	e.a.diskUsage.mu.Unlock()
	if !fresh {
		t.Fatalf("want a fresh snapshot before the job")
	}
	// The re-scan blocks in the library, so the drop is observed before
	// the pass stores again.
	release := make(chan struct{})
	defer close(release)
	e.a.diskGames = func() ([]types.Game, error) { <-release; return nil, nil }
	e.a.jobFinished(types.Job{Kind: jobs.KindMove, Source: "custom", GameID: "g"})
	e.a.diskUsage.mu.Lock()
	fresh = e.a.diskUsage.fresh()
	e.a.diskUsage.mu.Unlock()
	if fresh {
		t.Fatalf("snapshot survived a finished move")
	}
}

// A changed game folder drops the snapshot: the folders changed.
func TestDiskUsageInvalidatedByFolderChange(t *testing.T) {
	t.Parallel()
	e := scanEnv(t)
	e.a.GetDiskUsage()
	waitDiskIdle(t, e.a)
	release := make(chan struct{})
	defer close(release)
	e.a.diskGames = func() ([]types.Game, error) { <-release; return nil, nil }
	if _, err := e.a.AddGameFolder(filepath.Join(t.TempDir(), "Games")); err != nil {
		t.Fatal(err)
	}
	e.a.diskUsage.mu.Lock()
	fresh := e.a.diskUsage.fresh()
	e.a.diskUsage.mu.Unlock()
	if fresh {
		t.Fatalf("snapshot survived a new game folder")
	}
}

// The snapshot goes stale on the injected clock's TTL.
func TestDiskUsageTTLRenewsWithClock(t *testing.T) {
	t.Parallel()
	e := scanEnv(t)
	now := time.Now()
	e.a.diskUsage.now = func() time.Time { return now }
	e.a.GetDiskUsage()
	waitDiskIdle(t, e.a)
	if got := e.a.GetDiskUsage(); got.Scanning {
		t.Fatalf("fresh snapshot served as scanning")
	}
	now = now.Add(diskUsageTTL + time.Minute)
	if got := e.a.GetDiskUsage(); !got.Scanning {
		t.Fatalf("stale snapshot didn't start a scan")
	}
	waitDiskIdle(t, e.a)
	if got := e.a.GetDiskUsage(); got.Scanning {
		t.Fatalf("renewed snapshot served as scanning")
	}
}

// A finished pass tells the UI once.
func TestDiskUsageEmitsWhenScanFinishes(t *testing.T) {
	t.Parallel()
	e := scanEnv(t)
	h := &diskHost{}
	e.a.host = h
	e.a.GetDiskUsage()
	waitDiskEvent(t, h)
	waitDiskIdle(t, e.a)
	if h.count() != 1 {
		t.Fatalf("events = %v, want one diskusage", h.events)
	}
}

// A finished job re-sizes its game only: the game's new bytes are measured
// fresh while every other game keeps its figures.
func TestDiskUsageJobRefreshesItsGameOnly(t *testing.T) {
	t.Parallel()
	drive := evalTemp(t)
	xDir := filepath.Join(drive, "Games", "X")
	writeFileOf(t, xDir, 1<<20)
	yDir := filepath.Join(drive, "Games", "Y")
	writeFileOf(t, yDir, 2<<20)
	mk := func(id, dir string, size int64) types.Game {
		return types.Game{ID: id, Title: id, Source: "custom", Store: types.StoreCustom, InstallDir: dir, Executable: filepath.Join(dir, "run.sh"), IsInstalled: true, SizeBytes: size, Native: true}
	}
	games := []types.Game{mk("x", xDir, 0), mk("y", yDir, 5<<20)}
	e := scanEnv(t, games...)
	e.a.GetDiskUsage()
	snap := waitDiskIdle(t, e.a)
	if len(snap.Games) != 2 {
		t.Fatalf("games = %+v, want 2", snap.Games)
	}
	// Both games change on disk, but only x's job finished: x is re-sized,
	// y keeps its figures.
	writeFileOf(t, filepath.Join(xDir, "more"), 1<<20)
	writeFileOf(t, filepath.Join(yDir, "more"), 1<<20)
	e.a.jobFinished(types.Job{Kind: jobs.KindMove, Source: "custom", GameID: "x"})
	snap = waitDiskIdle(t, e.a)
	sized := map[string]types.GameSize{}
	for _, g := range snap.Games {
		sized[g.GameID] = g
	}
	if !sized["x"].Measured || sized["x"].SizeBytes != 2<<20 {
		t.Fatalf("x = %+v, want the re-measured size", sized["x"])
	}
	if !sized["y"].FromPlugin || sized["y"].SizeBytes != 5<<20 {
		t.Fatalf("y = %+v, want the kept store figure", sized["y"])
	}
}

// Nothing walks before the view is opened: a finished job with no snapshot
// this session only marks it stale, and the next GetDiskUsage starts the pass.
func TestDiskUsageNoScanBeforeFirstView(t *testing.T) {
	t.Parallel()
	e := scanEnv(t)
	var calls atomic.Int64
	e.a.diskGames = func() ([]types.Game, error) { calls.Add(1); return nil, nil }
	e.a.jobFinished(types.Job{Kind: jobs.KindMove, Source: "custom", GameID: "g"})
	time.Sleep(200 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatalf("library listed %d times with the view never opened", calls.Load())
	}
	e.a.diskUsage.mu.Lock()
	scanning := e.a.diskUsage.scanning
	e.a.diskUsage.mu.Unlock()
	if scanning {
		t.Fatalf("a pass runs with the view never opened")
	}
	if got := e.a.GetDiskUsage(); !got.Scanning {
		t.Fatalf("first view didn't start a pass")
	}
	waitDiskIdle(t, e.a)
	if calls.Load() != 1 {
		t.Fatalf("library listed %d times, want 1", calls.Load())
	}
}

// OpenGameFolder refuses a game with no folder it could open, with a code
// the UI translates. Opening a real folder shells out to xdg-open, like
// OpenPluginsFolder, so no test does that here.
func TestOpenGameFolderWithoutFolderFailsCoded(t *testing.T) {
	t.Parallel()
	e := scanEnv(t)
	if err := e.a.OpenGameFolder(types.Game{ID: "x", Title: "X"}); apperr.CodeOf(err) != "storage.openFailed" {
		t.Fatalf("err = %v, want storage.openFailed", err)
	}
}
