package cloudsync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"platypus/internal/types"
)

// BeforeLaunch runs the pre-launch step for a game Platypus is about to start
// itself. It reads the cloud state, stops there when the store's own client is
// running, pulls what changed only in the cloud, and tells the caller whether
// to go on. When the user has to decide (a conflict, an unreachable store, the
// client) the decision carries the prompt and a "prompt" event went out; the
// user's answer comes back as another launch with anyway set ("launch anyway":
// the cloud step is skipped for this launch only).
func (c *Orchestrator) BeforeLaunch(ctx context.Context, g Game, cc types.CloudContext, anyway bool) Decision {
	d := c.beforeLaunch(ctx, g, cc, anyway)
	// The prompt goes out once the job is over, so the UI's answer (resolve,
	// another launch) never meets a busy game.
	if !d.Proceed && d.Prompt != "" {
		ev := types.CloudEvent{GameID: g.ID, Kind: types.CloudEvPrompt, Prompt: d.Prompt, Message: d.Message, Status: d.Status, Title: g.Title}
		if d.Status != nil && d.Status.Client != nil {
			ev.Client = d.Status.Client.Name
		}
		c.emit(ev)
	}
	return d
}

func (c *Orchestrator) beforeLaunch(ctx context.Context, g Game, cc types.CloudContext, anyway bool) Decision {
	mode := c.Mode(g)
	if mode == types.CloudOff || !c.o.Backend.Offers(ctx, g.PluginID) {
		return Decision{Proceed: true}
	}
	// A launch ends a post-exit sync still waiting for the game's files to settle.
	c.mu.Lock()
	push := c.jobs[g.ID]
	if push != nil && push.kind != "push" {
		push = nil
	}
	if push != nil {
		push.canceled = true
		push.cancel()
	}
	c.mu.Unlock()
	if push != nil {
		c.cancelAtPlugin(push)
		select {
		case <-push.done:
		case <-time.After(3 * time.Second):
		}
	}

	// Claim the game first, also for "launch anyway": a push that is still
	// running holds its files, and the launch must not start over it.
	jctx, j, err := c.begin(ctx, g, "pull", "")
	if err != nil {
		return c.prompt(g, types.CloudPromptOffline, "Cloud saves are still being synced for this game. Try again in a moment.", nil)
	}
	defer c.end(g, j)
	if anyway {
		return Decision{Proceed: true, Sync: true}
	}

	st, err := c.status(jctx, g, cc)
	if err != nil {
		return c.failed(g, mode, err, nil)
	}
	if cl := st.Client; cl != nil && cl.Running {
		return c.clientRunning(g, st)
	}
	switch st.State {
	case types.CloudStateUnsupported:
		c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvStatus, Status: &st})
		return Decision{Proceed: true, Status: &st}
	case types.CloudStateOffline, types.CloudStateError:
		return c.unreachable(g, mode, st)
	}
	note := ""
	if st.Counts.Download > 0 {
		downloads := st.Counts.Download
		pulled, err := c.call(jctx, j, g, cc, "cloud.pull", nil)
		if errors.Is(err, ErrCanceled) {
			c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvCanceled, Message: "Cloud sync canceled."})
			return c.prompt(g, types.CloudPromptOffline, "Cloud sync was canceled, so this game's saves may be out of date.", &st)
		}
		if err != nil {
			return c.failed(g, mode, err, &st)
		}
		st = pulled
		if cl := st.Client; cl != nil && cl.Running {
			return c.clientRunning(g, st)
		}
		if st.State == types.CloudStateOffline || st.State == types.CloudStateError {
			return c.unreachable(g, mode, st)
		}
		note = fmt.Sprintf("Downloaded %d file%s from the cloud", downloads, plural(downloads))
	}
	c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvStatus, Status: &st, Message: note})
	if st.State == types.CloudStateConflict {
		return c.prompt(g, types.CloudPromptConflict, st.Message, &st)
	}
	return Decision{Proceed: true, Sync: true, Status: &st}
}

func (c *Orchestrator) prompt(g Game, kind, msg string, st *types.CloudStatus) Decision {
	return Decision{Prompt: kind, Message: msg, Status: st}
}

// failed handles a request that ended in an error: the same as an unreachable store.
func (c *Orchestrator) failed(g Game, mode string, err error, st *types.CloudStatus) Decision {
	msg := "Cloud saves couldn't be checked right now."
	if st == nil {
		st = &types.CloudStatus{State: types.CloudStateError, CheckedAt: time.Now().Unix()}
	}
	st.Message, st.Detail = msg, err.Error()
	return c.unreachableMsg(g, mode, msg, st)
}

func (c *Orchestrator) unreachable(g Game, mode string, st types.CloudStatus) Decision {
	msg := st.Message
	if msg == "" {
		msg = "Cloud saves couldn't be reached."
	}
	return c.unreachableMsg(g, mode, msg, &st)
}

