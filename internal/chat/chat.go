// Package chat keeps the one-to-one conversations the user has with friends on
// the store plugins that offer the `chat` capability (protocol 1.9,
// docs/plugins/protocol.md, "Chat").
//
// The store keeps the history. Core keeps, in memory only, the conversations the
// user opened or received a message in: a bounded number of messages each and a
// bounded number of conversations per plugin. Nothing is written to disk, and
// message text never reaches the logs. Every change of a conversation goes to
// Emit as a whole View, stamped with a Seq that rises with each change, so a UI
// keeps the newest one when they arrive out of order.
package chat

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"hydra-launcher/internal/jsonrpc"
)

// Methods core calls. The notifications the plugin sends are the constants below.
const (
	MethodHistory  = "chat.history"
	MethodSend     = "chat.send"
	MethodMarkRead = "chat.markRead"

	NoteMessage = "chat.message"
	NoteTyping  = "chat.typing"
	NoteUnread  = "chat.unread"
)

// Limits. Text is counted in characters; ids are cut to MaxIDLen.
const (
	MaxText   = 4096
	MaxIDLen  = 64
	typingFor = 5 * time.Second
)

// Message is one line of a conversation. A message the user sent that the
// store hasn't confirmed yet has no id: it is "sending" until the store answers,
// or "failed" when it didn't go through (the user can send it again).
type Message struct {
	ID       string `json:"id"`
	From     string `json:"from"`
	Text     string `json:"text"`
	SentAt   int64  `json:"sentAt"`
	Status   string `json:"status,omitempty"`
	ClientID string `json:"clientId,omitempty"`
}

// View is one conversation as the UI shows it.
type View struct {
	Seq      uint64    `json:"seq"`
	Plugin   string    `json:"plugin"`
	FriendID string    `json:"friendId"`
	Open     bool      `json:"open"`
	Loaded   bool      `json:"loaded"`
	More     bool      `json:"more"`
	Unread   int       `json:"unread"`
	Typing   bool      `json:"typing"`
	Messages []Message `json:"messages"`
}

// Event is a View plus whether the change is a new message the user should be
// told about: it came from a friend while the conversation wasn't open, and the
// store's message notifications are on.
type Event struct {
	View
	Alert bool `json:"alert"`
}

// Backend reaches the plugins. The app wires it to the plugin manager.
type Backend interface {
	// Offers starts the plugin if needed and reports whether it offers chat
	// right now (chat with social, at protocol 1.9 or later).
	Offers(ctx context.Context, pluginID string) bool
	Call(ctx context.Context, pluginID, method string, params, result any) error
}

// Options configure a Manager. Zero values pick the defaults in the comments.
type Options struct {
	Backend Backend
	// Emit receives every change of a conversation.
	Emit func(Event)
	// Alert says whether the user wants to hear about new messages from a
	// store. Nil means yes.
	Alert func(pluginID string) bool
	// Logf receives what went wrong. It never gets message text.
	Logf func(format string, args ...any)
	// Page is how many messages one history call asks for. Default 50.
	Page int
	// MaxMessages bounds the messages kept per conversation. Default 200.
	MaxMessages int
	// MaxConversations bounds the conversations kept per plugin. Default 256.
	MaxConversations int
	// Timeout bounds one call to the plugin. Default 15 s.
	Timeout time.Duration
	// RetryDelay is the wait before the one retry of a failed send. Default 1 s.
	RetryDelay time.Duration
}

type key struct{ plugin, friend string }

// conv is one conversation. Its fields are guarded by mu. Lock order: the
// manager's mu may be held while taking a conversation's mu, never the reverse.
type conv struct {
	plugin, friend string

	mu          sync.Mutex
	seq         uint64
	open        bool
	loaded      bool
	more        bool
	unread      int
	typing      bool
	typingTimer *time.Timer
	msgs        []Message
	sending     int    // sends in flight: the conversation is kept until they end
	used        uint64 // last use, for eviction
}

