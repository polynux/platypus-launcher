# Electron shell (spike, issue #29)

The UI can run in Electron with the Go App as a child process. The Wails build
still works; both entry points build from the same code.

```
Electron main ──spawn──> platypus-core (Go, the App without a window)
   │  stdin/stdout: JSON-RPC 2.0, one object per line (internal/corerpc)
   │   requests  → App's exported methods, positional JSON args
   │   notifications ← events ("event" {name, data})
   │   requests  ← dialog.openFile / dialog.openDirectory
   │  stderr: the core's log, shown in the terminal
   │
   ├─ preload (sandboxed): window.platypus = { invoke, on, openExternal }
   └─ page app://platypus/  ← frontend (frontend/src/effect/bridge.ts picks
                              the bridge when window.platypus exists, else Wails)
```

## Try it

Needs Go, bun, node ≥ 22 and an Electron (the system one works:
`PLATYPUS_ELECTRON=/usr/bin/electron`; otherwise `bun install` in `electron/`
downloads one).

```sh
cd electron && bun install          # once (ELECTRON_SKIP_BINARY_DOWNLOAD=1 to use the system Electron)
node scripts/run.mjs start          # builds core + shell (+ frontend if missing), runs it
node scripts/run.mjs start --rebuild   # also rebuilds frontend/dist
node scripts/run.mjs dev            # same, frontend from Vite with hot reload
```

With your real data (it reads and writes `~/.local/share/platypus` exactly as
the Wails build does, so don't run both at once): the same commands, no extra
setup. To look first without touching anything, use a scratch home:
`HOME=/tmp/platypus-try node scripts/run.mjs start`, then install a plugin
there with `PLATYPUS_PLUGINS=/tmp/platypus-try/.local/share/platypus/plugins sh plugins/install.sh echo`.

Environment: `PLATYPUS_ELECTRON` (binary), `PLATYPUS_ELECTRON_ARGS` (extra
flags, e.g. `--ozone-platform=x11`), `PLATYPUS_DEVTOOLS=1` (F12),
`PLATYPUS_REMOTE_DEBUGGING_PORT` (for Playwright over CDP, loopback).

The core alone: `go build -o platypus-core ./cmd/platypus-core` (no GTK/WebKit linked).

## Pieces

- `internal/app/host.go`: the `Host` interface (events, log, dialogs).
  `host_wails.go` is the Wails one; the Electron core (`cmd/platypus-core`)
  sends events as notifications and dialogs as requests to Electron.
- `internal/corerpc`: reflection dispatch of `*App`'s exported methods;
  `initialize {protocol}` handshake (core protocol 1.0: same major, shell minor
  ≤ core minor); the App starts while `initialize` answers.
- Art: `/steam-art/` and `/platypus-art/` are served by the core on
  `127.0.0.1:<random>`, requests need a per-run random token. Only Electron's
  main process holds it: the `app://` handler adds it and proxies. The page
  never sees the token and CSP has no `connect-src` beyond itself.
- Security: contextIsolation, sandbox, no nodeIntegration, permissions denied,
  navigation and `window.open` blocked (http/https go to `shell.openExternal`),
  IPC accepted only from `app://platypus`, only for methods the core listed,
  strict CSP (scripts from the app only; images may come from https and
  loopback). The dev CSP adds `unsafe-inline` and Vite's websocket.

## What the spike does not do

- Types: the Electron path is typed by `go run ./cmd/gen-ts`, which reflects
  over the App's exported methods and the structs they exchange
  (`internal/gents`) into `frontend/src/gen/` (checked in; a Go test fails
  when it is stale). The Wails path still uses `frontend/wailsjs/go/models.ts`
  and the generated `App.d.ts` (kept current with `wails generate module`)
  until step 2 removes it.
- Packaging (electron-builder / AppImage), auto-update, a menu, the window
  icon, remembering window size/position.
- Event delivery when no window is open (events are dropped, as in Wails).
- `GPU`: `PLATYPUS_GPU` and the DMA-BUF switch only concern WebKitGTK. Chromium
  logs `'--ozone-platform=wayland' is not compatible with Vulkan` (harmless:
  GPU compositing and rasterization report "enabled", Vulkan off).
