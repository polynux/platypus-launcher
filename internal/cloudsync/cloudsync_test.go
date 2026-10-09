package cloudsync

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/types"
)

type call struct {
	Method string
	Params map[string]any
}

// fakeBackend plays a store plugin: handlers answer by method.
type fakeBackend struct {
	mu       sync.Mutex
	offers   bool
	calls    []call
	handlers map[string]func(ctx context.Context, params map[string]any) (any, error)
}

func newBackend() *fakeBackend {
	return &fakeBackend{offers: true, handlers: map[string]func(context.Context, map[string]any) (any, error){}}
}

func (f *fakeBackend) Offers(context.Context, string) bool { return f.offers }

func (f *fakeBackend) Call(ctx context.Context, _ string, method string, params, result any) error {
	raw, _ := json.Marshal(params)
	var p map[string]any
	_ = json.Unmarshal(raw, &p)
	f.mu.Lock()
	f.calls = append(f.calls, call{method, p})
	h := f.handlers[method]
	f.mu.Unlock()
	if h == nil {
		return errors.New("no handler for " + method)
	}
	res, err := h(ctx, p)
	if err != nil || result == nil {
		return err
	}
	out, _ := json.Marshal(res)
	return json.Unmarshal(out, result)
}

func (f *fakeBackend) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		out = append(out, c.Method)
	}
	return out
}

func (f *fakeBackend) count(method string) int {
	n := 0
	for _, m := range f.methods() {
		if m == method {
			n++
		}
	}
	return n
}

func (f *fakeBackend) last(method string) call {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.calls) - 1; i >= 0; i-- {
		if f.calls[i].Method == method {
			return f.calls[i]
		}
	}
	return call{}
}

func (f *fakeBackend) answer(method string, res any) {
	f.mu.Lock()
	f.handlers[method] = func(context.Context, map[string]any) (any, error) { return res, nil }
	f.mu.Unlock()
}

type fakeSettings struct{ plugin, game string }

func (s fakeSettings) CloudPluginMode(string) string { return s.plugin }
func (s fakeSettings) CloudGameMode(string) string   { return s.game }

type events struct {
	mu   sync.Mutex
	list []types.CloudEvent
}

func (e *events) emit(ev types.CloudEvent) {
	e.mu.Lock()
	e.list = append(e.list, ev)
	e.mu.Unlock()
}

func (e *events) kinds() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []string
	for _, ev := range e.list {
		out = append(out, ev.Kind)
	}
	return out
}

func (e *events) find(kind string) *types.CloudEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := len(e.list) - 1; i >= 0; i-- {
		if e.list[i].Kind == kind {
			ev := e.list[i]
			return &ev
		}
	}
	return nil
}

var game = Game{ID: "p.echo.tiny", PluginID: "echo", StoreID: "tiny", Title: "Echo Tiny"}
var cctx = types.CloudContext{OS: "linux", Home: "/home/u", InstallDir: "/home/u/Games/echo/tiny"}

type rig struct {
	be     *fakeBackend
	ev     *events
	o      *Orchestrator
	sleeps []time.Duration
	state  string
	jobN   int
}

func newRig(t *testing.T, set fakeSettings) *rig {
	t.Helper()
	r := &rig{be: newBackend(), ev: &events{}, state: filepath.Join(t.TempDir(), "cloud.json")}
	r.o = r.build(set)
	t.Cleanup(func() { r.o.Shutdown(time.Second) })
	return r
}

var rigMu sync.Mutex

func (r *rig) build(set fakeSettings) *Orchestrator {
	return New(Options{
		Backend: r.be, Settings: set, StatePath: r.state, Emit: r.ev.emit,
		Sleep: func(ctx context.Context, d time.Duration) error {
			rigMu.Lock()
			r.sleeps = append(r.sleeps, d)
			rigMu.Unlock()
			return ctx.Err()
		},
		NewJobID: func() string {
			rigMu.Lock()
			defer rigMu.Unlock()
			r.jobN++
			return "job-" + string(rune('0'+r.jobN))
		},
	})
}

