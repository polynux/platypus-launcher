package app

import "context"

// Entry points for the shells that run the App (the Wails window in the repo
// root, the Electron core in cmd/platypus-core).
//
// They are package-level functions, not methods: the core serves the App's
// exported methods to the UI over RPC (internal/corerpc), and the lifecycle
// must stay out of that surface. The RPC surface is the methods below; these
// three only wire a shell up to them.

// SetHost sets how the App reaches the shell it runs in: events, log,
// dialogs (host.go). A shell calls it before Startup.
func SetHost(a *App, h Host) { a.host = h }

// Startup prepares the App to serve: settings, plugins, jobs and the rest
// (see startup in app.go). The core calls it while answering initialize, so
// every later call finds the App ready.
func Startup(a *App, ctx context.Context) { a.startup(ctx) }

// Shutdown stops games Platypus started directly and every store plugin
// (see shutdown in app.go).
func Shutdown(a *App, ctx context.Context) { a.shutdown(ctx) }
