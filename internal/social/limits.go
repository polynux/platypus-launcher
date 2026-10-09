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
		f.Game.ID = clip(f.Game.ID, maxGameIDLen)
		f.Game.Title = clip(f.Game.Title, maxTitleLen)
	}
	return true
}

// clean applies the limits to the user's own account.
func (s *Self) clean() {
	s.Name = clip(s.Name, maxNameLen)
	s.AvatarURL = webAvatar(s.AvatarURL)
}
