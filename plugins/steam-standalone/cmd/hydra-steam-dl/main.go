// Command hydra-steam-dl downloads Steam depots for the steam-standalone
// plugin, replacing DepotDownloader for installs and logins. It speaks
// one subcommand per argv:
//
//	hydra-steam-dl size|install|update|verify|auth
//
// The request (app id, branch, dirs, and the refresh token) arrives as one
// JSON object on stdin — the token never goes in argv (visible in
// /proc/<pid>/cmdline) and is never logged. Progress and the final result
// go to stdout as JSON lines: {"type":"progress",...} during the job and
// a last line that is either {"type":"result",...} or
// {"type":"error","code":...,"message":...} with code "auth-required"
// when Steam says the login is no good, else "error". The auth subcommand
// is interactive (qr/code-needed/waiting events, code replies on stdin —
// see auth.go) and stores the login where the plugin finds it.
//
// State lives in HYDRA_PLUGIN_DATA: job files under jobs/ and installed
// manifests under manifests/<app>/, like the plan's disk layout.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fatal("error", "usage: hydra-steam-dl size|install|update|verify|auth")
	}
	if os.Args[1] == "auth" {
		if err := runAuth(context.Background()); err != nil {
			emit(map[string]any{"type": "error", "code": "error", "message": err.Error()})
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
	ctx := context.Background()
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
	default:
		fatal("error", "usage: hydra-steam-dl size|install|update|verify|auth")
	}
	if runErr != nil {
		if ce, ok := runErr.(cmErr); ok && ce.auth {
			emit(map[string]any{"type": "error", "code": "auth-required", "message": ce.msg})
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
	InstallDir   string `json:"install_dir"`
	JobID        string `json:"job_id"`
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
	dir := os.Getenv("HYDRA_PLUGIN_DATA")
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