func status(state string, counts types.CloudCounts, extra ...func(*types.CloudStatus)) types.CloudStatus {
	st := types.CloudStatus{State: state, Counts: counts, Stamp: "s1"}
	for _, f := range extra {
		f(&st)
	}
	return st
}

func conflictStatus() types.CloudStatus {
	return status(types.CloudStateConflict, types.CloudCounts{Conflict: 1}, func(s *types.CloudStatus) {
		s.Message = "1 file changed on both sides"
		s.Conflicts = []types.CloudConflict{{Path: "save.dat",
			Local: types.CloudFile{Size: 10, Mtime: 100}, Remote: types.CloudFile{Size: 12, Mtime: 200, Machine: "Deck"}}}
	})
}

func TestPullHappensBeforeLaunchWhenTheCloudHasNewerFiles(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Download: 2, Same: 1}))
	r.be.answer("cloud.pull", status(types.CloudStateSynced, types.CloudCounts{Same: 3}))

	d := r.o.BeforeLaunch(context.Background(), game, cctx)
	if !d.Proceed || !d.Sync || d.Prompt != "" {
		t.Fatalf("decision %+v", d)
	}
	if got := r.be.methods(); !slices.Equal(got, []string{"cloud.status", "cloud.pull"}) {
		t.Fatalf("calls %v, want status then pull", got)
	}
	pull := r.be.last("cloud.pull")
	if pull.Params["id"] != "tiny" || pull.Params["jobId"] == "" || pull.Params["ctx"] == nil {
		t.Fatalf("pull params %+v", pull.Params)
	}
	if st := r.o.Last(game.ID); st == nil || st.State != types.CloudStateSynced {
		t.Fatalf("last status %+v", st)
	}
}

func TestACleanGameLaunchesWithoutAPull(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStateSynced, types.CloudCounts{Same: 3}))
	if d := r.o.BeforeLaunch(context.Background(), game, cctx); !d.Proceed || !d.Sync {
		t.Fatalf("%+v", d)
	}
	if r.be.count("cloud.pull") != 0 {
		t.Fatal("pulled although nothing changed in the cloud")
	}
}

func TestAConflictBlocksTheLaunchUntilTheUserChooses(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", conflictStatus())

	d := r.o.BeforeLaunch(context.Background(), game, cctx)
	if d.Proceed || d.Prompt != types.CloudPromptConflict || d.Status == nil || len(d.Status.Conflicts) != 1 {
		t.Fatalf("a conflict must block: %+v", d)
	}
	if ev := r.ev.find(types.CloudEvPrompt); ev == nil || ev.Prompt != types.CloudPromptConflict || ev.GameID != game.ID {
		t.Fatalf("no conflict prompt event: %v", r.ev.kinds())
	}
	if r.be.count("cloud.pull")+r.be.count("cloud.push") != 0 {
		t.Fatal("moved files while a conflict is open")
	}

	// The same launch again is blocked again: nothing was decided.
	if d := r.o.BeforeLaunch(context.Background(), game, cctx); d.Proceed {
		t.Fatal("the second attempt went through without a choice")
	}

	// The user resolves, and the launch goes through.
	r.be.answer("cloud.resolve", status(types.CloudStateSynced, types.CloudCounts{Same: 3}))
	r.be.answer("cloud.status", status(types.CloudStateSynced, types.CloudCounts{Same: 3}))
	if _, err := r.o.Resolve(context.Background(), game, cctx, map[string]string{"save.dat": "remote"}); err != nil {
		t.Fatal(err)
	}
	if ch := r.be.last("cloud.resolve").Params["choices"].(map[string]any); ch["save.dat"] != "remote" {
		t.Fatalf("choices %v", ch)
	}
	if d := r.o.BeforeLaunch(context.Background(), game, cctx); !d.Proceed {
		t.Fatalf("still blocked after resolving: %+v", d)
	}
}