// Manager holds the conversations of every plugin.
type Manager struct {
	opt Options

	mu    sync.Mutex
	convs map[key]*conv
	clock uint64
}

// NewManager creates a manager with nothing in it.
func NewManager(o Options) *Manager {
	if o.Page <= 0 {
		o.Page = 50
	}
	if o.MaxMessages <= 0 {
		o.MaxMessages = 200
	}
	if o.MaxConversations <= 0 {
		o.MaxConversations = 256
	}
	if o.Timeout <= 0 {
		o.Timeout = 15 * time.Second
	}
	if o.RetryDelay <= 0 {
		o.RetryDelay = time.Second
	}
	return &Manager{opt: o, convs: map[key]*conv{}}
}

// ValidText reports why text can't be sent: empty after trimming, not UTF-8, or
// longer than MaxText characters.
func ValidText(text string) error {
	switch {
	case strings.TrimSpace(text) == "":
		return errors.New("the message is empty")
	case !utf8.ValidString(text):
		return errors.New("the message is not valid text")
	case utf8.RuneCountInString(text) > MaxText:
		return fmt.Errorf("the message is longer than %d characters", MaxText)
	}
	return nil
}

// Open shows a conversation: it is marked open, its unread count is cleared
// (and the store told, with the newest message), and its history is loaded the
// first time. It returns the conversation as it is now.
func (m *Manager) Open(ctx context.Context, pluginID, friendID string) (View, error) {
	if err := m.offers(ctx, pluginID, friendID); err != nil {
		return View{}, err
	}
	c := m.conv(pluginID, friendID, true)
	c.mu.Lock()
	c.open = true
	c.unread = 0
	loaded := c.loaded
	upTo := newestID(c.msgs)
	c.mu.Unlock()
	m.emit(c, false)
	if upTo != "" {
		m.markRead(ctx, c, upTo)
	}
	if !loaded {
		if err := m.history(ctx, c, ""); err != nil {
			return c.snapshot(), err
		}
	}
	return c.snapshot(), nil
}

// Close marks a conversation not open: messages from now on count as unread.
func (m *Manager) Close(pluginID, friendID string) {
	m.mu.Lock()
	c, ok := m.convs[key{pluginID, friendID}]
	m.mu.Unlock()
	if !ok {
		return
	}
	c.mu.Lock()
	c.open = false
	c.mu.Unlock()
	m.emit(c, false)
}

// Older loads the page of messages before the oldest one the conversation
// holds. It does nothing when the store has no more.
func (m *Manager) Older(ctx context.Context, pluginID, friendID string) (View, error) {
	if err := m.offers(ctx, pluginID, friendID); err != nil {
		return View{}, err
	}
	c := m.conv(pluginID, friendID, true)
	c.mu.Lock()
	more := c.more
	before := oldestID(c.msgs)
	c.mu.Unlock()
	if !more || before == "" {
		return c.snapshot(), nil
	}
	err := m.history(ctx, c, before)
	return c.snapshot(), err
}

// Send queues a message to a friend and returns once the store answered (or
// the one retry failed). The message is on screen at once, "sending", and
// becomes "failed" when the store refuses it. Its clientId is the same on the
// retry, so the store never shows it twice.
func (m *Manager) Send(ctx context.Context, pluginID, friendID, text string) error {
	if err := ValidText(text); err != nil {
		return err
	}
	if err := m.offers(ctx, pluginID, friendID); err != nil {
		return err
	}
	clientID, err := newClientID()
	if err != nil {
		return err
	}
	c := m.conv(pluginID, friendID, true)
	c.mu.Lock()
	c.msgs = append(c.msgs, Message{From: "self", Text: text, SentAt: nowMillis(), Status: "sending", ClientID: clientID})
	c.sending++
	c.mu.Unlock()
	m.emit(c, false)
	return m.deliver(ctx, c, clientID, text)
}

