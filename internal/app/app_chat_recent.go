package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
	"unicode/utf8"

	"platypus/internal/chat"
)

// Recent chat conversations (issue #143): one preview row per friend, so the
// Chats tab lists recent conversations after a reload, most recent first,
// read or unread. The chat manager only holds what the session saw; every
// change is recorded here and the file is the restart bridge. Previews hold
// message text (clipped), user data like the activity feed, never logged.
// Unread is never persisted: after a restart every row comes back read, and
// only the store's live snapshot (RefreshUnread) sets unread again.

const (
	chatRecentFileVersion = 1
	chatRecentFileName    = "chat-recents.json"
	// maxChatRecents bounds the persisted rows; the manager bounds live
	// conversations the same way, so nothing held is ever dropped here.
	maxChatRecents = 256
	// chatRecentPreviewRunes caps a persisted preview: the Chats tab only
	// shows a snippet, and less message text lands on disk.
	chatRecentPreviewRunes = 200
	// chatRecentSaveDelay debounces file writes off the plugin reader: a
	// burst of incoming messages marks rows dirty, and one background save
	// covers them all. Shutdown, logout and removal flush at once instead.
	chatRecentSaveDelay = 2 * time.Second
)

// chatRecent is one conversation's preview: the friend and its last
// message's clipped preview, plus when it last changed.
type chatRecent struct {
	Plugin   string `json:"plugin"`
	FriendID string `json:"friendId"`
	From     string `json:"from,omitempty"`
	Text     string `json:"text,omitempty"`
	SentAt   int64  `json:"sentAt,omitempty"`
	// At is the last change in unix ms: the file's most-recent-first order
	// and what the cap drops by. The listing itself orders by the message's
	// SentAt instead: opening an old conversation rewrites the file without
	// moving its row.
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
// overwritten. Old files still load: their unread counts are ignored (C1),
// and rows without a preview carry nothing and are dropped.
func (a *App) openChatRecents(dataDir string) {
	a.chatRecentMu.Lock()
	defer a.chatRecentMu.Unlock()
	a.chatRecentPath = ""
	a.chatRecents = map[chatRecentKey]chatRecent{}
	a.chatRecentSeq = map[chatRecentKey]uint64{}
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
		if r.Plugin == "" || r.FriendID == "" || r.Text == "" {
			continue
		}
		a.chatRecents[chatRecentKey{r.Plugin, r.FriendID}] = r
	}
	trimChatRecents(a.chatRecents)
}

// writeChatRecentsFile writes rows atomically. Callers hold no lock; the rows
// are already a snapshot.
func writeChatRecentsFile(path string, recents []chatRecent) {
	if path == "" {
		return
	}
	f := chatRecentFile{Version: chatRecentFileVersion, Recents: recents}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(path)
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
	_ = os.Rename(tmp.Name(), path)
}

// sortedChatRecents snapshots the rows, most recent change first. Callers
// hold a.chatRecentMu.
func (a *App) sortedChatRecentsLocked() []chatRecent {
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
	return recents
}

