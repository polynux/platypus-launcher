# Store plugin protocol (api 1, draft)

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
  "api": 1,
  "store": "epic",
  "exec": "bin/hydra-epic",
  "capabilities": ["library", "install", "launch", "auth", "discard"],
  "requires": [{ "bin": "legendary", "hint": "uv tool install legendary-gl" }],
  "install_dir": "epic",
  "permissions": { "network": ["*.epicgames.com"], "read": ["~/.config/legendary"], "write": ["~/.config/legendary"] }
}
```

- `id` must match the folder name; `steam` and `custom` are reserved.
- `capabilities` lists what the plugin does (tracked in the conformance suite:
  `library`, `install`, `launch`, `auth`, and the optional `discard`). `discard`
  means the plugin answers `job.discard` (see Jobs) — declare it only when the
  method is really implemented.
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

- Environment: `HYDRA_PLUGIN_API=1`, and `HYDRA_PLUGIN_DATA`, a private folder
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
| `-32602` | missing or invalid params |
| `-32601` | unknown method (Hydra treats it as "not implemented" for `game.details`) |
| `-32000` | anything else the user should read |

Match `-32001` on the tool's real logged-out signal, not on a substring that
other errors can contain.

| Method | Params | Result |
|---|---|---|
| `initialize` | `{ api }` | `{ name, version, api, capabilities, missing }` |
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
  "native": false,
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

## Out of scope

Plugins may only install and launch what the user's own account is entitled to,
through the store's own APIs. No DRM or licence circumvention, and no Steam API
emulators.
