package main

// The fake `verify` and `move` capabilities (protocol 1.4) and game.relocated.
// Verify checks a fixed set of echoFiles files of an installed game. Damage is
// kept per game in state.json: the `config` knob "damage" ({"tiny": 3}) sets
// how many files are damaged, a check reports them, and a repair (repair:true)
// restores them. Move renames the game's folder and records the new one; a
// move can be cancelled with job.cancel before it renames anything. The
// relocated call takes the new folder of a game that Hydra moved itself.
// See docs/plugins/protocol.md, "Verify and move".

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// echoFiles is how many files a check looks at.
const echoFiles = 12

// verifyReport is job.done's verify (protocol 1.4).
type verifyReport struct {
	Checked  int `json:"checked"`
	Damaged  int `json:"damaged"`
	Missing  int `json:"missing"`
	Repaired int `json:"repaired"`
}

// configureMaintenance applies the maintenance knobs of a `config` request.
func configureMaintenance(raw json.RawMessage) {
	var present map[string]json.RawMessage
	_ = json.Unmarshal(raw, &present)
	v, ok := present["damage"]
	if !ok {
		return
	}
	damage := map[string]int{}
	if json.Unmarshal(v, &damage) != nil {
		return
	}
	stMu.Lock()
	defer stMu.Unlock()
	st.Damage = damage
	save()
}

// jobStart registers a job's stop channel, as installs do. False means the
// job is already running and must not start a second copy.
func jobStart(jobID string) (chan struct{}, bool) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	if _, running := cancels[jobID]; running {
		return nil, false
	}
	stop := make(chan struct{})
	cancels[jobID] = stop
	return stop, true
}

func jobEnd(jobID string) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	delete(cancels, jobID)
}

// pause waits one step of a fake job; false means the job was cancelled.
func pause(stop chan struct{}, d time.Duration) bool {
	select {
	case <-stop:
		return false
	case <-time.After(d):
		return true
	}
}

func stepDuration() time.Duration {
	ms := 100
	if v := os.Getenv("ECHO_STEP_MS"); v != "" {
		fmt.Sscanf(v, "%d", &ms)
	}
	return time.Duration(ms) * time.Millisecond
}

func startVerify(p params) (any, error) {
	if p.ID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	g, ok := games[p.ID]
	if !ok || g.external != "" || g.client {
		return nil, fmt.Errorf("%q can't be verified", p.ID)
	}
	stMu.Lock()
	_, installed := st.Installed[p.ID]
	stMu.Unlock()
	if !installed {
		return nil, fmt.Errorf("%s is not installed", g.title)
	}
	stop, fresh := jobStart(p.JobID)
	if !fresh {
		return map[string]any{}, nil
	}
	go func() {
		defer jobEnd(p.JobID)
		for done := 0; done < echoFiles; done += 4 {
			if !pause(stop, stepDuration()) {
				return
			}
			notify("job.progress", map[string]any{"jobId": p.JobID, "phase": "checking", "bytes": done + 4, "total": echoFiles})
		}
		stMu.Lock()
		damaged := st.Damage[p.ID]
		report := verifyReport{Checked: echoFiles, Damaged: damaged}
		if p.Repair {
			report = verifyReport{Checked: echoFiles, Repaired: damaged}
			st.Damage[p.ID] = 0
			save()
		}
		stMu.Unlock()
		notify("job.done", map[string]any{"jobId": p.JobID, "verify": report})
	}()
	return nil, nil
}

func startMove(p params) (any, error) {
	if p.ID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	g, ok := games[p.ID]
	if !ok || g.external != "" || g.client {
		return nil, fmt.Errorf("%q can't be moved", p.ID)
	}
	stMu.Lock()
	dir, installed := st.Installed[p.ID]
	stMu.Unlock()
	if !installed {
		return nil, fmt.Errorf("%s is not installed", g.title)
	}
	if p.From != dir {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: %s is in %s, not %s", g.title, dir, p.From)}
	}
	if !filepath.IsAbs(p.To) {
		return nil, &rpcError{Code: -32602, Message: "invalid params: to must be an absolute path"}
	}
	if _, err := os.Lstat(p.To); err == nil {
		return nil, fmt.Errorf("%s already exists", p.To)
	}
	stop, fresh := jobStart(p.JobID)
	if !fresh {
		return map[string]any{}, nil
	}
	go func() {
		defer jobEnd(p.JobID)
		for i := 1; i <= 3; i++ {
			if !pause(stop, stepDuration()) {
				return
			}
			notify("job.progress", map[string]any{"jobId": p.JobID, "phase": "moving", "bytes": i, "total": 3})
		}
		select {
		case <-stop:
			return
		default:
		}
		if err := os.Rename(p.From, p.To); err != nil {
			notify("job.failed", map[string]any{"jobId": p.JobID, "error": err.Error()})
			return
		}
		stMu.Lock()
		st.Installed[p.ID] = p.To
		save()
		stMu.Unlock()
		notify("job.done", map[string]any{"jobId": p.JobID})
	}()
	return nil, nil
}

// relocated records a folder Hydra moved for this game (game.relocated). The
// game must be where Hydra says it was.
func relocated(p params) (any, error) {
	stMu.Lock()
	defer stMu.Unlock()
	dir, installed := st.Installed[p.ID]
	if !installed {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: %q is not installed", p.ID)}
	}
	if dir != p.From {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: %q is in %s, not %s", p.ID, dir, p.From)}
	}
	if !filepath.IsAbs(p.To) {
		return nil, &rpcError{Code: -32602, Message: "invalid params: to must be an absolute path"}
	}
	st.Installed[p.ID] = p.To
	save()
	return map[string]any{}, nil
}
