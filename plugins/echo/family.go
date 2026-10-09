package main

// The fake `family` capability (protocol 1.8, docs/plugins/protocol.md,
// "Family"): a family group of two members, and the games they lend. The
// borrowed games are installed here and launch through a store client (a
// 5 s sleep), as the client game does. The ECHO_FAMILY_IN_USE knob (or the
// `config` key familyInUse) makes a borrowed game "in use", so launching it
// answers -32002 with the member's name; ECHO_FAMILY_WARN (familyWarn) sends
// a family.warning with that many seconds when a borrowed game launches.
//
// The "shared" game is lent by both members, and their copies differ (DLC).
// Its copies are listed in library.list; the user's pick (family.setPreferredCopy)
// is kept in state.json in HYDRA_PLUGIN_DATA, so it survives a restart. The
// ECHO_COPY_IN_USE and ECHO_COPY_UNAVAILABLE knobs (owner ids; `config`
// copyInUse and copyUnavailable) mark one member's copy in use or unavailable.

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

// echoCopy is one member's copy of a shared game: its DLC, a note, and
// whether it can be borrowed now.
type echoCopy struct {
	owner     string
	dlc       []copyDLC
	note      string
	available bool
}

type copyDLC struct{ id, name string }

// sharedCopies are the copies of the "shared" game, by game id. Ana's copy
// has the soundtrack, Ben's has the soundtrack and the track pack.
var sharedCopies = map[string][]echoCopy{
	"shared": {
		{owner: "ana", dlc: []copyDLC{{"soundtrack", "Soundtrack"}}, note: "Ana's copy has the soundtrack", available: true},
		{owner: "ben", dlc: []copyDLC{{"soundtrack", "Soundtrack"}, {"track-pack", "Track pack"}}, available: true},
	},
}

// copyOf finds the copy of gameID that owner lends, if the game has copies.
func copyOf(gameID, owner string) (echoCopy, bool) {
	for _, c := range sharedCopies[gameID] {
		if c.owner == owner {
			return c, true
		}
	}
	return echoCopy{}, false
}

// effectiveOwnerLocked is the member whose copy a borrowed game is played
// from: the one the user picked, else the game's lender. Callers hold stMu.
func effectiveOwnerLocked(gameID string) string {
	if pref := st.PreferredCopy[gameID]; pref != "" {
		if _, ok := copyOf(gameID, pref); ok {
			return pref
		}
	}
	return games[gameID].lentBy
}

// copiesList answers the copies of a game for library.list: one entry per
// owner, with the in-use and unavailable knobs applied.
func copiesList(gameID string) []map[string]any {
	knobMu.Lock()
	defer knobMu.Unlock()
	var out []map[string]any
	for _, c := range sharedCopies[gameID] {
		dlc := []map[string]any{}
		for _, d := range c.dlc {
			dlc = append(dlc, map[string]any{"id": d.id, "name": d.name})
		}
		e := map[string]any{
			"ownerId": c.owner, "ownerName": familyName(c.owner), "dlc": dlc,
			"available": c.available && !slices.Contains(kn.CopyUnavailable, c.owner),
			"inUse":     slices.Contains(kn.CopyInUse, c.owner),
		}
		if c.note != "" {
			e["note"] = c.note
		}
		out = append(out, e)
	}
	return out
}

// setPreferredCopy answers family.setPreferredCopy: the copy of a shared game
// the user picked. Only a game's listed owners are accepted, and an
// unavailable copy answers -32002.
func setPreferredCopy(p params) (any, error) {
	if p.ID == "" || p.OwnerID == "" {
		return nil, bad("invalid params: id and ownerId are required")
	}
	c, ok := copyOf(p.ID, p.OwnerID)
	if !ok {
		return nil, bad("%s has no copy from %q", p.ID, p.OwnerID)
	}
	knobMu.Lock()
	unavailable := slices.Contains(kn.CopyUnavailable, p.OwnerID)
	knobMu.Unlock()
	if !c.available || unavailable {
		return nil, &rpcError{Code: codeUnavailable, Message: fmt.Sprintf("the copy from %s isn't available right now", familyName(p.OwnerID))}
	}
	stMu.Lock()
	defer stMu.Unlock()
	if st.PreferredCopy == nil {
		st.PreferredCopy = map[string]string{}
	}
	st.PreferredCopy[p.ID] = p.OwnerID
	save()
	return map[string]any{}, nil
}

// borrowedLaunch is the launch-time part of a borrowed game, played from the
// copy of owner: in use (the game's knob, or the owner's copy knob) is -32002
// with the owner's name, and otherwise the warning knob (if set) is raised
// before the launch answer.
func borrowedLaunch(gameID, owner string) error {
	knobMu.Lock()
	inUse := slices.Contains(kn.FamilyInUse, gameID) || slices.Contains(kn.CopyInUse, owner)
	unavailable := slices.Contains(kn.CopyUnavailable, owner)
	warn := kn.FamilyWarn
	knobMu.Unlock()
	name := familyName(owner)
	if unavailable {
		return &rpcError{Code: codeUnavailable, Message: fmt.Sprintf("the copy from %s isn't available right now", name)}
	}
	if inUse {
		return &rpcError{Code: codeUnavailable, Message: fmt.Sprintf("Family member %s is playing this game", name)}
	}
	if warn > 0 {
		notify("family.warning", map[string]any{
			"id":          gameID,
			"message":     fmt.Sprintf("%s wants to play this game: you have %d seconds left.", name, warn),
			"secondsLeft": warn,
		})
	}
	return nil
}
