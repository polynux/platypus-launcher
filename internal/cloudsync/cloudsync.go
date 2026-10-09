// Package cloudsync decides when a game's cloud saves are synced and what the
// user is asked, and leaves the how to the store plugin (cloud.* requests,
// docs/plugins/protocol.md "Cloud saves"): a pull before Hydra launches a game
// itself, a push after it exits, the conflict flow, and the rule that Hydra
// does not sync while the store's own client is running.
//
// The package talks to plugins through the narrow Backend interface, so it is
// tested with a fake and used with the plugin manager.
package cloudsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"hydra-launcher/internal/types"
)

// Game is the part of a library game the orchestrator needs.
type Game struct {
	ID       string // Hydra's game id, p.<plugin>.<store id>
	PluginID string
	StoreID  string // the plugin's id for the game
	Title    string
}

// Backend reaches the plugin that owns a game.
type Backend interface {
	// Offers reports whether the plugin offers cloud saves right now: the
	// capability is on and the negotiated protocol is 1.2 or later. It may
	// start the plugin to find out.
	Offers(ctx context.Context, pluginID string) bool
	// Call sends one request to the plugin.
	Call(ctx context.Context, pluginID, method string, params, result any) error
}

// Settings holds the user's choices.
type Settings interface {
	CloudPluginMode(pluginID string) string // "" when never chosen
	CloudGameMode(gameID string) string     // "" when the game follows its plugin
}

// DefaultMode applies when neither the game nor its plugin has a mode: sync
// clean cases on its own, ask on a conflict.
const DefaultMode = types.CloudAsk

// Options configure an Orchestrator.
type Options struct {
	Backend  Backend
	Settings Settings
	// StatePath is where pending pushes and confirmations survive a restart;
	// empty keeps them in memory only.
	StatePath string
	// Emit receives what the UI should show; nil drops events.
	Emit func(types.CloudEvent)
	// Logf receives diagnostics: ids and counts, never paths or credentials.
	Logf func(format string, args ...any)

	// StableDelay is the pause between the readings that must agree before a
	// push (default 2 s). StablePasses bounds the readings (default 15).
	StableDelay  time.Duration
	StablePasses int
	// Sleep waits for d or until ctx ends; tests replace it.
	Sleep func(ctx context.Context, d time.Duration) error
	// StatusTimeout and JobTimeout bound one cloud.status and one
	// pull/push/resolve (defaults 1 min and 15 min).
	StatusTimeout time.Duration
	JobTimeout    time.Duration
	// NewJobID makes job ids; the default is random.
	NewJobID func() string
}

// Errors the orchestrator returns.
var (
	ErrBusy     = errors.New("cloud saves are already being synced for this game")
	ErrCanceled = errors.New("cloud sync was canceled")
)

// Decision is the answer to "may this game launch?".
type Decision struct {
	// Proceed: launch now. Otherwise Prompt says what the user must decide
	// (a "prompt" event with the same content was emitted).
	Proceed bool
	Prompt  string
	Message string
	Status  *types.CloudStatus
	// Sync: the launch is followed by a post-exit push. Call Launching before
	// starting the game.
	Sync bool
}

type job struct {
	id       string
	kind     string // "pull", "push", "resolve"
	pluginID string
	cancel   context.CancelFunc
	canceled bool
	done     chan struct{} // closed when the job has ended
}

// Pending is a push owed for a game that was launched: it survives Hydra
// quitting while the game ran.
type Pending struct {
	GameID   string             `json:"game_id"`
	PluginID string             `json:"plugin_id"`
	StoreID  string             `json:"store_id"`
	Title    string             `json:"title"`
	JobID    string             `json:"job_id"`
	Started  bool               `json:"started"` // the push with JobID was sent; it may have run
	Ctx      types.CloudContext `json:"ctx"`
	Since    int64              `json:"since"`
}

type fileState struct {
	Version int       `json:"version"`
	Pending []Pending `json:"pending"`
	// Confirmed are the games whose user said "launch anyway" while the store
	// was unreachable; `auto` mode stops asking for them.
	Confirmed map[string]bool `json:"confirmed,omitempty"`
}

// Orchestrator runs the cloud sync policy. It is safe for concurrent use.
type Orchestrator struct {
	o      Options
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	closing  bool
	last     map[string]types.CloudStatus
	jobs     map[string]*job // by game id: at most one job per game
	allow    map[string]bool // one-shot "launch anyway"
	progress map[string]time.Time
	state    fileState
	wg       sync.WaitGroup
}