// flushChatRecents stops the pending save and writes dirty rows now. The
// debounced background save, shutdown, logout and removal share it; tests use
// it instead of sleeping for the debounce.
func (a *App) flushChatRecents() {
	a.chatRecentMu.Lock()
	if a.chatRecentTimer != nil {
		a.chatRecentTimer.Stop()
		a.chatRecentTimer = nil
	}
	var recents []chatRecent
	path := ""
	if a.chatRecentDirty && a.chatRecentPath != "" {
		recents = a.sortedChatRecentsLocked()
		a.chatRecentDirty = false
		path = a.chatRecentPath
	}
	a.chatRecentMu.Unlock()
	if path != "" {
		writeChatRecentsFile(path, recents)
	}
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

// clipChatPreview caps a preview to a snippet, in runes.
func clipChatPreview(s string) string {
	if utf8.RuneCountInString(s) <= chatRecentPreviewRunes {
		return s
	}
	return string([]rune(s)[:chatRecentPreviewRunes])
}

// recordChatRecent folds one conversation change into the previews. A dropped
// (evicted) conversation stays: the bound only trims memory, not recency.
// Views without a message record nothing: typing, empty histories and unread
// seeds carry no preview, and unread is never persisted (C1), so there is
// nothing to keep. The map updates under the lock; the file follows on a
// short debounce, never on the caller's (often the plugin reader's) path.
// A view older than the one already seen for that conversation is dropped:
// the manager stamps each change with a rising Seq, so a late emit can't put
// an old preview back.
func (a *App) recordChatRecent(v chat.View) {
	if v.Plugin == "" || v.FriendID == "" {
		return
	}
	var from, text string
	var sentAt int64
	if len(v.Messages) > 0 {
		last := v.Messages[len(v.Messages)-1]
		from, text, sentAt = last.From, clipChatPreview(last.Text), last.SentAt
	}
	if text == "" {
		return
	}
	next := chatRecent{
		Plugin: v.Plugin, FriendID: v.FriendID,
		From: from, Text: text, SentAt: sentAt,
		At: time.Now().UnixMilli(),
	}
	a.chatRecentMu.Lock()
	defer a.chatRecentMu.Unlock()
	if a.chatRecents == nil {
		a.chatRecents = map[chatRecentKey]chatRecent{}
	}
	if a.chatRecentSeq == nil {
		a.chatRecentSeq = map[chatRecentKey]uint64{}
	}
	k := chatRecentKey{v.Plugin, v.FriendID}
	if last, ok := a.chatRecentSeq[k]; ok && v.Seq < last {
		return
	}
	a.chatRecentSeq[k] = v.Seq
	if old, ok := a.chatRecents[k]; ok && old == next {
		return
	}
	a.chatRecents[k] = next
	trimChatRecents(a.chatRecents)
	if a.chatRecentPath == "" {
		return
	}
	a.chatRecentDirty = true
	if a.chatRecentTimer == nil {
		a.chatRecentTimer = time.AfterFunc(chatRecentSaveDelay, func() { a.flushChatRecents() })
	}
}

// forgetChatPlugin drops a plugin's rows and its live conversations: a
// logout or removal must not leave the previous account's previews behind.
// The window hears about it: its rows are its own (some never came back
// through a view), so it can't tell they are gone otherwise.
func (a *App) forgetChatPlugin(id string) {
	if id == "" {
		return
	}
	a.chatRecentMu.Lock()
	changed := false
	for k := range a.chatRecents {
		if k.plugin == id {
			delete(a.chatRecents, k)
			changed = true
		}
	}
	for k := range a.chatRecentSeq {
		if k.plugin == id {
			delete(a.chatRecentSeq, k)
		}
	}
	if changed && a.chatRecentPath != "" {
		a.chatRecentDirty = true
	}
	a.chatRecentMu.Unlock()
	if a.chat != nil {
		a.chat.Forget(id)
	}
	if changed {
		a.flushChatRecents()
	}
	a.chatCleared(id)
}

// pruneChatRecentsOfRemoved drops the rows of plugins a scan no longer
// finds, after a scan that succeeded (like forgetSettingsOfRemoved, which
// owns the present set).
func (a *App) pruneChatRecentsOfRemoved(present map[string]bool) {
	pruned := map[string]bool{}
	a.chatRecentMu.Lock()
	for k := range a.chatRecents {
		if !present[k.plugin] {
			delete(a.chatRecents, k)
			pruned[k.plugin] = true
		}
	}
	for k := range a.chatRecentSeq {
		if !present[k.plugin] {
			delete(a.chatRecentSeq, k)
			pruned[k.plugin] = true
		}
	}
	if len(pruned) > 0 && a.chatRecentPath != "" {
		a.chatRecentDirty = true
	}
	a.chatRecentMu.Unlock()
	if a.chat != nil {
		for id := range pruned {
			a.chat.Forget(id)
		}
	}
	if len(pruned) > 0 {
		a.flushChatRecents()
		for id := range pruned {
			a.chatCleared(id)
		}
	}
}

// chatOffers says whether a plugin's rows may be listed: installed with the
// chat capability. Readiness is not required, so rows survive until the
// plugin starts on demand. Without a plugin manager there is nothing to
// check against, and rows are kept.
func (a *App) chatOffers(id string) bool {
	if a.plugins == nil {
		return true
	}
	return a.plugins.Has(id, "chat")
}

// ChatConversations lists the recent conversations with friends, most recent
// first, read or unread: the live conversations overlaid on the persisted
// previews, so a reload still lists what the session saw. Views the manager
// holds win (their unread and open state are current); a persisted preview
// fills in the last message when memory holds none, and covers the
// conversations memory dropped. Persisted rows always come back read:
// unread comes from the live manager alone. Rows of plugins that no longer
// offer chat are skipped. It never returns nil.
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
		if !a.chatOffers(r.Plugin) {
			continue
		}
		v := chat.View{Plugin: r.Plugin, FriendID: r.FriendID, More: r.Text != ""}
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

// lastChatAt is what the listing sorts by: the last message's SentAt, or 0
// when the conversation holds none (a live unread seed, or a persisted row
// without a preview), which sorts last. The file's own order uses At instead
// (see chatRecent.At): that one is the last change, the listing is the last
// message.
func lastChatAt(v chat.View) int64 {
	if len(v.Messages) > 0 {
		return v.Messages[len(v.Messages)-1].SentAt
	}
	return 0
}