func TestConflictAfterAPullStillBlocks(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Download: 1, Conflict: 1}))
	r.be.answer("cloud.pull", conflictStatus())
	d := r.o.BeforeLaunch(context.Background(), game, cctx)
	if d.Proceed || d.Prompt != types.CloudPromptConflict {
		t.Fatalf("%+v", d)
	}
	if r.be.count("cloud.pull") != 1 {
		t.Fatal("the files that changed only in the cloud should still be pulled")
	}
}

func TestAnUnreachableStoreAsksForConfirmation(t *testing.T) {
	for _, state := range []string{types.CloudStateOffline, types.CloudStateError} {
		r := newRig(t, fakeSettings{})
		r.be.answer("cloud.status", status(state, types.CloudCounts{}, func(s *types.CloudStatus) { s.Message = "no route to the store" }))
		d := r.o.BeforeLaunch(context.Background(), game, cctx)
		if d.Proceed || d.Prompt != types.CloudPromptOffline || d.Message != "no route to the store" {
			t.Fatalf("%s: %+v", state, d)
		}
		// "Launch anyway" lets exactly one launch through.
		r.o.Allow(game.ID, true)
		if d := r.o.BeforeLaunch(context.Background(), game, cctx); !d.Proceed {
			t.Fatalf("%s: launch anyway didn't launch: %+v", state, d)
		}
		before := len(r.be.methods())
		if d := r.o.BeforeLaunch(context.Background(), game, cctx); d.Proceed {
			t.Fatalf("%s: the confirmation outlived one launch in ask mode", state)
		}
		if len(r.be.methods()) == before {
			t.Fatalf("%s: the second launch skipped the cloud step", state)
		}
	}
}

func TestAutoModeStopsAskingOnceTheUserConfirmedAnUnreachableStore(t *testing.T) {
	r := newRig(t, fakeSettings{plugin: types.CloudAuto})
	r.be.answer("cloud.status", status(types.CloudStateOffline, types.CloudCounts{}, func(s *types.CloudStatus) { s.Message = "offline" }))
	if d := r.o.BeforeLaunch(context.Background(), game, cctx); d.Proceed {
		t.Fatal("auto mode went past an unreachable store before the user ever confirmed")
	}
	r.o.Allow(game.ID, true)
	r.o.BeforeLaunch(context.Background(), game, cctx) // consumes the one-shot
	d := r.o.BeforeLaunch(context.Background(), game, cctx)
	if !d.Proceed || r.ev.find(types.CloudEvSkipped) == nil {
		t.Fatalf("auto mode still asks after the confirmation: %+v %v", d, r.ev.kinds())
	}
	// The confirmation is remembered across restarts.
	again := r.build(fakeSettings{plugin: types.CloudAuto})
	if d := again.BeforeLaunch(context.Background(), game, cctx); !d.Proceed {
		t.Fatalf("confirmation lost on restart: %+v", d)
	}
	// ... and conflicts are never skipped.
	r.be.answer("cloud.status", conflictStatus())
	if d := r.o.BeforeLaunch(context.Background(), game, cctx); d.Proceed {
		t.Fatal("auto mode launched over a conflict")
	}
}

func TestAPluginErrorIsTreatedLikeAnUnreachableStore(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.handlers["cloud.status"] = func(context.Context, map[string]any) (any, error) { return nil, errors.New("the plugin crashed") }
	d := r.o.BeforeLaunch(context.Background(), game, cctx)
	if d.Proceed || d.Prompt != types.CloudPromptOffline || d.Message == "" {
		t.Fatalf("%+v", d)
	}
}

func TestAnUnknownStateIsAnError(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", map[string]any{"state": "wobbly"})
	if d := r.o.BeforeLaunch(context.Background(), game, cctx); d.Proceed || d.Prompt != types.CloudPromptOffline {
		t.Fatalf("%+v", d)
	}
}