// Resend sends again a message that failed, under its own clientId.
func (m *Manager) Resend(ctx context.Context, pluginID, friendID, clientID string) error {
	if err := m.offers(ctx, pluginID, friendID); err != nil {
		return err
	}
	c := m.conv(pluginID, friendID, false)
	if c == nil {
		return errors.New("no such conversation")
	}
	c.mu.Lock()
	i := indexClient(c.msgs, clientID)
	if i < 0 || c.msgs[i].Status != "failed" {
		c.mu.Unlock()
		return errors.New("only a message that failed can be sent again")
	}
	c.msgs[i].Status = "sending"
	text := c.msgs[i].Text
	c.sending++
	c.mu.Unlock()
	m.emit(c, false)
	return m.deliver(ctx, c, clientID, text)
}

// Unread lists the conversations of a plugin with unread messages, without
// their messages.
func (m *Manager) Unread(pluginID string) []View {
	out := []View{}
	for _, c := range m.plugin(pluginID) {
		v := c.snapshot()
		if v.Unread > 0 {
			v.Messages = []Message{}
			out = append(out, v)
		}
	}
	return out
}

// Snapshot returns a conversation as it is now, or an empty one when the
// manager holds none for the friend.
func (m *Manager) Snapshot(pluginID, friendID string) View {
	c := m.conv(pluginID, friendID, false)
	if c == nil {
		return View{Plugin: pluginID, FriendID: friendID, Messages: []Message{}}
	}
	return c.snapshot()
}

// Forget drops every conversation of a plugin from memory (the plugin's chat
// went away, or the launcher forgets the store).
func (m *Manager) Forget(pluginID string) {
	m.mu.Lock()
	var dropped []*conv
	for k, c := range m.convs {
		if k.plugin == pluginID {
			dropped = append(dropped, c)
			delete(m.convs, k)
		}
	}
	m.mu.Unlock()
	for _, c := range dropped {
		c.mu.Lock()
		if c.typingTimer != nil {
			c.typingTimer.Stop()
		}
		c.mu.Unlock()
	}
}

// Notify applies a plugin notification. Methods other than the three chat
// notifications are ignored, and so is a malformed one. It must not block: it
// runs on the plugin's reader, so anything that calls the plugin goes in a
// goroutine.
func (m *Manager) Notify(pluginID, method string, params json.RawMessage) {
	switch method {
	case NoteMessage:
		var p struct {
			FriendID string  `json:"friendId"`
			Message  Message `json:"message"`
		}
		if json.Unmarshal(params, &p) == nil {
			m.incoming(pluginID, p.FriendID, p.Message)
		}
	case NoteUnread:
		var p struct {
			FriendID string `json:"friendId"`
			Count    int    `json:"count"`
		}
		if json.Unmarshal(params, &p) == nil {
			m.setUnread(pluginID, p.FriendID, p.Count)
		}
	case NoteTyping:
		var p struct {
			FriendID string `json:"friendId"`
		}
		if json.Unmarshal(params, &p) == nil {
			m.typing(pluginID, p.FriendID)
		}
	}
}

// offers checks the ids and that the plugin offers chat right now.
func (m *Manager) offers(ctx context.Context, pluginID, friendID string) error {
	if pluginID == "" || friendID == "" || len(friendID) > MaxIDLen {
		return errors.New("a friend id is required")
	}
	if m.opt.Backend == nil || !m.opt.Backend.Offers(ctx, pluginID) {
		return fmt.Errorf("chat is off for %s", pluginID)
	}
	return nil
}

