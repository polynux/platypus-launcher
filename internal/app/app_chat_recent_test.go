package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"platypus/internal/chat"
)

// recentsEnv is chatEnv with the production emit hook: every conversation
// change folds into the recents, as startChat wires it.
func recentsEnv(t *testing.T) *App {
	t.Helper()
	a, sink := chatEnv(t)
	a.chat.Forget("echo")
	st := a.settings
	a.chat = chat.NewManager(chat.Options{
		Backend: chatBackend{a},
		Alert:   st.ChatAlerts,
		Emit: func(e chat.Event) {
			if !e.Dropped {
				a.recordChatRecent(e.View)
			}
			sink.emit(e)
		},
		Logf: func(string, ...any) {},
	})
	return a
}

// waitRecent polls until the recents hold friend's conversation with text as
// its preview. The preview is recorded on the plugin's reader goroutine,
// just after the manager's snapshot flips, so tests sync on it before they
// drop memory or reopen the file.
func waitRecent(t *testing.T, a *App, friend, text string) {
	t.Helper()
	waitSocial(t, "the reply in the recents", func() bool {
		for _, v := range a.ChatConversations() {
			if v.Plugin == "echo" && v.FriendID == friend && len(v.Messages) > 0 &&
				v.Messages[len(v.Messages)-1].Text == text {
				return true
			}
		}
		return false
	})
}
func TestChatConversationsSurviveForget(t *testing.T) {
	// The Chats tab after a reload: the recents list the session's read and
	// unread conversations, most recent first, once memory is dropped.
	t.Parallel()
	a := recentsEnv(t)
	// Memory only until a data dir opens: the listing still works.
	a.openChatRecents(t.TempDir())
	t.Cleanup(a.flushChatRecents)
	echoConfig(t, a, map[string]any{"botDelayMs": 10})

	if _, err := a.ChatOpen("echo", chatAna); err != nil {
		t.Fatal(err)
	}
	a.ChatClose("echo", chatAna)
	if _, err := a.ChatOpen("echo", chatBot); err != nil {
		t.Fatal(err)
	}
	a.ChatClose("echo", chatBot)
	if err := a.ChatSend("echo", chatBot, "hi there"); err != nil {
		t.Fatal(err)
	}
	waitSocial(t, "the bot's reply, unread", func() bool {
		return a.chat.Snapshot("echo", chatBot).Unread == 1
	})
	// The preview is recorded on the plugin reader, just after the snapshot
	// above flips: wait for it too, before dropping memory.
	waitRecent(t, a, chatBot, "ereht ih")

	// A restart drops the conversations from memory; the recents cover them.
	a.chat.Forget("echo")
	if got := a.ChatUnread(); len(got) != 0 {
		t.Fatalf("ChatUnread after forgetting holds %d conversations", len(got))
	}
	got := a.ChatConversations()
	if len(got) != 2 {
		t.Fatalf("ChatConversations() = %+v, want the bot and ana", got)
	}
	// Most recent first: the bot's exchange came after ana's history load.
	if got[0].FriendID != chatBot || got[1].FriendID != chatAna {
		t.Fatalf("order = [%s %s], want [bot ana]", got[0].FriendID, got[1].FriendID)
	}
	if got[0].Unread != 0 {
		t.Fatalf("bot unread %d, want 0: persisted rows come back read, only the live snapshot sets unread (C1)", got[0].Unread)
	}
	if len(got[0].Messages) == 0 || got[0].Messages[len(got[0].Messages)-1].Text != "ereht ih" {
		t.Fatalf("bot preview = %+v, want the reversed reply last", got[0].Messages)
	}
	if len(got[1].Messages) == 0 || got[1].Messages[len(got[1].Messages)-1].Text != "tell me how it runs" {
		t.Fatalf("ana preview = %+v, want her history's last line", got[1].Messages)
	}
	if got[1].Unread != 0 {
		t.Fatalf("ana unread %d, want 0 (read, still listed)", got[1].Unread)
	}
}

