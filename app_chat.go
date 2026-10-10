package main

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"platypus/internal/chat"
	"platypus/internal/plugins"
	"platypus/internal/social"
)

// chatBackend reaches plugins for the chat orchestrator.
type chatBackend struct{ a *App }

// Offers: the plugin runs, has initialized, and offers chat with social right
// now. Chat needs the friends list, so a plugin offering only chat gets none.
func (b chatBackend) Offers(ctx context.Context, pluginID string) bool {
	if _, err := b.a.plugins.Initialize(ctx, pluginID); err != nil {
		return false
	}
	return b.a.plugins.Has(pluginID, "chat") && b.a.plugins.Has(pluginID, "social")
}

func (b chatBackend) Call(ctx context.Context, pluginID, method string, params, result any) error {
	return b.a.plugins.Call(ctx, pluginID, method, params, result)
}

// startChat creates the conversation manager. Its events reach the UI on the
// "chat" event; a new message alerts only when the user keeps alerts on for
// the store.
func (a *App) startChat(ctx context.Context) {
	a.chat = chat.NewManager(chat.Options{
		Backend: chatBackend{a},
		Alert:   a.settings.ChatAlerts,
		Emit:    func(e chat.Event) { a.host.Emit("chat", e) },
		Logf:    func(f string, args ...any) { a.host.Infof("chat: "+f, args...) },
	})
}

// chatPlugins are the ready plugins that offer chat.
func (a *App) chatPlugins() []string {
	var ids []string
	if a.plugins == nil {
		return ids
	}
	for _, p := range a.plugins.Plugins() {
		if p.Status == plugins.StatusReady && a.plugins.Has(p.Manifest.ID, "chat") {
			ids = append(ids, p.Manifest.ID)
		}
	}
	return ids
}

// errNoChat is what the chat calls answer before the manager exists.
var errNoChat = errors.New("chat is not running")

// ChatOpen shows a friend's conversation: its history loads the first time,
// and its unread messages are marked read.
func (a *App) ChatOpen(pluginID, friendID string) (chat.View, error) {
	if a.chat == nil {
		return chat.View{}, errNoChat
	}
	return a.chat.Open(a.ctx, pluginID, friendID)
}

// ChatClose says the conversation is no longer on screen: new messages count
// as unread again.
func (a *App) ChatClose(pluginID, friendID string) {
	if a.chat != nil {
		a.chat.Close(pluginID, friendID)
	}
}

// ChatOlder loads the page of messages before the ones the conversation holds.
func (a *App) ChatOlder(pluginID, friendID string) (chat.View, error) {
	if a.chat == nil {
		return chat.View{}, errNoChat
	}
	return a.chat.Older(a.ctx, pluginID, friendID)
}

// ChatSend sends a message to a friend. The outcome shows on the message: a
// refused send is "failed", and ChatResend sends it again.
func (a *App) ChatSend(pluginID, friendID, text string) error {
	if a.chat == nil {
		return errNoChat
	}
	return a.chat.Send(a.ctx, pluginID, friendID, text)
}

// ChatResend sends again a message that failed, under the same clientId.
func (a *App) ChatResend(pluginID, friendID, clientID string) error {
	if a.chat == nil {
		return errNoChat
	}
	return a.chat.Resend(a.ctx, pluginID, friendID, clientID)
}

// ChatUnread lists the conversations with unread messages, on every store
// that offers chat, without their messages.
func (a *App) ChatUnread() []chat.View {
	out := []chat.View{}
	if a.chat == nil {
		return out
	}
	for _, id := range a.chatPlugins() {
		out = append(out, a.chat.Unread(id)...)
	}
	return out
}

// ChatAlerts says, per store that offers chat, whether the user hears about new
// messages from friends.
func (a *App) ChatAlerts() map[string]bool {
	out := map[string]bool{}
	for _, id := range a.chatPlugins() {
		out[id] = a.settings.ChatAlerts(id)
	}
	return out
}

// SetChatAlerts turns a store's message alerts on or off.
func (a *App) SetChatAlerts(pluginID string, on bool) error {
	if !a.plugins.Has(pluginID, "chat") {
		return errors.New(pluginID + " has no chat")
	}
	return a.settings.SetChatAlerts(pluginID, on)
}

// chatNotify routes a chat notification from a plugin to the manager. Only a
// store that is running at protocol 1.9 and offers chat with social may send
// them, and only about a friend on its own list. Anything else is dropped here.
func (a *App) chatNotify(id, method string, params json.RawMessage) bool {
	if !strings.HasPrefix(method, "chat.") {
		return false
	}
	if a.chat == nil || !a.chatSpoken(id) {
		return true
	}
	var p struct {
		FriendID string `json:"friendId"`
	}
	if json.Unmarshal(params, &p) != nil || !a.chatFriend(id, p.FriendID) {
		return true
	}
	a.chat.Notify(id, method, params)
	return true
}

// chatSpoken says whether a store is running, initialized, at protocol 1.9 or
// later, and offers both chat and social: the same as chatBackend.Offers, but
// without starting the plugin.
func (a *App) chatSpoken(id string) bool {
	if a.plugins == nil {
		return false
	}
	rt := a.plugins.Runtime(id)
	return rt.Initialized && rt.Speaks(plugins.Version{Major: 1, Minor: 9}) &&
		slices.Contains(rt.Capabilities, "chat") && slices.Contains(rt.Capabilities, "social")
}

// chatFriend says whether a friend is on a store's list.
func (a *App) chatFriend(id, friendID string) bool {
	if a.social == nil || friendID == "" {
		return false
	}
	return slices.ContainsFunc(a.social.View(id).Friends, func(f social.Friend) bool { return f.ID == friendID })
}
