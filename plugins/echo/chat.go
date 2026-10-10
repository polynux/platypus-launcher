package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
)

// Chat (protocol 1.9, docs/plugins/protocol.md "Chat"): fake conversations with
// three friends of the social fixture (social.go). Ana has a short history,
// Gus a long one that pages, and Ivy Chen is the echo bot: she answers each
// message with its text reversed, after a typing notice and a delay.
//
// Knobs, all set with `config`:
//
//	chat: { tick: 4000,       a new message from a friend every tick ms (0 = none)
//	        botDelayMs: 800,  how long the echo bot takes to answer
//	        sendFail: 2,      the next 2 chat.send calls answer -32000 (the store refuses)
//	        sendCrash: 1 }    the next chat.send exits the process: a transport error, retried once
//
// ECHO_CHAT_TICK sets tick at start. `recorded` reports the chat.send calls
// Platypus made under "chat" (text and clientId, in order) and the markRead calls.

const (
	chatBotID     = "7656119800000009" // Ivy Chen: the echo bot
	chatMaxText   = 4096
	chatPageLimit = 50
	chatMaxPage   = 200
)

// chatFriends are the friends a conversation exists with, by id.
var chatFriends = map[string]bool{
	"7656119800000001": true, "7656119800000002": true, "7656119800000007": true, chatBotID: true,
}

// chatBFriends are echo-b's own friends with chat: the store only sends and
// answers for them, since its friends list is its own (socialFixtureB).
var chatBFriends = []string{"8656119800000001", "8656119800000002", "8656119800000003"}

// chatFixture is the history echo starts with.
var chatFixture = map[string][]chatMsg{
	"7656119800000001": {
		{ID: "a1", From: "7656119800000001", Text: "hey, are you on the new build?", SentAt: 1791300000000},
		{ID: "a2", From: "self", Text: "not yet, downloading now", SentAt: 1791300060000},
		{ID: "a3", From: "7656119800000001", Text: "tell me how it runs", SentAt: 1791300120000},
	},
	"7656119800000007": chatLongFixture(60),
	chatBotID: {
		{ID: "b1", From: chatBotID, Text: "hi! I reverse what you write.", SentAt: 1791200000000},
	},
}

// chatLongFixture is a history of n messages, alternating, for paging.
func chatLongFixture(n int) []chatMsg {
	out := make([]chatMsg, 0, n)
	for i := 1; i <= n; i++ {
		from := "7656119800000007"
		if i%2 == 0 {
			from = "self"
		}
		out = append(out, chatMsg{ID: fmt.Sprintf("g%03d", i), From: from, Text: fmt.Sprintf("message %d", i), SentAt: 1791000000000 + int64(i)*1000})
	}
	return out
}

type chatMsg struct {
	ID     string `json:"id"`
	From   string `json:"from"`
	Text   string `json:"text"`
	SentAt int64  `json:"sentAt"`
}

var (
	chatMu      sync.Mutex
	chatConvs   = map[string][]chatMsg{}
	chatSentIDs = map[string]chatMsg{} // by clientId
	chatUnread  = map[string]int{}
	chatNext    = 1000
	chatTick    time.Duration
	chatStop    chan struct{}
	chatBotWait = 800 * time.Millisecond
	chatFailN   int
	chatCrashN  int
	chatRecord  struct {
		Sent  []map[string]string `json:"sent"`
		Marks []map[string]string `json:"marks"`
	}
	chatReady bool
)

// loadChat sets up the fixture and reads ECHO_CHAT_TICK (ms).
func loadChat() {
	if ms, err := strconv.Atoi(os.Getenv("ECHO_CHAT_TICK")); err == nil && ms > 0 {
		chatTick = time.Duration(ms) * time.Millisecond
	}
	chatMu.Lock()
	defer chatMu.Unlock()
	chatConvs = map[string][]chatMsg{}
	for id, msgs := range chatFixture {
		chatConvs[id] = append([]chatMsg(nil), msgs...)
	}
	chatReady = true
	restartChatTicker()
}

// configureChat applies the chat part of a `config` request.
func configureChat(raw json.RawMessage) {
	var present struct {
		Chat *struct {
			Tick       *int `json:"tick"`
			BotDelayMs *int `json:"botDelayMs"`
			SendFail   *int `json:"sendFail"`
			SendCrash  *int `json:"sendCrash"`
		} `json:"chat"`
	}
	if json.Unmarshal(raw, &present) != nil || present.Chat == nil {
		return
	}
	chatMu.Lock()
	defer chatMu.Unlock()
	if t := present.Chat.Tick; t != nil {
		chatTick = time.Duration(*t) * time.Millisecond
		restartChatTicker()
	}
	if d := present.Chat.BotDelayMs; d != nil {
		chatBotWait = time.Duration(*d) * time.Millisecond
	}
	if n := present.Chat.SendFail; n != nil {
		chatFailN = *n
	}
	if n := present.Chat.SendCrash; n != nil {
		chatCrashN = *n
	}
}