// The previews persist across restarts: a fresh App over the same data dir
// lists them, always read (C1: unread comes only from the live snapshot).
func TestChatConversationsPersistAcrossRestarts(t *testing.T) {
	t.Parallel()
	a := recentsEnv(t)
	dir := t.TempDir()
	a.openChatRecents(dir)
	t.Cleanup(a.flushChatRecents)
	echoConfig(t, a, map[string]any{"botDelayMs": 10})
	if _, err := a.ChatOpen("echo", chatBot); err != nil {
		t.Fatal(err)
	}
	a.ChatClose("echo", chatBot)
	if err := a.ChatSend("echo", chatBot, "hi there"); err != nil {
		t.Fatal(err)
	}
	waitSocial(t, "the bot's reply, unread", func() bool {
		return a.chat.Snapshot("echo", chatBot).Unread == 1
	})
	waitRecent(t, a, chatBot, "ereht ih")
	// The save is debounced (C2): flush instead of sleeping for it.
	a.flushChatRecents()
	if _, err := os.Stat(filepath.Join(dir, chatRecentFileName)); err != nil {
		t.Fatalf("no recents file: %v", err)
	}

	b := &App{host: stderrHost{}}
	b.openChatRecents(dir)
	got := b.ChatConversations()
	if len(got) != 1 || got[0].FriendID != chatBot || got[0].Unread != 0 {
		t.Fatalf("after restart: %+v, want the bot read with its preview", got)
	}
	if len(got[0].Messages) == 0 || got[0].Messages[len(got[0].Messages)-1].Text != "ereht ih" {
		t.Fatalf("after restart preview = %+v", got[0].Messages)
	}
}

