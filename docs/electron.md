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
flags, e.g. `--ozone-platform=x11`), `PLATYPUS_DEVTOOLS=1` (F12, dev only —
`bun run dev` sets it; `bun run start` honours it when set in the
environment), `PLATYPUS_REMOTE_DEBUGGING_PORT` (for Playwright over CDP, dev
only, loopback). A packaged app ignores both debug variables: no remote
debugging in production.

The core alone: `go build -o platypus-core ./cmd/platypus-core` (no GUI toolkit linked).

## Headless dev on a server

No display there: `bun run dev:headless` starts Xvfb on a free display and
runs `bun run dev` on it (extra args pass through,
`PLATYPUS_DATA_DIR` and friends work as usual). `bun run dev:vnc` adds
x11vnc on that display and the noVNC web client, so the app opens from a
LAN browser: the script prints the `http://<host>.local:<port>/vnc.html`
URL (also reachable over the Tailscale address). Both pick a free display
and free ports per run (web from 6090, VNC from 5910, never 6080), log
everything to one file under /tmp (its path prints first), and stop all
they started on Ctrl-C or app exit — several worktrees can run side by
side. Overrides: `PLATYPUS_XVFB_DISPLAY`, `PLATYPUS_VNC_PORT`,
`PLATYPUS_WEB_PORT` (exact values; a busy one is refused).

One-time VNC setup: `x11vnc -storepasswd` (without `~/.vnc/passwd` the
script refuses to run unprotected).

## Sandbox dev run (`bun run dev:sandbox`, issue #68)

