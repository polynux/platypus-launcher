package plugtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"platypus/internal/jsonrpc"
)

// The chat capability (protocol 1.9, docs/plugins/protocol.md, "Chat"): the
// keys the history and the send answer carry, and the refusals core relies on.
// The unread snapshot (chat.unreadCounts, protocol 1.13) is optional: a store
// without it answers -32601, and core keeps the notification-only behaviour.
var (
	ChatHistoryKeys      = []string{"messages", "more"}
	ChatMessageKeys      = []string{"id", "from", "text", "sentAt"}
	ChatSendKeys         = []string{"id", "sentAt"}
	ChatUnreadCountsKeys = []string{"counts"}
	ChatUnreadCountKeys  = []string{"friendId", "unread"}
)

// checkChat holds a plugin offering `chat` (with `social`) to the request and
// answer shapes: a friend it doesn't know, an empty or over-long text, and a
// missing clientId are -32602; a history page has the documented keys and
// oldest-first messages; a repeated clientId answers the same id; and a sent
// message is in the history afterwards.
func (s *Session) checkChat(t *testing.T) {
	t.Helper()
	friend := s.firstFriend(t)

	s.wantCode("chat.history", map[string]any{}, jsonrpc.CodeInvalidParams)
	s.wantCode("chat.history", map[string]any{"friendId": "no-such-friend"}, jsonrpc.CodeInvalidParams)
	s.wantCode("chat.send", map[string]any{"friendId": friend, "text": "", "clientId": "plugtest-empty"}, jsonrpc.CodeInvalidParams)
	s.wantCode("chat.send", map[string]any{"friendId": friend, "text": strings.Repeat("x", 4097), "clientId": "plugtest-long"}, jsonrpc.CodeInvalidParams)
	s.wantCode("chat.send", map[string]any{"friendId": friend, "text": "no id"}, jsonrpc.CodeInvalidParams)
	s.wantCode("chat.markRead", map[string]any{"friendId": "no-such-friend", "upTo": "x"}, jsonrpc.CodeInvalidParams)

	var page json.RawMessage
	if err := s.call("chat.history", map[string]any{"friendId": friend, "limit": 5}, &page); err != nil {
		t.Fatalf("chat.history: %v", err)
	}
	checkKeys(t, "chat.history", page, ChatHistoryKeys)
	var h struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(page, &h); err != nil {
		t.Fatalf("chat.history: %v", err)
	}
	var lastSent int64
	for _, raw := range h.Messages {
		checkKeys(t, "chat message", raw, ChatMessageKeys)
		var m struct {
			ID     string `json:"id"`
			From   string `json:"from"`
			SentAt int64  `json:"sentAt"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("chat message: %v", err)
		}
		if m.ID == "" || (m.From != "self" && m.From != friend) {
			t.Errorf("chat message without an id, or from neither self nor the friend: %s", raw)
		}
		if m.SentAt < lastSent {
			t.Errorf("chat.history is not oldest first: %s", raw)
		}
		lastSent = m.SentAt
	}

	clientID := fmt.Sprintf("plugtest-%d", time.Now().UnixNano())
	params := map[string]any{"friendId": friend, "text": "plugtest hello", "clientId": clientID}
	var first, again struct {
		ID     string `json:"id"`
		SentAt int64  `json:"sentAt"`
	}
	if err := s.call("chat.send", params, &first); err != nil {
		t.Fatalf("chat.send: %v", err)
	}
	if first.ID == "" {
		t.Fatal("chat.send answered without an id")
	}
	if err := s.call("chat.send", params, &again); err != nil {
		t.Fatalf("chat.send again with the same clientId: %v", err)
	}
	if again.ID != first.ID {
		t.Errorf("chat.send is not idempotent per clientId: %q then %q", first.ID, again.ID)
	}
	if err := s.call("chat.markRead", map[string]any{"friendId": friend, "upTo": first.ID}, nil); err != nil {
		t.Fatalf("chat.markRead: %v", err)
	}

	var after struct {
		Messages []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	if err := s.call("chat.history", map[string]any{"friendId": friend, "limit": 5}, &after); err != nil {
		t.Fatalf("chat.history after a send: %v", err)
	}
	ids := []string{}
	for _, m := range after.Messages {
		ids = append(ids, m.ID)
	}
	if !slices.Contains(ids, first.ID) {
		t.Errorf("the sent message %q is not in the history %v", first.ID, ids)
	}

	s.checkUnreadCounts(t)
}

// checkUnreadCounts holds the unread snapshot (chat.unreadCounts, protocol
// 1.13) to its shape: {counts: [{friendId, unread}]}. It is optional: a store
// that doesn't implement it answers -32601, which is fine.
func (s *Session) checkUnreadCounts(t *testing.T) {
	t.Helper()
	var snap json.RawMessage
	if err := s.call("chat.unreadCounts", map[string]any{}, &snap); err != nil {
		var re *jsonrpc.Error
		if errors.As(err, &re) && re.Code == jsonrpc.CodeMethodNotFound {
			return
		}
		t.Fatalf("chat.unreadCounts: %v", err)
	}
	checkKeys(t, "chat.unreadCounts", snap, ChatUnreadCountsKeys)
	var res struct {
		Counts []json.RawMessage `json:"counts"`
	}
	if err := json.Unmarshal(snap, &res); err != nil {
		t.Fatalf("chat.unreadCounts: %v", err)
	}
	for _, raw := range res.Counts {
		checkKeys(t, "chat unread count", raw, ChatUnreadCountKeys)
		var c struct {
			FriendID string `json:"friendId"`
			Unread   int    `json:"unread"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatalf("chat unread count: %v", err)
		}
		if c.FriendID == "" || c.Unread < 0 {
			t.Errorf("chat unread count without a friend, or negative: %s", raw)
		}
	}
}

// firstFriend is the id of a friend the store has, from social.state.
func (s *Session) firstFriend(t *testing.T) string {
	t.Helper()
	if err := s.call("social.start", map[string]any{}, nil); err != nil {
		t.Fatalf("chat needs a friend, and social.start failed: %v", err)
	}
	defer func() { _ = s.call("social.stop", map[string]any{}, nil) }()
	var snap struct {
		Friends []struct {
			ID string `json:"id"`
		} `json:"friends"`
	}
	if err := s.call("social.state", map[string]any{}, &snap); err != nil {
		t.Fatalf("social.state: %v", err)
	}
	if len(snap.Friends) == 0 || snap.Friends[0].ID == "" {
		t.Fatal("chat needs a friend, and the store lists none")
	}
	return snap.Friends[0].ID
}