func TestUnsupportedGamesLaunchAsIsAndAreNotSynced(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStateUnsupported, types.CloudCounts{}, func(s *types.CloudStatus) { s.Message = "no Auto-Cloud rules" }))
	d := r.o.BeforeLaunch(context.Background(), game, cctx)
	if !d.Proceed || d.Sync {
		t.Fatalf("%+v", d)
	}
}

func TestModeOffMakesNoCalls(t *testing.T) {
	for _, set := range []fakeSettings{{plugin: types.CloudOff}, {game: types.CloudOff, plugin: types.CloudAuto}} {
		r := newRig(t, set)
		d := r.o.BeforeLaunch(context.Background(), game, cctx)
		if !d.Proceed || d.Sync {
			t.Fatalf("%+v", d)
		}
		r.o.Launching(game, cctx)
		r.o.Exited(game.ID, false)
		r.o.Shutdown(time.Second)
		if len(r.be.methods()) != 0 {
			t.Fatalf("off mode called the plugin: %v", r.be.methods())
		}
	}
}

func TestAGameOverrideBeatsThePluginDefault(t *testing.T) {
	r := newRig(t, fakeSettings{plugin: types.CloudOff, game: types.CloudAuto})
	if m := r.o.Mode(game); m != types.CloudAuto {
		t.Fatal(m)
	}
	r = newRig(t, fakeSettings{plugin: "nonsense"})
	if m := r.o.Mode(game); m != DefaultMode {
		t.Fatalf("an unusable saved mode should fall back to %s, got %s", DefaultMode, m)
	}
}

func TestWithoutTheCapabilityNothingIsCalled(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.offers = false
	if d := r.o.BeforeLaunch(context.Background(), game, cctx); !d.Proceed || d.Sync {
		t.Fatalf("%+v", d)
	}
	if _, err := r.o.Check(context.Background(), game, cctx); err == nil {
		t.Fatal("Check succeeded for a plugin without cloud saves")
	}
	if _, err := r.o.Resolve(context.Background(), game, cctx, map[string]string{"a": "local"}); err == nil {
		t.Fatal("Resolve succeeded for a plugin without cloud saves")
	}
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, false)
	r.o.Shutdown(time.Second)
	if len(r.be.methods()) != 0 {
		t.Fatalf("the plugin was called: %v", r.be.methods())
	}
}

func TestTheOfficialClientRunningStopsTheSync(t *testing.T) {
	running := func(can bool) func(*types.CloudStatus) {
		return func(s *types.CloudStatus) {
			s.Client = &types.CloudClient{Running: true, Name: "Steam", CanLaunch: can}
		}
	}
	t.Run("can launch through the client", func(t *testing.T) {
		r := newRig(t, fakeSettings{})
		r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Download: 1}, running(true)))
		d := r.o.BeforeLaunch(context.Background(), game, cctx)
		if d.Proceed || d.Prompt != types.CloudPromptClient {
			t.Fatalf("%+v", d)
		}
		if ev := r.ev.find(types.CloudEvPrompt); ev == nil || ev.Client != "Steam" {
			t.Fatalf("the prompt must name the client: %+v", ev)
		}
		if r.be.count("cloud.pull") != 0 {
			t.Fatal("pulled while the client runs")
		}
		r.o.Allow(game.ID, false) // "launch without syncing"
		if d := r.o.BeforeLaunch(context.Background(), game, cctx); !d.Proceed {
			t.Fatalf("%+v", d)
		}
	})
	t.Run("cannot", func(t *testing.T) {
		r := newRig(t, fakeSettings{})
		r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Download: 1}, running(false)))
		d := r.o.BeforeLaunch(context.Background(), game, cctx)
		if !d.Proceed || d.Sync {
			t.Fatalf("the game should launch unsynced with a banner: %+v", d)
		}
		ev := r.ev.find(types.CloudEvSkipped)
		if ev == nil || ev.Client != "Steam" || ev.Message == "" {
			t.Fatalf("no explanation: %v", r.ev.kinds())
		}
		if r.be.count("cloud.pull") != 0 {
			t.Fatal("pulled while the client runs")
		}
	})
	t.Run("after exit", func(t *testing.T) {
		r := newRig(t, fakeSettings{})
		r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Upload: 1}, running(true)))
		r.o.Launching(game, cctx)
		r.o.Exited(game.ID, false)
		r.o.Wait()
		if r.be.count("cloud.push") != 0 {
			t.Fatal("pushed while the client runs")
		}
		if r.ev.find(types.CloudEvSkipped) == nil || len(r.o.PendingPushes()) != 0 {
			t.Fatalf("%v %v", r.ev.kinds(), r.o.PendingPushes())
		}
	})
}

