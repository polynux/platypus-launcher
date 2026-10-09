// Package social keeps the friends lists of the store plugins that offer the
// `social` capability (protocol 1.6, docs/plugins/protocol.md, "Social").
//
// Each plugin gets one subscription: started when the launcher starts (or when
// the user switches the plugin's friends on), stopped when it is switched off
// or the launcher quits, and restarted with backoff when the plugin process
// goes away. The subscription caches the plugin's snapshot, merges its pushes
// and hands the whole view of the plugin to the UI after every change. Core
// also reports what the user plays (social.setPlaying) for games of the
// plugin; it never talks to a store itself.
package social

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"hydra-launcher/internal/jsonrpc"
)

// Methods core calls. Notifications the plugin sends are the constants below.
const (
	MethodStart      = "social.start"
	MethodStop       = "social.stop"
	MethodState      = "social.state"
	MethodSetStatus  = "social.setStatus"
	MethodSetPlaying = "social.setPlaying"

	NoteConnection = "social.connection"
	NoteSelf       = "social.self"
	NoteFriends    = "social.friends"
)

// Statuses are the states the user can set for themselves (social.setStatus).
var Statuses = []string{"online", "away", "busy", "snooze", "invisible"}

// Connection states a plugin reports (social.connection).
const (
	ConnConnecting   = "connecting"
	ConnOnline       = "online"
	ConnReconnecting = "reconnecting"
	ConnReplaced     = "replaced"
	ConnOffline      = "offline"
	ConnLoggedOut    = "loggedOut"
)

// Connection is the plugin's session with its store.
type Connection struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

// Self is the user's own account on the store.
type Self struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	State     string `json:"state"`
}

// Game is what a friend plays. ID is Hydra's key (p.<plugin>.<id>) when the
// game is in the library, and empty otherwise.
type Game struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
}

// Friend is one entry of a friends list.
type Friend struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	State     string `json:"state"`
	Game      *Game  `json:"game,omitempty"`
	LastSeen  int64  `json:"lastSeen,omitempty"`
}

// View is what the UI shows for one plugin. Every change emits a whole View,
// so the UI replaces its copy and never merges.
type View struct {
	Plugin     string     `json:"plugin"`
	Enabled    bool       `json:"enabled"`
	Running    bool       `json:"running"`
	Connection Connection `json:"connection"`
	Self       *Self      `json:"self,omitempty"`
	Friends    []Friend   `json:"friends"`
}

// Backend reaches the plugins. The app wires it to the plugin manager.
type Backend interface {
	// Offers starts the plugin if needed and reports whether it offers social
	// right now.
	Offers(ctx context.Context, pluginID string) bool
	Call(ctx context.Context, pluginID, method string, params, result any) error
}

// Options configure a Manager.
type Options struct {
	Backend Backend
	// Enabled says whether the user keeps a plugin's friends list on.
	Enabled func(pluginID string) bool
	// Emit receives every change of a plugin's View.
	Emit func(View)
	Logf func(format string, args ...any)
	// MinBackoff and MaxBackoff bound the wait before a restart. Zero means
	// 2 s and 5 min.
	MinBackoff, MaxBackoff time.Duration
	// StableAfter is how long a session must last to count as healthy: only
	// then does the next restart wait MinBackoff again. Zero means 60 s.
	StableAfter time.Duration
}

// stopTimeout bounds social.stop when the subscription ends.
const stopTimeout = 3 * time.Second

// Manager runs one subscription per plugin.
type Manager struct {
	opt Options

	mu   sync.Mutex
	subs map[string]*sub
}

// sub is one plugin's subscription. Its fields are guarded by mu, except the
// channels, which are created once.
type sub struct {
	id     string
	cancel context.CancelFunc
	done   chan struct{}
	exited chan struct{} // the plugin process went away
	kick   chan struct{} // the playing game changed

	mu        sync.Mutex
	view      View
	paused    bool   // replaced or logged out: no restart until the plugin connects again
	started   bool   // social.start answered for this process, so it must be stopped
	connected bool   // social.state answered too: the friends are live
	playing   string // store game id the user plays; "" for none
	sent      string // what the current process was last told about playing
}

