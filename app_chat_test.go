package main

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"

	"hydra-launcher/internal/chat"
)

const (
	chatAna = "7656119800000001" // echo's friend with a short history
	chatBot = "7656119800000009" // echo's auto-reply friend: reverses what the user writes
)

// chatSink records what the conversation manager emits.
type chatSink struct {
	mu  sync.Mutex
	evs []chat.Event
}

func (s *chatSink) emit(e chat.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, e)
}

func (s *chatSink) alerts(friend string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range s.evs {
		if e.Alert && e.FriendID == friend {
			n++
		}
	}
	return n
}

// chatEnv is the social environment with a conversation manager on the echo
// plugin, whose manifest declares the chat capability.
func chatEnv(t *testing.T) (*App, *chatSink) {
	t.Helper()
	a, st := socialEnv(t)
	sink := &chatSink{}
	a.chat = chat.NewManager(chat.Options{
		Backend: chatBackend{a},
		Alert:   st.ChatAlerts,
		Emit:    sink.emit,
		Logf:    func(string, ...any) {},
	})
	t.Cleanup(func() { a.chat.Forget("echo") })
	return a, sink
}

// echoChatSent is the chat.send calls echo received, in order.
func echoChatSent(t *testing.T, a *App) []map[string]string {
	t.Helper()
	var rec struct {
		Chat struct {
			Sent []map[string]string `json:"sent"`
		} `json:"chat"`
	}
	if err := a.plugins.Call(context.Background(), "echo", "recorded", nil, &rec); err != nil {
		t.Fatalf("recorded: %v", err)
	}
	return rec.Chat.Sent
}

// echoConfig sets echo's chat knobs.
func echoConfig(t *testing.T, a *App, chatKnobs map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"chat": chatKnobs})
	var out map[string]any
	if err := a.plugins.Call(context.Background(), "echo", "config", json.RawMessage(body), &out); err != nil {
		t.Fatalf("config: %v", err)
	}
}

// echoCallCount is how many times echo received a method.
func echoCallCount(t *testing.T, a *App, method string) int {
	t.Helper()
	var calls map[string]int
	if err := a.plugins.Call(context.Background(), "echo", "calls", nil, &calls); err != nil {
		t.Fatalf("calls: %v", err)
	}
	return calls[method]
}

func messageTexts(v chat.View) []string {
	out := []string{}
	for _, m := range v.Messages {
		out = append(out, m.Text)
	}
	return out
}

// Reading and answering a friend through the echo plugin: the history loads,
// a send is confirmed with the store's id, and the echo bot's answer counts as
// unread and alerts until the conversation is opened.
func TestChatWithTheEchoPlugin(t *testing.T) {
	a, sink := chatEnv(t)
	ctx := context.Background()
	echoConfig(t, a, map[string]any{"botDelayMs": 10})

	v, err := a.ChatOpen("echo", chatAna)
	if err != nil {
		t.Fatalf("ChatOpen: %v", err)
	}
	if !v.Loaded || !v.Open || len(v.Messages) != 3 || v.Messages[0].ID != "a1" {
		t.Fatalf("history: %+v", v)
	}

	if err := a.ChatSend("echo", chatAna, "hello"); err != nil {
		t.Fatalf("ChatSend: %v", err)
	}
	v = a.chat.Snapshot("echo", chatAna)
	if last := v.Messages[len(v.Messages)-1]; last.Text != "hello" || last.Status != "" || last.ID == "" || last.From != "self" {
		t.Fatalf("sent message: %+v", last)
	}
	if sent := echoChatSent(t, a); len(sent) != 1 || sent[0]["text"] != "hello" || sent[0]["friendId"] != chatAna || sent[0]["clientId"] == "" {
		t.Fatalf("echo received %v", sent)
	}

	// The bot answers while its conversation is closed.
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
	if got := a.ChatUnread(); len(got) != 1 || got[0].FriendID != chatBot || got[0].Unread != 1 {
		t.Fatalf("ChatUnread() = %+v", got)
	}
	if sink.alerts(chatBot) == 0 {
		t.Fatal("the bot's reply didn't alert while closed")
	}

	// Opening it reads it: the count clears and the store hears about it.
	v, err = a.ChatOpen("echo", chatBot)
	if err != nil {
		t.Fatal(err)
	}
	if v.Unread != 0 || !slices.Contains(messageTexts(v), "ereht ih") {
		t.Fatalf("after opening: unread %d, texts %v", v.Unread, messageTexts(v))
	}
	if len(a.ChatUnread()) != 0 {
		t.Fatal("ChatUnread still lists the conversation")
	}
	waitSocial(t, "markRead reached echo", func() bool {
		var rec struct {
			Chat struct {
				Marks []map[string]string `json:"marks"`
			} `json:"chat"`
		}
		if err := a.plugins.Call(ctx, "echo", "recorded", nil, &rec); err != nil {
			return false
		}
		return len(rec.Chat.Marks) > 0 && rec.Chat.Marks[len(rec.Chat.Marks)-1]["friendId"] == chatBot
	})
}

// The store's message alerts, when the user switches them off, don't alert.
func TestChatAlertsSwitch(t *testing.T) {
	a, sink := chatEnv(t)
	echoConfig(t, a, map[string]any{"botDelayMs": 10})
	if !a.ChatAlerts()["echo"] {
		t.Fatal("alerts are off by default")
	}
	if err := a.SetChatAlerts("echo", false); err != nil {
		t.Fatal(err)
	}
	if a.ChatAlerts()["echo"] {
		t.Fatal("alerts still on after switching off")
	}
	a.ChatClose("echo", chatBot)
	if _, err := a.ChatOpen("echo", chatBot); err != nil {
		t.Fatal(err)
	}
	a.ChatClose("echo", chatBot)
	if err := a.ChatSend("echo", chatBot, "quiet please"); err != nil {
		t.Fatal(err)
	}
	waitSocial(t, "the bot's reply", func() bool {
		return a.chat.Snapshot("echo", chatBot).Unread == 1
	})
	if n := sink.alerts(chatBot); n != 0 {
		t.Fatalf("%d alerts with the store's alerts off", n)
	}
	if err := a.SetChatAlerts("echo", true); err != nil {
		t.Fatal(err)
	}
}