// New returns an orchestrator, loading the saved state.
func New(o Options) *Orchestrator {
	if o.StableDelay == 0 {
		o.StableDelay = 2 * time.Second
	}
	if o.StablePasses == 0 {
		o.StablePasses = 15
	}
	if o.StatusTimeout == 0 {
		o.StatusTimeout = time.Minute
	}
	if o.JobTimeout == 0 {
		o.JobTimeout = 15 * time.Minute
	}
	if o.Sleep == nil {
		o.Sleep = sleep
	}
	if o.NewJobID == nil {
		o.NewJobID = newJobID
	}
	c := &Orchestrator{
		o: o, last: map[string]types.CloudStatus{}, jobs: map[string]*job{},
		allow: map[string]bool{}, progress: map[string]time.Time{},
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.load()
	return c
}

func (c *Orchestrator) logf(format string, args ...any) {
	if c.o.Logf != nil {
		c.o.Logf(format, args...)
	}
}

func (c *Orchestrator) emit(e types.CloudEvent) {
	if c.o.Emit != nil {
		c.o.Emit(e)
	}
}

// Mode is the effective sync mode for a game: its own choice, else its
// plugin's default, else DefaultMode.
func (c *Orchestrator) Mode(g Game) string {
	if m := c.o.Settings.CloudGameMode(g.ID); types.ValidCloudMode(m) {
		return m
	}
	if m := c.o.Settings.CloudPluginMode(g.PluginID); types.ValidCloudMode(m) {
		return m
	}
	return DefaultMode
}

// Info is what the game page shows without asking the plugin anything.
func (c *Orchestrator) Info(g Game) types.CloudInfo {
	info := types.CloudInfo{Mode: c.Mode(g), GameMode: c.o.Settings.CloudGameMode(g.ID)}
	info.PluginMode = c.o.Settings.CloudPluginMode(g.PluginID)
	if !types.ValidCloudMode(info.PluginMode) {
		info.PluginMode = DefaultMode
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if st, ok := c.last[g.ID]; ok {
		st := st
		info.Status = &st
	}
	if j := c.jobs[g.ID]; j != nil {
		info.Busy = j.kind
	}
	return info
}

// Last is the most recent status this session, nil before the first check.
func (c *Orchestrator) Last(gameID string) *types.CloudStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	if st, ok := c.last[gameID]; ok {
		return &st
	}
	return nil
}

// ---- status ----

func (c *Orchestrator) remember(gameID string, st types.CloudStatus) {
	c.mu.Lock()
	c.last[gameID] = st
	c.mu.Unlock()
}

// status asks the plugin for the game's cloud state (read-only).
func (c *Orchestrator) status(ctx context.Context, g Game, cc types.CloudContext) (types.CloudStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, c.o.StatusTimeout)
	defer cancel()
	var st types.CloudStatus
	if err := c.o.Backend.Call(ctx, g.PluginID, "cloud.status", map[string]any{"id": g.StoreID, "ctx": cc}, &st); err != nil {
		if cerr := ctx.Err(); cerr != nil && errors.Is(err, cerr) {
			return types.CloudStatus{}, fmt.Errorf("the store didn't answer in time: %w", cerr)
		}
		return types.CloudStatus{}, err
	}
	st.Normalize()
	c.remember(g.ID, st)
	return st, nil
}

// Check is "Check now": a read-only comparison, remembered and shown.
func (c *Orchestrator) Check(ctx context.Context, g Game, cc types.CloudContext) (types.CloudStatus, error) {
	if !c.o.Backend.Offers(ctx, g.PluginID) {
		return types.CloudStatus{}, errors.New("this plugin doesn't offer cloud saves")
	}
	st, err := c.status(ctx, g, cc)
	if err != nil {
		return st, err
	}
	c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvStatus, Status: &st})
	return st, nil
}

// sameReading says whether two readings describe the same local files.
func sameReading(a, b types.CloudStatus) bool {
	if a.Stamp != "" || b.Stamp != "" {
		return a.Stamp == b.Stamp && a.State == b.State
	}
	if a.State != b.State || a.Counts != b.Counts || len(a.Conflicts) != len(b.Conflicts) {
		return false
	}
	ja, _ := json.Marshal(a.Conflicts)
	jb, _ := json.Marshal(b.Conflicts)
	return string(ja) == string(jb)
}

// ---- jobs ----

func (c *Orchestrator) begin(parent context.Context, g Game, kind, jobID string) (context.Context, *job, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return nil, nil, ErrCanceled
	}
	if c.jobs[g.ID] != nil {
		return nil, nil, ErrBusy
	}
	ctx, cancel := context.WithCancel(parent)
	if jobID == "" {
		jobID = c.o.NewJobID()
	}
	j := &job{id: jobID, kind: kind, pluginID: g.PluginID, cancel: cancel, done: make(chan struct{})}
	c.jobs[g.ID] = j
	c.wg.Add(1)
	return ctx, j, nil
}