// history loads a page of the conversation, older than before ("" for the
// newest page), and merges it in.
func (m *Manager) history(ctx context.Context, c *conv, before string) error {
	params := map[string]any{"friendId": c.friend, "limit": m.opt.Page}
	if before != "" {
		params["before"] = before
	}
	var res struct {
		Messages []Message `json:"messages"`
		More     bool      `json:"more"`
	}
	if err := m.call(ctx, c.plugin, MethodHistory, params, &res); err != nil {
		m.logf("chat %s: history: %s", c.plugin, describe(err))
		return err
	}
	in := make([]Message, 0, len(res.Messages))
	for _, msg := range res.Messages {
		if msg, ok := cleanMessage(msg, c.friend, nowMillis()); ok {
			in = append(in, msg)
		}
	}
	c.mu.Lock()
	c.msgs = mergeMessages(c.msgs, in)
	c.more = res.More
	c.loaded = true
	m.bound(c)
	c.mu.Unlock()
	m.emit(c, false)
	return nil
}

// deliver sends one message, retrying once after a transport error, and
// settles its entry: the store's id replaces the clientId, or it is marked
// failed.
func (m *Manager) deliver(ctx context.Context, c *conv, clientID, text string) error {
	var res struct {
		ID     string `json:"id"`
		SentAt int64  `json:"sentAt"`
	}
	params := map[string]any{"friendId": c.friend, "text": text, "clientId": clientID}
	var err error
	for attempt := 1; ; attempt++ {
		res.ID, res.SentAt = "", 0
		err = m.call(ctx, c.plugin, MethodSend, params, &res)
		if err == nil && res.ID == "" {
			err = errors.New("the store answered without a message id")
		}
		if err == nil || attempt == 2 || !transient(err) || ctx.Err() != nil {
			break
		}
		if !sleep(ctx, m.opt.RetryDelay) {
			break
		}
	}
	c.mu.Lock()
	c.sending--
	i := indexClient(c.msgs, clientID)
	switch {
	case i < 0:
		// The entry was dropped (the conversation was forgotten): nothing to settle.
	case err != nil:
		c.msgs[i].Status = "failed"
	case indexID(c.msgs, res.ID) >= 0:
		// The store's notification for this message came first: the entry is it.
		c.msgs = slicesDelete(c.msgs, i)
	default:
		c.msgs[i].ID = res.ID
		c.msgs[i].Status = ""
		c.msgs[i].ClientID = ""
		if res.SentAt > 0 {
			c.msgs[i].SentAt = res.SentAt
		}
		slices.SortStableFunc(c.msgs, func(a, b Message) int { return cmp.Compare(a.SentAt, b.SentAt) })
	}
	c.mu.Unlock()
	m.emit(c, false)
	if err != nil {
		m.logf("chat %s: send: %s", c.plugin, describe(err))
	}
	return err
}

// markRead tells the store the user has read up to upTo. A failure is logged;
// the local count is already clear.
func (m *Manager) markRead(ctx context.Context, c *conv, upTo string) {
	params := map[string]any{"friendId": c.friend, "upTo": upTo}
	if err := m.call(ctx, c.plugin, MethodMarkRead, params, nil); err != nil {
		m.logf("chat %s: markRead: %s", c.plugin, describe(err))
	}
}

// incoming applies a chat.message notification.
func (m *Manager) incoming(pluginID, friendID string, msg Message) {
	if friendID == "" || len(friendID) > MaxIDLen {
		return
	}
	msg, ok := cleanMessage(msg, friendID, nowMillis())
	if !ok {
		return
	}
	c := m.conv(pluginID, friendID, true)
	c.mu.Lock()
	if indexID(c.msgs, msg.ID) >= 0 {
		c.mu.Unlock()
		return
	}
	fromFriend := msg.From != "self"
	c.msgs = mergeMessages(c.msgs, []Message{msg})
	m.bound(c)
	if fromFriend {
		c.typing = false
		if c.typingTimer != nil {
			c.typingTimer.Stop()
		}
		if !c.open {
			c.unread++
		}
	}
	alert := fromFriend && !c.open && (m.opt.Alert == nil || m.opt.Alert(pluginID))
	markUpTo := ""
	if fromFriend && c.open {
		markUpTo = msg.ID
	}
	c.mu.Unlock()
	m.emit(c, alert)
	if markUpTo != "" {
		go m.markReadBackground(c, markUpTo)
	}
}

