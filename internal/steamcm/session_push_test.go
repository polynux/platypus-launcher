// One-shot Session push collection (family.running, issue #178): a push
// riding before its reply surfaces through the hook during Call, a push
// queued behind the logon response is collected purely by DrainPushes,
// silence times out, a heartbeat during the drain is answered, and a
// dropped socket ends the drain. Over the steamcmtest fake; EMsg numbers
// per docs/research §4.1.
package steamcm_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"platypus/internal/depot"
	"platypus/internal/steamcm"
	"platypus/internal/steamcm/steamcmtest"
)

// pushTarget is the service notification the helper listens for; the
// hook matches on it, like runFamilyRunning does.
const pushTarget = "FamilyGroupsClient.NotifyRunningApps#1"

// runningBody encodes one NotifyRunningApps push: field 1 the family
// group, one field-2 entry {1: appid, 3: {1: player, 2: owner}}.
func runningBody(appID uint32, player, owner uint64) []byte {
	pair := &depot.Enc{}
	pair.Fixed64(1, player)
	pair.Fixed64(2, owner)
	entry := &depot.Enc{}
	entry.Varint(1, uint64(appID))
	entry.MsgField(3, pair)
	body := &depot.Enc{}
	body.Varint(1, 424242)
	body.MsgField(2, entry)
	return body.Bytes()
}

func dialSession(t *testing.T) *steamcm.Session {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := steamcm.Dial(ctx, testToken())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// pushHook collects pushes with the running-apps target; the count and
// last body are read under mu.
type pushHook struct {
	mu     sync.Mutex
	count  int
	last   []byte
	target string
}

func (h *pushHook) fn(_ uint32, hdr depot.Msg, body []byte) {
	if string(hdr.Bytes(12)) != pushTarget {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.last = body
	h.target = string(hdr.Bytes(12))
}

func (h *pushHook) got() (int, []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count, h.last
}

func decodeRunning(t *testing.T, body []byte) (uint64, uint64, uint64) {
	t.Helper()
	m, err := depot.Decode(body)
	if err != nil {
		t.Fatalf("decoding the push: %v", err)
	}
	raw := m.AllBytes(2)
	if len(raw) != 1 {
		t.Fatalf("push entries = %d, want 1", len(raw))
	}
	em, err := depot.Decode(raw[0])
	if err != nil {
		t.Fatalf("decoding the entry: %v", err)
	}
	praw := em.AllBytes(3)
	if len(praw) != 1 {
		t.Fatalf("push pairs = %d, want 1", len(praw))
	}
	pm, err := depot.Decode(praw[0])
	if err != nil {
		t.Fatalf("decoding the pair: %v", err)
	}
	return em.Int(1), pm.Int(1), pm.Int(2)
}

// A push scheduled for the call rides in before the reply: the hook sees
// it during Call, and Call still returns the reply.
func TestSessionPushBeforeReply(t *testing.T) {
	want := runningBody(5002, 76561200, 76561199)
	f := steamcmtest.New(t, steamcmtest.Config{
		ServiceHandler: func(target string, _ []byte) ([]byte, int) {
			return []byte("reply"), 1
		},
		Pushes: []steamcmtest.Push{{
			EMsg:   steamcm.EMsgServiceMethodResponse,
			Target: pushTarget,
			Body:   want,
			After:  1,
		}},
	})
	useFake(t, f)
	s := dialSession(t)
	var h pushHook
	s.SetPushHook(h.fn)
	got, err := s.Call("FamilyGroups.GetFamilyGroup#1", []byte{1}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "reply" {
		t.Fatalf("Call reply = %q, want the canned reply", got)
	}
	if n, body := h.got(); n != 1 {
		t.Fatalf("hook pushes = %d, want 1", n)
	} else if app, player, owner := decodeRunning(t, body); app != 5002 || player != 76561200 || owner != 76561199 {
		t.Fatalf("push = (%d, %d, %d), want (5002, 76561200, 76561199)", app, player, owner)
	}
}

// A push queued behind the logon response is collected purely by
// DrainPushes: no Call involved, and the stop func ends the wait at the
// first push instead of the timeout.
func TestSessionDrainCollectsQueuedPush(t *testing.T) {
	want := runningBody(5001, 76561200, 76561199)
	f := steamcmtest.New(t, steamcmtest.Config{
		Pushes: []steamcmtest.Push{{
			EMsg:   steamcm.EMsgServiceMethodResponse,
			Target: pushTarget,
			Body:   want,
			After:  0,
		}},
	})
	useFake(t, f)
	s := dialSession(t)
	var h pushHook
	s.SetPushHook(h.fn)
	stop := func() bool {
		n, _ := h.got()
		return n > 0
	}
	start := time.Now()
	s.DrainPushes(10*time.Second, stop)
	if el := time.Since(start); el > 5*time.Second {
		t.Fatalf("drain with a queued push took %v, want well under the timeout", el)
	}
	if n, body := h.got(); n != 1 {
		t.Fatalf("hook pushes = %d, want 1", n)
	} else if app, _, _ := decodeRunning(t, body); app != 5001 {
		t.Fatalf("push app = %d, want 5001", app)
	}
}

// Silence is the normal exit: the drain waits out the timeout and the
// hook saw nothing.
func TestSessionDrainTimeout(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{})
	useFake(t, f)
	s := dialSession(t)
	var h pushHook
	s.SetPushHook(h.fn)
	start := time.Now()
	s.DrainPushes(300*time.Millisecond, nil)
	if el := time.Since(start); el < 300*time.Millisecond || el > 3*time.Second {
		t.Fatalf("silent drain took %v, want about the timeout", el)
	}
	if n, _ := h.got(); n != 0 {
		t.Fatalf("hook pushes = %d, want none", n)
	}
}

// A server heartbeat during the drain is answered, not hooked: the fake
// records the client's heartbeat reply and the hook stays empty.
func TestSessionDrainAnswersHeartbeat(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{SendServerHeartbeat: true})
	useFake(t, f)
	s := dialSession(t)
	var h pushHook
	s.SetPushHook(h.fn)
	s.DrainPushes(500*time.Millisecond, nil)
	if n, _ := h.got(); n != 0 {
		t.Fatalf("hook pushes = %d, want none for a heartbeat", n)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, ev := range f.Events() {
			if ev == "heartbeat" {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fake never saw the heartbeat reply: %v", f.Events())
}

// A socket drop ends the drain: it returns instead of hanging to the
// timeout.
func TestSessionDrainEndsOnDrop(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{DropAfter: 200 * time.Millisecond})
	useFake(t, f)
	s := dialSession(t)
	var h pushHook
	s.SetPushHook(h.fn)
	start := time.Now()
	s.DrainPushes(10*time.Second, nil)
	if el := time.Since(start); el > 5*time.Second {
		t.Fatalf("drain over a dropped socket took %v, want well under the timeout", el)
	}
}