The real app in a throwaway sandbox, for trying library, runners and
game behaviours without touching your data (test layers #66–#67):

```sh
bun run dev:sandbox          # Xvfb, isolated HOME + PLATYPUS_* under /tmp
bun run dev:sandbox --vnc    # also x11vnc + noVNC, prints the browser URL
bun run dev:sandbox --clean  # delete the sandbox dir on exit (kept by default)
```

The sandbox (`apps/desktop/scripts/sandbox.sh`) installs echo from the
working tree, approves it and logs it in (code `letmein`), puts `testbin/`
first on PATH, registers the fake Proton build as a runner
(`compatibilitytools.d/FakeProton-Sandbox`) and seeds the library
(`ECHO_GAMES=30`, `ECHO_SEED=7`, one `ECHO_FIXTURES` game per behaviour:
run / clean exit / fail / crash). Vite listens on 5193, remote debugging
(loopback only) on a free port from 9233 — both print at startup, and the
debugging port answers `/json/list` for CDP scripts. Two sandboxes started
together claim distinct displays and ports, so both run at once. Ctrl-C or
app exit stops everything it started, by PID; the sandbox dir stays for
inspection unless `--clean` was given. Details (env knobs, CDP driving,
port claiming): `docs/testing.md`.

Ubuntu 24.04 restricts unprivileged user namespaces, so Electron under
~/dev needs an AppArmor profile with `userns`, e.g.
`/etc/apparmor.d/platypus-electron`:

```
abi <abi/4.0>,
include <tunables/global>

profile platypus-electron /home/*/dev/**/electron/dist/electron flags=(unconfined) {
  userns,
  include if exists <local/platypus-electron>
}
```

loaded with `sudo apparmor_parser -r /etc/apparmor.d/platypus-electron`.

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

## Minimize to tray while a game runs (issue #49)

Launcher setting "Minimize to tray while a game runs" (`minimize_to_tray`
in settings.json, off by default; Settings → Minimize to tray, en/fr). The
shell reads it at startup and re-reads it on every session event, so a
change applies from the next launch.

- First session start with the switch on: the window hides to a tray icon
  (`new Tray()` on `packaging/icons/appicon.png`). The menu is Show
  Platypus, one Stop row per running game, Quit. Clicking the icon or Show
  brings the window back and focuses it.
- Last session end: the tray icon is destroyed and the window shows again
  with `showInactive()` — it never steals focus (the user may be in the game
  that just exited, or in another app).
- The icon exists only while hidden, never permanently: no panel clutter, no
  stale icon, and GNOME users without a tray host never get a dead one.
- No tray host (GNOME without the AppIndicator extension, a throwing Tray
  constructor, `PLATYPUS_NO_TRAY=1`): the window minimizes instead of hiding,
  so it is never unreachable. Launching Platypus again (the single-instance
  lock) shows a hidden window too (`restoreWindow` in
  `apps/desktop/src/tray.ts`: show, then restore, then focus — `restore()`
  alone never unhides a hidden window).
- Closing the window (X) still quits, stopping games through the core's
  shutdown; only session starts hide.

## Use less while hidden or unfocused (issue #49)

- Renderer: `backgroundThrottling` stays on. While `document.hidden`
  (minimized, tray-hidden, another desktop) — or blurred while a game runs —
  the UI pauses (`usePauseWhileGameRuns` in
  `apps/ui/src/lib/pauseWhileHidden.ts`): the `platypus-paused` class freezes
  animations and stops transitions, videos pause (the Lightbox pauses its
  trailer too). The QR login poll already backs off to ≥ 15 s when hidden;
  the family-warning countdown stops its 1 s timer while hidden and resumes
  on show.
- Core: one game-running gate (`deferWhileGameRuns` in
  `internal/app/app_tray.go`): the auto artwork lookup after an add and the
  save-list manifest refresh wait for the last session to end, then run in
  order. Everything the user asks for (rescans, searches, applies, game
  pages) runs right away; library listings are already on-demand, so there
  is no rescan timer to gate.

### Measurements

Xvfb 1280×800 with software rendering (no GPU: absolute RSS is higher than
on a real desktop; the deltas are what matter), scratch HOME, the echo
plugin's Echo Tiny with `ECHO_GAME_SECONDS=300`, mean of 10 rounds 2 s
apart after ~8 s settle:

| state | Electron CPU / RSS | core CPU / RSS |
|---|---|---|
| idle, focused, before (1e174d8, fresh HOME) | 0.74% / 680 MB | 1.96% / 126 MB |
| idle, focused, after (switch off, fresh HOME) | 0.40% / 676 MB | 1.07% / 98 MB |
| idle, unfocused (xterm focused) | 0.50% / 692 MB | 0.87% / 47 MB |
| game running, window open | 0.31% / 693 MB | 0.74% / 47 MB |
| game running, hidden to tray | 0.50% / 925 MB | 0.61% / 39 MB |
| after exit, window back | 0.45% / 696 MB | 0.50% / 39 MB |

Idle is at parity with before when the switch is off; CPU stays flat in
every state. The hidden window's +230 MB sits in the browser process and is
released when the window shows again (696 MB after exit, no leak) — under
SwiftShader this looks like a software-compositing artifact; worth
re-checking on real GPU hardware, where hiding should release surfaces
instead. Core RSS differences between runs are Go scavenger age (the longer
a core has run, the lower), not workload.

## What the shell does not do yet

- Types: the bridge is typed by `go run ./cmd/gen-ts` (or `bun run gen-ts`
  from the root), which reflects over the App's exported methods and the
  structs they exchange (`internal/gents`) into `apps/ui/src/gen/` (checked
  in; a Go test fails when it is stale).
- Desktop polish: the window remembers size/position/maximized
  (`window-state.json` in userData, clamped to the displays now attached), the
  window and the AppImage/`.desktop` use `packaging/icons`, a second launch
  shows the running window even when it hides in the tray (single instance),
  and quitting (or SIGTERM/
  SIGINT) shuts the core down first so its games and plugin helpers die with
  it. There is deliberately no app menu, and the tray icon exists only while
  hidden for a running game (see above); reload and DevTools keys are disabled in production.
- Store view: each embedded store view and store popup answers F12 /
  Ctrl+Shift+I with DevTools for its own contents (detached), under the same
  conditions as the main window (development with `PLATYPUS_DEVTOOLS=1`).
- Packaging (electron-builder / AppImage), auto-update.
- Event delivery when no window is open (events are dropped).
- `GPU`: history — `PLATYPUS_GPU` and the DMA-BUF switch concerned only
  WebKitGTK, the previous shell. Chromium logs
  `'--ozone-platform=wayland' is not compatible with Vulkan` (harmless:
  GPU compositing and rasterization report "enabled", Vulkan off).
