package social

import (
	"net/url"
	"unicode/utf8"
)

// Limits on what a store may push. Core cuts a value down to its limit rather
// than refusing the push, so one long name doesn't drop the whole list.
const (
	maxFriends   = 5000
	maxIDLen     = 64
	maxNameLen   = 128
	maxGameIDLen = 256
	maxTitleLen  = 256
	// Invite caps (protocol 1.24): a join fragment is a command line
	// (256, like GetLaunchCommandLine's own cap); the store and lobby
	// ids are short.
	maxConnectLen = 256
	maxStoreIDLen = 64
	maxLobbyIDLen = 64
)

// clip shortens s to n characters.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// webAvatar keeps an http or https URL with a host, the only images the UI
// loads. Anything else (file:, javascript:, data:) comes back empty.
func webAvatar(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return raw
}

// clean applies the limits to a friend. It reports false when the friend has
// no id, which is what merge keys on.
func (f *Friend) clean() bool {
	f.ID = clip(f.ID, maxIDLen)
	if f.ID == "" {
		return false
	}
	f.Name = clip(f.Name, maxNameLen)
	f.AvatarURL = webAvatar(f.AvatarURL)
	if f.Game != nil {
		f.Game.clean()
	}
	return true
}

// clean cuts a friend's game down to the limits.
func (g *Game) clean() {
	g.ID = clip(g.ID, maxGameIDLen)
	g.Title = clip(g.Title, maxTitleLen)
	g.StoreID = clip(g.StoreID, maxStoreIDLen)
	g.Connect = clip(g.Connect, maxConnectLen)
	g.LobbyID = clip(g.LobbyID, maxLobbyIDLen)
}

// clean applies the limits to an invite. Over-long fragments are dropped at
// the boundary, never clipped, and the kind must match its field against
// the join allowlist (ParseJoinArgs): anything else is refused. It reports
// false for what mergeInvites drops.
func (iv *Invite) clean() bool {
	iv.From.ID = clip(iv.From.ID, maxIDLen)
	if iv.From.ID == "" {
		return false
	}
	iv.From.Name = clip(iv.From.Name, maxNameLen)
	if iv.From.Name == "" {
		iv.From.Name = iv.From.ID
	}
	if len(iv.Connect) > maxConnectLen || len(iv.Game.StoreID) > maxStoreIDLen || len(iv.LobbyID) > maxLobbyIDLen {
		return false
	}
	iv.Game.clean()
	if iv.Game.StoreID == "" {
		return false
	}
	_, ok := ParseJoinArgs(iv.Kind, iv.Connect, iv.LobbyID)
	return ok
}

// clean applies the limits to the user's own account.
func (s *Self) clean() {
	s.Name = clip(s.Name, maxNameLen)
	s.AvatarURL = webAvatar(s.AvatarURL)
}
