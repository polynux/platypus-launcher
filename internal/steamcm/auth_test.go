package steamcm

import (
	"context"
	"strings"
	"testing"
	"time"
)

func setAuth(t *testing.T, fn func(*fakeConfig)) {
	t.Helper()
	currentFake.mu.Lock()
	fn(&currentFake.cfg)
	currentFake.mu.Unlock()
}

func TestAuthUnauthedUses9804(t *testing.T) {
	// Regression: an unauthenticated client must send service calls as
	// EMsg 9804 (NonAuthed), never 151 — the real CM closes the socket
	// (1000) on a pre-logon 151, which surfaced as "CM connection
	// dropped: websocket: close 1000". The fake rejects 151 pre-logon
	// the same way, so this fails on the old code.
	f, host := startFake(t)
	withFakeCM(t, host)
	setAuth(t, func(c *fakeConfig) {
		c.AuthQRApproved = true
	})
	ctx := context.Background()
	s, err := DialUnauth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	qr, err := s.BeginQR(0)
	if err != nil {
		t.Fatalf("BeginQR over 9804: %v", err)
	}
	if qr.ChallengeURL == "" {
		t.Fatal("empty challenge URL")
	}
	found := false
	for _, ev := range f.log() {
		if strings.HasPrefix(ev, "service_unauth Authentication.BeginAuthSessionViaQR#1") {
			found = true
			// Header for unauthed calls: steamid 0, session 0, job set.
			if !strings.Contains(ev, "steamid=0") || !strings.Contains(ev, "session=0") {
				t.Fatalf("unauthed header must carry steamid 0 and session 0: %q", ev)
			}
			if strings.Contains(ev, "job=0") {
				t.Fatalf("unauthed header must carry jobid_source: %q", ev)
			}
		}
		if strings.HasPrefix(ev, "rejected 151") {
			t.Fatalf("client sent 151 before logon: %q", ev)
		}
	}
	if !found {
		t.Fatalf("no unauthed BeginQR in fake log: %v", f.log())
	}
}

func TestAuthQRSuccess(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	setAuth(t, func(c *fakeConfig) {
		c.AuthQRApproved = true
		c.AuthQRRefresh = false
		c.AuthBeginEresult = 0
		c.AuthPollEresult = 0
	})
	ctx := context.Background()
	s, err := DialUnauth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	qr, err := s.BeginQR(0)
	if err != nil {
		t.Fatal(err)
	}
	if qr.ChallengeURL == "" || qr.ClientID == 0 || len(qr.RequestID) == 0 {
		t.Fatalf("bad QR begin: %+v", qr)
	}
	var token string
	for i := 0; i < 20; i++ {
		st, err := s.PollStatus(qr.ClientID, qr.RequestID, 0)
		if err != nil {
			t.Fatal(err)
		}
		if st.RefreshToken != "" {
			token = st.RefreshToken
			break
		}
		time.Sleep(time.Duration(qr.Interval*float32(time.Second)) / 5)
	}
	if token == "" {
		t.Fatal("QR never approved")
	}
	if id, err := SteamIDFromToken(token); err != nil || id == 0 {
		t.Fatalf("bad refresh token: %v %d", err, id)
	}
}

