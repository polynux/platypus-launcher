package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"
)

// Social (protocol 1.6, docs/plugins/protocol.md "Social"): a fake friends
// list. social.start turns it on: a connection state, the user's own
// name and status, and a dozen friends in mixed states. Two of them play
// something: one echo game that is in the library (p.echo.tiny) and one
// title that is not. The `tick` knob (ms; ECHO_SOCIAL_TICK sets it at start)
// changes one friend every tick and pushes it as an incremental update.
//
// Knobs, all set with `config`:
//
//	social: { connection: "online" | "connecting" | "reconnecting" | "replaced" | "loggedOut" | "offline",
//	          tick: 500 }   the connection state to report; the incremental push interval (0 = none)
//
// `recorded` answers the setStatus and setPlaying calls Hydra made, in order.
// ECHO_UNAVAILABLE=social.start makes the start fail, like a store that is down.
//
// echo-b is this same binary under another plugin.json (plugins/echo-b runs it):
// its manifest id picks a second friends list with its own user, so Hydra can
// show two stores at once. See manifestID.

// socialFixture is the friends list echo serves.
var socialFixture = []socialFriend{
	{ID: "7656119800000001", Name: "Ana Ruiz", State: "online", Game: &socialGame{ID: "p.echo.tiny", Title: "Echo Tiny"}},
	{ID: "7656119800000002", Name: "Bo Lindqvist", State: "away"},
	{ID: "7656119800000003", Name: "Cy Okafor", State: "busy", Game: &socialGame{Title: "Some Other Game"}},
	{ID: "7656119800000004", Name: "Dee Marsh", State: "snooze"},
	{ID: "7656119800000005", Name: "Eli Strand", State: "offline", LastSeen: 1791398361},
	{ID: "7656119800000006", Name: "Fay Ito", State: "offline", LastSeen: 1791300000},
	{ID: "7656119800000007", Name: "Gus Varga", State: "online", Game: &socialGame{ID: "p.echo.slow", Title: "Echo Slow"}},
	{ID: "7656119800000008", Name: "Hal Brenner", State: "looking"},
	{ID: "7656119800000009", Name: "Ivy Chen", State: "online"},
	{ID: "7656119800000010", Name: "Jon Keller", State: "offline", LastSeen: 1791200000},
	{ID: "7656119800000011", Name: "Kai Mensah", State: "offline", LastSeen: 1790900000},
	{ID: "7656119800000012", Name: "Lou Petit", State: "online", Game: &socialGame{Title: "Untitled Survival"}},
}

// socialFixtureB is the friends list of echo-b, the second store. Its friends
// play no library game, so only echo's own library ever links to a game.
var socialFixtureB = []socialFriend{
	{ID: "8656119800000001", Name: "Mira Vale", State: "online"},
	{ID: "8656119800000002", Name: "Noor Haddad", State: "busy", Game: &socialGame{Title: "Echo Slow"}},
	{ID: "8656119800000003", Name: "Pim de Vries", State: "away"},
	{ID: "8656119800000004", Name: "Rosa Lind", State: "offline", LastSeen: 1791300000},
}

type socialFriend struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	State    string      `json:"state"`
	Game     *socialGame `json:"game,omitempty"`
	LastSeen int64       `json:"lastSeen,omitempty"`
}

type socialGame struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
}

var (
	socialMu      sync.Mutex
	socialOn      bool
	socialConn    = "online"
	socialTick    time.Duration
	socialStop    chan struct{}
	socialSelf    = map[string]string{"name": "Echo Player", "state": "online"}
	socialFriends []socialFriend
	socialRecord  = map[string][]any{}
)

func loadSocial() {
	if ms, err := strconv.Atoi(os.Getenv("ECHO_SOCIAL_TICK")); err == nil && ms > 0 {
		socialTick = time.Duration(ms) * time.Millisecond
	}
	if manifestID() == "echo-b" {
		socialFixture = socialFixtureB
		socialSelf = map[string]string{"name": "Echo B Player", "state": "online"}
	}
}

// manifestID is the id in the plugin.json of the working directory (Hydra runs
// a plugin from its folder). echo's own folder, or no manifest, is "echo".
func manifestID() string {
	var m struct {
		ID string `json:"id"`
	}
	if data, err := os.ReadFile("plugin.json"); err == nil && json.Unmarshal(data, &m) == nil && m.ID != "" {
		return m.ID
	}
	return "echo"
}

