// Command platypus-steam-dl downloads Steam depots for the steam-standalone
// plugin, replacing DepotDownloader for installs and logins. It speaks
// one subcommand per argv:
//
//	platypus-steam-dl size|install|update|verify|terms|versions|unlock|auth|cloud|family-running|dlc
//
// The request (app id, branch, dirs, and the refresh token) arrives as one
// JSON object on stdin — the token never goes in argv (visible in
// /proc/<pid>/cmdline) and is never logged. Progress and the final result
// go to stdout as JSON lines: {"type":"progress",...} during the job and
// a last line that is either {"type":"result",...} or
// {"type":"error","code":...,"message":...} with code "auth-required"
// when Steam says the login is no good, "wrong-password" (plus "branch")
// when Steam refused the tried beta password, else "error". The auth
// is interactive (qr/code-needed/waiting events, code replies on stdin —
// see auth.go) and stores the login where the plugin finds it.
//
// State lives in PLATYPUS_PLUGIN_DATA: job files under jobs/ and installed
// manifests under manifests/<app>/, like the plan's disk layout.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
)

func main() {
	if len(os.Args) != 2 {
		fatal("error", "usage: platypus-steam-dl size|install|update|verify|terms|versions|unlock|auth|cloud|family-running|workshop")
	}
	if os.Args[1] == "auth" {
		if err := runAuth(context.Background()); err != nil {
			emit(map[string]any{"type": "error", "code": "error", "message": err.Error()})
			os.Exit(1)
		}
		return
	}
	if os.Args[1] == "social" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if err := runSocial(ctx); err != nil {
			os.Exit(1)
		}
		return
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal("error", fmt.Sprintf("couldn't read the request: %v", err))
	}
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		fatal("error", fmt.Sprintf("bad request: %v", err))
	}
	if req.RefreshToken == "" {
		req.RefreshToken = tokenFromFile()
	}
	if req.RefreshToken == "" {
		fatal("auth-required", "log in to Steam first")
	}
	if os.Args[1] == "cloud" {
		// STEAM-12: SIGINT (job.cancel) cancels the context, so an
		// in-flight download aborts and the baseline keeps only files
		// actually written.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if runErr := runCloud(ctx, req, req.Op, req.Ctx, req.Choices); runErr != nil {
			if ce, ok := runErr.(cmErr); ok && ce.auth {
				emit(map[string]any{"type": "error", "code": "auth-required", "message": ce.msg})
			} else {
				emit(map[string]any{"type": "error", "code": "error", "message": runErr.Error()})
			}
			os.Exit(1)
		}
		return
	}
	// STEAM-12: SIGINT (job.cancel) cancels the context instead of killing
	// immediately, so DownloadStream aborts and the deferred flusher saves
	// JobState + staging before exit; the next run fetches only missing.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var runErr error
	switch os.Args[1] {
	case "size":
		runErr = runSize(ctx, req)
	case "install":
		runErr = runInstall(ctx, req)
	case "update":
		runErr = runUpdate(ctx, req)
	case "verify":
		runErr = runVerify(ctx, req)
	case "terms":
		runErr = runTerms(ctx, req)
	case "versions":
		runErr = runVersions(ctx, req)
	case "unlock":
		runErr = runUnlock(ctx, req)
	case "family-running":
		runErr = runFamilyRunning(ctx, req)
	case "dlc":
		runErr = runDLC(ctx, req)
	case "workshop":
		runErr = runWorkshop(ctx, req)
	default:
		fatal("error", "usage: platypus-steam-dl size|install|update|verify|terms|versions|unlock|auth|cloud|family-running|dlc|workshop")
	}
	if runErr != nil {
		var wpe wrongPasswordError
		if ce, ok := runErr.(cmErr); ok && ce.auth {
			emit(map[string]any{"type": "error", "code": "auth-required", "message": ce.msg})
		} else if errors.As(runErr, &wpe) {
			// Structured refusal: the plugin drops the stored secret
			// for this branch (a transport error never does).
			emit(map[string]any{"type": "error", "code": "wrong-password", "branch": wpe.Branch, "message": wpe.Error()})
		} else if _, ok := runErr.(notFoundError); ok {
			emit(map[string]any{"type": "error", "code": "not-found", "message": runErr.Error()})
		} else {
			emit(map[string]any{"type": "error", "code": "error", "message": runErr.Error()})
		}
		os.Exit(1)
	}
}

type request struct {
	RefreshToken string `json:"refresh_token"`
	AppID        uint32 `json:"app_id"`
	Branch       string `json:"branch"`
	// BranchPassword is the single password for Branch (legacy); Target
	// installs take theirs from BranchPasswords by resolved branch.
	BranchPassword  string            `json:"branch_password"`
	BranchPasswords map[string]string `json:"branch_passwords"`
	Target          *versionTarget    `json:"target"`
	InstallDir      string            `json:"install_dir"`
	JobID           string            `json:"job_id"`
	// DownloadLimit is the global speed cap in bytes per second (protocol
	// 1.21, issue #207); 0 means unlimited. One limiter spans every depot
	// of the run, so the cap is for the whole job.
	DownloadLimit int64 `json:"download_limit"`
	// Repair, on a verify run, re-downloads the bad files after the check
	// (protocol 1.4): without it the run only reads.
	Repair bool `json:"repair"`
	// DLC is the protocol 1.5 add-on selection (value ids, as strings):
	// depots whose dlcappid names another DLC are skipped. Nil installs
	// the old set (the legacy request shape without the field); an empty
	// list installs the base game only.
	DLC []string `json:"dlc"`
	// Op is the cloud subcommand (status|pull|push|resolve); Ctx the
	// CloudContext and Choices the resolve paths, for the cloud op only.
	Op      string          `json:"op"`
	Ctx     json.RawMessage `json:"ctx"`
	Choices json.RawMessage `json:"choices"`
	// Items are the workshop subcommand's published files (id, hcontent).
	Items []workshopItem `json:"items"`
	// FamilyGroupID skips the ForUser lookup for the family-running read
	// when the caller already knows the group (the plugin's Web API
	// fetch does); zero means look it up over the CM.
	FamilyGroupID uint64 `json:"family_group_id"`
}

type cmErr struct {
	msg  string
	auth bool
}

func (e cmErr) Error() string { return e.msg }

func emit(v map[string]any) {
	raw, _ := json.Marshal(v)
	fmt.Println(string(raw))
}

func fatal(code, msg string) {
	emit(map[string]any{"type": "error", "code": code, "message": msg})
	os.Exit(1)
}

// tokenFromFile reads the refresh token from its 0600 file in the plugin's
// data folder, when the request carried none on stdin. It tightens a
// pre-existing 0644 file to 0600 on first touch.
func tokenFromFile() string {
	dir := os.Getenv("PLATYPUS_PLUGIN_DATA")
	if dir == "" {
		return ""
	}
	raw, err := os.ReadFile(dir + "/refresh.token")
	if err != nil {
		return ""
	}
	_ = os.Chmod(dir+"/refresh.token", 0o600)
	// Strip trailing newlines; tokens are single-line JWTs.
	for len(raw) > 0 && (raw[len(raw)-1] == '\n' || raw[len(raw)-1] == '\r') {
		raw = raw[:len(raw)-1]
	}
	return string(raw)
}