func (c *Orchestrator) end(g Game, j *job) {
	j.cancel()
	c.mu.Lock()
	if c.jobs[g.ID] == j {
		delete(c.jobs, g.ID)
	}
	delete(c.progress, g.ID)
	c.mu.Unlock()
	close(j.done)
	c.wg.Done()
}

// call runs one pull/push/resolve request for a job begun with begin.
func (c *Orchestrator) call(ctx context.Context, j *job, g Game, cc types.CloudContext, method string, extra map[string]any) (types.CloudStatus, error) {
	params := map[string]any{"id": g.StoreID, "ctx": cc, "jobId": j.id}
	for k, v := range extra {
		params[k] = v
	}
	rctx, cancel := context.WithTimeout(ctx, c.o.JobTimeout)
	defer cancel()
	var st types.CloudStatus
	err := c.o.Backend.Call(rctx, g.PluginID, method, params, &st)
	c.mu.Lock()
	canceled := j.canceled
	c.mu.Unlock()
	if canceled || ctx.Err() != nil {
		return types.CloudStatus{}, ErrCanceled
	}
	if err != nil {
		return types.CloudStatus{}, err
	}
	st.Normalize()
	c.remember(g.ID, st)
	return st, nil
}

// Cancel stops the game's running cloud job, if any, and tells the plugin
// (job.cancel) so it stops working too. It reports whether there was one.
func (c *Orchestrator) Cancel(gameID string) bool {
	c.mu.Lock()
	j := c.jobs[gameID]
	if j != nil {
		j.canceled = true
	}
	c.mu.Unlock()
	if j == nil {
		return false
	}
	j.cancel()
	c.cancelAtPlugin(j)
	return true
}

func (c *Orchestrator) cancelAtPlugin(j *job) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.o.Backend.Call(ctx, j.pluginID, "job.cancel", map[string]any{"jobId": j.id}, nil)
	}()
}

// Progress routes a plugin's cloud.progress notification to the UI. Reports
// for a job Hydra isn't running are dropped.
func (c *Orchestrator) Progress(pluginID string, raw json.RawMessage) {
	var p types.CloudProgress
	if json.Unmarshal(raw, &p) != nil || p.JobID == "" {
		return
	}
	c.mu.Lock()
	gameID := ""
	for id, j := range c.jobs {
		if j.id == p.JobID && j.pluginID == pluginID {
			gameID = id
		}
	}
	// A burst of notifications costs the UI nothing but the last one: the end of a phase always goes out.
	now := time.Now()
	if gameID == "" || (p.Done < p.Total && now.Sub(c.progress[gameID]) < 100*time.Millisecond) {
		c.mu.Unlock()
		return
	}
	c.progress[gameID] = now
	c.mu.Unlock()
	c.emit(types.CloudEvent{GameID: gameID, Kind: types.CloudEvProgress, JobID: p.JobID, Phase: p.Phase, Done: p.Done, Total: p.Total})
}

// Resolve settles conflicts with the user's choices (path → "local" or
// "remote"); paths left out stay conflicts.
func (c *Orchestrator) Resolve(ctx context.Context, g Game, cc types.CloudContext, choices map[string]string) (types.CloudStatus, error) {
	if len(choices) == 0 {
		return types.CloudStatus{}, errors.New("nothing was chosen")
	}
	for path, v := range choices {
		if path == "" || (v != "local" && v != "remote") {
			return types.CloudStatus{}, fmt.Errorf("the choice for %q must be local or remote", path)
		}
	}
	if known := c.Last(g.ID); known != nil && len(known.Conflicts) > 0 {
		listed := map[string]bool{}
		for _, cf := range known.Conflicts {
			listed[cf.Path] = true
		}
		for path := range choices {
			if !listed[path] {
				return types.CloudStatus{}, fmt.Errorf("%q isn't one of the conflicts; check again first", path)
			}
		}
	}
	if !c.o.Backend.Offers(ctx, g.PluginID) {
		return types.CloudStatus{}, errors.New("this plugin doesn't offer cloud saves")
	}
	jctx, j, err := c.begin(ctx, g, "resolve", "")
	if err != nil {
		return types.CloudStatus{}, err
	}
	defer c.end(g, j)
	st, err := c.call(jctx, j, g, cc, "cloud.resolve", map[string]any{"choices": choices})
	if err != nil {
		return st, err
	}
	c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvStatus, Status: &st})
	return st, nil
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