// A corrupt or newer recents file is moved aside and loads empty, like the
// other data files; an old file without the newer fields still loads.
func TestChatRecentsFileVersions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, chatRecentFileName)
	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{host: stderrHost{}}
	a.openChatRecents(dir)
	if len(a.ChatConversations()) != 0 {
		t.Fatal("a corrupt file loaded conversations")
	}
	if matches, _ := filepath.Glob(path + ".bad-*"); len(matches) != 1 {
		t.Fatalf("corrupt file not moved aside: %v", matches)
	}

	if err := os.WriteFile(path, []byte(`{"version":999,"recents":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &App{host: stderrHost{}}
	b.openChatRecents(dir)
	if len(b.ChatConversations()) != 0 {
		t.Fatal("a newer file loaded conversations")
	}

	old := `{"version":1,"recents":[{"plugin":"echo","friendId":"9","unread":2,"text":"stale hi","sentAt":11,"at":12}]}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &App{host: stderrHost{}}
	c.openChatRecents(dir)
	got := c.ChatConversations()
	if len(got) != 1 || got[0].FriendID != "9" || got[0].Unread != 0 {
		t.Fatalf("old file: %+v, want the row read: persisted unread never applies (C1)", got)
	}
	if len(got[0].Messages) == 0 || got[0].Messages[0].Text != "stale hi" {
		t.Fatalf("old file: %+v, want the preview kept", got)
	}

	// An old row without a preview carries nothing now: no unread is
	// persisted and there is no text to show, so it is dropped at load.
	older := `{"version":1,"recents":[{"plugin":"echo","friendId":"8","unread":2}]}`
	if err := os.WriteFile(path, []byte(older), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &App{host: stderrHost{}}
	d.openChatRecents(dir)
	if got := d.ChatConversations(); len(got) != 0 {
		t.Fatalf("preview-less row: %+v, want nothing listed", got)
	}
}

// Without a data dir the recents are memory only: the listing still works
// in the session, and nothing is written.
func TestChatConversationsMemoryOnly(t *testing.T) {
	t.Parallel()
	a := recentsEnv(t)
	echoConfig(t, a, map[string]any{"botDelayMs": 10})
	if _, err := a.ChatOpen("echo", chatAna); err != nil {
		t.Fatal(err)
	}
	got := a.ChatConversations()
	if len(got) != 1 || got[0].FriendID != chatAna {
		t.Fatalf("ChatConversations() = %+v, want ana from memory", got)
	}
}

// A persisted preview is a snippet (C2/C4): a long message is clipped to
// about 200 runes before it reaches the file.
func TestChatRecentPreviewIsClipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := &App{host: stderrHost{}}
	a.openChatRecents(dir)
	t.Cleanup(a.flushChatRecents)
	long := strings.Repeat("é", 500)
	a.recordChatRecent(chat.View{
		Plugin: "echo", FriendID: "1",
		Messages: []chat.Message{{From: "1", Text: long, SentAt: 7}},
	})
	a.flushChatRecents()
	b := &App{host: stderrHost{}}
	b.openChatRecents(dir)
	got := b.ChatConversations()
	if len(got) != 1 || utf8.RuneCountInString(got[0].Messages[0].Text) != chatRecentPreviewRunes {
		t.Fatalf("preview holds %d runes, want %d", utf8.RuneCountInString(got[0].Messages[0].Text), chatRecentPreviewRunes)
	}
	if want := strings.Repeat("é", chatRecentPreviewRunes); got[0].Messages[0].Text != want {
		t.Fatal("preview is not the message's first 200 runes")
	}
}

// Recording never writes on the caller's path (C2): a burst of incoming
// messages marks rows dirty, and one background save covers them. Tests
// flush instead of sleeping for the debounce.
func TestChatRecentsSaveIsDebounced(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := &App{host: stderrHost{}}
	a.openChatRecents(dir)
	t.Cleanup(a.flushChatRecents)
	a.recordChatRecent(chat.View{
		Plugin: "echo", FriendID: "1",
		Messages: []chat.Message{{From: "1", Text: "hi", SentAt: 7}},
	})
	if _, err := os.Stat(filepath.Join(dir, chatRecentFileName)); !os.IsNotExist(err) {
		t.Fatal("record wrote the file synchronously; saves must be debounced")
	}
	a.flushChatRecents()
	b := &App{host: stderrHost{}}
	b.openChatRecents(dir)
	if got := b.ChatConversations(); len(got) != 1 || got[0].FriendID != "1" {
		t.Fatalf("after flush: %+v, want the row", got)
	}
}

// Logging out ends the account's chats (C3): its rows are deleted and its
// live conversations forgotten, so the next account starts with no Chats.
func TestChatRecentsClearedOnLogout(t *testing.T) {
	t.Parallel()
	a := recentsEnv(t)
	dir := t.TempDir()
	a.openChatRecents(dir)
	t.Cleanup(a.flushChatRecents)
	a.recordChatRecent(chat.View{
		Plugin: "echo", FriendID: chatBot,
		Messages: []chat.Message{{From: chatBot, Text: "hi", SentAt: 1}},
	})
	a.flushChatRecents()
	if got := a.ChatConversations(); len(got) != 1 {
		t.Fatalf("before logout: %+v, want the row", got)
	}
	if _, err := a.PluginLogout("echo"); err != nil {
		t.Fatal(err)
	}
	if got := a.ChatConversations(); len(got) != 0 {
		t.Fatalf("after logout: %+v, want no rows", got)
	}
	if v := a.chat.Snapshot("echo", chatBot); len(v.Messages) != 0 || v.Unread != 0 {
		t.Fatalf("after logout the live conversation survived: %+v", v)
	}
	b := &App{host: stderrHost{}}
	b.openChatRecents(dir)
	if got := b.ChatConversations(); len(got) != 0 {
		t.Fatalf("after logout the file still lists: %+v", got)
	}
}

// Rows of plugins that no longer offer chat are never listed (C3): an
// uninstalled store's friend ids and previews stay off the Chats tab, even
// though its rows are still on disk until the next rescan prunes them.
func TestChatConversationsSkipsPluginsWithoutChat(t *testing.T) {
	t.Parallel()
	a := recentsEnv(t)
	dir := t.TempDir()
	a.openChatRecents(dir)
	t.Cleanup(a.flushChatRecents)
	a.recordChatRecent(chat.View{
		Plugin: "echo", FriendID: chatBot,
		Messages: []chat.Message{{From: chatBot, Text: "hi", SentAt: 1}},
	})
	a.recordChatRecent(chat.View{
		Plugin: "ghost", FriendID: "1",
		Messages: []chat.Message{{From: "1", Text: "old account", SentAt: 2}},
	})
	got := a.ChatConversations()
	if len(got) != 1 || got[0].Plugin != "echo" {
		t.Fatalf("ChatConversations() = %+v, want only echo", got)
	}
}

// A rescan that no longer finds a plugin takes its rows with it (C3).
func TestChatRecentsPrunedOnRemoval(t *testing.T) {	t.Parallel()
	a := recentsEnv(t)
	dir := t.TempDir()
	a.openChatRecents(dir)
	t.Cleanup(a.flushChatRecents)
	a.recordChatRecent(chat.View{
		Plugin: "echo", FriendID: chatBot,
		Messages: []chat.Message{{From: chatBot, Text: "hi", SentAt: 1}},
	})
	a.recordChatRecent(chat.View{
		Plugin: "ghost", FriendID: "1",
		Messages: []chat.Message{{From: "1", Text: "old account", SentAt: 2}},
	})
	a.flushChatRecents()
	a.forgetSettingsOfRemoved(a.plugins.Plugins())
	got := a.ChatConversations()
	if len(got) != 1 || got[0].Plugin != "echo" {
		t.Fatalf("after the rescan: %+v, want only echo", got)
	}
	b := &App{host: stderrHost{}}
	b.openChatRecents(dir)
	if got := b.ChatConversations(); len(got) != 1 || got[0].Plugin != "echo" {
		t.Fatalf("after the rescan the file still lists: %+v", got)
	}
}

// A view that arrives late is older than one already recorded (C5): the
// manager stamps every change with a rising seq, so the preview keeps the
// newer message instead of being rewritten by the straggler.
func TestChatRecentsDropAnOutOfOrderView(t *testing.T) {
	t.Parallel()
	a := &App{host: stderrHost{}}
	a.openChatRecents(t.TempDir())
	t.Cleanup(a.flushChatRecents)
	view := func(seq uint64, text string) chat.View {
		return chat.View{
			Seq: seq, Plugin: "echo", FriendID: "1",
			Messages: []chat.Message{{From: "1", Text: text, SentAt: 3}},
		}
	}
	a.recordChatRecent(view(9, "newest"))
	a.recordChatRecent(view(4, "stale"))
	got := a.ChatConversations()
	if len(got) != 1 || got[0].Messages[0].Text != "newest" {
		t.Fatalf("preview = %+v, want the newer message kept", got)
	}
	a.recordChatRecent(view(10, "newer still"))
	got = a.ChatConversations()
	if len(got) != 1 || got[0].Messages[0].Text != "newer still" {
		t.Fatalf("preview = %+v, want the newer message recorded", got)
	}
}

// chatClearedHost records the "chat" events an App emits in tests.
type chatClearedHost struct {
	mu     sync.Mutex
	events []chat.Event
}

func (h *chatClearedHost) Emit(event string, data ...any) {
	if event != "chat" || len(data) == 0 {
		return
	}
	e, ok := data[0].(chat.Event)
	if !ok {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, e)
}

func (h *chatClearedHost) Infof(string, ...any)  {}
func (h *chatClearedHost) Warnf(string, ...any)  {}
func (h *chatClearedHost) Errorf(string, ...any) {}

func (h *chatClearedHost) OpenFile(FileDialog) (string, error) {
	return "", errors.New("no dialog in tests")
}

func (h *chatClearedHost) OpenDirectory(DirectoryDialog) (string, error) {
	return "", errors.New("no dialog in tests")
}

// Logging out clears the account's chats in the open window too (C6): the
// rows it holds are its own, so the core says they are gone with a dropped
// event that names the store and no friend.
func TestChatRecentsClearedOnLogoutIsAnnounced(t *testing.T) {
	t.Parallel()
	a := recentsEnv(t)
	host := &chatClearedHost{}
	a.host = host
	a.recordChatRecent(chat.View{
		Plugin: "echo", FriendID: chatBot,
		Messages: []chat.Message{{From: chatBot, Text: "hi", SentAt: 1}},
	})
	if _, err := a.PluginLogout("echo"); err != nil {
		t.Fatal(err)
	}
	host.mu.Lock()
	defer host.mu.Unlock()
	for _, e := range host.events {
		if e.Dropped && e.Plugin == "echo" && e.FriendID == "" {
			return
		}
	}
	t.Fatalf("logout emitted %+v, want a dropped event for the whole store", host.events)
}