// markReadBackground tells the store about a message that arrived in an open
// conversation. It runs off the plugin's reader, which it must not block.
func (m *Manager) markReadBackground(c *conv, upTo string) {
	ctx, cancel := context.WithTimeout(context.Background(), m.opt.Timeout)
	defer cancel()
	m.markRead(ctx, c, upTo)
}

// setUnread applies a chat.unread notification: the store's own count.
func (m *Manager) setUnread(pluginID, friendID string, count int) {
	if friendID == "" || len(friendID) > MaxIDLen {
		return
	}
	if count < 0 {
		count = 0
	}
	c := m.conv(pluginID, friendID, count > 0)
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.open {
		count = 0
	}
	c.unread = count
	c.mu.Unlock()
	m.emit(c, false)
}

// typing applies a chat.typing notification: the friend shows as typing for a
// few seconds, unless the next message comes first.
func (m *Manager) typing(pluginID, friendID string) {
	if friendID == "" || len(friendID) > MaxIDLen {
		return
	}
	c := m.conv(pluginID, friendID, true)
	c.mu.Lock()
	c.typing = true
	if c.typingTimer != nil {
		c.typingTimer.Stop()
	}
	c.typingTimer = time.AfterFunc(typingFor, func() {
		c.mu.Lock()
		c.typing = false
		c.mu.Unlock()
		m.emit(c, false)
	})
	c.mu.Unlock()
	m.emit(c, false)
}

// conv returns a plugin's conversation with a friend. With create, it is made
// if missing, evicting the least recently used conversation that is closed and
// has nothing in flight when the plugin is at its bound.
func (m *Manager) conv(pluginID, friendID string, create bool) *conv {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clock++
	k := key{pluginID, friendID}
	if c, ok := m.convs[k]; ok {
		c.mu.Lock()
		c.used = m.clock
		c.mu.Unlock()
		return c
	}
	if !create {
		return nil
	}
	if m.countLocked(pluginID) >= m.opt.MaxConversations {
		m.evictLocked(pluginID)
	}
	c := &conv{plugin: pluginID, friend: friendID, used: m.clock, msgs: []Message{}}
	m.convs[k] = c
	return c
}

func (m *Manager) countLocked(pluginID string) int {
	n := 0
	for k := range m.convs {
		if k.plugin == pluginID {
			n++
		}
	}
	return n
}

// evictLocked drops the least recently used conversation of a plugin that
// nothing holds open or waits on.
func (m *Manager) evictLocked(pluginID string) {
	var victim key
	var oldest uint64
	found := false
	for k, c := range m.convs {
		if k.plugin != pluginID {
			continue
		}
		c.mu.Lock()
		idle := !c.open && c.sending == 0
		used := c.used
		c.mu.Unlock()
		if idle && (!found || used < oldest) {
			victim, oldest, found = k, used, true
		}
	}
	if found {
		delete(m.convs, victim)
	}
}

// plugin lists a plugin's conversations.
func (m *Manager) plugin(pluginID string) []*conv {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*conv
	for k, c := range m.convs {
		if k.plugin == pluginID {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b *conv) int { return strings.Compare(a.friend, b.friend) })
	return out
}

// bound keeps the newest MaxMessages of a conversation; what it drops is still
// at the store, so the conversation has more. Callers hold c.mu.
func (m *Manager) bound(c *conv) {
	if over := len(c.msgs) - m.opt.MaxMessages; over > 0 {
		c.msgs = slices.Clone(c.msgs[over:])
		c.more = true
	}
}

// snapshot is the conversation as it is now, without a new Seq.
func (c *conv) snapshot() View {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.viewLocked()
}