// unreachableMsg: `ask` asks every time; `auto` goes on without the saves
// synced once the user confirmed it for the game.
func (c *Orchestrator) unreachableMsg(g Game, mode, msg string, st *types.CloudStatus) Decision {
	c.mu.Lock()
	confirmed := c.state.Confirmed[g.ID]
	c.mu.Unlock()
	if mode == types.CloudAuto && confirmed {
		c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvSkipped, Message: "Not synced: " + msg, Status: st})
		return Decision{Proceed: true, Sync: true, Status: st}
	}
	return c.prompt(g, types.CloudPromptOffline, msg, st)
}

// clientRunning is the official-client policy: Platypus doesn't sync while the
// store's own client runs. If the plugin can start the game through the
// client the user is offered that; otherwise the launch goes on unsynced and
// the game page explains.
func (c *Orchestrator) clientRunning(g Game, st types.CloudStatus) Decision {
	name := st.Client.Name
	if st.Client.CanLaunch {
		return c.prompt(g, types.CloudPromptClient,
			fmt.Sprintf("%s is running. Two programs writing the same saves can cause a cloud conflict, so Platypus won't sync this game while it is open.", name), &st)
	}
	msg := fmt.Sprintf("Not synced: %s is running and would write the same saves. Close %s to sync this game.", name, name)
	c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvSkipped, Message: msg, Status: &st, Client: name})
	return Decision{Proceed: true, Status: &st}
}

// Confirm remembers that the user launched a game anyway while its store was
// unreachable: `auto` mode stops asking for it from now on.
func (c *Orchestrator) Confirm(gameID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Confirmed == nil {
		c.state.Confirmed = map[string]bool{}
	}
	c.state.Confirmed[gameID] = true
	c.persist()
}

// Launching records the push the game will owe when it exits and returns its
// job id: the exit and an abandoned launch name their own record by it. Call
// it before the game starts, so a Platypus that quits while it runs still pushes
// next time.
func (c *Orchestrator) Launching(g Game, cc types.CloudContext) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := Pending{GameID: g.ID, PluginID: g.PluginID, StoreID: g.StoreID, Title: g.Title, JobID: c.o.NewJobID(), Ctx: cc, Since: time.Now().Unix()}
	if old := c.pendingFor(g.ID); old != nil {
		*old = p
	} else {
		c.state.Pending = append(c.state.Pending, p)
	}
	c.persist()
	return p.JobID
}

// Abandon forgets the push recorded by Launching under jobID, for a launch
// that never started. A record a later launch wrote is left alone.
func (c *Orchestrator) Abandon(gameID, jobID string) { c.dropPending(gameID, jobID) }

// Exited starts the post-exit sync of a game that ran, from the record the
// launch under jobID wrote. failed marks a launch that died at once: nothing
// was played, so nothing is pushed. It returns immediately; the work runs in
// the background.
func (c *Orchestrator) Exited(gameID, jobID string, failed bool) {
	c.mu.Lock()
	p := c.pendingJob(gameID, jobID)
	if p == nil {
		c.mu.Unlock()
		return
	}
	pending := *p
	closing := c.closing
	c.mu.Unlock()
	if failed {
		c.dropPending(gameID, jobID)
		return
	}
	if closing {
		return // Platypus is quitting: the push stays owed and runs at the next start
	}
	if run := c.startPostExit(pending); run != nil {
		go run()
	}
}

// Resume runs the pushes owed from an earlier run, one after another. Call it
// once the plugins are discovered.
func (c *Orchestrator) Resume() {
	c.mu.Lock()
	if c.closing {
		c.mu.Unlock()
		return
	}
	todo := append([]Pending(nil), c.state.Pending...)
	c.wg.Add(1)
	c.mu.Unlock()
	go func() {
		defer c.wg.Done()
		for _, p := range todo {
			if c.ctx.Err() != nil {
				return
			}
			if run := c.startPostExit(p); run != nil {
				run()
			}
		}
	}()
}

// Wait returns once no cloud job is running.
func (c *Orchestrator) Wait() { c.wg.Wait() }

// startPostExit claims the game for its post-exit push and returns the work,
// or nil when there is nothing to do now (mode off, a job already running,
// Platypus quitting). The claim is made before the work runs, so
// a launch right after the exit already finds the push to cancel.
func (c *Orchestrator) startPostExit(p Pending) (run func()) {
	g := Game{ID: p.GameID, PluginID: p.PluginID, StoreID: p.StoreID, Title: p.Title}
	if c.Mode(g) == types.CloudOff {
		c.dropPending(g.ID, p.JobID)
		return nil
	}
	jctx, j, err := c.begin(c.ctx, g, "push", p.JobID)
	if err != nil {
		return nil
	}
	return func() {
		defer c.end(g, j)
		// Asking may start the plugin, which can take a while: it happens
		// inside the claimed job, off the caller's goroutine.
		if !c.o.Backend.Offers(jctx, g.PluginID) {
			return // the plugin is off or can't do it now; the push stays owed
		}
		c.postExit(jctx, j, g, p.Ctx)
	}
}