func TestPushHappensAfterExitOnceTheFilesAreStable(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Upload: 2, Same: 1}))
	r.be.answer("cloud.push", status(types.CloudStateSynced, types.CloudCounts{Same: 3}))

	r.o.Launching(game, cctx)
	if p := r.o.PendingPushes(); len(p) != 1 || p[0].GameID != game.ID {
		t.Fatalf("no push recorded when the game launched: %+v", p)
	}
	r.o.Exited(game.ID, false)
	r.o.Wait()

	if got := r.be.methods(); !slices.Equal(got, []string{"cloud.status", "cloud.status", "cloud.push"}) {
		t.Fatalf("calls %v, want two readings then the push", got)
	}
	if !slices.Equal(r.sleeps, []time.Duration{2 * time.Second}) {
		t.Fatalf("sleeps %v, want one 2 s pause between the two readings", r.sleeps)
	}
	ev := r.ev.find(types.CloudEvPushed)
	if ev == nil || ev.Message != "Saved 2 files to the cloud" || ev.Status == nil {
		t.Fatalf("no pushed event: %v %+v", r.ev.kinds(), ev)
	}
	if len(r.o.PendingPushes()) != 0 {
		t.Fatal("the push is still owed after it completed")
	}
}

func TestAGameThatKeepsWritingDelaysThePush(t *testing.T) {
	r := newRig(t, fakeSettings{})
	n := 0
	r.be.handlers["cloud.status"] = func(context.Context, map[string]any) (any, error) {
		n++
		stamp := "settled"
		if n <= 3 {
			stamp = string(rune('a' + n)) // the files change on each of the first three readings
		}
		return types.CloudStatus{State: types.CloudStatePending, Counts: types.CloudCounts{Upload: 1}, Stamp: stamp}, nil
	}
	r.be.answer("cloud.push", status(types.CloudStateSynced, types.CloudCounts{Same: 1}))
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, false)
	r.o.Wait()
	// Readings: a, b, c, settled, settled -> stable on the fifth.
	if r.be.count("cloud.status") != 5 || r.be.count("cloud.push") != 1 || len(r.sleeps) != 4 {
		t.Fatalf("status %d, push %d, sleeps %v", r.be.count("cloud.status"), r.be.count("cloud.push"), r.sleeps)
	}
}

func TestWithoutAStampTheCountsDecideStability(t *testing.T) {
	r := newRig(t, fakeSettings{})
	n := 0
	r.be.handlers["cloud.status"] = func(context.Context, map[string]any) (any, error) {
		n++
		return types.CloudStatus{State: types.CloudStatePending, Counts: types.CloudCounts{Upload: min(n, 2)}}, nil
	}
	r.be.answer("cloud.push", status(types.CloudStateSynced, types.CloudCounts{Same: 2}))
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, false)
	r.o.Wait()
	if r.be.count("cloud.status") != 3 || r.be.count("cloud.push") != 1 {
		t.Fatalf("status %d, push %d", r.be.count("cloud.status"), r.be.count("cloud.push"))
	}
}