func TestAuthQRRefresh(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	setAuth(t, func(c *fakeConfig) {
		c.AuthQRApproved = true
		c.AuthQRRefresh = true
		c.AuthBeginEresult = 0
		c.AuthPollEresult = 0
	})
	ctx := context.Background()
	s, err := DialUnauth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	qr, err := s.BeginQR(0)
	if err != nil {
		t.Fatal(err)
	}
	first := qr.ChallengeURL
	var refreshed, done bool
	for i := 0; i < 20; i++ {
		st, err := s.PollStatus(qr.ClientID, qr.RequestID, 0)
		if err != nil {
			t.Fatal(err)
		}
		if st.NewChallengeURL != "" && st.NewChallengeURL != first {
			refreshed = true
		}
		if st.RefreshToken != "" {
			done = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !refreshed {
		t.Fatal("poll never returned a refreshed challenge URL")
	}
	if !done {
		t.Fatal("refreshed QR never approved")
	}
}

func TestAuthPasswordEmail(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	setAuth(t, func(c *fakeConfig) {
		c.AuthGuard = "email"
		c.AuthBeginEresult = 0
		c.AuthPollEresult = 0
	})
	ctx := context.Background()
	s, err := DialUnauth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mod, exp, ts, err := s.GetPasswordRSAPublicKey("TestUser", 0)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := EncryptPassword("correct-horse", mod, exp)
	if err != nil {
		t.Fatal(err)
	}
	beg, err := s.BeginCredentials("TestUser", enc, ts, 0)
	if err != nil {
		t.Fatal(err)
	}
	ct, needCode, _ := GuardNeeded(beg.Allowed)
	if !needCode || ct != GuardEmailCode {
		t.Fatalf("want email code, got %+v", beg.Allowed)
	}
	if err := s.UpdateGuardCode(beg.ClientID, beg.SteamID, "ABC12", ct, 0); err != nil {
		t.Fatal(err)
	}
	var token string
	for i := 0; i < 20; i++ {
		st, err := s.PollStatus(beg.ClientID, beg.Request, 0)
		if err != nil {
			t.Fatal(err)
		}
		if st.RefreshToken != "" {
			token = st.RefreshToken
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if token == "" {
		t.Fatal("email login never issued a token")
	}
}

func TestAuthPasswordTOTP(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	setAuth(t, func(c *fakeConfig) {
		c.AuthGuard = "totp"
		c.AuthBeginEresult = 0
		c.AuthPollEresult = 0
	})
	ctx := context.Background()
	s, err := DialUnauth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mod, exp, ts, err := s.GetPasswordRSAPublicKey("TestUser", 0)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := EncryptPassword("correct-horse", mod, exp)
	if err != nil {
		t.Fatal(err)
	}
	beg, err := s.BeginCredentials("TestUser", enc, ts, 0)
	if err != nil {
		t.Fatal(err)
	}
	ct, needCode, _ := GuardNeeded(beg.Allowed)
	if !needCode || ct != GuardDeviceCode {
		t.Fatalf("want TOTP code, got %+v", beg.Allowed)
	}
	if err := s.UpdateGuardCode(beg.ClientID, beg.SteamID, "123456", ct, 0); err != nil {
		t.Fatal(err)
	}
	var token string
	for i := 0; i < 20; i++ {
		st, err := s.PollStatus(beg.ClientID, beg.Request, 0)
		if err != nil {
			t.Fatal(err)
		}
		if st.RefreshToken != "" {
			token = st.RefreshToken
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if token == "" {
		t.Fatal("TOTP login never issued a token")
	}
}

func TestAuthPasswordConfirm(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	setAuth(t, func(c *fakeConfig) {
		c.AuthGuard = "confirm"
		c.AuthMobileApproved = true
		c.AuthBeginEresult = 0
		c.AuthPollEresult = 0
	})
	ctx := context.Background()
	s, err := DialUnauth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mod, exp, ts, err := s.GetPasswordRSAPublicKey("TestUser", 0)
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := EncryptPassword("correct-horse", mod, exp)
	beg, err := s.BeginCredentials("TestUser", enc, ts, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, needCode, needConfirm := GuardNeeded(beg.Allowed)
	if needCode || !needConfirm {
		t.Fatalf("want mobile confirmation, got %+v", beg.Allowed)
	}
	var token string
	for i := 0; i < 20; i++ {
		st, err := s.PollStatus(beg.ClientID, beg.Request, 0)
		if err != nil {
			t.Fatal(err)
		}
		if st.RefreshToken != "" {
			token = st.RefreshToken
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if token == "" {
		t.Fatal("mobile confirmation never issued a token")
	}
}

func TestAuthWrongPassword(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	setAuth(t, func(c *fakeConfig) { c.AuthBeginEresult = 0 })
	ctx := context.Background()
	s, err := DialUnauth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mod, exp, ts, _ := s.GetPasswordRSAPublicKey("TestUser", 0)
	enc, _ := EncryptPassword("wrong-horse", mod, exp)
	_, err = s.BeginCredentials("TestUser", enc, ts, 0)
	if err == nil || !strings.Contains(err.Error(), "wrong account name or password") {
		t.Fatalf("want wrong-password message, got %v", err)
	}
}

func TestAuthBadCode(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	setAuth(t, func(c *fakeConfig) { c.AuthGuard = "email" })
	ctx := context.Background()
	s, err := DialUnauth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mod, exp, ts, _ := s.GetPasswordRSAPublicKey("TestUser", 0)
	enc, _ := EncryptPassword("correct-horse", mod, exp)
	beg, err := s.BeginCredentials("TestUser", enc, ts, 0)
	if err != nil {
		t.Fatal(err)
	}
	ct, _, _ := GuardNeeded(beg.Allowed)
	if err := s.UpdateGuardCode(beg.ClientID, beg.SteamID, "WRONG", ct, 0); err == nil ||
		!strings.Contains(err.Error(), "wrong Steam Guard code") {
		t.Fatalf("want bad-code message, got %v", err)
	}
}

func TestAuthRateLimitStops(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	setAuth(t, func(c *fakeConfig) { c.AuthBeginEresult = 84 })
	ctx := context.Background()
	s, err := DialUnauth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mod, exp, ts, _ := s.GetPasswordRSAPublicKey("TestUser", 0)
	enc, _ := EncryptPassword("correct-horse", mod, exp)
	before := len(currentFake.log())
	_, err = s.BeginCredentials("TestUser", enc, ts, 0)
	if err == nil || !strings.Contains(err.Error(), "84") {
		t.Fatalf("want EResult 84, got %v", err)
	}
	// No retry loop: exactly one Begin call, no polls.
	after := len(currentFake.log())
	if after-before != 1 {
		t.Fatalf("rate limit retried: %d new events, want 1", after-before)
	}
}
