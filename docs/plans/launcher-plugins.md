# Plan: store plugins and launcher-free installs

Status: proposal, nothing implemented. Branch `plan/launcher-plugins`.

## Goal

1. Anyone can add support for a store or launcher without touching Hydra's core.
2. Hydra can install, update and uninstall games the user owns **without the store's
   official launcher**, where the store allows it through its own APIs and the user's
   own login.

Non-goal: bypassing DRM, licence checks or ownership. A plugin downloads what the
user's account is entitled to, nothing else. Games with launcher-bound DRM (Denuvo,
Ubisoft Connect, EA app) still need their launcher at runtime; the plugin can say so.

## Where we are

- Steam is hard-wired (`internal/steam`), scanned from local manifests, launched via
  the Steam client.
- `internal/plugins` is a static in-memory list; toggles do nothing (PRODUCT.md).
- No install, no auth, no download code anywhere.

## Prior art

Heroic Games Launcher already does launcher-free installs on Linux by driving
open-source CLIs, which proves the approach and gives us ready-made backends:

| Store  | CLI                     | Auth                    | Install without launcher |
|--------|-------------------------|-------------------------|--------------------------|
| Epic   | legendary               | browser login → code    | yes                      |
| GOG    | gogdl (Heroic)          | browser login → code    | yes (DRM-free)           |
| Amazon | nile                    | browser login           | yes                      |
| Steam  | SteamCMD / DepotDownloader | Steam login + Steam Guard | yes, but the game still needs the Steam client running for Steamworks |
| Itch   | butler                  | API key                 | yes                      |

## Design: out-of-process plugins over JSON-RPC

A plugin is a directory with a manifest and an executable. Hydra starts it and talks
JSON-RPC 2.0 over stdin/stdout (one JSON object per line), the same model as LSP.

```
~/.local/share/hydra/plugins/epic-legendary/
  plugin.json
  bin/hydra-epic          # any language
```

```json
{
  "id": "epic-legendary",
  "name": "Epic Games (legendary)",
  "version": "0.1.0",
  "api": 1,
  "store": "epic",
  "exec": "bin/hydra-epic",
  "capabilities": ["library", "install", "launch", "auth", "cloud-saves"],
  "requires": [{ "bin": "legendary", "hint": "pipx install legendary-gl" }]
}
```

Why this over the alternatives:

- **Go `plugin` package**: Linux-only, must be built with the exact same Go toolchain
  and dependency versions as Hydra. Every Hydra release breaks every plugin. Rejected.
- **Embedded JS/Lua (goja, gopher-lua)**: sandboxable, but plugins mostly wrap
  existing CLIs and need processes, files and network anyway, so the sandbox buys
  little and the API surface we'd have to expose is large.
- **WASM (wazero)**: best isolation, any language, but no process spawning, so it
  can't wrap legendary/gogdl. Revisit for pure-HTTP plugins later.
- **Subprocess + JSON-RPC**: any language, crash-isolated (a broken plugin can't take
  Hydra down), trivially wraps existing CLIs, testable from a shell. Cost: we define
  and version a protocol. Chosen.

Steam stays built in, but is refactored behind the same Go interface so the core has
one code path for all stores.

### Protocol (api 1)

Hydra → plugin requests:

| Method               | Returns                                                     |
|----------------------|-------------------------------------------------------------|
| `initialize`         | plugin info, capabilities, missing requirements             |
| `auth.status`        | `{ loggedIn, user }`                                        |
| `auth.begin`         | `{ url }` to open in the browser, or `{ fields }` for a form |
| `auth.complete`      | given the code/fields, finishes login                       |
| `library.list`       | owned games: id, title, art URLs, installed state, size     |
| `game.install`       | starts a job: `{ jobId }`                                   |
| `game.uninstall`     | starts a job                                                |
| `game.update`        | starts a job                                                |
| `game.launchSpec`    | `{ exe, args, cwd, env, needsWine, runnerHint }`, or `{ delegate: "store-client" }` |
| `job.cancel`         | cancels an install or update                                |

Plugin → Hydra notifications: `job.progress` (bytes, total, speed, phase),
`job.done`, `job.failed`, `log`.

Hydra owns the runner pipeline. A plugin returns a launch *spec* and never executes
the game itself, so every store gets the same Proton/Wine handling, wrappers, tweaks
and session tracking (`internal/session`).

### Core changes

- `internal/stores`: a `Store` interface (`Library`, `Install`, `LaunchSpec`, …) with
  two implementations: built-in Steam, and an `rpcStore` adapter around a plugin process.
- `internal/plugins`: discovery (scan the plugins dir, validate manifests), process
  lifecycle (start on demand, restart with backoff, kill on shutdown), the JSON-RPC
  client, and `requires` checks.
- Jobs: a queue with persisted state so a crash or restart can resume an install
  (`~/.local/share/hydra/jobs.json`), plus a disk-space check before starting.
- Persistence: library cache and per-game settings move from session-only memory to
  `~/.local/share/hydra/` (needed anyway: settings are lost on restart today).
- Credentials: never stored by Hydra. Each plugin keeps tokens where its CLI already
  does (legendary: `~/.config/legendary`). Hydra only sees `auth.status`.

### UI changes

- Plugins view: installed plugins with real status (running, missing requirement,
  logged out), login button, enable/disable that actually takes effect, "open
  plugins folder".
- Library: owned but not installed games, shown as a filter ("installed / owned").
- Game page: Install button with size and target folder, then progress.
- A Downloads view (the design's sidebar has room) with the queue, speed, pause/cancel.

## Milestones

1. **Plumbing**: `Store` interface, Steam behind it, persisted settings, plugin
   discovery and JSON-RPC client, plus a tiny `echo` test plugin. No user-visible change.
2. **Epic via legendary**: auth, owned library, launch of games already installed by
   legendary. First real third-party plugin.
3. **Installs**: job queue, progress events, Install/Uninstall UI, Downloads view.
4. **GOG via gogdl**, then Amazon via nile: should need no core changes. If they do,
   fix the protocol before calling it stable.
5. **Plugin authoring**: protocol reference, `hydra-plugin` scaffold, and a conformance
   test that runs a plugin through every method.
6. Optional: Steam installs via DepotDownloader.

## Open decisions

1. **Bundle or require the CLIs?** Recommendation: require them (`requires` with an
   install hint) for v1, and bundle later once we have a release pipeline. Bundling
   Python tools (legendary) means shipping a Python runtime.
2. **Plugin trust.** A plugin is an arbitrary executable. Recommendation: install only
   from a local folder or a curated index for now, show the manifest and requirements
   before enabling, no auto-update.
3. **Steam installs.** DepotDownloader works, but Steam-only games still need the client
   running to start. Worth it only for people without Steam installed. Recommendation:
   last milestone, optional.
4. **Where games install.** Recommendation: one library root per plugin, default
   `~/Games/<store>/`, configurable in Settings.