func TestAFilesThatNeverSettleAreNotPushed(t *testing.T) {
	r := newRig(t, fakeSettings{})
	n := 0
	r.be.handlers["cloud.status"] = func(context.Context, map[string]any) (any, error) {
		n++
		return types.CloudStatus{State: types.CloudStatePending, Counts: types.CloudCounts{Upload: 1}, Stamp: string(rune('a' + n))}, nil
	}
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, false)
	r.o.Wait()
	if r.be.count("cloud.push") != 0 {
		t.Fatal("pushed files that were still changing")
	}
	if r.ev.find(types.CloudEvFailed) == nil || len(r.o.PendingPushes()) != 1 {
		t.Fatalf("the push should stay owed and tell the user: %v %v", r.ev.kinds(), r.o.PendingPushes())
	}
}

func TestNothingToUploadMeansNoPush(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStateSynced, types.CloudCounts{Same: 3}))
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, false)
	r.o.Wait()
	if r.be.count("cloud.push") != 0 || len(r.o.PendingPushes()) != 0 {
		t.Fatalf("%v %v", r.be.methods(), r.o.PendingPushes())
	}
}

func TestAConflictAfterExitIsReportedNotPushed(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", conflictStatus())
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, false)
	r.o.Wait()
	ev := r.ev.find(types.CloudEvFailed)
	if r.be.count("cloud.push") != 0 || ev == nil || ev.Status == nil || len(ev.Status.Conflicts) != 1 {
		t.Fatalf("%v %+v", r.be.methods(), ev)
	}
}

func TestAFailedLaunchPushesNothing(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, true)
	r.o.Shutdown(time.Second)
	if len(r.be.methods()) != 0 || len(r.o.PendingPushes()) != 0 {
		t.Fatalf("%v %v", r.be.methods(), r.o.PendingPushes())
	}
}

func TestAnAbandonedLaunchOwesNothing(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.o.Launching(game, cctx)
	r.o.Abandon(game.ID)
	if len(r.o.PendingPushes()) != 0 {
		t.Fatal("still owed")
	}
}

func TestAPushInterruptedByHydraQuittingRunsAtTheNextStart(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Upload: 1}))
	r.o.Launching(game, cctx)

	// Hydra quits while the game runs: the game's exit arrives during shutdown.
	r.o.Shutdown(time.Second)
	r.o.Exited(game.ID, false)
	if len(r.be.methods()) != 0 {
		t.Fatalf("started a sync during shutdown: %v", r.be.methods())
	}
	owed := r.o.PendingPushes()
	if len(owed) != 1 {
		t.Fatalf("the push wasn't kept: %+v", owed)
	}

	// Next start.
	r.be.answer("cloud.push", status(types.CloudStateSynced, types.CloudCounts{Same: 1}))
	next := r.build(fakeSettings{})
	if got := next.PendingPushes(); len(got) != 1 || got[0].GameID != game.ID || got[0].Ctx.InstallDir != cctx.InstallDir {
		t.Fatalf("the owed push didn't survive a restart: %+v", got)
	}
	next.Resume()
	next.Wait()
	if r.be.count("cloud.push") != 1 {
		t.Fatalf("calls %v", r.be.methods())
	}
	if len(next.PendingPushes()) != 0 {
		t.Fatal("still owed after it ran")
	}
	var saved fileState
	data := mustRead(t, r.state)
	if err := json.Unmarshal(data, &saved); err != nil || len(saved.Pending) != 0 {
		t.Fatalf("state file %s: %v", data, err)
	}
}

func TestAnInterruptedPushIsResentWithTheSameJobIDButAFailedOneIsNot(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Upload: 1}))
	var pushes []string
	blocked := make(chan struct{})
	r.be.handlers["cloud.push"] = func(ctx context.Context, p map[string]any) (any, error) {
		pushes = append(pushes, p["jobId"].(string))
		if len(pushes) == 1 {
			close(blocked)
			<-ctx.Done() // Hydra quits mid-push
			return nil, ctx.Err()
		}
		return status(types.CloudStateOffline, types.CloudCounts{}, func(s *types.CloudStatus) { s.Message = "offline" }), nil
	}
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, false)
	<-blocked
	r.o.Shutdown(50 * time.Millisecond)

	next := r.build(fakeSettings{})
	next.Resume()
	next.Wait() // second push: offline -> still owed, with a new id
	if len(pushes) != 2 || pushes[0] != pushes[1] {
		t.Fatalf("an interrupted push must be resent under its job id: %v", pushes)
	}
	owed := next.PendingPushes()
	if len(owed) != 1 || owed[0].JobID == pushes[1] || owed[0].Started {
		t.Fatalf("a push that ended offline needs a fresh job id next time: %+v (used %v)", owed, pushes)
	}
}