func (c *conv) viewLocked() View {
	return View{
		Seq: c.seq, Plugin: c.plugin, FriendID: c.friend,
		Open: c.open, Loaded: c.loaded, More: c.more, Unread: c.unread, Typing: c.typing,
		Messages: slices.Clone(c.msgs),
	}
}

// emit stamps the conversation with the next Seq and hands it to Emit. The
// stamp is taken under c.mu, so a later change always carries a higher Seq.
func (m *Manager) emit(c *conv, alert bool) {
	c.mu.Lock()
	c.seq++
	ev := Event{View: c.viewLocked(), Alert: alert}
	c.mu.Unlock()
	if m.opt.Emit != nil {
		m.opt.Emit(ev)
	}
}

// call runs one request to the plugin, bounded by Timeout.
func (m *Manager) call(ctx context.Context, pluginID, method string, params, result any) error {
	ctx, cancel := context.WithTimeout(ctx, m.opt.Timeout)
	defer cancel()
	return m.opt.Backend.Call(ctx, pluginID, method, params, result)
}

// logf logs what went wrong. Callers pass ids and the kind of failure only.
func (m *Manager) logf(format string, args ...any) {
	if m.opt.Logf != nil {
		m.opt.Logf(format, args...)
	}
}

// cleanMessage applies the limits to an incoming message and reports false
// when it has no id. "from" is "self" for the user's own messages and the
// friend's id for the rest.
func cleanMessage(msg Message, friendID string, now int64) (Message, bool) {
	msg.ID = clip(msg.ID, MaxIDLen)
	if msg.ID == "" {
		return msg, false
	}
	if msg.From != "self" {
		msg.From = friendID
	}
	msg.Text = clip(msg.Text, MaxText)
	if msg.SentAt <= 0 {
		msg.SentAt = now
	}
	msg.Status, msg.ClientID = "", ""
	return msg, true
}

// mergeMessages merges add into list: an id already there is skipped, the rest
// go in by sentAt (equal times keep their order). Entries without an id (the
// user's messages in flight) stay where they are.
func mergeMessages(list, add []Message) []Message {
	out := slices.Clone(list)
	if out == nil {
		out = []Message{}
	}
	for _, msg := range add {
		if msg.ID != "" && indexID(out, msg.ID) >= 0 {
			continue
		}
		i := len(out)
		for j, x := range out {
			if x.SentAt > msg.SentAt {
				i = j
				break
			}
		}
		out = slices.Insert(out, i, msg)
	}
	return out
}

func indexID(list []Message, id string) int {
	if id == "" {
		return -1
	}
	return slices.IndexFunc(list, func(x Message) bool { return x.ID == id })
}

func indexClient(list []Message, clientID string) int {
	if clientID == "" {
		return -1
	}
	return slices.IndexFunc(list, func(x Message) bool { return x.ClientID == clientID })
}

// newestID is the id of the newest message the store has confirmed.
func newestID(list []Message) string {
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].ID != "" {
			return list[i].ID
		}
	}
	return ""
}

// oldestID is the id of the oldest message the store has confirmed.
func oldestID(list []Message) string {
	for _, msg := range list {
		if msg.ID != "" {
			return msg.ID
		}
	}
	return ""
}

func slicesDelete(list []Message, i int) []Message {
	return slices.Delete(slices.Clone(list), i, i+1)
}

// transient reports whether a failed call may succeed on a retry: the plugin
// didn't answer (it went away, or the call timed out), as opposed to answering
// with an error.
func transient(err error) bool {
	var rpc *jsonrpc.Error
	return !errors.As(err, &rpc)
}

// describe names the kind of a failure for the log, never what it said.
func describe(err error) string {
	var rpc *jsonrpc.Error
	if errors.As(err, &rpc) {
		return fmt.Sprintf("the store answered error %d", rpc.Code)
	}
	return "the plugin didn't answer"
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func newClientID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func nowMillis() int64 { return time.Now().UnixMilli() }

// sleep waits d, or returns false when ctx ends first.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
