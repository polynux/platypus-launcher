# Store plugin protocol (1.1, draft)

A store plugin is a folder in `~/.local/share/hydra/plugins/<id>/` with a
`plugin.json` and an executable. Hydra starts the executable on demand and talks
JSON-RPC 2.0 over its stdin/stdout, one JSON object per line. Anything the plugin
writes to stderr goes to `~/.local/share/hydra/logs/plugin-<id>.log`.

The protocol is still a draft: it can change until GOG and Amazon plugins work
without core changes (plan milestone 4). Working examples: `plugins/echo` (Go, a
fake store) and `plugins/epic-legendary` (Python, wraps legendary).

Plugins with no executable — fix recipes and tweak packs — use the same folder
layout with `"kind": "data"` and need no approval; see
[data-plugins.md](data-plugins.md).

## Manifest

```json
{
  "id": "epic-legendary",
  "name": "Epic Games (legendary)",
  "version": "0.1.0",
  "author": "you",
  "description": "One sentence for the Plugins view.",
  "manifestVersion": 1,
  "protocol": { "versions": ["1.0"], "transport": "jsonrpc-stdio" },
  "store": "epic",
  "exec": "bin/hydra-epic",
  "capabilities": ["library", "install", "launch", "auth", "discard"],
  "needs": [],
  "requires": [{ "bin": "legendary", "hint": "uv tool install legendary-gl" }],
  "install_dir": "epic",
  "permissions": { "network": ["*.epicgames.com"], "read": ["~/.config/legendary"], "write": ["~/.config/legendary"] }
}
```

- `id` must match the folder name; `steam` and `custom` are reserved.
- `manifestVersion` is the schema of this file (absent means 1). Hydra lists a
  manifest with a newer schema as "needs a newer Hydra" without reading the rest.
  Unknown fields in a manifest of a known schema are ignored.