func TestCancelStopsAPushAndKeepsItOwed(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Upload: 1}))
	started := make(chan string, 1)
	r.be.handlers["cloud.push"] = func(ctx context.Context, p map[string]any) (any, error) {
		started <- p["jobId"].(string)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	cancelled := make(chan string, 1)
	r.be.handlers["job.cancel"] = func(_ context.Context, p map[string]any) (any, error) {
		cancelled <- p["jobId"].(string)
		return map[string]any{}, nil
	}
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, false)
	id := <-started
	if info := r.o.Info(game); info.Busy != "push" {
		t.Fatalf("the game should be busy: %+v", info)
	}
	if !r.o.Cancel(game.ID) {
		t.Fatal("Cancel found no job")
	}
	select {
	case got := <-cancelled:
		if got != id {
			t.Fatalf("job.cancel for %q, want %q", got, id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the plugin was never told job.cancel")
	}
	r.o.Wait()
	if r.ev.find(types.CloudEvCanceled) == nil {
		t.Fatalf("no canceled event: %v", r.ev.kinds())
	}
	if len(r.o.PendingPushes()) != 1 {
		t.Fatal("a canceled push should stay owed")
	}
	if r.o.Cancel(game.ID) {
		t.Fatal("Cancel found a job that had ended")
	}
}

func TestCancelDuringThePreLaunchPullBlocksTheLaunchWithAnExplanation(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Download: 1}))
	r.be.answer("job.cancel", map[string]any{})
	started := make(chan struct{})
	r.be.handlers["cloud.pull"] = func(ctx context.Context, p map[string]any) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	done := make(chan Decision, 1)
	go func() { done <- r.o.BeforeLaunch(context.Background(), game, cctx) }()
	<-started
	if !r.o.Cancel(game.ID) {
		t.Fatal("no job to cancel")
	}
	d := <-done
	if d.Proceed || d.Prompt != types.CloudPromptOffline || d.Message == "" {
		t.Fatalf("%+v", d)
	}
	if r.ev.find(types.CloudEvCanceled) == nil {
		t.Fatalf("%v", r.ev.kinds())
	}
}

func TestLaunchingAgainEndsAPostExitSyncStillWaiting(t *testing.T) {
	r := newRig(t, fakeSettings{})
	reading := make(chan struct{}, 1)
	r.be.handlers["cloud.status"] = func(ctx context.Context, p map[string]any) (any, error) {
		select {
		case reading <- struct{}{}:
		default:
		}
		return types.CloudStatus{State: types.CloudStatePending, Counts: types.CloudCounts{Upload: 1}, Stamp: time.Now().String()}, nil
	}
	r.be.answer("cloud.push", status(types.CloudStateSynced, types.CloudCounts{Same: 1}))
	r.be.answer("job.cancel", map[string]any{})
	r.o = New(Options{Backend: r.be, Settings: fakeSettings{}, Emit: r.ev.emit, StatePath: r.state,
		Sleep: func(ctx context.Context, d time.Duration) error { <-ctx.Done(); return ctx.Err() }}) // waits for the cancel
	r.o.Launching(game, cctx)
	r.o.Exited(game.ID, false)
	<-reading
	r.be.answer("cloud.status", status(types.CloudStateSynced, types.CloudCounts{Same: 1}))
	d := r.o.BeforeLaunch(context.Background(), game, cctx)
	if !d.Proceed {
		t.Fatalf("relaunch while the old sync was waiting: %+v", d)
	}
	r.o.Wait()
	if r.be.count("cloud.push") != 0 {
		t.Fatal("the abandoned post-exit sync pushed")
	}
}

