package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"platypus/internal/chat"
)

// Recent chat conversations (issue #143): one preview row per friend, so the
// Chats tab lists recent conversations after a reload, most recent first,
// read or unread. The chat manager only holds what the session saw; every
// change is recorded here and the file is the restart bridge. Message text
// is user data like the activity feed, never logged.

const (
	chatRecentFileVersion = 1
	chatRecentFileName    = "chat-recents.json"
	// maxChatRecents bounds the persisted rows; the manager bounds live
	// conversations the same way, so nothing held is ever dropped here.
	maxChatRecents = 256
)

// chatRecent is one conversation's preview: the friend, the last message's
// preview, when it last changed, and the unread count.
type chatRecent struct {
	Plugin   string `json:"plugin"`
	FriendID string `json:"friendId"`
	From     string `json:"from,omitempty"`
	Text     string `json:"text,omitempty"`
	SentAt   int64  `json:"sentAt,omitempty"`
	Unread   int    `json:"unread,omitempty"`
	// At is the last change in unix ms: the row's most-recent-first order.
	// A conversation whose messages were dropped still sorts by when it
	// last changed, not by a missing message.
	At int64 `json:"at"`
}

type chatRecentKey struct{ plugin, friend string }

type chatRecentFile struct {
	Version int          `json:"version"`
	Recents []chatRecent `json:"recents"`
}

// openChatRecents loads the persisted previews. A missing file is empty; a
// corrupt or newer one is moved aside and Open returns empty, like the
// session log and the activity feed, so history is never silently
// overwritten. Old files still load: unknown fields stay zero.
func (a *App) openChatRecents(dataDir string) {
	a.chatRecentMu.Lock()
	defer a.chatRecentMu.Unlock()
	a.chatRecentPath = ""
	a.chatRecents = map[chatRecentKey]chatRecent{}
	if dataDir == "" {
		return
	}
	a.chatRecentPath = filepath.Join(dataDir, chatRecentFileName)
	raw, err := os.ReadFile(a.chatRecentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		if a.host != nil {
			a.host.Errorf("chat recents: %v", err)
		}
		return
	}
	var f chatRecentFile
	if err := json.Unmarshal(raw, &f); err != nil || f.Version > chatRecentFileVersion {
		bad := fmt.Sprintf("%s.bad-%d", a.chatRecentPath, time.Now().Unix())
		_ = os.Rename(a.chatRecentPath, bad)
		if a.host != nil {
			if err != nil {
				a.host.Errorf("chat recents are corrupt, moved to %s: %v", bad, err)
			} else {
				a.host.Errorf("chat recents are from a newer Platypus, moved to %s", bad)
			}
		}
		a.chatRecents = map[chatRecentKey]chatRecent{}
		return
	}
	for _, r := range f.Recents {
		if r.Plugin == "" || r.FriendID == "" {
			continue
		}
		if r.Unread < 0 {
			r.Unread = 0
		}
		a.chatRecents[chatRecentKey{r.Plugin, r.FriendID}] = r
	}
	trimChatRecents(a.chatRecents)
}

// saveChatRecentsLocked writes the previews atomically. Callers hold
// a.chatRecentMu.
func (a *App) saveChatRecentsLocked() {
	if a.chatRecentPath == "" {
		return
	}
	recents := make([]chatRecent, 0, len(a.chatRecents))
	for _, r := range a.chatRecents {
		recents = append(recents, r)
	}
	sort.Slice(recents, func(i, j int) bool {
		if recents[i].At != recents[j].At {
			return recents[i].At > recents[j].At
		}
		if recents[i].Plugin != recents[j].Plugin {
			return recents[i].Plugin < recents[j].Plugin
		}
		return recents[i].FriendID < recents[j].FriendID
	})
	f := chatRecentFile{Version: chatRecentFileVersion, Recents: recents}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(a.chatRecentPath)
	_ = os.MkdirAll(dir, 0o755)
	tmp, err := os.CreateTemp(dir, ".chat-recents-*.json")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmp.Name(), a.chatRecentPath)
}

// trimChatRecents drops the stalest rows past the cap, in place.
func trimChatRecents(m map[chatRecentKey]chatRecent) {
	for len(m) > maxChatRecents {
		var victim chatRecentKey
		var oldest int64
		first := true
		for k, r := range m {
			if first || r.At < oldest {
				victim, oldest, first = k, r.At, false
			}
		}
		delete(m, victim)
	}
}

