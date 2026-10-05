# Plan: store plugins and launcher-free installs

Status: decisions taken (see end). Milestones 1 (plumbing, `feat/plugins-plumbing`),
2 (store plugins end to end, `feat/plugins-e2e`) and 3 (data plugins,
`feat/plugins-data`) implemented. The protocol as built is in
[docs/plugins/protocol.md](../plugins/protocol.md); "Changes during implementation" below
lists where it differs from this plan.

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
  "requires": [{ "bin": "legendary", "hint": "pipx install legendary-gl" }],
  "install_dir": "epic",
  "permissions": {
    "network": ["*.epicgames.com", "*.epicgames.dev"],
    "read": ["~/.config/legendary"],
    "write": ["~/.config/legendary"]
  }
}
```

`install_dir` is optional: relative to Hydra's base games folder, or absolute if the
user overrides it per plugin (see Install folders).

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

As first planned; the reference is now [docs/plugins/protocol.md](../plugins/protocol.md).

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

- `internal/stores`: a `Store` interface (`Library`, `Details`, `Launch`) with two
  implementations: built-in Steam, and `stores.Plugin` around a plugin process. Installs
  go through `internal/jobs`, not the interface.
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

## Plugin kinds and what third parties can do

Two kinds of plugin, because most useful extensions don't need code at all:

**Data plugins** (no executable): fix recipes, tweak packs, runner download sources,
art sources. A JSON file validated against a schema. They can't run anything, so they
are safe to share freely and need no trust prompt. Hydra applies them through the
same paths as its built-in fixes and tweaks.

**Store plugins** (executable, the protocol above): library, auth, install, launch
spec. Third-party store plugins get exactly the same protocol as first-party ones;
an ecosystem where outsiders get a weaker API isn't worth building.

### Realistic security scope for third-party store plugins

A store plugin is arbitrary code running as the user. We can't make that fully safe,
and pretending otherwise is worse than saying so. What we can enforce, in order of
cost:

1. **Protocol boundary** (v1, cheap). Plugins never execute anything through Hydra:
   they return a launch *spec*, and Hydra checks it. The executable and working dir
   must be inside that plugin's install folder, env keys are filtered against a
   denylist (`LD_PRELOAD`, `LD_LIBRARY_PATH`, `PATH`, …), and the spec is shown in the
   launch command preview. Plugins can't see other plugins' data or settings.
2. **Explicit install, no surprises** (v1). Only from a local folder or a git URL the
   user pastes. Before enabling, Hydra shows the manifest: what it runs, declared
   network hosts and paths, required tools. The executable's hash is pinned; if it
   changes, the plugin is disabled until the user re-approves. No auto-update.
3. **Sandbox with bubblewrap** (v2, Linux-native, what Flatpak uses). Run the plugin
   under `bwrap` with the filesystem reduced to: its own folder, its install folder,
   the paths it declared in `permissions.read`/`write`, and a private `/tmp`. Declared
   permissions become enforced permissions. Network stays allowed: bwrap can only
   cut it entirely, and every store plugin needs it. Host filtering would need a
   proxy, so it isn't planned.
4. **Not in scope**: auditing plugin code, a signed central store, sandboxing the
   *games* plugins download. Games run with the same trust as anything the user
   installs, same as Heroic or Lutris.

Plugins also can't inject UI. Hydra renders everything from protocol data (library
entries, auth forms, progress), so a plugin can't spoof Hydra's own dialogs.

## Install folders

- Hydra setting **Games folder**, default `~/Games`, configurable in the launcher
  settings.
- Each store plugin installs into `<Games folder>/<install_dir>` (`install_dir` from
  the manifest, defaulting to the plugin id), e.g. `~/Games/epic/`.
- The user can override the folder per plugin in the Plugins view, including an
  absolute path on another disk. Hydra passes the resolved folder to the plugin in
  `game.install`; plugins never pick paths themselves. That keeps the launch-spec
  check from point 1 simple: everything must live under the folder Hydra handed out.
- Before a job starts, Hydra checks free space against the size the plugin reports.

## Steam without the Steam client: a separate plugin

Built-in Steam support stays as it is: scan the client's libraries, launch through the
client. A separate plugin, **steam-standalone**, handles installing and launching
without the Steam client UI.

**Install / update**: DepotDownloader (needs `dotnet`, required, not bundled). Logs in
with the user's account and Steam Guard, lists owned apps, downloads their depots
into `<Games folder>/steam/`. Updates are a diff against the installed manifest.

**Launch** comes in three cases, detected per game:

1. **No Steamworks dependency** (no `steam_api*.dll` / `libsteam_api.so`): launch
   directly with Hydra's runner pipeline. Fully launcher-free.
2. **Uses Steamworks, works without a client**: many games only call
   `SteamAPI_Init` for achievements or overlay and continue if it fails. We launch
   directly and set `SteamAppId`/`SteamGameId` so the game doesn't relaunch itself
   through Steam.
3. **Requires the client** (`SteamAPI_Init` failure is fatal, or Steam's DRM wrapper):
   Hydra starts the Steam client hidden (`steam -silent`) and launches the game
   *outside* Steam's library, with the same env. The game talks to the running client
   for licence and achievements, so the user never sees Steam, but it's running.

Detecting 2 vs 3 can't be done reliably by inspection. The plugin tries direct, and if
the game exits within a few seconds with the Steamworks init error in its log, it
marks the game "needs client" and uses mode 3 from then on. Community data plugins
can pre-seed that flag.

Out of scope: Steam API emulators (replacing `steam_api.dll` with a stub such as
Goldberg). That circumvents Steam's licence check, the same category as DRM bypass,
and isn't something we'll ship or document even for owned games.

Why a separate plugin: it needs its own auth flow (Steam Guard), its own library
source (owned, not just installed), a download engine and per-game launch-mode state.
None of that belongs in the scanner, and users happy with the Steam client never need
it.

## Milestones

Reordered after milestone 1: plugins had no visible effect yet, so the whole store
plugin path (old milestones 3 and 4) came before data plugins.

1. **Plumbing** (done): `Store` interface with Steam behind it, persisted settings,
   Games folder setting, plugin discovery, manifest validation and hash pinning,
   JSON-RPC client, a tiny `echo` test plugin.
2. **Store plugins end to end** (done): Plugins view with approval (manifest shown
   first), enable/disable, login; plugin libraries merged into Library and Dashboard
   with an installed/owned filter; launch-spec validation and session tracking for
   plugin games; job queue with progress, cancel, free-space check and resume;
   Install/Uninstall/Update on the game page; Downloads view; Epic via legendary as
   the first real plugin.
3. **Data plugins** (done, `feat/plugins-data`): schema, loader, and moving the
   built-in curated fixes and tweaks to that format (`internal/dataplug/builtin`,
   embedded and read through the same loader). Validates the trust-free path.
4. **GOG via gogdl** (done, `feat/plugins-gog`), then Amazon via nile. They should
   need no core changes; if they do, fix the protocol before calling api 1 stable.
5. **Plugin authoring**: the protocol reference exists; add a `hydra-plugin` scaffold
   and a conformance test.
6. **steam-standalone**: DepotDownloader auth and installs, then the three launch modes.
7. **bubblewrap sandbox** for store plugins, enforcing declared permissions.

## Changes during implementation

- **GOG (milestone 4).** `gog-gogdl` wraps Heroic's gogdl the way
  `epic-legendary` wraps legendary; no core changes were needed, and the
  protocol additions of the gaps commit (`game.details`, `native`) are enough:
  GOG Linux installers report `native` and launch their start script without
  Proton. The plugin keeps its own installed.json in `HYDRA_PLUGIN_DATA`
  because gogdl has no "list installed" command. Tested against a fake gogdl
  only (testdata/gogdl), like epic-legendary.
- **Data plugins (milestone 3).** A manifest with `"kind": "data"` and no `exec`
  declares JSON-only content (`docs/plugins/data-plugins.md`); discovery marks it
  ready once the files validate, with no approval. The built-in curated fixes
  (`fixes.Service.GetCuratedFix`) and hardware tweaks (`fixes.SuggestTweaks`)
  moved verbatim into embedded data plugin files (`internal/dataplug/builtin`)
  and are applied through the same registry as user-installed data plugins.
  Built-in behavior is unchanged, checked by a test that pins order, titles and
  env vars; first-match-wins keeps plugins from overriding built-in recipes or
  tweak ids.

- **Small gaps left by milestone 2** (fixed on `feat/plugins-gaps`):
  - `game.details` gives plugin games real game-page metadata (summary,
    screenshots, genres, developers, publishers, release date, a better hero
    image, and store-level achievements when the plugin has them). Plugins
    without metadata just don't implement it; Hydra then shows the library
    entry only, as before.
  - `library.list` takes a `native` flag: a plugin game with a Linux build
    launches without the Proton pipeline, and its game page and settings show
    no runner, DX layer, sync mode or Proton flags.
  - Games added by hand (sideloaded executables) persist in settings.json
    (`AddCustomGame`/`RemoveCustomGame`) instead of living for one session.

- **Games are keyed by source, not store type.** `Game.Source` is `steam`, `custom` or a
  plugin id; `Game.Store` is only the storefront for display. Plugin game ids are
  `p.<plugin>.<game>`. This leaves room for steam-standalone next to built-in Steam.
- **Store-client launches say how to find the game.** `game.launchSpec` with
  `delegate: "store-client"` must return `command` (a tool from `requires`) and
  `find.cmdline`, the strings that identify the game's process. Without that,
  sessions can't be tracked. Steam itself uses the same mechanism internally.
- **Hydra allocates job ids** and passes them to `game.install`/`update`/`uninstall`,
  so a job keeps its id across plugin restarts and resumes. Added
  `game.installSize` for the free-space check, and `auth.logout`.
- **Auth prompts are uniform**: `auth.begin` returns an optional URL and always a list
  of fields; the URL flow is "open the page, paste the code".
- **Launch previews**: `game.launchSpec` takes `preview`, so the settings dialog's live
  command preview doesn't fetch tokens; specs can list `redact` prefixes so tokens
  stay out of previews and launch logs.
- **Games installed before Hydra**: legendary may already have games outside Hydra's
  folder. The launch check refuses them until the user allows that game's folder
  (system folders and the home folder itself can't be allowed).
- **Plugins get `HYDRA_PLUGIN_DATA`**, a private folder for their own state.
- **Crash handling**: the plugin restarts lazily (next call, with backoff), but a
  crash during a job requeues the job at once, which restarts the plugin.
- **The placeholder plugin catalog is gone.** The Plugins view shows discovered
  plugins and labels Steam and ProtonDB as built in.
- **Updating a plugin means replacing the file** (rename over it). Overwriting a
  running executable fails on Linux, and the new hash needs approval anyway.

## Decisions

1. **CLIs are required, not bundled.** Manifests list them in `requires` with an install
   hint; the Plugins view shows what's missing and the command to install it.
2. **Third-party scope**: same protocol as first party; data plugins need no trust;
   store plugins get explicit install, hash pinning and launch-spec validation in v1,
   and a bubblewrap sandbox in v2 (see above).
3. **Steam without the client**: a separate `steam-standalone` plugin with
   DepotDownloader and three launch modes, last in line because it's the most involved.
4. **Install folders**: a base Games folder in Hydra's settings, a per-plugin subfolder
   by default, and a per-plugin override.