// NewManager creates a manager; nothing runs until Start.
func NewManager(o Options) *Manager {
	return &Manager{opt: o, subs: map[string]*sub{}}
}

// Start subscribes to a plugin's friends. It does nothing when the plugin is
// already subscribed.
func (m *Manager) Start(pluginID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.subs[pluginID]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &sub{
		id:     pluginID,
		cancel: cancel,
		done:   make(chan struct{}),
		exited: make(chan struct{}, 1),
		kick:   make(chan struct{}, 1),
		view:   View{Plugin: pluginID, Running: true, Friends: []Friend{}, Connection: Connection{State: ConnConnecting}},
	}
	m.subs[pluginID] = s
	m.emit(s)
	go m.run(ctx, s)
}

// Stop ends a plugin's subscription, asks the plugin to stop, and waits for
// that. The view stays, with Running false, so the UI can say so.
func (m *Manager) Stop(pluginID string) {
	m.mu.Lock()
	s, ok := m.subs[pluginID]
	if ok {
		delete(m.subs, pluginID)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	s.cancel()
	<-s.done
	s.mu.Lock()
	s.view.Running = false
	s.view.Connection = Connection{State: ConnOffline}
	s.view.Self = nil
	s.view.Friends = []Friend{}
	s.mu.Unlock()
	m.emit(s)
}

// StopAll ends every subscription (launcher shutdown).
func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.subs))
	for id := range m.subs {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Stop(id)
	}
}

// Running reports whether a plugin has a subscription.
func (m *Manager) Running(pluginID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.subs[pluginID]
	return ok
}

// View returns a plugin's current view. A plugin without a subscription has
// an empty view with Running false.
func (m *Manager) View(pluginID string) View {
	m.mu.Lock()
	s, ok := m.subs[pluginID]
	m.mu.Unlock()
	if !ok {
		return m.idle(pluginID)
	}
	return m.snapshot(s)
}

func (m *Manager) idle(pluginID string) View {
	return View{Plugin: pluginID, Enabled: m.enabled(pluginID), Friends: []Friend{}, Connection: Connection{State: ConnOffline}}
}

// Exited tells the subscription that the plugin process went away; the
// subscription restarts it after a backoff.
func (m *Manager) Exited(pluginID string) {
	m.mu.Lock()
	s, ok := m.subs[pluginID]
	m.mu.Unlock()
	if ok {
		select {
		case s.exited <- struct{}{}:
		default:
		}
	}
}

