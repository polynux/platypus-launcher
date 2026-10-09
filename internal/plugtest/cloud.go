package plugtest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/types"
)

// The keys docs/plugins/protocol.md ("Cloud saves") defines for the cloud.*
// payloads.
var (
	CloudStatusKeys   = []string{"state", "message", "counts", "conflicts", "checkedAt", "backupDir", "stamp", "client"}
	CloudCountsKeys   = []string{"same", "download", "upload", "conflict", "unmapped"}
	CloudConflictKeys = []string{"path", "local", "remote"}
	CloudFileKeys     = []string{"size", "mtime", "machine"}
	CloudClientKeys   = []string{"running", "name", "canLaunch"}
	CloudProgressKeys = []string{"id", "jobId", "phase", "done", "total"}
)

var cloudStates = []string{"synced", "pending", "conflict", "unsupported", "offline", "error"}

// recordCloudProgress keeps a cloud.progress notification as the host saw it.
func (s *Session) recordCloudProgress(raw json.RawMessage) {
	s.evMu.Lock()
	s.progress = append(s.progress, raw)
	s.evMu.Unlock()
}

// checkCloudStatus holds one cloud.* answer to the shapes the protocol defines.
func checkCloudStatus(t *testing.T, what string, raw json.RawMessage) types.CloudStatus {
	t.Helper()
	checkKeys(t, what, raw, CloudStatusKeys)
	var obj map[string]json.RawMessage
	_ = json.Unmarshal(raw, &obj)
	if c, ok := obj["counts"]; ok {
		checkKeys(t, what+" counts", c, CloudCountsKeys)
	} else {
		t.Errorf("%s has no counts", what)
	}
	if c, ok := obj["client"]; ok {
		checkKeys(t, what+" client", c, CloudClientKeys)
	}
	var list []map[string]json.RawMessage
	_ = json.Unmarshal(obj["conflicts"], &list)
	for _, c := range list {
		cr, _ := json.Marshal(c)
		checkKeys(t, what+" conflict", cr, CloudConflictKeys)
		for _, side := range []string{"local", "remote"} {
			checkKeys(t, what+" conflict "+side, c[side], CloudFileKeys)
		}
	}

	var st types.CloudStatus
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("%s isn't a CloudStatus: %v: %s", what, err, raw)
	}
	if !slices.Contains(cloudStates, st.State) {
		t.Fatalf("%s state %q isn't one of %v", what, st.State, cloudStates)
	}
	switch st.State {
	case "unsupported", "offline", "error":
		if st.Message == "" {
			t.Errorf("%s is %s without a message: the user is owed the reason", what, st.State)
		}
	case "conflict":
		if len(st.Conflicts) == 0 || st.Counts.Conflict != len(st.Conflicts) {
			t.Errorf("%s is a conflict, but counts.conflict is %d and it lists %d file(s)", what, st.Counts.Conflict, len(st.Conflicts))
		}
	}
	for _, c := range st.Conflicts {
		if c.Path == "" {
			t.Errorf("%s lists a conflict with no path", what)
		}
	}
	if st.Client != nil && st.Client.Name == "" {
		t.Errorf("%s reports the store client without a name", what)
	}
	return st
}

