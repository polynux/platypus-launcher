package app

import (
	"os"
	"path/filepath"
	"testing"

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
	if got[0].Unread != 1 {
		t.Fatalf("bot unread %d, want 1", got[0].Unread)
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
// lists them with their unread counts, and a live snapshot's unread merges
// with the persisted preview.
func TestChatConversationsPersistAcrossRestarts(t *testing.T) {
	t.Parallel()
	a := recentsEnv(t)
	dir := t.TempDir()
	a.openChatRecents(dir)
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
	if _, err := os.Stat(filepath.Join(dir, chatRecentFileName)); err != nil {
		t.Fatalf("no recents file: %v", err)
	}

	b := &App{host: stderrHost{}}
	b.openChatRecents(dir)
	got := b.ChatConversations()
	if len(got) != 1 || got[0].FriendID != chatBot || got[0].Unread != 1 {
		t.Fatalf("after restart: %+v, want the bot with 1 unread", got)
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

	old := `{"version":1,"recents":[{"plugin":"echo","friendId":"9","unread":2}]}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &App{host: stderrHost{}}
	c.openChatRecents(dir)
	got := c.ChatConversations()
	if len(got) != 1 || got[0].FriendID != "9" || got[0].Unread != 2 || len(got[0].Messages) != 0 {
		t.Fatalf("old file: %+v, want the unread-only row", got)
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
