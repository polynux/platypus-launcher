# Store plugin protocol (1.5, draft)

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
  (`{ "min": "1.0", "max": "1.3" }`; a range without `max` covers the later minors of
  its own major only, never the next major); `transport` defaults to `jsonrpc-stdio`.
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
| `game.installSize` | `{ id, target?, options? }` | `{ downloadBytes, diskBytes }` |
| `game.install` | `{ id, installDir, jobId, title?, target?, options? }` | `{}` once started |
| `game.update` | `{ id, installDir, jobId, title?, target?, options? }` | `{}` once started |
| `game.uninstall` | `{ id, jobId }` | `{}` once started |
| `job.cancel` | `{ jobId }` | `{}` |
| `job.discard` | `{ jobId, id, installDir }` | `{}` |
| `game.launchSpec` | `{ id, preview, via? }` | `LaunchSpec` |
| `cloud.status` | `{ id, ctx }` | `CloudStatus` (1.2, [Cloud saves](#cloud-saves-12)) |
| `cloud.pull` | `{ id, ctx, jobId }` | `CloudStatus` |
| `cloud.push` | `{ id, ctx, jobId }` | `CloudStatus` |
| `cloud.resolve` | `{ id, ctx, jobId, choices }` | `CloudStatus` |
| `versions.list` | `{ id, ledger?, pinned? }` | `VersionList` (1.3, `versions` capability) |
| `versions.unlock` | `{ id, channel, password }` | `{ unlocked }` (optional) |

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
| `job.done` | `{ jobId, installed? }` (`installed` only with the `versions` capability, see [Versions](#versions)) |
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

`via` (optional, 1.2) asks for a particular launch mode. Hydra sends `"via": "client"`
only when the plugin's `CloudStatus.client.canLaunch` said it can: the user chose
"Launch through <client>" because the store's client is running. Answer with a
store-client spec for the game; a plugin that has none answers `-32000` with a message.

## Cloud saves (1.2)

Capability `cloud-saves`, protocol 1.2 and later. A plugin that speaks only 1.0 or 1.1
keeps working unchanged: the capability is off for it, Hydra sends no `cloud.*`
request and the game page shows no cloud row.

**Who does what.** Hydra decides *when* and what the user sees: a per-plugin
default and a per-game override (`off`, `ask`, `auto`), a `cloud.pull` before it
launches a game itself, a `cloud.push` after the game exits, the conflict dialog
and the toasts. The plugin decides *how*: which files, how they compare, how
they move. Hydra never calls these for a game the store's own client launches
(the client syncs it) or for a game whose `features` leave out `cloud-saves`.

`ctx` (`CloudContext`) describes the launch the call is for; the plugin never
learns Hydra's prefix layout:

```json
{ "os": "linux", "home": "/home/u", "xdgData": "/home/u/.local/share", "xdgConfig": "/home/u/.config",
  "installDir": "/home/u/Games/echo/tiny",
  "prefixDir": "/home/u/.local/share/hydra/prefixes/p.echo.tiny/pfx", "winUser": "steamuser" }
```

`os` is `linux` for a native build and `windows` for a game run through Wine or Proton;
`prefixDir` and `winUser` are set only for `windows` (`prefixDir` is the folder that holds
`drive_c`, `winUser` the Windows user name inside it).

| Method | Params | Does |
|---|---|---|
| `cloud.status` | `{ id, ctx }` | Compares both sides. Read-only: no file written, nothing uploaded |
| `cloud.pull` | `{ id, ctx, jobId }` | Downloads what changed **only** in the cloud |
| `cloud.push` | `{ id, ctx, jobId }` | Uploads what changed **only** on this device |
| `cloud.resolve` | `{ id, ctx, jobId, choices: { "<path>": "local" \| "remote" } }` | Settles the listed conflicts: `remote` backs up the local file and takes the cloud copy, `local` replaces the cloud copy |

All four answer a `CloudStatus` that describes the state **after** the call:

```json
{ "state": "conflict", "message": "shown to the user",
  "counts": { "same": 3, "download": 0, "upload": 1, "conflict": 1, "unmapped": 0 },
  "conflicts": [ { "path": "Profile/save.dat",
      "local":  { "size": 3705, "mtime": 1791398361 },
      "remote": { "size": 3718, "mtime": 1734286632, "machine": "Deck" } } ],
  "checkedAt": 1791458728, "backupDir": "/home/u/.local/share/hydra/plugin-data/echo/cloud-backups/tiny",
  "stamp": "a1b2c3", "client": { "running": true, "name": "Steam", "canLaunch": true } }
```

- `state` is one of `synced`, `pending` (something to move, no conflict), `conflict`, `unsupported`,
  `offline` (the store could not be reached) and `error`. Hydra turns any other value into `error`.
  Expected failures are answers with `offline` or `error` and a `message`, not JSON-RPC errors; a JSON-RPC
  error on these methods is shown as `error` with its message. `-32602` is for missing `id`, `ctx` or `jobId`.
- `message` is written for people and shown as is. For `unsupported` it is the reason ("no Auto-Cloud rules",
  "encrypted cloud files", "saves go through the Steamworks API"); Hydra never invents one.
- `counts` tallies files; `unmapped` counts cloud files the plugin could not place on this device.
- `conflicts` lists each file changed on both sides, or with no record of a common ancestor. Hydra never
  chooses for the user and never sends a `choices` path the last status did not list.
- `checkedAt` is when the comparison ran (unix seconds). `backupDir` is where replaced local files were
  copied; keeping the last few backups per game is the plugin's job, Hydra only shows the folder.
- `stamp` (optional) is an opaque fingerprint of the local save files. After a game exits Hydra reads
  `cloud.status` twice, 2 s apart, and pushes only when the two readings agree (the game may still be
  writing); without a `stamp` it compares `counts` and `conflicts`.
- `client` (optional) is the **official client policy**, the same for every store. `running` says the
  store's own client is up; `name` is how to call it ("Steam"); `canLaunch` says the plugin can start the game
  through it (`game.launchSpec` with `"via": "client"`). While `running` is true Hydra calls neither `cloud.pull`
  nor `cloud.push` for that game, because two writers on one save folder produce exactly the conflicts the
  sync exists to avoid. It offers "Launch through <name>" when `canLaunch` is true, and otherwise explains
  on the game page. Report `client` on every answer.

Rules for the plugin: never overwrite a local file that differs from the cloud and from the last state both
sides agreed on (that is a `conflict`), never delete anything in the cloud, back up any local file a download
replaces, keep the baseline under `HYDRA_PLUGIN_DATA`, and never log paths or tokens.

**Jobs.** `cloud.pull`, `cloud.push` and `cloud.resolve` carry a `jobId` Hydra made up. They obey `job.cancel`
(the request then ends with `-32000` "canceled"; Hydra has already given up on the answer) and are idempotent
for a repeated `jobId`: the second request returns the first one's result without redoing the work. Hydra
repeats a `jobId` when a push was interrupted by Hydra quitting and runs at the next start. A plugin that offers
`cloud-saves` receives `job.cancel` even when it does not offer `install`. Progress is a notification:

| Notification | Params |
|---|---|
| `cloud.progress` | `{ id, jobId, phase, done, total }` (`phase` is `check`, `download` or `upload`; `done` and `total` count files) |

**When Hydra calls what.** Before it launches a game directly: `cloud.status`; if the client runs it stops
there (see above); on `conflict` the launch waits for the user; on `offline` or `error` the user confirms
"launch anyway" (`auto` skips the question once the user confirmed it for that game); a status with files to
download is followed by `cloud.pull`. After a direct launch exits: two `cloud.status` readings, then
`cloud.push` if there is something to upload. An `unsupported` game is launched as is. The conflict dialog
calls `cloud.resolve`, and "Check now" calls `cloud.status`.

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
  (`protocols: ["1.0", "1.1", "1.2", "1.3", "1.4", "1.5"]`, and the legacy `api: 1`); the plugin answers with
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
- **Revisions.** 1.0: the first plugin releases. 1.1: the capability registry, runtime `capabilities` and
  `features` in `initialize`, `-32002`. 1.2: the `cloud.*` methods, `cloud.progress` and the `via` parameter of
  `game.launchSpec` (cloud saves). 1.3: the `versions.*` methods, `target` on installs and updates, and
  `installed` on `job.done` (versions). 1.5: `game.options`, and `options` on installs, updates and
  `game.installSize` (install options and DLC). A capability whose `since` is newer than the negotiated protocol is off
  for that plugin (Hydra drops it from the runtime set and logs it), so a plugin that only agreed to 1.1 never
  receives a 1.2 or 1.3 method.
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
| `cloud-saves` | 1.2 | `cloud.status`, `cloud.pull`, `cloud.push`, `cloud.resolve` | | yes |
| `install-options` | 1.5 | `game.options` | | yes |
| `versions` | 1.3 | `versions.list` | `versions.unlock` | yes |
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
{ "protocol": "1.3",
  "capabilities": ["library", "install", "versions"],
  "features": { "versions": { "channels": true, "builds": true, "password": false } } }
```

A tool that went missing or a logged-out state switches a capability or a
feature off here without touching the manifest (whose hash the user approved).
A capability that a later revision introduced than the one negotiated stays off
too: a plugin that settled on 1.1 offers no `cloud-saves` (1.2), whatever its
manifest lists, and is never sent `cloud.*`.
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

## Versions

The `versions` capability (protocol 1.3) lets the user choose which version of
a game to install, hold it there, and go back to one they had. It is for stores
where that is possible: GOG's builds and branches, Steam's branches and
manifest ids, Epic's previously saved manifests. A store with one build leaves
it out, or leaves it off for a game through `LibraryGame.features`.

```
versions.list   { id, ledger?, pinned? } -> VersionList
versions.unlock { id, channel, password } -> { unlocked }    # optional
game.installSize / game.install / game.update { …, target? }
job.done { jobId, installed? }
```

`channel` is a moving pointer (a Steam or GOG branch); `build` is immutable.
The `features` flags for the capability are `channels`, `builds`, `password` and
`manual`: what this store can list, unlock and take typed. They only say what
the Plugins view shows; Hydra still handles any empty list.

```json
{
  "channels": [{ "id": "beta", "name": "beta", "description": "Release candidates.",
                 "updatedAt": "2026-09-30", "buildId": "25240193",
                 "requires": "none", "unlocked": true, "current": false }],
  "builds":   [{ "id": "58076094395251196", "name": "1.5 (CS) GOG 0.2", "publishedAt": "2024-11-05",
                 "channel": "beta", "source": "store", "available": true, "current": false }],
  "manual":   { "label": "Steam manifest ids", "hint": "one per depot, from SteamDB" },
  "links":    [{ "label": "Depots on SteamDB", "url": "https://steamdb.info/app/480/depots/" }],
  "updateAvailable": false,
  "pinned":   { "kind": "build", "id": "58076094395251196" }
}
```

- `channels[].requires` is `"none"` or `"password"`; `unlocked` says whether the
  password was accepted (Hydra shows a password form for the locked ones).
  `current` marks the channel the installed game follows. Both are optional.
- `builds[].source` is `"store"` (the store lists it), `"archive"` (it comes
  from Hydra's install history, `ledger`) or `"manual"`. Hydra marks them in the
  UI as "from the store", "from your install history" and "typed by you".
  `available: false` means the plugin knows the build but can't fetch it now;
  leave it out for `true`. `current` marks the installed build.
- `manual` asks the user to type something the store doesn't list (Steam: one
  manifest id per depot). `label` and `hint` are shown as written next to a
  text box (several lines allowed). Hydra's UI sends the text as it was typed,
  in `target.manual.input`; parse it as your hint says. Other keys are allowed
  for callers that build a target themselves.
- `links` (optional, at most 10) point at a page that helps the user pick, such
  as a game's depots on SteamDB. Hydra only opens them in the browser (http and
  https only) and never fetches them.
- `updateAvailable` is relative to the pin: with a `pinned` in the request, whether
  the installed version differs from it; without one, whether a newer build
  exists. `pinned` in the answer is ignored: Hydra's own pin is authoritative.
- `ledger` (request) is Hydra's install history for the game, oldest first:
  `[{ store, game, build, channel?, versionName?, date, archiveRef? }]`. List the
  entries your store no longer lists as `source: "archive"` and say whether they
  can still be fetched. `archiveRef` is whatever you put in `installed`; Hydra
  never reads it (Epic: the saved manifest's name; Steam: depot → manifest ids).

**Target.** `game.install` and `game.update` carry `target` when the user (or a
pin) picked a version: `{ channel?, build?, manual? }`. A `channel` alone means
"that branch's current build"; a `build` is exactly that build (with a
`channel`, the build within it); `manual` is the typed values (`{ "input": "<text>" }` from Hydra's UI). Without `target`,
install the store's default. A pin makes Hydra send the same `target` on every
`game.update`, so `update` may be a downgrade. `game.installSize` takes the
same `target` so the free-space check is for that version. A target for a
locked channel is refused (`-32000` with a message the user can act on). Hydra
resends the same `target` with the same `jobId` when it resumes a job.

**Passwords.** `versions.unlock` validates a password and the plugin keeps it
in `HYDRA_PLUGIN_DATA`; answer `{ "unlocked": false }` (or `-32000` with a
message) for a wrong one. Hydra holds a password only for the length of that
call: it is never written to a job, the settings or the install history, never
logged, and a target never names one. A resumed job re-uses the stored unlock.
Don't write the password to your log either. Don't implement the method when the
store has no passwords: it is optional and the first `-32601` is remembered.

**Ledger.** After a `job.done` that carries `installed`, Hydra appends
`{ store, game, build, channel, versionName, date, archiveRef }` to the game's
history (`install-history.json` in Hydra's data folder), so installing the same
build twice in a row refreshes one entry rather than adding two:

```json
{ "jobId": "…", "installed": { "build": "58076094395251196", "channel": "beta",
  "versionName": "1.5", "archiveRef": { "depots": { "481": "7845…" } } } }
```

Send `installed` for every install and update when the capability is on;
`build` is required, the rest optional. `archiveRef` is any JSON, at most 64 KiB.

**Pinning.** Hydra stores the pin per game (a `build` or a `channel`), passes it
as `target` on `game.update` (and on a reinstall), and hides the game's update
prompt while it is set. It does not ask you to know about pins, except through
`versions.list`'s `pinned` request field so you can compute `updateAvailable`.
"Return to latest" clears the pin; the next update installs the default.

**Safety.** Before installing a version other than the store's default, the UI
warns that cloud saves or online play may not match, and asks to confirm. Hydra
backs up nothing itself.

**Errors.** `-32602` for a missing `id`; `-32001` when the store needs a login;
`-32002` (with a message) when the store can't list versions right now.

## Install options (1.5)

The `install-options` capability (protocol 1.5) lets the user choose what an
install includes: a language, add-on content (DLC), optional packs, and on/off
toggles. Hydra shows them in the install dialog before the install starts, and
on an installed game under "Add-ons & options".

```
game.options { id } -> { groups: [
  { id: "language", label: "Language", kind: "select", values: [{ id: "en", label: "English" }], default: "en" },
  { id: "dlc", label: "Add-ons", kind: "multi", dlc: true,
    values: [{ id: "123", label: "Season Pass", sizeBytes: 1048576, owned: true, installed: false },
             { id: "456", label: "Deluxe", owned: false, storeUrl: "https://store.example/deluxe" }] },
  { id: "packs", label: "Optional packs", kind: "multi", values: [...] },
  { id: "subs", label: "Subtitles", kind: "bool", default: false } ] }
game.install / game.update { ..., options: { language: "en", dlc: ["123"], packs: [], subs: true } }
game.installSize { id, options? } -> { downloadBytes, diskBytes }
```

- **Groups.** `kind` is `select` (one value; `default` is its id), `multi` (any
  number of values) or `bool` (on or off; `default` is true or false, no
  `values`). A group with `dlc: true` is DLC: its values say whether the user
  `owned` them and whether they are `installed`. An unowned value is shown
  greyed out, with `storeUrl` (an https link) when the store has one. Core
  ignores a group of any other `kind`, so a plugin can add kinds without
  breaking older Hydra versions.
- **Choices.** `options` has one entry per group: a value id for `select`, a
  list of value ids for `multi`, a bool for `bool`. Hydra sends the complete set
  it resolved (every group, defaults included). A value left out of a `multi`
  list is one the user switched off, so a plugin that can remove an add-on does
  so when it is left out of an update.
- **Owned DLC.** Hydra never installs an unowned DLC: it refuses to queue the
  install, and the dialog says why. A plugin refuses it too (`-32602`) if it is
  asked anyway. An unowned value that is already installed may stay in the list.
- **Remembered.** Hydra keeps each game's last choices and sends them on every
  later install, reinstall and plain update. A job keeps its options, so a
  resumed job sends the same ones.
- **Changing options on an installed game** is a `game.update` with the new
  `options` (and the pin's `target`, if any).
- **Free space.** `game.installSize` takes the same `options`, so the free-space
  check counts the chosen add-ons and packs.
- **Errors.** `-32602` for a missing `id`, or for a choice the store doesn't
  have. A game the store doesn't install gets an empty `groups` list.

## Out of scope

Plugins may only install and launch what the user's own account is entitled to,
through the store's own APIs. No DRM or licence circumvention, and no Steam API
emulators.