// checkCloud drives cloud.* the way Hydra does: parameter errors, the shape of
// every answer, the idempotent jobId, resolve, and the cloud.progress shape.
func (s *Session) checkCloud(t *testing.T, game types.Game) {
	t.Helper()
	ctx := types.CloudContext{
		OS: "linux", Home: s.Games, XDGData: filepath.Join(s.Games, ".local/share"), XDGConfig: filepath.Join(s.Games, ".config"),
		InstallDir: filepath.Join(s.Folder, game.StoreAppID),
	}
	id := game.StoreAppID

	// Parameter errors come before any work.
	for _, m := range []string{"cloud.status", "cloud.pull", "cloud.push", "cloud.resolve"} {
		s.wantCode(m, map[string]any{}, jsonrpc.CodeInvalidParams)
		s.wantCode(m, map[string]any{"id": id}, jsonrpc.CodeInvalidParams)
	}
	for _, m := range []string{"cloud.pull", "cloud.push", "cloud.resolve"} {
		s.wantCode(m, map[string]any{"id": id, "ctx": ctx}, jsonrpc.CodeInvalidParams)
	}

	var raw json.RawMessage
	if err := s.call("cloud.status", map[string]any{"id": id, "ctx": ctx}, &raw); err != nil {
		t.Fatalf("cloud.status: %v", err)
	}
	st := checkCloudStatus(t, "cloud.status", raw)

	for _, m := range []string{"cloud.pull", "cloud.push"} {
		params := map[string]any{"id": id, "ctx": ctx, "jobId": "conf-" + m}
		var first, again json.RawMessage
		if err := s.call(m, params, &first); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
		checkCloudStatus(t, m, first)
		// A repeated jobId is the same job: Hydra resends one after a restart.
		if err := s.call(m, params, &again); err != nil {
			t.Fatalf("%s with a repeated jobId: %v", m, err)
		}
		a, b := checkCloudStatus(t, m+" (repeated)", again), checkCloudStatus(t, m, first)
		if a.State != b.State || a.Counts != b.Counts {
			t.Errorf("%s answered %+v, then %+v for the same jobId", m, b, a)
		}
	}

	// resolve: a path nobody listed is refused; the listed ones settle.
	s.wantCode("cloud.resolve", map[string]any{"id": id, "ctx": ctx, "jobId": "conf-resolve-bad",
		"choices": map[string]string{"no/such/file": "local"}}, jsonrpc.CodeInvalidParams)
	if err := s.call("cloud.status", map[string]any{"id": id, "ctx": ctx}, &raw); err != nil {
		t.Fatalf("cloud.status: %v", err)
	}
	st = checkCloudStatus(t, "cloud.status", raw)
	if len(st.Conflicts) > 0 {
		choices := map[string]string{}
		for _, c := range st.Conflicts {
			choices[c.Path] = "local"
		}
		var res json.RawMessage
		if err := s.call("cloud.resolve", map[string]any{"id": id, "ctx": ctx, "jobId": "conf-resolve", "choices": choices}, &res); err != nil {
			t.Fatalf("cloud.resolve: %v", err)
		}
		if after := checkCloudStatus(t, "cloud.resolve", res); after.State == "conflict" && st.Client == nil {
			t.Errorf("cloud.resolve left the conflicts it was given choices for: %+v", after)
		}
	}

	s.evMu.Lock()
	progress := slices.Clone(s.progress)
	s.evMu.Unlock()
	for _, p := range progress {
		checkKeys(t, "cloud.progress", p, CloudProgressKeys)
		var cp types.CloudProgress
		_ = json.Unmarshal(p, &cp)
		if cp.JobID == "" || !slices.Contains([]string{"check", "download", "upload"}, cp.Phase) || cp.Done < 0 || cp.Total < cp.Done {
			t.Errorf("cloud.progress %s isn't {id, jobId, phase check|download|upload, done <= total}", p)
		}
	}
}

// CloudConformance runs only the cloud.* checks of Conformance, for a plugin
// that can be put in different cloud states (echo's ECHO_CLOUD knob).
func (s Suite) CloudConformance(t *testing.T) *Session {
	t.Helper()
	sess := s.Start(t)
	sess.checkHandshake(t)
	if !sess.m.Has(s.ID, "cloud-saves") {
		t.Fatalf("%s doesn't offer cloud-saves", s.ID)
	}
	if s.Login != nil {
		s.Login(t, sess)
	}
	games, err := sess.Store.Library(context.Background())
	if err != nil {
		t.Fatalf("library.list: %v", err)
	}
	sess.checkCloud(t, s.PickGame(t, games))
	return sess
}

// Has reports whether the plugin offers the capability right now.
func (s *Session) Has(capability string) bool { return s.m.Has(s.id, capability) }
