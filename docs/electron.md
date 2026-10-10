# Desktop shell (issue #29)

The app is the Go App as a child process with the UI in a desktop window.
The shell (`apps/desktop`, Electron + Chromium) spawns the core
(`cmd/platypus-core`, the App without a window) and shows the UI
(`apps/ui`) inside it.

```
Desktop main ──spawn──> platypus-core (Go, the App without a window)
   │  stdin/stdout: JSON-RPC 2.0, one object per line (internal/corerpc)
   │   requests  → App's exported methods, positional JSON args
   │   notifications ← events ("event" {name, data})
   │   requests  ← dialog.openFile / dialog.openDirectory
   │  stderr: the core's log, shown in the terminal
   │
   ├─ preload (sandboxed): window.platypus = { invoke, on, openExternal }
   └─ page app://platypus/  ← the UI (apps/ui/src/effect/bridge.ts calls it by
                              method name; outside the shell every call fails loudly)
```

## Try it

Needs Go, bun and node ≥ 22, and an Electron (the system one works:
`PLATYPUS_ELECTRON=/usr/bin/electron`; otherwise the root `bun install`
downloads one).

```sh
bun install                        # once (ELECTRON_SKIP_BINARY_DOWNLOAD=1 to use the system Electron)
bun run start                      # builds core + shell (+ UI if missing), runs it
bun run start -- --rebuild         # also rebuilds apps/ui/dist
bun run dev                        # same, UI from Vite with hot reload
```

With your real data (it reads and writes `~/.local/share/platypus`): the same commands, no extra
setup. To look first without touching anything, use a scratch home:
`HOME=/tmp/platypus-try bun run start`, then install a plugin
there with `PLATYPUS_PLUGINS=/tmp/platypus-try/.local/share/platypus/plugins sh plugins/install.sh echo`.

Environment: `PLATYPUS_ELECTRON` (binary), `PLATYPUS_ELECTRON_ARGS` (extra
flags, e.g. `--ozone-platform=x11`), `PLATYPUS_DEVTOOLS=1` (F12, dev only),
`PLATYPUS_REMOTE_DEBUGGING_PORT` (for Playwright over CDP, dev only,
loopback). A packaged app ignores both debug variables: no remote debugging
in production.

The core alone: `go build -o platypus-core ./cmd/platypus-core` (no GUI toolkit linked).

## Packaging

```sh
bun run package
```

Builds the UI, the shell and the core (`-trimpath`, `CGO_ENABLED=0`,
linux-x64), then packs `packaging/dist/Platypus-0.0.0.AppImage` (plus a
tar.gz) with electron-builder (`packaging/electron-builder.yml`). The
AppImage bundles the app's own pinned Electron (apps/desktop's exact
`electron` dependency, passed as `-c.electronVersion`), the built UI
(`resources/frontend`) and the core binary (`resources/platypus-core`); the
shell finds them next to itself when packaged (`apps/desktop/src/paths.ts`),
so no repo checkout is needed to run it.

Store plugins are NOT bundled: they install per user with
`plugins/install.sh <name>` (default dir `~/.local/share/platypus/plugins`,
override with `PLATYPUS_PLUGINS`), and approval pins the installed folder.
Bundling them would ship code the user never approved and rot with the
release; the curated data plugins could one day be baked in instead
(follow-up).

Trying the AppImage without touching your data:

```sh
HOME=/tmp/platypus-try ./packaging/dist/Platypus-0.0.0.AppImage --no-sandbox
# then, in another shell:
PLATYPUS_PLUGINS=/tmp/platypus-try/.local/share/platypus/plugins sh plugins/install.sh echo
```

(`--no-sandbox` only where the kernel forbids user namespaces, e.g. some
containers; a normal desktop does not need it.)

## Pieces

- `internal/app/host.go`: the `Host` interface (events, log, dialogs).
  The core (`cmd/platypus-core`) sends events as notifications and dialogs
  as requests to the shell.
- `internal/corerpc`: reflection dispatch of `*App`'s exported methods;
  `initialize {protocol}` handshake (core protocol 1.0: same major, shell minor
  ≤ core minor); the App starts while `initialize` answers.
- Art: `/steam-art/` and `/platypus-art/` are served by the core on
  `127.0.0.1:<random>`, requests need a per-run random token. Only the
  shell's main process holds it: the `app://` handler adds it and proxies. The page
  never sees the token and CSP has no `connect-src` beyond itself.
- Security: contextIsolation, sandbox, no nodeIntegration, permissions denied,
  navigation and `window.open` blocked (http/https go to `shell.openExternal`),
  IPC accepted only from `app://platypus`, only for methods the core listed,
  strict CSP (scripts from the app only; images may come from https and
  loopback). The dev CSP adds `unsafe-inline` and Vite's websocket. The core
  is spawned with explicit stdio pipes and nothing else (libuv closes every
  other fd in the child, so Chromium's sockets never reach it; plugins and
  games get only 0/1/2 in turn — Go's os/exec closes the rest, and nothing in
  internal/ passes ExtraFiles). Remote debugging exists only in development.

## What the shell does not do yet

- Types: the bridge is typed by `go run ./cmd/gen-ts` (or `bun run gen-ts`
  from the root), which reflects over the App's exported methods and the
  structs they exchange (`internal/gents`) into `apps/ui/src/gen/` (checked
  in; a Go test fails when it is stale).
- Desktop polish: the window remembers size/position/maximized
  (`window-state.json` in userData, clamped to the displays now attached), the
  window and the AppImage/`.desktop` use `packaging/icons`, a second launch
  focuses the running window (single instance), and quitting (or SIGTERM/
  SIGINT) shuts the core down first so its games and plugin helpers die with
  it. There is deliberately no app menu and no tray icon (follow-up if asked
  for); reload and DevTools keys are disabled in production.
- Packaging (electron-builder / AppImage), auto-update.
- Event delivery when no window is open (events are dropped).
- `GPU`: history — `PLATYPUS_GPU` and the DMA-BUF switch concerned only
  WebKitGTK, the previous shell. Chromium logs
  `'--ozone-platform=wayland' is not compatible with Vulkan` (harmless:
  GPU compositing and rasterization report "enabled", Vulkan off).
