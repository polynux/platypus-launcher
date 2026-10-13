// The core: the App without a window. The UI shell (the desktop app, see
// apps/desktop/)
// spawns this binary and talks to it over stdio: JSON-RPC 2.0, one object per
// line (internal/corerpc). Build it with
// `go build -o platypus-core ./cmd/platypus-core` (no GTK/WebKit linked).
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"platypus/internal/app"
	"platypus/internal/corerpc"
	"platypus/internal/paths"
)

func main() {
	// Stdout carries the protocol and nothing else: keep our own handle on it
	// and point fd 1 at stderr, so a stray print can't corrupt the stream.
	// Close-on-exec on the duplicate: plugins and games the core spawns must
	// not inherit the protocol fd (Go's os/exec only closes close-on-exec
	// fds, and a child holding our stdout open would stall EOF and could
	// inject stream lines).
	protoFd, err := syscall.Dup(1)
	if err != nil {
		fatal(err)
	}
	syscall.CloseOnExec(protoFd)
	if err := syscall.Dup3(2, 1, 0); err != nil {
		fatal(err)
	}
	proto := os.NewFile(uintptr(protoFd), "protocol")

	// Folders resolve once, from the environment (issue #41). The shell
	// passes PLATYPUS_* through when it spawns the core.
	cfg := paths.Resolve()
	a := app.NewAppWithOptions(app.Options{
		DataDir: cfg.DataDir, CacheDir: cfg.CacheDir, PluginsDir: cfg.PluginsDir, Home: cfg.Home,
	})
	srv := corerpc.New(a)
	app.SetHost(a, coreHost{srv: srv})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	art, err := serveArt(a)
	if err != nil {
		fatal(err)
	}
	srv.Info = map[string]any{"artURL": art.url, "artToken": art.token}
	srv.OnInitialize = func() error {
		app.Startup(a, ctx)
		return nil
	}

	// SIGTERM/SIGINT end the stream like the shell closing our stdin does.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		_ = os.Stdin.Close()
	}()

	if err := srv.Serve(os.Stdin, proto); err != nil {
		fmt.Fprintln(os.Stderr, "core: reading the shell:", err)
	}
	app.Shutdown(a, ctx)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "core:", err)
	os.Exit(1)
}

// coreHost is the host of the Electron shell: events are notifications, the
// native dialogs are requests the shell answers, logs go to stderr (the shell
// forwards them).
type coreHost struct {
	srv *corerpc.Server
}

var _ app.Host = coreHost{}

func (h coreHost) Emit(event string, data ...any) {
	var payload any
	if len(data) > 0 {
		payload = data[0]
	}
	_ = h.srv.Notify("event", map[string]any{"name": event, "data": payload})
}

func (coreHost) Infof(format string, args ...any)  { logf("info", format, args...) }
func (coreHost) Warnf(format string, args ...any)  { logf("warn", format, args...) }
func (coreHost) Errorf(format string, args ...any) { logf("error", format, args...) }

func logf(level, format string, args ...any) {
	fmt.Fprintf(os.Stderr, level+" "+format+"\n", args...)
}

func (h coreHost) OpenFile(d app.FileDialog) (string, error) {
	var path string
	err := h.srv.Call(context.Background(), "dialog.openFile", d, &path)
	return path, err
}

func (h coreHost) OpenDirectory(d app.DirectoryDialog) (string, error) {
	var path string
	err := h.srv.Call(context.Background(), "dialog.openDirectory", d, &path)
	return path, err
}

type artServer struct{ url, token string }

// serveArt serves the store plugins' local art (including the bundled Steam
// plugin's library cache files), the user's custom art and the store plugin
// logos over HTTP
// on 127.0.0.1, for the shell's art protocol to fetch from. Every request must
// carry this run's random token, so another local process can't read the art
// (or probe the file paths) through the port.
func serveArt(a *app.App) (artServer, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return artServer{}, err
	}
	token := hex.EncodeToString(b)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return artServer{}, err
	}
	handler := app.PluginArt(a, app.PluginGameArt(a, app.CustomArt(a, http.NotFoundHandler())))
	go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Platypus-Token")), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	return artServer{url: "http://" + l.Addr().String(), token: token}, nil
}