// recordChatRecent folds one conversation change into the previews. A dropped
// (evicted) conversation stays: the bound only trims memory, not recency.
// Typing alone changes nothing and skips the save.
func (a *App) recordChatRecent(v chat.View) {
	if v.Plugin == "" || v.FriendID == "" {
		return
	}
	var from, text string
	var sentAt int64
	if len(v.Messages) > 0 {
		last := v.Messages[len(v.Messages)-1]
		from, text, sentAt = last.From, last.Text, last.SentAt
	}
	next := chatRecent{
		Plugin: v.Plugin, FriendID: v.FriendID,
		From: from, Text: text, SentAt: sentAt, Unread: v.Unread,
		At: time.Now().UnixMilli(),
	}
	if next.Unread < 0 {
		next.Unread = 0
	}
	a.chatRecentMu.Lock()
	defer a.chatRecentMu.Unlock()
	if a.chatRecents == nil {
		a.chatRecents = map[chatRecentKey]chatRecent{}
	}
	k := chatRecentKey{v.Plugin, v.FriendID}
	old, ok := a.chatRecents[k]
	if ok && old.Unread == next.Unread &&
		old.From == next.From && old.Text == next.Text && old.SentAt == next.SentAt {
		return
	}
	// A view without messages (an unread seed the snapshot backfilled, or a
	// refresh racing a restart) must not wipe the stored preview: the last
	// message is still the conversation's, only the count is new.
	if next.Text == "" && ok && old.Text != "" {
		next.From, next.Text, next.SentAt = old.From, old.Text, old.SentAt
		if old.Unread == next.Unread {
			return
		}
	}
	a.chatRecents[k] = next
	trimChatRecents(a.chatRecents)
	a.saveChatRecentsLocked()
}

// ChatConversations lists the recent conversations with friends, most recent
// first, read or unread: the live conversations overlaid on the persisted
// previews, so a reload still lists what the session saw. Views the manager
// holds win (their unread and open state are current); a persisted preview
// fills in the last message when memory holds none, and covers the
// conversations memory dropped. It never returns nil.
func (a *App) ChatConversations() []chat.View {
	live := map[chatRecentKey]chat.View{}
	if a.chat != nil {
		for _, id := range a.chatPlugins() {
			for _, v := range a.chat.Conversations(id) {
				live[chatRecentKey{v.Plugin, v.FriendID}] = v
			}
		}
	}
	a.chatRecentMu.Lock()
	recents := make([]chatRecent, 0, len(a.chatRecents))
	for _, r := range a.chatRecents {
		recents = append(recents, r)
	}
	a.chatRecentMu.Unlock()
	out := []chat.View{}
	seen := map[chatRecentKey]bool{}
	for k, v := range live {
		if len(v.Messages) == 0 {
			for _, r := range recents {
				if r.Plugin == k.plugin && r.FriendID == k.friend && r.Text != "" {
					v.Messages = []chat.Message{{From: r.From, Text: r.Text, SentAt: r.SentAt}}
					break
				}
			}
		}
		out = append(out, v)
		seen[k] = true
	}
	for _, r := range recents {
		k := chatRecentKey{r.Plugin, r.FriendID}
		if seen[k] {
			continue
		}
		v := chat.View{Plugin: r.Plugin, FriendID: r.FriendID, Unread: r.Unread, More: r.Text != ""}
		if r.Text != "" {
			v.Messages = []chat.Message{{From: r.From, Text: r.Text, SentAt: r.SentAt}}
		} else {
			v.Messages = []chat.Message{}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		ai, aj := lastChatAt(out[i]), lastChatAt(out[j])
		if ai != aj {
			return ai > aj
		}
		if out[i].Plugin != out[j].Plugin {
			return out[i].Plugin < out[j].Plugin
		}
		return out[i].FriendID < out[j].FriendID
	})
	if out == nil {
		out = []chat.View{}
	}
	return out
}

// lastChatAt is what a conversation sorts by: its last message, or, when it
// holds none, its sequence (a live unread seed) — zero for a persisted row
// without a preview, which sorts last.
func lastChatAt(v chat.View) int64 {
	if len(v.Messages) > 0 {
		return v.Messages[len(v.Messages)-1].SentAt
	}
	return 0
}