// postExit waits for the game's files to settle, then pushes what changed on
// this device. A push that couldn't complete stays owed.
func (c *Orchestrator) postExit(jctx context.Context, j *job, g Game, cc types.CloudContext) {
	fail := func(err error) {
		if errors.Is(err, ErrCanceled) || c.ctx.Err() != nil {
			c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvCanceled, Message: "Cloud sync canceled."})
			return // stays owed
		}
		c.logf("cloud push of %s failed: %v", g.ID, err)
		c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvFailed, Message: fmt.Sprintf("%s's saves weren't uploaded: %v", g.Title, err)})
	}

	st, err := c.stable(jctx, g, cc)
	if err != nil {
		if c.jobCanceled(j) || errors.Is(err, context.Canceled) {
			err = ErrCanceled
		}
		fail(err)
		return
	}
	if c.jobCanceled(j) {
		// Canceled by a launch that is taking over the game: the push stays owed.
		fail(ErrCanceled)
		return
	}
	if cl := st.Client; cl != nil && cl.Running {
		c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvSkipped, Client: cl.Name, Status: &st,
			Message: fmt.Sprintf("Not uploaded: %s is running and handles its own sync.", cl.Name)})
		c.dropPending(g.ID, j.id)
		return
	}
	switch st.State {
	case types.CloudStateUnsupported, types.CloudStateSynced:
		c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvStatus, Status: &st})
		c.dropPending(g.ID, j.id)
		return
	case types.CloudStateConflict:
		c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvFailed, Status: &st,
			Message: fmt.Sprintf("%s's saves changed on this device and in the cloud. Choose which to keep on the game page.", g.Title)})
		c.dropPending(g.ID, j.id)
		return
	case types.CloudStateOffline, types.CloudStateError:
		msg := st.Message
		if msg == "" {
			msg = "the store couldn't be reached"
		}
		fail(errors.New(msg))
		return
	}
	if st.Counts.Upload == 0 {
		c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvStatus, Status: &st})
		c.dropPending(g.ID, j.id)
		return
	}

	// A push that may have started is resent with the same job id (the plugin
	// answers a repeated id with the first run's result); one that finished
	// without success gets a fresh id next time.
	c.markStarted(g.ID, j.id, true)
	res, err := c.call(jctx, j, g, cc, "cloud.push", nil)
	if err != nil {
		if !errors.Is(err, ErrCanceled) {
			c.markStarted(g.ID, j.id, false) // it failed outright: a retry is a new job
		}
		fail(err)
		return
	}
	switch res.State {
	case types.CloudStateOffline, types.CloudStateError:
		c.markStarted(g.ID, j.id, false)
		msg := res.Message
		if msg == "" {
			msg = "the store couldn't be reached"
		}
		fail(errors.New(msg))
		return
	case types.CloudStateConflict:
		c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvFailed, Status: &res,
			Message: fmt.Sprintf("%s's saves changed in the cloud meanwhile. Choose which to keep on the game page.", g.Title)})
		c.dropPending(g.ID, j.id)
		return
	}
	c.emit(types.CloudEvent{GameID: g.ID, Kind: types.CloudEvPushed, Status: &res, JobID: j.id,
		Message: fmt.Sprintf("Saved %d file%s to the cloud", st.Counts.Upload, plural(st.Counts.Upload)), Done: int64(st.Counts.Upload)})
	c.dropPending(g.ID, j.id)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (c *Orchestrator) jobCanceled(j *job) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return j.canceled
}

// markStarted records whether the push under the pending job id was sent. When
// it finished without success the job id is replaced.
func (c *Orchestrator) markStarted(gameID, jobID string, started bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p := c.pendingJob(gameID, jobID); p != nil {
		p.Started = started
		if !started {
			p.JobID = c.o.NewJobID()
		}
		c.persist()
	}
}

// stable reads the cloud state until two readings 2 s apart agree: a game that
// just exited (a Wine game's processes especially) may still be writing.
func (c *Orchestrator) stable(ctx context.Context, g Game, cc types.CloudContext) (types.CloudStatus, error) {
	prev, err := c.status(ctx, g, cc)
	if err != nil {
		return prev, err
	}
	for i := 0; i < c.o.StablePasses; i++ {
		if err := c.o.Sleep(ctx, c.o.StableDelay); err != nil {
			return prev, err
		}
		cur, err := c.status(ctx, g, cc)
		if err != nil {
			return cur, err
		}
		if sameReading(prev, cur) {
			return cur, nil
		}
		prev = cur
	}
	return prev, errors.New("the game's save files kept changing")
}

// Shutdown stops new syncs, gives the running ones up to grace to finish, then
// cancels what is left. Pushes that didn't finish stay owed for the next start.
func (c *Orchestrator) Shutdown(grace time.Duration) {
	c.mu.Lock()
	c.closing = true
	c.mu.Unlock()
	done := make(chan struct{})
	go func() { c.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(grace):
		c.logf("cloud sync still running at shutdown; its push stays owed")
	}
	c.cancel()
}
