package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/jsonrpc"
)

// fakeStore is a plugin that offers chat: it keeps the history per friend,
// answers chat.send idempotently per clientId, and records every call.
type fakeStore struct {
	mu       sync.Mutex
	offers   bool
	history  map[string][]Message
	sent     map[string]Message // by clientId
	nextID   int
	sendErrs []error // one per chat.send call, in order; nil means succeed
	calls    []string
	marks    []string
	more     bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{offers: true, history: map[string][]Message{}, sent: map[string]Message{}}
}

func (f *fakeStore) Offers(ctx context.Context, pluginID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.offers
}

func (f *fakeStore) Call(ctx context.Context, pluginID, method string, params, result any) error {
	raw, _ := json.Marshal(params)
	var p struct {
		FriendID string `json:"friendId"`
		Before   string `json:"before"`
		Limit    int    `json:"limit"`
		Text     string `json:"text"`
		ClientID string `json:"clientId"`
		UpTo     string `json:"upTo"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method)
	switch method {
	case MethodHistory:
		msgs := f.history[p.FriendID]
		end := len(msgs)
		if p.Before != "" {
			end = slices.IndexFunc(msgs, func(m Message) bool { return m.ID == p.Before })
		}
		start := max(0, end-p.Limit)
		page := slices.Clone(msgs[start:end])
		b, _ := json.Marshal(map[string]any{"messages": page, "more": start > 0})
		return json.Unmarshal(b, result)
	case MethodSend:
		if len(f.sendErrs) > 0 {
			err := f.sendErrs[0]
			f.sendErrs = f.sendErrs[1:]
			if err != nil {
				return err
			}
		}
		if m, ok := f.sent[p.ClientID]; ok {
			return json.Unmarshal([]byte(fmt.Sprintf(`{"id":%q,"sentAt":%d}`, m.ID, m.SentAt)), result)
		}
		f.nextID++
		m := Message{ID: fmt.Sprintf("m%d", f.nextID), From: "self", Text: p.Text, SentAt: time.Now().UnixMilli()}
		f.sent[p.ClientID] = m
		f.history[p.FriendID] = append(f.history[p.FriendID], m)
		return json.Unmarshal([]byte(fmt.Sprintf(`{"id":%q,"sentAt":%d}`, m.ID, m.SentAt)), result)
	case MethodMarkRead:
		f.marks = append(f.marks, p.UpTo)
		return nil
	}
	return &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not found: " + method}
}

// harness is a manager around a fake store, recording its events and logs.
type harness struct {
	t     *testing.T
	store *fakeStore
	m     *Manager
	mu    sync.Mutex
	evs   []Event
	logs  strings.Builder
	alert bool
}

func newHarness(t *testing.T, mut ...func(*Options)) *harness {
	t.Helper()
	h := &harness{t: t, store: newFakeStore(), alert: true}
	o := Options{
		Backend:    h.store,
		RetryDelay: time.Millisecond,
		Emit: func(e Event) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.evs = append(h.evs, e)
		},
		Logf: func(format string, args ...any) {
			h.mu.Lock()
			defer h.mu.Unlock()
			fmt.Fprintf(&h.logs, format+"\n", args...)
		},
		Alert: func(string) bool { return h.alert },
	}
	for _, f := range mut {
		f(&o)
	}
	h.m = NewManager(o)
	return h
}

// events returns the events emitted so far.
func (h *harness) events() []Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.evs)
}

func (h *harness) logText() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.logs.String()
}

func (h *harness) notify(method string, params any) {
	raw, err := json.Marshal(params)
	if err != nil {
		h.t.Fatal(err)
	}
	h.m.Notify("echo", method, raw)
}

func (h *harness) incoming(friend, id, text string, sentAt int64) {
	h.notify(NoteMessage, map[string]any{"friendId": friend, "message": Message{ID: id, From: friend, Text: text, SentAt: sentAt}})
}

func countOf(list []string, v string) int {
	n := 0
	for _, x := range list {
		if x == v {
			n++
		}
	}
	return n
}

func ids(msgs []Message) []string {
	out := []string{}
	for _, m := range msgs {
		out = append(out, m.ID)
	}
	return out
}

// eventually polls cond for a second: the markRead that follows an incoming
// message runs in the background.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s never happened", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// Messages from a notification and from history come out in sentAt order,
// whatever order they arrive in, and an id is never shown twice.
func TestOrderingAndDedupe(t *testing.T) {
	h := newHarness(t)
	h.store.history["ana"] = []Message{
		{ID: "h1", From: "ana", Text: "one", SentAt: 1000},
		{ID: "h2", From: "ana", Text: "two", SentAt: 2000},
	}
	h.incoming("ana", "n3", "three", 3000)
	h.incoming("ana", "n15", "late", 1500) // arrives late, belongs between h1 and h2
	h.incoming("ana", "h2", "two again", 2000)

	v, err := h.m.Open(context.Background(), "echo", "ana")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(v.Messages), []string{"h1", "n15", "h2", "n3"}; !slices.Equal(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	if !v.Loaded || !v.Open || v.More {
		t.Fatalf("view %+v", v)
	}
}

// An older page is merged in front of the messages the conversation holds.
func TestOlderPagesPrepend(t *testing.T) {
	h := newHarness(t, func(o *Options) { o.Page = 2 })
	var hist []Message
	for i := 1; i <= 5; i++ {
		hist = append(hist, Message{ID: fmt.Sprintf("h%d", i), From: "ana", Text: "t", SentAt: int64(i * 10)})
	}
	h.store.history["ana"] = hist
	v, err := h.m.Open(context.Background(), "echo", "ana")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(v.Messages); !slices.Equal(got, []string{"h4", "h5"}) || !v.More {
		t.Fatalf("first page %v more %v", got, v.More)
	}
	v, err = h.m.Older(context.Background(), "echo", "ana")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(v.Messages); !slices.Equal(got, []string{"h2", "h3", "h4", "h5"}) || !v.More {
		t.Fatalf("second page %v more %v", got, v.More)
	}
	v, _ = h.m.Older(context.Background(), "echo", "ana")
	v, _ = h.m.Older(context.Background(), "echo", "ana")
	if got := ids(v.Messages); !slices.Equal(got, []string{"h1", "h2", "h3", "h4", "h5"}) || v.More {
		t.Fatalf("last page %v more %v", got, v.More)
	}
}

// Off, or on a plugin that doesn't offer chat, nothing reaches the plugin.
func TestNothingSentWhenChatIsOff(t *testing.T) {
	h := newHarness(t)
	h.store.offers = false
	ctx := context.Background()
	if _, err := h.m.Open(ctx, "echo", "ana"); err == nil {
		t.Fatal("Open worked on a plugin that doesn't offer chat")
	}
	if err := h.m.Send(ctx, "echo", "ana", "hi"); err == nil {
		t.Fatal("Send worked on a plugin that doesn't offer chat")
	}
	if _, err := h.m.Older(ctx, "echo", "ana"); err == nil {
		t.Fatal("Older worked on a plugin that doesn't offer chat")
	}
	if len(h.store.calls) != 0 {
		t.Fatalf("the plugin received %v", h.store.calls)
	}
	if len(h.events()) != 0 {
		t.Fatalf("events for a conversation that never opened: %d", len(h.events()))
	}
}

// Empty, too long and malformed text is refused before it reaches the plugin.
func TestSendRefusesBadText(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	long := strings.Repeat("a", MaxText+1)
	for _, text := range []string{"", "   \n\t", long, "\xff\xfe"} {
		if err := h.m.Send(ctx, "echo", "ana", text); err == nil {
			t.Errorf("Send accepted %q", text[:min(len(text), 12)])
		}
	}
	if len(h.store.calls) != 0 {
		t.Fatalf("the plugin received %v", h.store.calls)
	}
	exact := strings.Repeat("é", MaxText)
	if err := h.m.Send(ctx, "echo", "ana", exact); err != nil {
		t.Fatalf("a message of exactly %d characters: %v", MaxText, err)
	}
}

// A send the store answers is confirmed with its id; the clientId is dropped
// from the message the UI shows.
func TestSendConfirmsWithTheStoreId(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.m.Send(ctx, "echo", "ana", "hello"); err != nil {
		t.Fatal(err)
	}
	v := h.m.Snapshot("echo", "ana")
	if len(v.Messages) != 1 || v.Messages[0].ID != "m1" || v.Messages[0].From != "self" || v.Messages[0].Status != "" || v.Messages[0].ClientID != "" {
		t.Fatalf("messages %+v", v.Messages)
	}
	// The first event after the send shows it "sending", the last one confirmed.
	evs := h.events()
	if evs[0].Messages[0].Status != "sending" || evs[0].Messages[0].ClientID == "" {
		t.Fatalf("first event %+v", evs[0].Messages)
	}
}

// A transport error is retried once under the same clientId: the store sees the
// message twice but keeps one, and the conversation shows it once.
func TestRetryKeepsOneMessage(t *testing.T) {
	h := newHarness(t)
	h.store.sendErrs = []error{errors.New("the plugin exited")}
	ctx := context.Background()
	if err := h.m.Send(ctx, "echo", "ana", "hello"); err != nil {
		t.Fatalf("the retry should have worked: %v", err)
	}
	if n := countOf(h.store.calls, MethodSend); n != 2 {
		t.Fatalf("chat.send called %d times, want 2", n)
	}
	v := h.m.Snapshot("echo", "ana")
	if len(v.Messages) != 1 || v.Messages[0].ID != "m1" || v.Messages[0].Status != "" {
		t.Fatalf("messages %+v", v.Messages)
	}
	if len(h.store.history["ana"]) != 1 {
		t.Fatalf("the store kept %d copies", len(h.store.history["ana"]))
	}
}

// An error answer from the store is not retried: the message is marked failed,
// and resending it reuses its clientId.
func TestAnswerErrorIsNotRetriedAndResends(t *testing.T) {
	h := newHarness(t)
	h.store.sendErrs = []error{&jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "nope"}}
	ctx := context.Background()
	if err := h.m.Send(ctx, "echo", "ana", "hello"); err == nil {
		t.Fatal("a refused send reported success")
	}
	if n := countOf(h.store.calls, MethodSend); n != 1 {
		t.Fatalf("chat.send called %d times, want 1", n)
	}
	v := h.m.Snapshot("echo", "ana")
	if len(v.Messages) != 1 || v.Messages[0].Status != "failed" {
		t.Fatalf("messages %+v", v.Messages)
	}
	clientID := v.Messages[0].ClientID
	if err := h.m.Resend(ctx, "echo", "ana", clientID); err != nil {
		t.Fatal(err)
	}
	v = h.m.Snapshot("echo", "ana")
	if len(v.Messages) != 1 || v.Messages[0].ID != "m1" || v.Messages[0].Status != "" {
		t.Fatalf("after resend %+v", v.Messages)
	}
	if err := h.m.Resend(ctx, "echo", "ana", clientID); err == nil {
		t.Fatal("a sent message was sent again")
	}
}

// Two sends of the same clientId (a lost answer) give one message on the store.
func TestSendIsIdempotentPerClientID(t *testing.T) {
	s := newFakeStore()
	ctx := context.Background()
	var first, second struct {
		ID     string `json:"id"`
		SentAt int64  `json:"sentAt"`
	}
	params := map[string]any{"friendId": "ana", "text": "hi", "clientId": "c-1"}
	if err := s.Call(ctx, "echo", MethodSend, params, &first); err != nil {
		t.Fatal(err)
	}
	if err := s.Call(ctx, "echo", MethodSend, params, &second); err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(s.history["ana"]) != 1 {
		t.Fatalf("first %+v second %+v, kept %d", first, second, len(s.history["ana"]))
	}
}

// Messages from a friend count as unread while the conversation is closed, and
// alert the user when the store's messages are on; opening clears them and
// tells the store what was read.
func TestUnreadAndAlerts(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.incoming("ana", "n1", "hey", 100)
	h.incoming("ana", "n2", "you there?", 200)
	v := h.m.Snapshot("echo", "ana")
	if v.Unread != 2 || v.Open {
		t.Fatalf("closed conversation: unread %d open %v", v.Unread, v.Open)
	}
	evs := h.events()
	if !evs[len(evs)-1].Alert {
		t.Fatal("the last message from a friend didn't alert")
	}
	if got := h.m.Unread("echo"); len(got) != 1 || got[0].Unread != 2 || len(got[0].Messages) != 0 {
		t.Fatalf("Unread() = %+v", got)
	}

	if _, err := h.m.Open(ctx, "echo", "ana"); err != nil {
		t.Fatal(err)
	}
	v = h.m.Snapshot("echo", "ana")
	if v.Unread != 0 || !v.Open {
		t.Fatalf("opened: unread %d open %v", v.Unread, v.Open)
	}
	eventually(t, "markRead with the newest message", func() bool {
		h.store.mu.Lock()
		defer h.store.mu.Unlock()
		return slices.Equal(h.store.marks, []string{"n2"})
	})

	h.incoming("ana", "n3", "still here", 300)
	v = h.m.Snapshot("echo", "ana")
	if v.Unread != 0 {
		t.Fatalf("a message in an open conversation counted as unread: %d", v.Unread)
	}
	if last := h.events()[len(h.events())-1]; last.Alert {
		t.Fatal("a message in an open conversation alerted")
	}
	eventually(t, "markRead for the open message", func() bool {
		h.store.mu.Lock()
		defer h.store.mu.Unlock()
		return slices.Equal(h.store.marks, []string{"n2", "n3"})
	})
}

// The store's own unread count replaces Hydra's, and the alert is off when the
// store's messages are off.
func TestUnreadNotificationAndAlertSetting(t *testing.T) {
	h := newHarness(t)
	h.alert = false
	h.notify(NoteUnread, map[string]any{"friendId": "ana", "count": 4})
	if v := h.m.Snapshot("echo", "ana"); v.Unread != 4 {
		t.Fatalf("unread %d, want 4 from the store", v.Unread)
	}
	h.incoming("ana", "n1", "hey", 100)
	if v := h.m.Snapshot("echo", "ana"); v.Unread != 5 {
		t.Fatalf("unread %d after a message", v.Unread)
	}
	if evs := h.events(); evs[len(evs)-1].Alert {
		t.Fatal("alerted with the store's messages off")
	}
	h.notify(NoteUnread, map[string]any{"friendId": "ana", "count": -3})
	if v := h.m.Snapshot("echo", "ana"); v.Unread != 0 {
		t.Fatalf("a negative count left %d", v.Unread)
	}
}

// Typing shows for a while and clears on the next message from the friend.
func TestTypingClearsOnMessage(t *testing.T) {
	h := newHarness(t)
	h.notify(NoteTyping, map[string]any{"friendId": "ana"})
	if !h.m.Snapshot("echo", "ana").Typing {
		t.Fatal("typing not shown")
	}
	h.incoming("ana", "n1", "done", 100)
	if h.m.Snapshot("echo", "ana").Typing {
		t.Fatal("typing still shown after the message")
	}
}

// The conversations kept per plugin and messages per conversation are bounded:
// the oldest messages go, and the least recently used closed conversation.
func TestBoundsAreEnforced(t *testing.T) {
	h := newHarness(t, func(o *Options) { o.MaxMessages = 3; o.MaxConversations = 2 })
	for i := 1; i <= 5; i++ {
		h.incoming("ana", fmt.Sprintf("n%d", i), "t", int64(i))
	}
	v := h.m.Snapshot("echo", "ana")
	if got := ids(v.Messages); !slices.Equal(got, []string{"n3", "n4", "n5"}) || !v.More {
		t.Fatalf("kept %v more %v", got, v.More)
	}
	h.incoming("bo", "b1", "t", 1)
	h.incoming("cy", "c1", "t", 1) // a third friend: the least recently used closed one goes
	if got := h.m.Snapshot("echo", "ana"); len(got.Messages) != 0 {
		t.Fatalf("ana's conversation survived the bound: %d messages", len(got.Messages))
	}
	if got := h.m.Snapshot("echo", "cy"); len(got.Messages) != 1 {
		t.Fatalf("cy's conversation was lost: %d messages", len(got.Messages))
	}
}

// Forgetting a plugin drops its conversations from memory.
func TestForgetDropsTheConversations(t *testing.T) {
	h := newHarness(t)
	h.incoming("ana", "n1", "t", 1)
	h.m.Forget("echo")
	if got := h.m.Snapshot("echo", "ana"); len(got.Messages) != 0 || got.Unread != 0 {
		t.Fatalf("after Forget: %+v", got)
	}
}

// Message text never reaches the logs, on any path: history, send, a failed
// send, and an incoming message.
func TestMessageTextNeverLogs(t *testing.T) {
	const secret = "TOP-SECRET-MESSAGE-TEXT-7731"
	h := newHarness(t)
	ctx := context.Background()
	h.store.history["ana"] = []Message{{ID: "h1", From: "ana", Text: secret, SentAt: 1}}
	h.store.sendErrs = []error{&jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: secret}, errors.New(secret)}
	if _, err := h.m.Open(ctx, "echo", "ana"); err != nil {
		t.Fatal(err)
	}
	_ = h.m.Send(ctx, "echo", "ana", secret)
	_ = h.m.Send(ctx, "echo", "ana", secret) // the transport error is retried and fails again
	h.incoming("ana", "n1", secret, 5)
	h.notify(NoteUnread, map[string]any{"friendId": "ana", "count": 1})
	if strings.Contains(h.logText(), secret) {
		t.Fatalf("message text reached the logs:\n%s", h.logText())
	}
	if h.logText() == "" {
		t.Fatal("expected the failed send to be logged")
	}
}

// Incoming text longer than the cap is cut to it, not dropped.
func TestIncomingTextIsCut(t *testing.T) {
	h := newHarness(t)
	h.incoming("ana", "n1", strings.Repeat("x", MaxText+50), 1)
	v := h.m.Snapshot("echo", "ana")
	if len(v.Messages) != 1 || len([]rune(v.Messages[0].Text)) != MaxText {
		t.Fatalf("incoming text not cut: %d messages", len(v.Messages))
	}
}

// Malformed notifications, and ones without a friend or a message id, change
// nothing.
func TestMalformedNotificationsAreIgnored(t *testing.T) {
	h := newHarness(t)
	h.m.Notify("echo", NoteMessage, json.RawMessage(`{"friendId":"ana"`))
	h.notify(NoteMessage, map[string]any{"friendId": "ana", "message": map[string]any{"text": "no id"}})
	h.notify(NoteMessage, map[string]any{"message": map[string]any{"id": "x", "text": "no friend"}})
	h.notify(NoteTyping, map[string]any{"friendId": ""})
	h.m.Notify("echo", "chat.sticker", json.RawMessage(`{"friendId":"ana"}`))
	if len(h.events()) != 0 {
		t.Fatalf("events for malformed notifications: %d", len(h.events()))
	}
}

// Every change carries a Seq higher than the one before it.
func TestEventsAreSequenced(t *testing.T) {
	h := newHarness(t)
	h.incoming("ana", "n1", "a", 1)
	h.incoming("ana", "n2", "b", 2)
	var last uint64
	for _, e := range h.events() {
		if e.Seq <= last {
			t.Fatalf("seq %d after %d", e.Seq, last)
		}
		last = e.Seq
	}
}