- `protocol` says which protocol versions the plugin speaks and over which
  transport; see [Versioning and compatibility](#versioning-and-compatibility).
  `versions` is a list (`["1.0", "1.1"]`) or an inclusive range
  (`{ "min": "1.0", "max": "1.3" }`); `transport` defaults to `jsonrpc-stdio`.
  The legacy `"api": 1` still works and means protocol `1.0` over
  `jsonrpc-stdio`.
- `capabilities` lists what the plugin can do, as names from the
  [capability registry](#capabilities). It is the promise the user approves;
  `initialize` says what is really on (see below). `needs` lists the capabilities
  Hydra must know for the plugin to be worth running; it must be a subset of
  `capabilities`. Other unknown names are ignored with a warning.
- `exec` is relative and must stay inside the folder (symlinks included).
- `requires` lists command-line tools looked up on `PATH`. A missing one keeps the
  plugin from starting; the Plugins view shows the hint.
- `permissions` are shown to the user before approval. They aren't enforced yet
  (bubblewrap sandbox, plan milestone 8).

Hydra runs nothing until the user approves the plugin, which pins the SHA-256 of
the whole plugin folder: `plugin.json` plus every regular file under it, in
sorted path order (symlinks count by their target). `__pycache__/` and `*.pyc`
are skipped, and Hydra starts plugins with `PYTHONDONTWRITEBYTECODE=1` so an
interpreter doesn't dirty the folder. If anything else changes (the executable,
a helper module it imports, the manifest), the plugin is disabled until the user
approves it again. To update a plugin, replace the files (write a new one and
rename it over the old one); Linux refuses to overwrite a running executable
in place.

## Process

- Environment: `HYDRA_PLUGIN_API=1` (the major protocol version), and `HYDRA_PLUGIN_DATA`, a private folder
  for the plugin's own state.
- Exit when stdin closes. Hydra kills the plugin's process group a couple of
  seconds later, and also when Hydra itself dies.
- Answer requests concurrently if they can take a while: Hydra may send
  `job.cancel` while a `library.list` is running.
- A crash is fine: Hydra restarts the plugin on the next request, with an
  increasing delay, and gives up after 5 quick crashes in a row.

## Requests (Hydra → plugin)

Errors are JSON-RPC errors. Their `message` is shown to the user as is, so write it
for people. Codes:

| Code | Use |
|---|---|
| `-32001` | the user has to log in first (`library.list`, `game.installSize`, `game.install`…) |
| `-32002` | unavailable right now: a borrowed game in use, a region lock, the store's service down. Not a login problem; the `message` says why and asking again later may work. Hydra shows it as is and never asks the user to log in |
| `-32602` | missing or invalid params |
| `-32601` | unknown method: "not offered". Hydra treats it as "not implemented" for the optional methods (`game.details`, …) and remembers it for the rest of the plugin run |
| `-32000` | anything else the user should read |

Match `-32001` on the tool's real logged-out signal, not on a substring that
other errors can contain.

| Method | Params | Result |
|---|---|---|
| `initialize` | `{ api, protocols }` | `{ name, version, protocol, capabilities, features, missing }` |
| `auth.status` | | `{ loggedIn, user?, qr? }` |
| `auth.begin` | | `{ url?, instructions?, qr?, fields: [{ id, label, secret?, placeholder? }] }` |
| `auth.complete` | `{ fields: { <id>: value } }` | `{ loggedIn, user? }` |
| `auth.logout` | | `{ loggedIn }` |
| `library.list` | | `{ games: [LibraryGame] }` |
| `game.details` | `{ id }` | `GameDetails` |
| `game.installSize` | `{ id }` | `{ downloadBytes, diskBytes }` |
| `game.install` | `{ id, installDir, jobId, title? }` | `{}` once started |
| `game.update` | `{ id, installDir, jobId, title? }` | `{}` once started |
| `game.uninstall` | `{ id, jobId }` | `{}` once started |
| `job.cancel` | `{ jobId }` | `{}` |
| `job.discard` | `{ jobId, id, installDir }` | `{}` |
| `game.launchSpec` | `{ id, preview }` | `LaunchSpec` |

`qr` (on `auth.begin`, and on `auth.status` while a QR login is pending) is a QR
code as block-art text: `"\u2588"` is a dark module, two characters per module,
one row per module. The dialog draws it as square modules and redraws it when
`auth.status` returns a new one (the store refreshes the code as it expires).
The user can scan it or fill the fields. When the QR login completes,
`auth.status` turns `loggedIn`; the dialog closes by itself. Steam uses this.

Two overlapping `auth.begin` calls (a double-submit, two windows) must not break
each other: the second reuses the first's login attempt, or invalidates it with a
clear error.

`auth.begin` with a `url`: Hydra shows a button that opens it in the user's
browser (http and https only), then the fields. Without a URL it's a plain form.
Credentials stay with the plugin or the tool it wraps; Hydra never stores them.

`LibraryGame`:

```json
{
  "id": "Goose", "title": "Untitled Goose Game",
  "installed": true, "installPath": "/home/u/Games/epic/UntitledGooseGame",
  "installable": true, "note": "shown on the game page, e.g. why it can't be installed",
  "version": "1.0", "updateAvailable": false, "sizeBytes": 12582912,
  "native": false, "features": ["install", "launch"],
  "coverUrl": "https://…", "heroUrl": "https://…", "bannerUrl": "https://…"
}
```

Set `native` when the game ships a Linux build: Hydra then launches it
directly, with no Proton runner and without the Proton-only settings (DX layer,
sync mode, Proton flags), on its game page too. `native` belongs on library
entries only; a launch spec says the same through `needsWine`.

Send only the keys listed here. Hydra ignores unknown keys, but new plugins copy
what the bundled ones send, so the conformance suite (`internal/plugtest`)
rejects extras on `library.list`, `game.details` and `game.launchSpec`. To add a
key, document it here and in the suite first.

`features` narrows the capabilities on offer for this game (names from the
registry, see [Capabilities](#capabilities)). Leave it out and every capability
the plugin offers applies to the game; `[]` means none. Hydra drops names the
plugin doesn't offer at runtime.

Hydra keys plugin games as `p.<plugin id>.<game id>`. List every owned game; the
Library shows the uninstalled ones under "owned".

`GameDetails` is the game page's metadata, fetched on demand. Stores without a
metadata source just don't implement the method; Hydra then shows the library
entry's art and title only.

```json
{
  "summary": "A troublesome goose.",
  "screenshots": ["https://…"], "genres": ["Comedy"],
  "developers": ["House House"], "publishers": ["Panic"],
  "releaseDate": "2019-09-20", "heroUrl": "https://…",
  "achievements": { "total": 12, "list": [{ "id": "prank", "name": "First prank", "icon": "https://…", "globalPercent": 84.1 }] }
}
```

For an id the store doesn't know, `game.details` fails with an error; a game the
store knows but has no metadata for gets `{}`. (Steam's store answers
`success: false` for delisted games and unknown ids alike, so it returns `{}` for
both.)

Every URL must be http(s); Hydra drops others. `achievements` describes what
the store shows about the game — if the plugin can't see the user's progress,
leave it out rather than guess. Keep `list` short (a highlight, say 10) and let
`total` carry the real count; Hydra notes the truncation.

## Jobs

Hydra creates the `jobId`, picks `installDir` (the user's Games folder plus the
plugin's `install_dir`, or the user's override) and checks free space against
`game.installSize` before calling `game.install`. `title` is the game's title
as Hydra lists it; use it for your own records instead of a second lookup, and
treat it as cosmetic: never a path component. Install into `installDir`, never
anywhere else, and give each game its own subfolder of it (Steam uses
`<name>_<appid>`, GOG the store's folder name): two games must never share a
directory, and a folder recorded by an earlier version of the plugin keeps working
from its recorded path. `game.update` brings the installed game up to date in
place; it is not a fresh `game.install`. `installSize.downloadBytes` is what
`job.progress.total` counts. Report progress with notifications:

| Notification | Params |
|---|---|
| `job.progress` | `{ jobId, phase?, bytes?, total?, speed?, percent? }` (`speed` in bytes/s; send it whenever the tool reports one) |
| `job.done` | `{ jobId }` |
| `job.failed` | `{ jobId, error }` |
| `log` | `{ level, message }` (goes to the plugin's log file) |

Hydra runs one job at a time and saves the queue. If Hydra or the plugin dies
mid-install, Hydra calls `game.install` again with the same `jobId`, so installs
must be resumable or at least idempotent: a `jobId` that is already running starts
no second copy and ends with one `job.done`. A paused job is the same thing: Hydra
stops it like a cancel, then calls `game.install`/`game.update` again with the
same `jobId`, so keep everything on disk during a pause — a paused download is
resumed later from what it already fetched, never fetched again. After
`job.cancel`, stop the work (an uninstall too: don't delete anything after the
cancel) and send neither `job.done` nor `job.failed`; Hydra has already marked
the job canceled and ignores later notifications for it. State that has to
outlive the process (what is installed, where) goes under `HYDRA_PLUGIN_DATA`,
not in memory.

### Discarding a canceled job's data

A canceled install usually leaves data behind: staging chunks, a tool's resume
files, a partially written install folder. The user can discard it with an
explicit action, and Hydra does it later of its own accord after a grace
period (it doesn't run for paused jobs). Hydra sends:

```
{ "method": "job.discard", "params": { "jobId": "…", "id": "…", "installDir": "/…" } }
```

The method is optional; declare the `discard` capability only if you can
honor it. Delete exactly what the job left: the staging area of that `jobId`
and, for an install that never finished, the game's own (partial) subfolder of
`installDir` — and the tool's resume files for that game. Never delete an
installed game's folder: a canceled *update* keeps the game installed, and
`id`/`installDir` identify the game and folder Hydra started the job with, so
recheck both against what is really installed. Stay inside `installDir` and
`HYDRA_PLUGIN_DATA` (symlinks resolved); answer with `-32602` for anything
unexpected. Discarding a job Hydra has already restarted yours (or forgotten)
is not an error: answer `{}`. A discard for a still-running job means the same
as `job.cancel` on it: stop and wait for your work to wind down first.

Plugins that don't implement the method: don't list the capability. Hydra then
only forgets the job and tells the user what may remain on disk.

## Launching

Hydra owns the runner pipeline (Proton/Wine, gamemode, gamescope, MangoHud,
per-game settings) and session tracking. A plugin returns a spec; it never starts
the game itself.

Direct launch:

```json
{
  "exe": "/home/u/Games/epic/Goose/Goose.exe",
  "args": ["-AUTH_PASSWORD=…"], "cwd": "/home/u/Games/epic/Goose",
  "env": { "DXVK_HUD": "fps" }, "needsWine": true,
  "redact": ["-AUTH_PASSWORD="]
}
```

Hydra refuses the spec when:

- `exe` or `cwd` isn't inside the plugin's install folder, or inside a folder the user
  explicitly allowed for this plugin (symlinks are resolved first). System folders
  and the home folder itself can't be allowed;
- `env` sets a key on the denylist: `PATH`, `HOME`, `LD_*`, `WINE*` (prefix and DLL
  overrides are Hydra's), `PROTON_*`, `STEAM_COMPAT_*`, `VK_*`, interpreter paths and
  similar. See `internal/stores/launchspec.go`.

A direct spec has exactly these keys: `exe`, `args`, `cwd`, `env`, `needsWine` and
`redact`. Whether the game is a Linux build shows only through `needsWine`.

Arguments starting with a `redact` prefix are shown as `prefix***` in the command
preview and the launch log. With `preview: true`, return the command without
fetching tokens or checking for updates: the settings dialog asks for a preview on
every change.

Store-client launch, for games the store's own client has to start:

```json
{
  "delegate": "store-client", "client": "Steam",
  "command": ["steam", "-applaunch", "620"],
  "find": { "cmdline": ["SteamLaunch", "AppId=620"] },
  "startTimeout": 180
}
```

`command[0]` must be one of the manifest's `requires`. Hydra runs the command,
then follows the game through `find`: the game is running while some process's
`/proc/<pid>/cmdline` (arguments separated by NUL) contains every string in
`cmdline`. Each string must be at least 3 characters.

## Versioning and compatibility

The protocol has a version, `major.minor`, and so does the manifest schema
(`manifestVersion`). Both are built so adding a feature never breaks a plugin
that doesn't know it.

- **Minor** versions are additive only: new methods, optional fields,
  capabilities and error codes. A plugin written for 1.0 keeps working on a Hydra
  that speaks 1.5. **Major** versions may break. New required behaviour comes only
  with a new major, and Hydra keeps speaking the previous major for a deprecation
  period.
- **Negotiation.** The manifest declares the versions the plugin speaks
  (`protocol.versions`). `initialize` carries the versions Hydra speaks
  (`protocols: ["1.0", "1.1"]`, and the legacy `api: 1`); the plugin answers with
  `protocol`, the highest it shares with Hydra. Hydra checks that the pick is one
  it offered and the manifest declares. A plugin that predates negotiation leaves
  `protocol` out and is taken to speak the highest version the manifest and Hydra
  share. A plugin never relies on a field or method it didn't negotiate.
- **Incompatible plugins are listed, never started.** A transport Hydra doesn't
  have, a `manifestVersion` newer than Hydra reads, or no common protocol version
  show up in the Plugins view as "needs a newer Hydra" (or "too old for this
  Hydra", when the plugin only speaks versions older than any Hydra still
  supports). Nothing is spawned and nothing counts as a crash.
- **Ignore what you don't know, both ways.** Unknown fields in manifests, params,
  results and notifications are ignored. An unknown method is answered `-32601`
  (Hydra probes optional methods once per plugin run and remembers the answer).
  Unknown capability names are ignored with a warning; use `needs` for the ones
  that are hard requirements.
- **Transport.** Today the only transport is `jsonrpc-stdio` (this document). It
  is an interface in `internal/plugins`: a future transport is a new
  implementation selected by `protocol.transport`, with the same methods,
  capabilities and error codes on top.

## Capabilities

A capability is a name in the manifest that stands for a group of methods and
the UI that depends on it. Hydra keeps them in a registry (`internal/plugins`,
`capabilities.go`); the conformance suite and the gate that sits in front of the
plugin read the same table. Names are lowercase words joined by `-`.

| Capability | Since | Methods | Optional methods | In use |
|---|---|---|---|---|
| `library` | 1.0 | `library.list` | `game.details` | yes |
| `install` | 1.0 | `game.installSize`, `game.install`, `game.update`, `game.uninstall`, `job.cancel` | | yes |
| `launch` | 1.0 | `game.launchSpec` | | yes |
| `auth` | 1.0 | `auth.status`, `auth.begin`, `auth.complete`, `auth.logout` | | yes |
| `discard` | 1.0 | `job.discard` | | yes |
| `cloud-saves` | 1.0 | `cloud.status`, `cloud.pull`, `cloud.push`, `cloud.resolve` | | not yet |
| `install-options` | 1.1 | `game.options` | | not yet |
| `versions` | 1.1 | `versions.list` | `versions.unlock` | not yet |
| `verify` | 1.1 | `game.verify` | | not yet |
| `move` | 1.1 | `game.move` | | not yet |
| `social` | 1.1 | `social.start`, `social.stop`, `social.state`, `social.self`, `social.friends` | `social.setStatus`, `social.setPlaying` | not yet |
| `achievements` | 1.1 | `achievements.list` | | not yet |
| `playtime` | 1.1 | `playtime.list` | | not yet |
| `redeem` | 1.1 | `redeem.submit` | | not yet |
| `family` | 1.1 | `family.members` | | not yet |

"Not yet" means Hydra recognises the name (no warning, usable in `needs`) but
has no code that calls it. The shapes of the 1.1 methods are proposals in
`docs/plans/launcher-parity.md` §2 and become fixed here when each one gets its
orchestrator; a plugin that declares one must answer it with `-32602` for
missing params before acting on anything, and must not answer `-32601` for a
required method.

**The manifest is the promise, `initialize` is the reality.** `initialize`
returns `capabilities`, the subset of the manifest's that is on right now (leave
it out to mean all of them), and `features`, depth flags per capability:

```json
{ "protocol": "1.1",
  "capabilities": ["library", "install", "versions"],
  "features": { "versions": { "channels": true, "builds": true, "password": false } } }
```

A tool that went missing or a logged-out state switches a capability or a
feature off here without touching the manifest (whose hash the user approved).
Listing a capability the manifest doesn't declare is a protocol error: Hydra
ignores it and notes it in the plugin's log, and the conformance suite fails.
Hydra shows and gates on this runtime set: it never sends a method of a
capability that is off (the call fails with `-32601` on Hydra's side), and the
Plugins view lists every declared capability, on or off, with its features.

**Per game.** `LibraryGame.features` narrows the capabilities for one game (a
Versions control must not appear for a game with a single build). Absent means
all the plugin offers.

**Surfaces.** Every UI surface that depends on a capability (the registry names
them) renders only when the plugin offers it and, if the game lists `features`,
the game does too. When something is declined or unavailable, the reason is text
the plugin wrote (as `CloudStatus.message` will be for cloud saves); Hydra never
invents one.

**Optional methods.** The first `-32601` for an optional method is remembered
until the plugin process exits; Hydra doesn't ask again in that run. `-32002` is
never remembered.

## Out of scope

Plugins may only install and launch what the user's own account is entitled to,
through the store's own APIs. No DRM or licence circumvention, and no Steam API
emulators.
