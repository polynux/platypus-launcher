// Auth subcommand (phase 6): QR and password logins in Go, replacing
// DepotDownloader. Speaks JSON lines like the other subcommands:
//
//	platypus-steam-dl auth
//
// Stdin starts with one request line:
//
//	{"mode":"qr"}
//	{"mode":"password","account":"...","password":"...","code":"... (optional)"}
//
// The password never goes in argv and is never logged. Stdout carries
// events until a final result or error:
//
//	{"type":"qr","challenge_url":"...","qr_art":"... block-art ..."}
//	{"type":"code-needed","guard_type":"email|device","message":"..."}
//	{"type":"waiting","message":"..."}
//	{"type":"result","account":"...","steamid":123}
//	{"type":"error","code":"...","message":"..."}
//
// A second stdin line {"code":"..."} answers a code-needed event when the
// initial request carried no code. On success the refresh token, account
// name and steamid land in the SAME files the plugin already uses
// (refresh.token 0600, token.json, session.json — see tokenStore below),
// so the existing login keeps working without logging in again.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"platypus/internal/steamcm"
)

type authRequest struct {
	Mode     string `json:"mode"`
	Account  string `json:"account"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

type authCodeReply struct {
	Code string `json:"code"`
}

func runAuth(ctx context.Context) error {
	// The first stdin line is the request; further lines (if any) answer
	// code-needed events. A closed stdin aborts the wait.
	br := bufio.NewReader(os.Stdin)
	line, err := br.ReadString('\n')
	if err != nil {
		// A single line without newline (tests) still counts.
		if len(line) == 0 {
			emit(map[string]any{"type": "error", "code": "error", "message": "auth request is missing"})
			os.Exit(1)
		}
	}
	var req authRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		emit(map[string]any{"type": "error", "code": "error", "message": fmt.Sprintf("bad auth request: %v", err)})
		os.Exit(1)
	}
	// Remaining replies (code answers) stream in the background so polls
	// stay responsive; cancel closes the reader.
	replies := make(chan authCodeReply, 2)
	go func() {
		for {
			l, err := br.ReadString('\n')
			if err != nil {
				if strings.TrimSpace(l) != "" {
					var r authCodeReply
					if json.Unmarshal([]byte(l), &r) == nil && r.Code != "" {
						replies <- r
					}
				}
				return
			}
			var r authCodeReply
			// Accept both {"code":"..."} and the full request shape.
			if json.Unmarshal([]byte(l), &r) == nil && r.Code != "" {
				replies <- r
				continue
			}
			var full authRequest
			if json.Unmarshal([]byte(l), &full) == nil && full.Code != "" {
				replies <- authCodeReply{Code: full.Code}
			}
		}
	}()
	var runErr error
	switch req.Mode {
	case "qr", "":
		runErr = runAuthQR(ctx, req)
	case "password":
		runErr = runAuthPassword(ctx, req, replies)
	default:
		runErr = fmt.Errorf("unknown auth mode %q", req.Mode)
	}
	if runErr != nil {
		if ce, ok := runErr.(*steamcm.CMError); ok {
			code := "error"
			if ce.EResult == 84 {
				code = "rate-limited"
			}
			emit(map[string]any{"type": "error", "code": code, "message": ce.Error()})
		} else {
			emit(map[string]any{"type": "error", "code": "error", "message": runErr.Error()})
		}
		os.Exit(1)
	}
	return nil
}

// qrArt renders the challenge URL as block-art the dialog draws: two
// characters per module ("██" dark, two spaces light), one row per
// module, no margins — the frontend derives the grid from the widest
// row, like the old DepotDownloader art.
func qrArt(challengeURL string) string {
	q, err := qrcode.New(challengeURL, qrcode.Medium)
	if err != nil {
		return ""
	}
	q.DisableBorder = true
	bm := q.Bitmap()
	rows := make([]string, len(bm))
	for y, row := range bm {
		var sb strings.Builder
		for _, dark := range row {
			if dark {
				sb.WriteString("██")
			} else {
				sb.WriteString("  ")
			}
		}
		rows[y] = sb.String()
	}
	return strings.Join(rows, "\n")
}

func runAuthQR(ctx context.Context, req authRequest) error {
	cm, err := steamcm.DialUnauth(ctx)
	if err != nil {
		return err
	}
	defer cm.Close()
	qr, err := cm.BeginQR(30 * time.Second)
	if err != nil {
		return err
	}
	art := qrArt(qr.ChallengeURL)
	emit(map[string]any{"type": "qr", "challenge_url": qr.ChallengeURL, "qr_art": art})
	clientID, requestID := qr.ClientID, qr.RequestID
	deadline := time.Now().Add(5 * time.Minute)
	interval := time.Duration(qr.Interval * float32(time.Second))
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("nobody scanned the QR code in time; try again")
		}
		// Poll with a timeout shorter than the overall deadline so a
		// dropped CM doesn't hang the dialog.
		st, err := cm.PollStatus(clientID, requestID, 30*time.Second)
		if err != nil {
			return err // 84 and expiry surface here, no retry loop
		}
		if st.NewClientID != 0 {
			clientID = st.NewClientID
		}
		if st.NewChallengeURL != "" && st.NewChallengeURL != qr.ChallengeURL {
			qr.ChallengeURL = st.NewChallengeURL
			emit(map[string]any{"type": "qr", "challenge_url": st.NewChallengeURL, "qr_art": qrArt(st.NewChallengeURL)})
		}
		if st.RefreshToken != "" {
			account := st.AccountName
			return storeToken(st.RefreshToken, account)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func runAuthPassword(ctx context.Context, req authRequest, replies <-chan authCodeReply) error {
	if strings.TrimSpace(req.Account) == "" || req.Password == "" {
		return fmt.Errorf("enter your Steam account name and password")
	}
	cm, err := steamcm.DialUnauth(ctx)
	if err != nil {
		return err
	}
	defer cm.Close()
	mod, exp, ts, err := cm.GetPasswordRSAPublicKey(req.Account, 30*time.Second)
	if err != nil {
		return err
	}
	enc, err := steamcm.EncryptPassword(req.Password, mod, exp)
	if err != nil {
		return err
	}
	beg, err := cm.BeginCredentials(req.Account, enc, ts, 30*time.Second)
	if err != nil {
		return err
	}
	codeType, needCode, needConfirm := steamcm.GuardNeeded(beg.Allowed)
	code := strings.TrimSpace(req.Code)
	if needCode && code == "" {
		// Ask once; the plugin feeds the guard it already collected,
		// or fails fast so the user retries with a fresh code.
		guard := "email"
		if codeType == steamcm.GuardDeviceCode {
			guard = "device"
		}
		msg := "Enter the Steam Guard code"
		for _, a := range beg.Allowed {
			if a.Message != "" {
				msg = a.Message
				break
			}
		}
		emit(map[string]any{"type": "code-needed", "guard_type": guard, "message": msg})
		select {
		case r := <-replies:
			code = strings.TrimSpace(r.Code)
		case <-time.After(5 * time.Minute):
			return fmt.Errorf("no Steam Guard code entered in time; try again")
		case <-ctx.Done():
			return ctx.Err()
		}
		if code == "" {
			return fmt.Errorf("Steam needs a Steam Guard code for this account")
		}
	}
	if needCode {
		if err := cm.UpdateGuardCode(beg.ClientID, beg.SteamID, code, codeType, 30*time.Second); err != nil {
			return err
		}
	} else if needConfirm {
		emit(map[string]any{"type": "waiting", "message": "Approve the login in the Steam mobile app and wait — this closes by itself."})
	}
	clientID, requestID := beg.ClientID, beg.Request
	interval := time.Duration(beg.Interval * float32(time.Second))
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(5 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("the Steam login timed out; try again")
		}
		st, err := cm.PollStatus(clientID, requestID, 30*time.Second)
		if err != nil {
			return err
		}
		if st.NewClientID != 0 {
			clientID = st.NewClientID
		}
		if st.RefreshToken != "" {
			account := st.AccountName
			if account == "" {
				account = req.Account
			}
			return storeToken(st.RefreshToken, account)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// storeToken persists the login where both the Python plugin and the Go
// downloader find it: refresh.token (0600, raw JWT), token.json (same
// keys the plugin already uses, plus steamid) and session.json (same
// account shape, no secret so 0644 is fine). Token files are chmodded
// after writing because WriteFile keeps a pre-existing 0644 mode. It
// emits the result line for the plugin to forward.
func storeToken(refreshToken, account string) error {
	if refreshToken == "" {
		return fmt.Errorf("Steam issued no login token")
	}
	steamID, err := steamcm.SteamIDFromToken(refreshToken)
	if err != nil {
		return fmt.Errorf("Steam's login token didn't parse: %v", err)
	}
	if account == "" {
		account = fmt.Sprintf("%d", steamID)
	}
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "refresh.token"), []byte(refreshToken+"\n"), 0o600); err != nil {
		return err
	}
	_ = os.Chmod(filepath.Join(dir, "refresh.token"), 0o600)
	// token.json keeps the keys the plugin already reads
	// (refresh_token, account, source) so the current login keeps
	// working; steamid is new and ignored by older readers.
	type tokenFile struct {
		RefreshToken string `json:"refresh_token"`
		Account      string `json:"account"`
		SteamID      string `json:"steamid"`
		Source       []any  `json:"source"`
	}
	var mtime int64
	if fi, err := os.Stat(filepath.Join(dir, "refresh.token")); err == nil {
		mtime = fi.ModTime().Unix()
	}
	tf := tokenFile{RefreshToken: refreshToken, Account: account, SteamID: fmt.Sprintf("%d", steamID),
		Source: []any{filepath.Join(dir, "refresh.token"), mtime}}
	raw, _ := json.Marshal(tf)
	if err := os.WriteFile(filepath.Join(dir, "token.json"), raw, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(filepath.Join(dir, "token.json"), 0o600)
	sf, _ := json.Marshal(map[string]string{"account": account})
	if err := os.WriteFile(filepath.Join(dir, "session.json"), sf, 0o644); err != nil {
		return err
	}
	emit(map[string]any{"type": "result", "account": account, "steamid": steamID})
	return nil
}