func TestProgressReachesTheUIOnlyForRunningJobs(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", status(types.CloudStatePending, types.CloudCounts{Download: 1}))
	var jobID string
	r.be.handlers["cloud.pull"] = func(ctx context.Context, p map[string]any) (any, error) {
		jobID = p["jobId"].(string)
		raw, _ := json.Marshal(types.CloudProgress{ID: "tiny", JobID: jobID, Phase: "download", Done: 1, Total: 4})
		r.o.Progress("echo", raw)
		other, _ := json.Marshal(types.CloudProgress{ID: "tiny", JobID: "not-mine", Phase: "download", Done: 1, Total: 4})
		r.o.Progress("echo", other)
		wrongPlugin, _ := json.Marshal(types.CloudProgress{ID: "tiny", JobID: jobID, Phase: "download", Done: 2, Total: 4})
		r.o.Progress("other-plugin", wrongPlugin)
		return status(types.CloudStateSynced, types.CloudCounts{Same: 1}), nil
	}
	r.o.BeforeLaunch(context.Background(), game, cctx)
	var progress []types.CloudEvent
	r.ev.mu.Lock()
	for _, e := range r.ev.list {
		if e.Kind == types.CloudEvProgress {
			progress = append(progress, e)
		}
	}
	r.ev.mu.Unlock()
	if len(progress) != 1 || progress[0].GameID != game.ID || progress[0].Phase != "download" || progress[0].Done != 1 || progress[0].Total != 4 {
		t.Fatalf("%+v", progress)
	}
}

func TestResolveRefusesChoicesTheLastStatusDidNotList(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", conflictStatus())
	if _, err := r.o.Check(context.Background(), game, cctx); err != nil {
		t.Fatal(err)
	}
	for name, ch := range map[string]map[string]string{
		"unknown path": {"other.dat": "local"},
		"bad value":    {"save.dat": "both"},
		"empty":        {},
	} {
		if _, err := r.o.Resolve(context.Background(), game, cctx, ch); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if r.be.count("cloud.resolve") != 0 {
		t.Fatal("a refused choice reached the plugin")
	}
}

func TestOnlyOneJobRunsPerGame(t *testing.T) {
	r := newRig(t, fakeSettings{})
	r.be.answer("cloud.status", conflictStatus())
	r.o.Check(context.Background(), game, cctx)
	release := make(chan struct{})
	started := make(chan struct{})
	r.be.handlers["cloud.resolve"] = func(ctx context.Context, p map[string]any) (any, error) {
		close(started)
		<-release
		return status(types.CloudStateSynced, types.CloudCounts{Same: 1}), nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := r.o.Resolve(context.Background(), game, cctx, map[string]string{"save.dat": "local"})
		done <- err
	}()
	<-started
	if _, err := r.o.Resolve(context.Background(), game, cctx, map[string]string{"save.dat": "local"}); !errors.Is(err, ErrBusy) {
		t.Fatalf("a second job ran: %v", err)
	}
	if d := r.o.BeforeLaunch(context.Background(), game, cctx); d.Proceed {
		t.Fatalf("launched while a resolve was running: %+v", d)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestContextBuildsNativeAndWindowsLaunches(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	n := Context(false, "/g/tiny", "/ignored", "ignored")
	if n.OS != "linux" || n.Home != "/home/u" || n.XDGData != "/home/u/.local/share" || n.XDGConfig != "/cfg" || n.InstallDir != "/g/tiny" || n.PrefixDir != "" || n.WinUser != "" {
		t.Fatalf("native: %+v", n)
	}
	w := Context(true, "/g/tiny", "/p/pfx", "steamuser")
	if w.OS != "windows" || w.PrefixDir != "/p/pfx" || w.WinUser != "steamuser" {
		t.Fatalf("windows: %+v", w)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