// chatHistory answers chat.history: the messages before `before` (or the newest
// page), oldest first, and whether older ones exist.
func chatHistory(raw json.RawMessage) (any, error) {
	var p struct {
		FriendID string `json:"friendId"`
		Before   string `json:"before"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if !chatFriends[p.FriendID] {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: no friend %q", p.FriendID)}
	}
	limit := p.Limit
	if limit <= 0 {
		limit = chatPageLimit
	}
	limit = min(limit, chatMaxPage)
	chatMu.Lock()
	defer chatMu.Unlock()
	msgs := chatConvs[p.FriendID]
	end := len(msgs)
	if p.Before != "" {
		end = 0
		for i, m := range msgs {
			if m.ID == p.Before {
				end = i
				break
			}
		}
	}
	start := max(0, end-limit)
	return map[string]any{"messages": msgs[start:end], "more": start > 0}, nil
}

// chatSend answers chat.send. A clientId seen before answers with the first
// send's id and time. A send that fails by knob answers -32000 (not retried),
// a crash knob exits the process (retried once by core).
func chatSend(raw json.RawMessage) (any, error) {
	var p struct {
		FriendID string `json:"friendId"`
		Text     string `json:"text"`
		ClientID string `json:"clientId"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	switch {
	case !chatFriends[p.FriendID]:
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: no friend %q", p.FriendID)}
	case p.ClientID == "":
		return nil, &rpcError{Code: -32602, Message: "invalid params: clientId is required"}
	case p.Text == "" || utf8.RuneCountInString(p.Text) > chatMaxText:
		return nil, &rpcError{Code: -32602, Message: "invalid params: text must be 1 to 4096 characters"}
	}
	chatMu.Lock()
	if first, ok := chatSentIDs[p.ClientID]; ok {
		chatMu.Unlock()
		return map[string]any{"id": first.ID, "sentAt": first.SentAt}, nil
	}
	if chatCrashN > 0 {
		chatCrashN--
		chatMu.Unlock()
		os.Exit(3)
	}
	if chatFailN > 0 {
		chatFailN--
		chatMu.Unlock()
		return nil, &rpcError{Code: -32000, Message: "the store refused the message"}
	}
	chatNext++
	msg := chatMsg{ID: fmt.Sprintf("s%d", chatNext), From: "self", Text: p.Text, SentAt: time.Now().UnixMilli()}
	chatConvs[p.FriendID] = append(chatConvs[p.FriendID], msg)
	chatSentIDs[p.ClientID] = msg
	chatRecord.Sent = append(chatRecord.Sent, map[string]string{"friendId": p.FriendID, "text": p.Text, "clientId": p.ClientID})
	chatMu.Unlock()
	if p.FriendID == chatBotID {
		go chatBotReply(p.FriendID, p.Text)
	}
	return map[string]any{"id": msg.ID, "sentAt": msg.SentAt}, nil
}

// chatMarkRead answers chat.markRead: the friend's unread count clears.
func chatMarkRead(raw json.RawMessage) (any, error) {
	var p struct {
		FriendID string `json:"friendId"`
		UpTo     string `json:"upTo"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if !chatFriends[p.FriendID] {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: no friend %q", p.FriendID)}
	}
	chatMu.Lock()
	defer chatMu.Unlock()
	chatUnread[p.FriendID] = 0
	chatRecord.Marks = append(chatRecord.Marks, map[string]string{"friendId": p.FriendID, "upTo": p.UpTo})
	return map[string]any{}, nil
}

// chatRecorded is what `recorded` reports for chat: the sends and marks, in order.
func chatRecorded() any {
	chatMu.Lock()
	defer chatMu.Unlock()
	return map[string]any{"sent": chatRecord.Sent, "marks": chatRecord.Marks}
}

// chatBotReply answers a message to the echo bot: a typing notice, then the
// text reversed after the bot delay.
func chatBotReply(friendID, text string) {
	chatMu.Lock()
	wait := chatBotWait
	chatMu.Unlock()
	notify("chat.typing", map[string]string{"friendId": friendID})
	time.Sleep(wait)
	reply := reverse(text)
	chatMu.Lock()
	chatNext++
	msg := chatMsg{ID: fmt.Sprintf("s%d", chatNext), From: friendID, Text: reply, SentAt: time.Now().UnixMilli()}
	chatConvs[friendID] = append(chatConvs[friendID], msg)
	chatUnread[friendID]++
	unread := chatUnread[friendID]
	chatMu.Unlock()
	notify("chat.message", map[string]any{"friendId": friendID, "message": msg})
	notify("chat.unread", map[string]any{"friendId": friendID, "count": unread})
}

// restartChatTicker starts the periodic incoming messages at chatTick. Callers hold chatMu.
func restartChatTicker() {
	if chatStop != nil {
		close(chatStop)
		chatStop = nil
	}
	if chatTick <= 0 {
		return
	}
	stop := make(chan struct{})
	chatStop = stop
	go func(stop chan struct{}, every time.Duration) {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-tick.C:
				chatIncoming(i)
			}
		}
	}(stop, chatTick)
}

// chatIncoming delivers one message from a rotating friend (not the bot), with
// the store's unread count for that friend.
func chatIncoming(i int) {
	friends := []string{"7656119800000001", "7656119800000002", "7656119800000007"}
	if manifestID() == "echo-b" {
		friends = chatBFriends
	}
	friendID := friends[i%len(friends)]
	chatMu.Lock()
	if !chatReady {
		chatMu.Unlock()
		return
	}
	chatNext++
	msg := chatMsg{ID: fmt.Sprintf("i%d", chatNext), From: friendID, Text: fmt.Sprintf("ping %d", chatNext), SentAt: time.Now().UnixMilli()}
	chatConvs[friendID] = append(chatConvs[friendID], msg)
	chatUnread[friendID]++
	unread := chatUnread[friendID]
	chatMu.Unlock()
	notify("chat.message", map[string]any{"friendId": friendID, "message": msg})
	notify("chat.unread", map[string]any{"friendId": friendID, "count": unread})
}

// reverse reverses text by runes.
func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