// Without the social capability at runtime, chat is off: nothing is sent to the
// plugin's chat methods.
func TestChatNeedsSocial(t *testing.T) {
	t.Setenv("ECHO_OFF", "social")
	a, _ := chatEnv(t)
	if _, err := a.ChatOpen("echo", chatAna); err == nil {
		t.Fatal("chat opened on a plugin with social off")
	}
	if err := a.ChatSend("echo", chatAna, "hi"); err == nil {
		t.Fatal("chat sent on a plugin with social off")
	}
	if n := echoCallCount(t, a, "chat.history") + echoCallCount(t, a, "chat.send"); n != 0 {
		t.Fatalf("the plugin received %d chat calls", n)
	}
	if got := a.ChatUnread(); len(got) != 0 {
		t.Fatalf("ChatUnread lists %d conversations for a plugin without chat", len(got))
	}
}

// A plugin that settled on 1.8 never gets a chat method, whatever its manifest
// says, and has no chat in its alerts.
func TestChatStaysOffForAProtocol18Plugin(t *testing.T) {
	t.Setenv("ECHO_PROTOCOLS", "1.0,1.1,1.2,1.3,1.4,1.5,1.6,1.7,1.8")
	a, _ := chatEnv(t)
	if _, err := a.ChatOpen("echo", chatAna); err == nil {
		t.Fatal("chat opened on a 1.8 plugin")
	}
	if n := echoCallCount(t, a, "chat.history"); n != 0 {
		t.Fatalf("a 1.8 plugin received chat.history %d time(s)", n)
	}
	if _, ok := a.ChatAlerts()["echo"]; ok {
		t.Fatal("a 1.8 plugin is listed for chat alerts")
	}
	if err := a.SetChatAlerts("echo", false); err == nil {
		t.Fatal("alerts switched for a plugin without chat")
	}
}

// A send the store refuses shows as failed; resending it sends the same
// clientId again and the store keeps one message.
func TestChatSendFailureAndResend(t *testing.T) {
	a, _ := chatEnv(t)
	if _, err := a.ChatOpen("echo", chatAna); err != nil {
		t.Fatal(err)
	}
	echoConfig(t, a, map[string]any{"sendFail": 1})

	if err := a.ChatSend("echo", chatAna, "one more try"); err == nil {
		t.Fatal("a refused send reported success")
	}
	v := a.chat.Snapshot("echo", chatAna)
	failed := v.Messages[len(v.Messages)-1]
	if failed.Status != "failed" || failed.ClientID == "" {
		t.Fatalf("refused message: %+v", failed)
	}
	if err := a.ChatResend("echo", chatAna, failed.ClientID); err != nil {
		t.Fatalf("ChatResend: %v", err)
	}
	v = a.chat.Snapshot("echo", chatAna)
	if last := v.Messages[len(v.Messages)-1]; last.Status != "" || last.Text != "one more try" {
		t.Fatalf("after resend: %+v", last)
	}
	if n := countText(echoChatSent(t, a), "one more try"); n != 1 {
		t.Fatalf("the store holds %d copies", n)
	}
}

// The plugin exits on a send: core retries once with the same clientId, and the
// message goes through once.
func TestChatSendRetriesAfterACrash(t *testing.T) {
	a, _ := chatEnv(t)
	if _, err := a.ChatOpen("echo", chatAna); err != nil {
		t.Fatal(err)
	}
	echoConfig(t, a, map[string]any{"sendCrash": 1})
	if err := a.ChatSend("echo", chatAna, "after a crash"); err != nil {
		t.Fatalf("the retry should have delivered it: %v", err)
	}
	v := a.chat.Snapshot("echo", chatAna)
	if last := v.Messages[len(v.Messages)-1]; last.Status != "" || last.Text != "after a crash" {
		t.Fatalf("after the retry: %+v", last)
	}
	if n := countText(echoChatSent(t, a), "after a crash"); n != 1 {
		t.Fatalf("the store holds %d copies after a retry", n)
	}
}

// Hydra refuses a send that is empty or too long before the plugin sees it.
func TestChatRefusesBadTextLocally(t *testing.T) {
	a, _ := chatEnv(t)
	if err := a.ChatSend("echo", chatAna, "   "); err == nil {
		t.Fatal("an empty message was sent")
	}
	if n := echoCallCount(t, a, "chat.send"); n != 0 {
		t.Fatalf("the plugin received %d chat.send for a refused message", n)
	}
}

// Echo's periodic messages reach the conversations through the plugin's
// notifications: unread counts, and a message in a closed conversation alerts.
func TestChatIncomingNotifications(t *testing.T) {
	a, sink := chatEnv(t)
	echoConfig(t, a, map[string]any{"tick": 20})
	waitSocial(t, "an incoming message alerted", func() bool {
		return sink.alerts("7656119800000001")+sink.alerts("7656119800000002")+sink.alerts("7656119800000007") > 0
	})
	echoConfig(t, a, map[string]any{"tick": 0})
	if got := a.ChatUnread(); len(got) == 0 {
		t.Fatal("no conversation shows unread messages")
	}
}

func countText(sent []map[string]string, text string) int {
	n := 0
	for _, s := range sent {
		if s["text"] == text {
			n++
		}
	}
	return n
}