// configureSocial applies the social part of a `config` request.
func configureSocial(raw json.RawMessage) {
	var present struct {
		Social *struct {
			Connection *string `json:"connection"`
			Tick       *int    `json:"tick"`
		} `json:"social"`
	}
	if json.Unmarshal(raw, &present) != nil || present.Social == nil {
		return
	}
	socialMu.Lock()
	defer socialMu.Unlock()
	if c := present.Social.Connection; c != nil {
		socialConn = *c
		if socialOn {
			notify("social.connection", map[string]string{"state": socialConn})
		}
	}
	if t := present.Social.Tick; t != nil {
		socialTick = time.Duration(*t) * time.Millisecond
		if socialOn {
			restartTicker()
		}
	}
}

// socialStart answers social.start: the connection goes online, the own
// profile and the full friends list are pushed.
func socialStart() (any, error) {
	socialMu.Lock()
	defer socialMu.Unlock()
	if socialOn {
		return map[string]any{}, nil
	}
	socialOn = true
	socialFriends = slices.Clone(socialFixture)
	notify("social.connection", map[string]string{"state": socialConn})
	notify("social.self", socialSelf)
	notify("social.friends", map[string]any{"full": true, "friends": socialFriends})
	restartTicker()
	return map[string]any{}, nil
}

// socialStopped answers social.stop: the friends stop changing.
func socialStopped() (any, error) {
	socialMu.Lock()
	defer socialMu.Unlock()
	if socialOn {
		socialOn = false
		stopTicker()
	}
	return map[string]any{}, nil
}

// socialSnapshot answers social.state with what social.start pushed.
func socialSnapshot() (any, error) {
	socialMu.Lock()
	defer socialMu.Unlock()
	if !socialOn {
		return map[string]any{"connection": map[string]string{"state": "offline"}, "friends": []any{}}, nil
	}
	return map[string]any{
		"connection": map[string]string{"state": socialConn},
		"self":       socialSelf,
		"friends":    socialFriends,
	}, nil
}

// socialSetStatus answers social.setStatus: the own status changes and the
// change is pushed back as social.self.
func socialSetStatus(p params) (any, error) {
	if !slices.Contains([]string{"online", "away", "busy", "snooze", "invisible"}, p.State) {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: state %q is not a status", p.State)}
	}
	socialMu.Lock()
	defer socialMu.Unlock()
	socialRecord["setStatus"] = append(socialRecord["setStatus"], p.State)
	if socialOn {
		socialSelf["state"] = p.State
		notify("social.self", socialSelf)
	}
	return map[string]any{}, nil
}

// socialSetPlaying answers social.setPlaying: `id` is the store game id, or
// null for no game. Anything else is a bad request.
func socialSetPlaying(raw json.RawMessage) (any, error) {
	var p struct {
		ID *string `json:"id"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	socialMu.Lock()
	defer socialMu.Unlock()
	var value any // null
	if p.ID != nil {
		value = *p.ID
	}
	socialRecord["setPlaying"] = append(socialRecord["setPlaying"], value)
	return map[string]any{}, nil
}

// socialRecorded answers `recorded`: the calls Hydra made to set the status
// and what the user plays.
func socialRecorded() any {
	socialMu.Lock()
	defer socialMu.Unlock()
	out := map[string]any{"setStatus": []any{}, "setPlaying": []any{}}
	for k, v := range socialRecord {
		out[k] = v
	}
	return out
}

// restartTicker starts the incremental pushes at socialTick. Callers hold socialMu.
func restartTicker() {
	stopTicker()
	if socialTick <= 0 {
		return
	}
	stop := make(chan struct{})
	socialStop = stop
	go func(stop chan struct{}, every time.Duration) {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-tick.C:
				pushChanged(i)
			}
		}
	}(stop, socialTick)
}

// stopTicker ends the pushes. Callers hold socialMu.
func stopTicker() {
	if socialStop != nil {
		close(socialStop)
		socialStop = nil
	}
}

// pushChanged flips one friend between online and offline and pushes just that
// friend, as an incremental social.friends.
func pushChanged(i int) {
	socialMu.Lock()
	defer socialMu.Unlock()
	if !socialOn || len(socialFriends) == 0 {
		return
	}
	k := i % len(socialFriends)
	f := &socialFriends[k]
	if f.State == "offline" {
		f.State = "online"
	} else {
		f.State = "offline"
	}
	notify("social.friends", map[string]any{"full": false, "friends": []socialFriend{*f}})
}
