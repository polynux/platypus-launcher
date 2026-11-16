// Family running-apps read (protocol 1.8, issue #178): which borrowed
// games are in use right now. One-shot CM session: GetFamilyGroupForUser
// (the user's group id) then GetFamilyGroup with send_running_apps, which
// makes Steam push FamilyGroupsClient.NotifyRunningApps#1 with the current
// running list. The push arrives after the reply, so the session drains a
// few bounded seconds for it; no push means nothing is running (not an
// error). Result: {"type":"result","running":[{"appid","player","owner"}]},
// steamids as strings (they don't fit a JSON number).
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"platypus/internal/depot"
)

// notifyRunningApps is the push GetFamilyGroup(send_running_apps=true)
// triggers. It arrives as a service notification with this
// target_job_name in the header (EMsg 146 on the wire).
const notifyRunningApps = "FamilyGroupsClient.NotifyRunningApps#1"

// runningWait bounds the post-reply wait for the push: Steam sends it
// right after the reply, so a few seconds is plenty; silence means
// nothing is running.
const runningWait = 3 * time.Second

// runningEntry is one borrowed game played by someone else: the app, the
// player and the copy's owner.
type runningEntry struct {
	appID  uint32
	player uint64
	owner  uint64
}

// runFamilyRunning answers the plugin's in-use read: the current running
// list, or an empty one when the user is in no group or Steam pushes
// nothing. Login problems stay auth errors through dialCM/callErr; a
// silent CM degrades to an empty list, never a failure.
func runFamilyRunning(ctx context.Context, req request) error {
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return err
	}
	defer cm.Close()
	// Pushes ride in before the reply (fakes) and after it (Steam): the
	// hook collects both, the drain below waits for the late ones. Each
	// push is the whole current state and replaces the previous one.
	var mu sync.Mutex
	var last []runningEntry
	cm.SetPushHook(func(_ uint32, hdr depot.Msg, body []byte) {
		if string(hdr.Bytes(12)) != notifyRunningApps {
			return
		}
		entries := decodeRunningApps(body)
		mu.Lock()
		last = entries
		mu.Unlock()
	})
	// Field numbers from steammessages_familygroups.steamclient.proto.
	fu := &depot.Enc{}
	fu.Fixed64(1, cm.SteamID())
	fu.Varint(2, 1) // include_family_group_response
	raw, err := cm.Call("FamilyGroups.GetFamilyGroupForUser#1", fu.Bytes(), 0)
	if err != nil {
		return callErr("family group", err)
	}
	m, err := depot.Decode(raw)
	if err != nil {
		return fmt.Errorf("decoding the family group: %w", err)
	}
	if m.Int(1) == 0 {
		emit(map[string]any{"type": "result", "running": []any{}})
		return nil
	}
	g := &depot.Enc{}
	g.Varint(1, m.Int(1))
	g.Varint(2, 1) // send_running_apps
	if _, err := cm.Call("FamilyGroups.GetFamilyGroup#1", g.Bytes(), 0); err != nil {
		return callErr("family running apps", err)
	}
	cm.DrainPushes(runningWait)
	mu.Lock()
	defer mu.Unlock()
	out := make([]any, 0, len(last))
	for _, e := range last {
		out = append(out, map[string]any{
			"appid":  e.appID,
			"player": fmt.Sprint(e.player),
			"owner":  fmt.Sprint(e.owner),
		})
	}
	emit(map[string]any{"type": "result", "running": out})
	return nil
}

// decodeRunningApps reads one NotifyRunningApps body: field 1 the family
// group id, repeated field 2 the running entries {1: appid, 3: {1: player
// steamid, 2: owner steamid}}. Malformed entries are skipped, so one bad
// entry never blanks the rest.
func decodeRunningApps(body []byte) []runningEntry {
	m, err := depot.Decode(body)
	if err != nil {
		return nil
	}
	var out []runningEntry
	for _, raw := range m.AllBytes(2) {
		em, err := depot.Decode(raw)
		if err != nil {
			continue
		}
		appID := uint32(em.Int(1))
		if appID == 0 {
			continue
		}
		pm, err := depot.Decode(em.Bytes(3))
		if err != nil {
			continue
		}
		player, owner := pm.Int(1), pm.Int(2)
		if player == 0 || owner == 0 {
			continue
		}
		out = append(out, runningEntry{appID: appID, player: player, owner: owner})
	}
	return out
}