// Notify applies a plugin notification. Methods other than the three social
// notifications are ignored.
func (m *Manager) Notify(pluginID, method string, params json.RawMessage) {
	m.mu.Lock()
	s, ok := m.subs[pluginID]
	m.mu.Unlock()
	if !ok {
		return
	}
	s.mu.Lock()
	switch method {
	case NoteConnection:
		var c Connection
		if json.Unmarshal(params, &c) != nil || c.State == "" {
			s.mu.Unlock()
			return
		}
		s.view.Connection = c
		switch c.State {
		case ConnReplaced, ConnLoggedOut:
			s.paused = true // stop fighting for the session until the plugin connects again
		case ConnOnline, ConnConnecting:
			s.paused = false
		}
	case NoteSelf:
		var self Self
		if json.Unmarshal(params, &self) != nil {
			s.mu.Unlock()
			return
		}
		s.view.Self = &self
	case NoteFriends:
		var push struct {
			Full    bool     `json:"full"`
			Friends []Friend `json:"friends"`
		}
		if json.Unmarshal(params, &push) != nil {
			s.mu.Unlock()
			return
		}
		if push.Full {
			s.view.Friends = merge(nil, push.Friends)
		} else {
			s.view.Friends = merge(s.view.Friends, push.Friends)
		}
	default:
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	m.emit(s)
}

// merge applies friends to list: an entry with a known id replaces it, a new
// one is appended. The result is never nil.
func merge(list, friends []Friend) []Friend {
	if list == nil {
		list = []Friend{}
	}
	for _, f := range friends {
		if f.ID == "" {
			continue
		}
		i := slices.IndexFunc(list, func(x Friend) bool { return x.ID == f.ID })
		if i >= 0 {
			list[i] = f
		} else {
			list = append(list, f)
		}
	}
	return list
}

// SetStatus changes the user's own status on a plugin's store. It needs a
// connected subscription and a store that lets the user change it.
func (m *Manager) SetStatus(ctx context.Context, pluginID, state string) error {
	if !slices.Contains(Statuses, state) {
		return fmt.Errorf("status must be one of %s, got %q", strings.Join(Statuses, ", "), state)
	}
	m.mu.Lock()
	s, ok := m.subs[pluginID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("friends for %s are off", pluginID)
	}
	s.mu.Lock()
	connected := s.connected
	s.mu.Unlock()
	if !connected {
		return errors.New("not connected to the store")
	}
	err := m.opt.Backend.Call(ctx, pluginID, MethodSetStatus, map[string]string{"state": state}, nil)
	if isMethodMissing(err) {
		return errors.New("this store doesn't let Hydra change your status")
	}
	return err
}

// GameStarted records that a game of a plugin with a subscription is running,
// when its id is a plugin game key (p.<plugin>.<id>). Other games are ignored.
// The plugin learns the store id, as social.setPlaying takes it.
func (m *Manager) GameStarted(gameID string) {
	pluginID, storeID, ok := m.owner(gameID)
	if !ok {
		return
	}
	m.setPlaying(pluginID, storeID, true)
}

// GameStopped clears the playing game of the plugin that owns gameID, when
// that game is the one recorded.
func (m *Manager) GameStopped(gameID string) {
	pluginID, storeID, ok := m.owner(gameID)
	if !ok {
		return
	}
	m.setPlaying(pluginID, storeID, false)
}

// owner finds the subscribed plugin a Hydra game key belongs to.
func (m *Manager) owner(gameID string) (pluginID, storeID string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.subs {
		if rest, found := strings.CutPrefix(gameID, "p."+id+"."); found && rest != "" {
			return id, rest, true
		}
	}
	return "", "", false
}

func (m *Manager) setPlaying(pluginID, storeID string, on bool) {
	m.mu.Lock()
	s, ok := m.subs[pluginID]
	m.mu.Unlock()
	if !ok {
		return
	}
	s.mu.Lock()
	if on {
		s.playing = storeID
	} else if s.playing == storeID {
		s.playing = ""
	} else {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// run keeps the plugin subscribed until ctx ends: connect, serve until the
// plugin goes away, back off, repeat.
func (m *Manager) run(ctx context.Context, s *sub) {
	defer close(s.done)
	minB, maxB, stable := m.opt.MinBackoff, m.opt.MaxBackoff, m.opt.StableAfter
	if minB <= 0 {
		minB = 2 * time.Second
	}
	if maxB <= 0 {
		maxB = 5 * time.Minute
	}
	if stable <= 0 {
		stable = time.Minute
	}
	backoff := minB
	for {
		if err := m.connect(ctx, s); err != nil {
			if ctx.Err() != nil {
				m.stopPlugin(s)
				return
			}
			m.logf("social %s: %v; retrying in %s", s.id, err, backoff)
			if !sleep(ctx, backoff) {
				m.stopPlugin(s)
				return
			}
			backoff = min(backoff*2, maxB)
			continue
		}
		began := time.Now()
		if !m.serve(ctx, s) {
			m.endSession(s)
			return
		}
		m.endSession(s)
		// A session that connected and then dropped at once doesn't reset the
		// wait: a plugin that crashes on start would otherwise restart every 2 s.
		if time.Since(began) > stable {
			backoff = minB
		}
		s.mu.Lock()
		paused := s.paused
		s.mu.Unlock()
		if paused {
			<-ctx.Done()
			return
		}
		m.logf("social %s: restarting in %s", s.id, backoff)
		if !sleep(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, maxB)
	}
}

// connect starts the plugin's social and seeds the view from its snapshot.
func (m *Manager) connect(ctx context.Context, s *sub) error {
	// An exit left by an earlier attempt (the process died while it failed)
	// must not end this session.
	select {
	case <-s.exited:
	default:
	}
	if !m.opt.Backend.Offers(ctx, s.id) {
		return errors.New("the plugin doesn't offer social right now")
	}
	if err := m.opt.Backend.Call(ctx, s.id, MethodStart, struct{}{}, nil); err != nil {
		return err
	}
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	var snap struct {
		Connection *Connection `json:"connection"`
		Self       *Self       `json:"self"`
		Friends    []Friend    `json:"friends"`
	}
	if err := m.opt.Backend.Call(ctx, s.id, MethodState, struct{}{}, &snap); err != nil {
		return err
	}
	s.mu.Lock()
	s.connected = true
	s.paused = false
	s.view.Connection = Connection{State: ConnConnecting}
	if snap.Connection != nil && snap.Connection.State != "" {
		s.view.Connection = *snap.Connection
	}
	s.view.Self = snap.Self
	s.view.Friends = merge(nil, snap.Friends)
	s.sent = "" // a new process knows of no game yet
	s.mu.Unlock()
	m.emit(s)
	m.sendPlaying(ctx, s)
	return nil
}

// serve reports the playing game while the plugin runs. It returns false when
// ctx ended (the subscription is stopping) and true when the process went away.
func (m *Manager) serve(ctx context.Context, s *sub) bool {
	for {
		select {
		case <-ctx.Done():
			m.stopPlugin(s)
			return false
		case <-s.exited:
			return true
		case <-s.kick:
			m.sendPlaying(ctx, s)
		}
	}
}

// stopPlugin asks a connected plugin to stop, bounded by stopTimeout.
func (m *Manager) stopPlugin(s *sub) {
	s.mu.Lock()
	started := s.started
	s.started = false
	s.mu.Unlock()
	if !started {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	if err := m.opt.Backend.Call(ctx, s.id, MethodStop, struct{}{}, nil); err != nil {
		m.logf("social %s: stop: %v", s.id, err)
	}
}

// endSession marks the subscription disconnected after its process went away.
// A session the store replaced or ended keeps its connection state, which is
// what the UI explains.
func (m *Manager) endSession(s *sub) {
	s.mu.Lock()
	s.connected = false
	s.started = false
	if !s.paused {
		s.view.Connection = Connection{State: ConnReconnecting, Message: "the store plugin stopped"}
	}
	s.mu.Unlock()
	m.emit(s)
}

// sendPlaying tells the plugin the game the user plays when it changed since
// the last message to this process: the id, or null when the game stopped.
// Plugins without the optional method answer -32601, which counts as sent.
func (m *Manager) sendPlaying(ctx context.Context, s *sub) {
	s.mu.Lock()
	want, sent := s.playing, s.sent
	s.mu.Unlock()
	if want == sent {
		return
	}
	var id any // null
	if want != "" {
		id = want
	}
	err := m.opt.Backend.Call(ctx, s.id, MethodSetPlaying, map[string]any{"id": id}, nil)
	if err != nil && !isMethodMissing(err) {
		if ctx.Err() == nil {
			m.logf("social %s: setPlaying: %v", s.id, err)
		}
		return // try again on the next change
	}
	s.mu.Lock()
	s.sent = want
	s.mu.Unlock()
}

func (m *Manager) snapshot(s *sub) View {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.view
	v.Enabled = m.enabled(s.id)
	v.Friends = slices.Clone(s.view.Friends)
	if v.Friends == nil {
		v.Friends = []Friend{}
	}
	return v
}

func (m *Manager) emit(s *sub) {
	if m.opt.Emit != nil {
		m.opt.Emit(m.snapshot(s))
	}
}

func (m *Manager) enabled(pluginID string) bool {
	return m.opt.Enabled == nil || m.opt.Enabled(pluginID)
}

func (m *Manager) logf(format string, args ...any) {
	if m.opt.Logf != nil {
		m.opt.Logf(format, args...)
	}
}

// isMethodMissing reports whether a call failed because the plugin doesn't
// have the method (JSON-RPC -32601).
func isMethodMissing(err error) bool {
	var rpc *jsonrpc.Error
	return errors.As(err, &rpc) && rpc.Code == jsonrpc.CodeMethodNotFound
}

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
