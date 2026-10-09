package main

// The fake `family` capability (protocol 1.8, docs/plugins/protocol.md,
// "Family"): a family group of two members, and two games they lend. The
// borrowed games are installed here and launch through a store client (a
// 5 s sleep), as the client game does. The ECHO_FAMILY_IN_USE knob (or the
// `config` key familyInUse) makes a borrowed game "in use", so launching it
// answers -32002 with the member's name; ECHO_FAMILY_WARN (familyWarn) sends
// a family.warning with that many seconds when a borrowed game launches.

import (
	"fmt"
	"slices"
)

type familyMember struct {
	id, name, role, avatar, state string
}

var echoFamily = []familyMember{
	{id: "ana", name: "Ana", role: "adult", avatar: "https://example.com/echo/ana.png", state: "online"},
	{id: "ben", name: "Ben", role: "child", state: "offline"},
}

// familyName is the display name of a member id ("" for an unknown one).
func familyName(id string) string {
	for _, m := range echoFamily {
		if m.id == id {
			return m.name
		}
	}
	return ""
}

// familyMembers answers family.members. Like the library, it needs a login.
func familyMembers() (any, error) {
	stMu.Lock()
	defer stMu.Unlock()
	if !st.LoggedIn {
		return nil, &rpcError{Code: codeAuthRequired, Message: "log in to Echo first"}
	}
	out := []map[string]any{}
	for _, m := range echoFamily {
		e := map[string]any{"id": m.id, "name": m.name, "role": m.role, "state": m.state}
		if m.avatar != "" {
			e["avatarUrl"] = m.avatar
		}
		out = append(out, e)
	}
	return out, nil
}

// borrowedLaunch is the launch-time part of a borrowed game: in use is
// -32002 with the owner's name, and otherwise the warning knob (if set) is
// raised before the launch answer.
func borrowedLaunch(gameID string) error {
	knobMu.Lock()
	inUse := slices.Contains(kn.FamilyInUse, gameID)
	warn := kn.FamilyWarn
	knobMu.Unlock()
	owner := familyName(games[gameID].lentBy)
	if inUse {
		return &rpcError{Code: codeUnavailable, Message: fmt.Sprintf("Family member %s is playing this game", owner)}
	}
	if warn > 0 {
		notify("family.warning", map[string]any{
			"id":          gameID,
			"message":     fmt.Sprintf("%s wants to play this game: you have %d seconds left.", owner, warn),
			"secondsLeft": warn,
		})
	}
	return nil
}
