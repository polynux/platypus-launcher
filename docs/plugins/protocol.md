# Store plugin protocol (1.17, draft)

A store plugin is a folder in `~/.local/share/platypus/plugins/<id>/` with a
`plugin.json` and an executable. Platypus starts the executable on demand and talks
JSON-RPC 2.0 over its stdin/stdout, one JSON object per line. Anything the plugin
writes to stderr goes to `~/.local/share/platypus/logs/plugin-<id>.log`.

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
  "exec": "bin/platypus-epic",
  "capabilities": ["library", "install", "launch", "auth", "discard"],
  "needs": [],
  "requires": [{ "bin": "legendary", "hint": "uv tool install legendary-gl" }],
  "install_dir": "epic",
  "permissions": { "network": ["*.epicgames.com"], "read": ["~/.config/legendary"], "write": ["~/.config/legendary"] }
}
```

- `id` must match the folder name; `steam` and `custom` are reserved.
- `manifestVersion` is the schema of this file (absent means 1). Platypus lists a
  manifest with a newer schema as "needs a newer Platypus" without reading the rest.
  Unknown fields in a manifest of a known schema are ignored.
- `protocol` says which protocol versions the plugin speaks and over which
  transport; see [Versioning and compatibility](#versioning-and-compatibility).
  `versions` is a list (`["1.0", "1.1"]`) or an inclusive range
  (`{ "min": "1.0", "max": "1.3" }`; a range without `max` covers the later minors of
  its own major only, never the next major); `transport` defaults to `jsonrpc-stdio`.
  The legacy `"api": 1` still works and means protocol `1.0` over
  `jsonrpc-stdio`.
- `store` is required when the plugin declares store capabilities (`library`, `install`, `launch`, etc.). It may be omitted by a plugin that provides no store capabilities, such as a compatibility facts provider.
- `capabilities` lists what the plugin can do, as names from the
  [capability registry](#capabilities). It is the promise the user approves;
  `initialize` says what is really on (see below). `needs` lists the capabilities
  Platypus must know for the plugin to be worth running; it must be a subset of
  `capabilities`. Other unknown names are ignored with a warning.
- `exec` is relative and must stay inside the folder (symlinks included).
- `gameIdPrefix` (1.15, optional) is the prefix of this plugin's game ids.
  The default is `p.<plugin id>.`. It must match `^[A-Za-z0-9._-]{1,32}$` and
  must not start with `p.`, and only bundled/first-party plugins (an allowlist
  in the core: `steam`) may set it, so a third-party plugin can't take over
  other ids. See [Offline launches](#offline-launches-115).
- `requires` lists command-line tools looked up on `PATH`. A missing one keeps the
  plugin from starting; the Plugins view shows the hint. A requirement with
  `"optional": true` (1.15) is a launch helper (`steam`, `flatpak`, `xdg-open`)
  that isn't reported as missing: the plugin starts without it. A launch spec
  that uses a helper still needs it declared and on `PATH` (see [Launching](#launching)).
- `permissions` are shown to the user before approval. They aren't enforced yet
  (bubblewrap sandbox, plan milestone 8).
- `storefront` declares the store's official website for the embedded store
  view (issue #17): `{ "url": "https://store.example/", "domains":
  ["store.example", "www.example"] }`. `url` must be https and its host inside
  `domains`, which are plain hostnames (no wildcards, paths, ports or IPs).
  Navigation in the view (links, server redirects, popups, `window.open`,
  form posts) stays inside `domains` (exact or subdomain match): leaving them
  never loads, and the toolbar offers to open the address in the user's
  browser instead (never auto-opened). Third-party payment providers are NOT
  listed there: paying through one shows that offer. The address strip's
  outside tint is only a fallback for a page that somehow committed outside.
  List every one of the store's own hosts checkout may need (e.g. Valve's own
  checkout hosts for Steam); don't add third-party processors. Omit
  `storefront` when the store has no website to embed (no store section entry).

Platypus runs nothing until the user approves the plugin, which pins the SHA-256 of
the whole plugin folder: `plugin.json` plus every regular file under it, in
sorted path order (symlinks count by their target). `__pycache__/` and `*.pyc`
are skipped, and Platypus starts plugins with `PYTHONDONTWRITEBYTECODE=1` so an
interpreter doesn't dirty the folder. If anything else changes (the executable,
a helper module it imports, the manifest), the plugin is disabled until the user
approves it again. To update a plugin, replace the files (write a new one and
rename it over the old one); Linux refuses to overwrite a running executable
in place.

## Process

- Environment: `PLATYPUS_PLUGIN_API=1` (the major protocol version), and `PLATYPUS_PLUGIN_DATA`, a private folder
  for the plugin's own state. Every environment variable of the protocol is prefixed `PLATYPUS_`; the
  protocol is unreleased, so there are no aliases for earlier names, and a plugin built against them must be
  updated and reinstalled.
- Exit when stdin closes. Platypus kills the plugin's process group a couple of
  seconds later, and also when Platypus itself dies.
- Answer requests concurrently if they can take a while: Platypus may send
  `job.cancel` while a `library.list` is running.
- A crash is fine: Platypus restarts the plugin on the next request, with an
  increasing delay, and gives up after 5 quick crashes in a row.

## Requests (Platypus → plugin)

Errors are JSON-RPC errors. Their `message` is shown to the user as is, so write it
for people. Codes:

| Code | Use |
|---|---|
| `-32001` | the user has to log in first (`library.list`, `game.installSize`, `game.install`…) |
| `-32002` | unavailable right now: a borrowed game in use, a region lock, the store's service down. Not a login problem; the `message` says why and asking again later may work. Platypus shows it as is and never asks the user to log in |
| `-32602` | missing or invalid params |
| `-32601` | unknown method: "not offered". Platypus treats it as "not implemented" for the optional methods (`game.details`, …) and remembers it for the rest of the plugin run |
| `-32000` | anything else the user should read |

Match `-32001` on the tool's real logged-out signal, not on a substring that
other errors can contain.

| Method | Params | Result |
|---|---|---|
| `initialize` | `{ api, protocols, locale }` | `{ name, version, protocol, capabilities, features, missing }` |
| `auth.status` | | `{ loggedIn, user?, qr? }` |
| `auth.begin` | | `{ url?, instructions?, qr?, fields: [{ id, label, secret?, placeholder? }] }` |
| `auth.complete` | `{ fields: { <id>: value } }` | `{ loggedIn, user? }` |
| `auth.logout` | | `{ loggedIn }` |
| `library.list` | | `{ games: [LibraryGame] }` |
| `game.details` | `{ id }` | `GameDetails` (with `terms` from 1.11, media from 1.12) |
| `game.installSize` | `{ id, target?, options? }` | `{ downloadBytes, diskBytes }` |
| `game.install` | `{ id, installDir, jobId, title?, target?, options? }` | `{}` once started |
| `game.update` | `{ id, installDir, jobId, title?, target?, options? }` | `{}` once started |
| `game.uninstall` | `{ id, jobId }` | `{}` once started |
| `game.verify` | `{ id, installDir, jobId, repair }` | `{}` once started (1.4, [Verify and move](#verify-and-move-14)) |
| `game.move` | `{ id, from, to, jobId }` | `{}` once started (1.4, [Verify and move](#verify-and-move-14)) |
| `game.relocated` | `{ id, from, to }` | `{}` (optional, 1.4, [Verify and move](#verify-and-move-14)) |
| `job.cancel` | `{ jobId }` | `{}` |
| `job.discard` | `{ jobId, id, installDir }` | `{}` |
| `game.launchSpec` | `{ id, preview, via?, executable? }` | `LaunchSpec` (`executable` 1.15, [Offline launches](#offline-launches-115)) |
| `cloud.status` | `{ id, ctx }` | `CloudStatus` (1.2, [Cloud saves](#cloud-saves-12)) |
| `cloud.pull` | `{ id, ctx, jobId }` | `CloudStatus` |
| `cloud.push` | `{ id, ctx, jobId }` | `CloudStatus` |
| `cloud.resolve` | `{ id, ctx, jobId, choices }` | `CloudStatus` |
| `versions.list` | `{ id, ledger?, pinned? }` | `VersionList` (1.3, `versions` capability) |
| `versions.unlock` | `{ id, channel, password }` | `{ unlocked }` (optional) |
| `family.members` | `{}` | `[Member]` (1.8, [Family](#family-18)) |
| `family.setPreferredCopy` | `{ id, ownerId }` | `{}` (1.8, optional, [Family](#family-18)) |
| `settings.schema` | `{}` | `{ groups: [Group] }` (1.8, [Plugin settings](#plugin-settings-18)) |
| `settings.apply` | `{ values, gameId? }` | `{}` (1.8, [Plugin settings](#plugin-settings-18)) |

`qr` (on `auth.begin`, and on `auth.status` while a QR login is pending) is a QR
code as block-art text: `"\u2588"` is a dark module, two characters per module,
one row per module. The dialog draws it as square modules and redraws it when
`auth.status` returns a new one (the store refreshes the code as it expires).
The user can scan it or fill the fields. When the QR login completes,
`auth.status` turns `loggedIn`; the dialog closes by itself. Steam uses this.

Two overlapping `auth.begin` calls (a double-submit, two windows) must not break
each other: the second reuses the first's login attempt, or invalidates it with a
clear error.

`auth.begin` with a `url`: Platypus shows a button that opens it in the user's
browser (http and https only), then the fields. Without a URL it's a plain form.
Credentials stay with the plugin or the tool it wraps; Platypus never stores them.

`LibraryGame` (`sharedBy`, `copies` and `preferredCopy`, 1.8: a game borrowed from a family member, see [Family](#family-18)):

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

Set `native` when the game ships a Linux build: Platypus then launches it
directly, with no Proton runner and without the Proton-only settings (DX layer,
sync mode, Proton flags), on its game page too. `native` belongs on library
entries only; a launch spec says the same through `needsWine`.

Art: the Library's portrait cards show `coverUrl`, falling back to
`bannerUrl` then `heroUrl` (cropped) when the portrait 404s, and Platypus
remembers misses for a while instead of requesting them on every render —
so always send the landscape images even for games with no portrait art.
With the `local-art` capability (1.15) the art URLs may be `file:///abs/path`
(see [Offline launches](#offline-launches-115)); otherwise only http(s) URLs
are kept.

Send only the keys listed here. Platypus ignores unknown keys, but new plugins copy
what the bundled ones send, so the conformance suite (`internal/plugtest`)
rejects extras on `library.list`, `game.details` and `game.launchSpec`. To add a
key, document it here and in the suite first.

`features` narrows the capabilities on offer for this game (names from the
registry, see [Capabilities](#capabilities)). Leave it out and every capability
the plugin offers applies to the game; `[]` means none. Platypus drops names the
plugin doesn't offer at runtime.

Platypus keys plugin games as `p.<plugin id>.<game id>` (or the manifest's
`gameIdPrefix`, 1.15). List every owned game; the
Library shows the uninstalled ones under "owned".

`GameDetails` is the game page's metadata, fetched on demand. Stores without a
metadata source just don't implement the method; Platypus then shows the library
entry's art and title only.

```json
{
  "summary": "A troublesome goose.",
  "screenshots": ["https://…"], "genres": ["Comedy"],
  "developers": ["House House"], "publishers": ["Panic"],
  "releaseDate": "2019-09-20", "heroUrl": "https://…",
  "achievements": { "total": 12, "list": [{ "id": "prank", "name": "First prank", "icon": "https://…", "globalPercent": 84.1 }] },
  "descriptionHtml": "<p>A troublesome goose.</p>",
  "website": "https://…", "storeUrl": "https://…",
  "languages": ["English", "French"], "controller": "full",
  "requirements": { "minimum": "<p>…</p>", "recommended": "<p>…</p>" },
  "ageRating": "12+", "tags": ["Comedy", "Puzzle"],
  "videos": [{ "url": "https://…/trailer.mp4", "thumbnail": "https://…", "title": "Trailer", "mime": "video/mp4" }]
}
```

The media and richer facts (`descriptionHtml`, `website`, `storeUrl`,
`languages`, `controller`, `requirements`, `ageRating`, `tags`, `videos`) are
protocol 1.12 (see [Media](#media-112)): Platypus reads them only from a
plugin that negotiated 1.12 or later. An older plugin's answer carries none.

For an id the store doesn't know, `game.details` fails with an error; a game the
store knows but has no metadata for gets `{}`. (Steam's store answers
`success: false` for delisted games and unknown ids alike, so it returns `{}` for
both.)

Every URL must be http(s); Platypus drops others. Media URLs (`videos[].url`,
`videos[].thumbnail`) must be https, except the local art server's loopback
http URLs; `website` and `storeUrl` must be https. HTML from stores is
untrusted: `descriptionHtml` and `requirements` are sanitized in core to a
small allowlist (`p`, `br`, `ul`, `ol`, `li`, `b`, `strong`, `i`, `em`,
`h2`-`h4`, `a[href https]`) — no `img`/`iframe`/`style`/`script` — and the UI
renders only that. Lists and lengths are bounded (screenshots 30, videos 20,
languages/tags 100). `achievements` describes what
the store shows about the game — if the plugin can't see the user's progress,
leave it out rather than guess. Keep `list` short (a highlight, say 10) and let
`total` carry the real count; Platypus notes the truncation. `storeUrl` is the
game's page on the store's website (an https link; anything else is dropped):
the game page offers it as "Open in store", which deep-links into the embedded
store view when the plugin declares a `storefront`.

## Runner (1.17)

The bundled `runner` capability owns Wine, Proton, umu, the native launch
wrappers (GameMode, gamescope, MangoHud), prefix actions, winetricks and
Wine/Proton crash hints. It is restricted to bundled/first-party plugin ids.
Runner environment keys (`WINE*`, `PROTON_*`, `STEAM_COMPAT_*`) are reserved to
this capability.

The core sends one `runner.wrap` request per launch Platypus starts itself or
settings preview, native games included. Store-client launches aren't wrapped:
the store-client command (its `command`, see [Launching](#launching)) runs as it
is, so no wrapper goes around the client, and the runner isn't asked. With no runner plugin installed, the store's direct spec runs as
it is. With one installed but not ready (changed on disk, or crashed past its
restart limit), a launch stops with `runner.unavailable` before any process starts.

```json
runner.list {} -> { "runners": [{ "id": "auto", "name": "Automatic", "kind": "proton", "version": "", "default": true }] }
runner.options {} -> { "groups": [{ "id": "runner", "label": "Runner", "fields": [{ "id": "runner_id", "kind": "select", "label": "Runner", "scope": "game", "default": "auto", "options": [{ "id": "auto", "label": "Automatic" }] }] }] }
runner.detectExecutable { "path": "/home/u/Games/goose.exe" } -> { "native": false, "kind": "windows-exe" }
runner.wrap { "spec": { "command": ["/games/a"], "env": {}, "cwd": "/games", "needsWine": true }, "game": {}, "runnerId": "auto", "options": {}, "tweaks": [], "prefixesFolder": "/data/prefixes", "preview": false } -> { "command": ["/usr/bin/wine", "/games/a"], "env": { "WINEPREFIX": "/data/prefixes/g" }, "cwd": "/games", "runner": { "id": "system:wine", "name": "System Wine (/usr/bin/wine)", "kind": "system", "path": "/usr/bin/wine", "system": true } }
runner.prefix { "game": {}, "runnerId": "auto", "prefixesFolder": "/data/prefixes" } -> { "dir": "/data/prefixes/g", "user": "" }
runner.actions { "game": {}, "runnerId": "auto", "prefixesFolder": "/data/prefixes" } -> { "actions": [{ "id": "open-prefix", "label": "Open prefix" }, { "id": "winetricks", "label": "Run winetricks" }] }
runner.runAction { "id": "winetricks", "verb": "vcrun2022", "game": {}, "runnerId": "auto", "prefixesFolder": "/data/prefixes" } -> {}
runner.runAction { "id": "stop", "force": false, "launch": { "env": {}, "runner": {} } } -> {}
runner.crashHints { "game": {}, "runnerId": "auto", "log": "…" } -> [{ "rule": "vcredist", "param": "vcruntime140_1.dll", "verb": "vcrun2022" }]
```

All nine methods are required when `runner` is enabled (`runner.detectExecutable`
is called for every sideloaded file, `runner.prefix` for every backup and cloud gate).
Errors use `-32602` for a bad request and `-32003` for a runner failure, with a
message. A failure the UI translates carries `error.data` as `{ "code": "<apperr
code>", "params": { … } }`, and the core shows its own translated text for it:

| `code` | When | `params` |
|---|---|---|
| `game.noRunner` | a Wine or Proton game and no runner is installed (`runner.wrap`) | none |
| `game.runnerMissing` | the game's runner isn't installed (`runner.wrap`, `winetricks`) | `id` |
| `crash.badVerb` | `winetricks` names a verb the runner doesn't know (`-32602`) | `verb` |
| `crash.noWinetricks` | winetricks isn't installed | none |
| `crash.noPrefix` | the game has no prefix yet | `title` |
| `crash.umuUnsupported` | umu's Proton build isn't there to run winetricks with | `title` |
| `crash.noProtonWine` | the Proton build has no Wine binaries winetricks can use | `title` |

The core accepts only these codes. Any other failure, or one without `data`,
shows the message under the call's generic code: `runner.wrapFailed` for a launch,
`crash.winetricksFailed` for a winetricks run.

- **`runner.list`**: each runner has `id`, `name`, `kind` (`proton`, `umu`,
  `wine`, `custom` or `system`), `version`, `default`, and optionally `path`,
  `system` and `custom`.
- **`runner.options`**: the same group and field schema as `settings.schema`
  (1.8), every field with `"scope": "game"`, labels and descriptions in the
  language the core sent at `initialize` (`locale`, or `locale.changed`; English
  when the runner has no catalogue for it). The fields are typed game settings:
  `runner_id`, `use_gamemode`, `use_gamescope`, `dx_layer`, `upscaler`,
  `shader_cache`, `mangohud_preset`, `sync_mode`, the eight `proton_*` switches
  (`proton_enable_nvapi`, `proton_hide_nvidia_gpu`, `proton_enable_wayland`,
  `proton_no_d3d11`, `proton_no_d3d12`, `proton_force_large_address_aware`,
  `proton_old_gl_string`, `proton_log`), `gamescope_resolution` and
  `gamescope_rate` (selects with common values), `gamescope_filter`, the gamescope
  switches (`gamescope_fullscreen`, `gamescope_borderless`, `gamescope_grab_cursor`,
  `gamescope_hdr`, `gamescope_vrr`, `gamescope_wayland`), `gamescope_args` (other
  arguments), `eac_runtime` and `battleye_runtime`. The wine prefix is a core
  section, not offered here. Every field is a `GameSettings` key, so the saved
  names don't change; a test fails when a settings field is neither offered,
  derived, nor core-owned.
- **Choices become env and arguments in the runner.** `runner.wrap` and
  `runner.env` turn the choices into the env vars the old settings UI used to
  write (`PROTON_*`, `MANGOHUD_CONFIG`, `WINE_FULLSCREEN_FSR`, `DXVK_STATE_CACHE`)
  and the gamescope arguments (switches, then `-F`, then `gamescope_args`). The
  user's own `env_vars` win over a derived value. The core doesn't build them.
- **`runner.env`**: `{ "game": {} }` -> `{ "env": { "PROTON_LOG": "1" }, "wrappers": ["gamemode", "mangohud"] }`.
  The env (the choices plus `env_vars`) and the wrappers a launch would add. The
  game page uses it for the Steam launch options, since Steam starts Steam games
  itself and Platypus can't pass them.
- **`runner.detectExecutable`**: `kind` is `elf`, `appimage`, `script`,
  `windows-exe`, `windows-installer`, `windows-script`, `windows-shortcut` or
  `unknown`; `native` is true for the Linux kinds. The core uses it for the
  default of "Run with" and for launch decisions.
- **`runner.wrap`**: `spec` is the store's direct spec (`command`, `env`, `cwd`,
  `needsWine`); `game` is the game as the core knows it, with its settings;
  `options` holds the game's other saved settings keys; `tweaks` holds the enabled
  data-plugin tweaks, which the bundled runner doesn't apply yet; `prefixesFolder`
  is the effective data folder. The result has `command`, `env`, optional `cwd`,
  and for a Wine or Proton launch `runner`, the runner picked. A native game's
  result has only `command`, `env` and `cwd`.
- **`runner.prefix`**: the Windows side of the game's prefix, for the runner it
  would launch with: `dir` is the folder its saves are read from (for Proton, the
  `pfx` folder; for umu, the effective umu prefix), and `user` the login inside it
  (`steamuser` for Proton and umu, empty for system Wine or no runner). It is
  pure: it reads the game, its settings, the runner list and the disk, and never
  asks the store or starts a process, so backups and cloud saves don't depend on
  the store being reachable. A native game's result is empty (`{}`), and so is a
  result with no `dir`. With no installed runner, a `pfx` folder in the prefix
  still means Proton's layout. Cloud saves and backups call it once per gate.
- **`runner.actions`**: `open-prefix` when the prefix exists, and `winetricks`
  when winetricks is installed and the prefix exists.
- **`runner.runAction`**: `winetricks` runs `verb`, which the runner validates
  against its catalogue (`-32602` when unknown); the verb runs with the game's
  prefix in the background, and its output goes to the plugin's data folder.
  `open-prefix` opens the prefix. `stop` is sent by the core when a launch ends
  or is force-stopped: `launch` is that launch's `runner.wrap` result, which the
  core doesn't read (it hands it back unchanged), and the runner finds the
  wineserver from its `runner` and `env`.
- **`runner.crashHints`**: returns `CrashHint` entries (`rule`, and `param`,
  `verb` or `action` when the rule has them). The core adds the log path and
  whether winetricks is available.

## Jobs

Platypus creates the `jobId`, picks `installDir` (the user's Games folder plus the
plugin's `install_dir`, or the user's override) and checks free space against
`game.installSize` before calling `game.install`. `title` is the game's title
as Platypus lists it; use it for your own records instead of a second lookup, and
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
| `job.done` | `{ jobId, installed?, verify? }` (`installed` only with the `versions` capability, see [Versions](#versions); `verify` only for a `game.verify` job, see [Verify and move](#verify-and-move-14)) |
| `job.failed` | `{ jobId, error }` |
| `log` | `{ level, message }` (goes to the plugin's log file) |

Platypus runs one job at a time and saves the queue. If Platypus or the plugin dies
mid-install, Platypus calls `game.install` again with the same `jobId`, so installs
must be resumable or at least idempotent: a `jobId` that is already running starts
no second copy and ends with one `job.done`. A paused job is the same thing: Platypus
stops it like a cancel, then calls `game.install`/`game.update` again with the
same `jobId`, so keep everything on disk during a pause — a paused download is
resumed later from what it already fetched, never fetched again. After
`job.cancel`, stop the work (an uninstall too: don't delete anything after the
cancel) and send neither `job.done` nor `job.failed`; Platypus has already marked
the job canceled and ignores later notifications for it. State that has to
outlive the process (what is installed, where) goes under `PLATYPUS_PLUGIN_DATA`,
not in memory.

### Discarding a canceled job's data

A canceled install usually leaves data behind: staging chunks, a tool's resume
files, a partially written install folder. The user can discard it with an
explicit action, and Platypus does it later of its own accord after a grace
period (it doesn't run for paused jobs). Platypus sends:

```
{ "method": "job.discard", "params": { "jobId": "…", "id": "…", "installDir": "/…" } }
```

The method is optional; declare the `discard` capability only if you can
honor it. Delete exactly what the job left: the staging area of that `jobId`
and, for an install that never finished, the game's own (partial) subfolder of
`installDir` — and the tool's resume files for that game. Never delete an
installed game's folder: a canceled *update* keeps the game installed, and
`id`/`installDir` identify the game and folder Platypus started the job with, so
recheck both against what is really installed. Stay inside `installDir` and
`PLATYPUS_PLUGIN_DATA` (symlinks resolved); answer with `-32602` for anything
unexpected. Discarding a job Platypus has already restarted yours (or forgotten)
is not an error: answer `{}`. A discard for a still-running job means the same
as `job.cancel` on it: stop and wait for your work to wind down first.

Plugins that don't implement the method: don't list the capability. Platypus then
only forgets the job and tells the user what may remain on disk.

## Launching

The core owns session tracking and process start/stop. When a bundled runner
plugin is available, it wraps each launch spec Platypus starts itself (including
native launches) and returns the command; without one, Platypus runs the store's
direct spec as-is. Store-client launches aren't wrapped: their `command` runs as
it is, so no wrapper goes around the store's client.
A plugin returns a spec; it never starts the game itself.

Direct launch:

```json
{
  "exe": "/home/u/Games/epic/Goose/Goose.exe",
  "args": ["-AUTH_PASSWORD=…"], "cwd": "/home/u/Games/epic/Goose",
  "env": { "DXVK_HUD": "fps" }, "needsWine": true,
  "redact": ["-AUTH_PASSWORD="]
}
```

Platypus refuses the spec when:

- `exe` or `cwd` isn't inside the plugin's install folder, or inside a folder the user
  explicitly allowed for this plugin (symlinks are resolved first). System folders
  and the home folder itself can't be allowed;
- `env` sets a key on the denylist: `PATH`, `HOME`, `LD_*`, `WINE*` (prefix and DLL
  overrides are Platypus's), `PROTON_*`, `STEAM_COMPAT_*`, `VK_*`, interpreter paths and
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

`command[0]` must be one of the manifest's `requires`. Platypus runs the command,
then follows the game through `find`: the game is running while some process's
`/proc/<pid>/cmdline` (arguments separated by NUL) contains every string in
`cmdline`. Each string must be at least 3 characters.

`via` (optional, 1.2) asks for a particular launch mode. Platypus sends `"via": "client"`
only when the plugin's `CloudStatus.client.canLaunch` said it can: the user chose
"Launch through <client>" because the store's client is running. Answer with a
store-client spec for the game; a plugin that has none answers `-32000` with a message.

`executable` (optional, 1.15) is the user's per-game executable override. Platypus
sends it when the user set one, instead of bypassing the plugin: answer with a
direct spec for that file.

## Cloud saves (1.2)

Capability `cloud-saves`, protocol 1.2 and later. A plugin that speaks only 1.0 or 1.1
keeps working unchanged: the capability is off for it, Platypus sends no `cloud.*`
request and the game page shows no cloud row.

**Who does what.** Platypus decides *when* and what the user sees: a per-plugin
default and a per-game override (`off`, `ask`, `auto`), a `cloud.pull` before it
launches a game itself, a `cloud.push` after the game exits, the conflict dialog
and the toasts. The plugin decides *how*: which files, how they compare, how
they move. Platypus never calls these for a game the store's own client launches
(the client syncs it) or for a game whose `features` leave out `cloud-saves`.

`ctx` (`CloudContext`) describes the launch the call is for; the plugin never
learns Platypus's prefix layout:

```json
{ "os": "linux", "home": "/home/u", "xdgData": "/home/u/.local/share", "xdgConfig": "/home/u/.config",
  "installDir": "/home/u/Games/echo/tiny",
  "prefixDir": "/home/u/.local/share/platypus/prefixes/p.echo.tiny/pfx", "winUser": "steamuser" }
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
  "checkedAt": 1791458728, "backupDir": "/home/u/.local/share/platypus/plugin-data/echo/cloud-backups/tiny",
  "stamp": "a1b2c3", "client": { "running": true, "name": "Steam", "canLaunch": true } }
```

- `state` is one of `synced`, `pending` (something to move, no conflict), `conflict`, `unsupported`,
  `offline` (the store could not be reached) and `error`. Platypus turns any other value into `error`.
  Expected failures are answers with `offline` or `error` and a `message`, not JSON-RPC errors; a JSON-RPC
  error on these methods is shown as `error` with its message. `-32602` is for missing `id`, `ctx` or `jobId`.
- `message` is written for people and shown as is. For `unsupported` it is the reason ("no Auto-Cloud rules",
  "encrypted cloud files", "saves go through the Steamworks API"); Platypus never invents one.
- `counts` tallies files; `unmapped` counts cloud files the plugin could not place on this device.
- `conflicts` lists each file changed on both sides, or with no record of a common ancestor. Platypus never
  chooses for the user and never sends a `choices` path the last status did not list.
- `checkedAt` is when the comparison ran (unix seconds). `backupDir` is where replaced local files were
  copied; keeping the last few backups per game is the plugin's job, Platypus only shows the folder.
- `stamp` (optional) is an opaque fingerprint of the local save files. After a game exits Platypus reads
  `cloud.status` twice, 2 s apart, and pushes only when the two readings agree (the game may still be
  writing); without a `stamp` it compares `counts` and `conflicts`.
- `client` (optional) is the **official client policy**, the same for every store. `running` says the
  store's own client is up; `name` is how to call it ("Steam"); `canLaunch` says the plugin can start the game
  through it (`game.launchSpec` with `"via": "client"`). While `running` is true Platypus calls neither `cloud.pull`
  nor `cloud.push` for that game, because two writers on one save folder produce exactly the conflicts the
  sync exists to avoid. It offers "Launch through <name>" when `canLaunch` is true, and otherwise explains
  on the game page. Report `client` on every answer.

Rules for the plugin: never overwrite a local file that differs from the cloud and from the last state both
sides agreed on (that is a `conflict`), never delete anything in the cloud, back up any local file a download
replaces, keep the baseline under `PLATYPUS_PLUGIN_DATA`, and never log paths or tokens.

**Jobs.** `cloud.pull`, `cloud.push` and `cloud.resolve` carry a `jobId` Platypus made up. They obey `job.cancel`
(the request then ends with `-32000` "canceled"; Platypus has already given up on the answer) and are idempotent
for a repeated `jobId`: the second request returns the first one's result without redoing the work. Platypus
repeats a `jobId` when a push was interrupted by Platypus quitting and runs at the next start. A plugin that offers
`cloud-saves` receives `job.cancel` even when it does not offer `install`. Progress is a notification:

| Notification | Params |
|---|---|
| `cloud.progress` | `{ id, jobId, phase, done, total }` (`phase` is `check`, `download` or `upload`; `done` and `total` count files) |

**When Platypus calls what.** Before it launches a game directly: `cloud.status`; if the client runs it stops
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
  capabilities and error codes. A plugin written for 1.0 keeps working on a Platypus
  that speaks 1.5. **Major** versions may break. New required behaviour comes only
  with a new major, and Platypus keeps speaking the previous major for a deprecation
  period.
- **Negotiation.** The manifest declares the versions the plugin speaks
  (`protocol.versions`). `initialize` carries the versions Platypus speaks
  (`protocols: ["1.0", "1.1", "1.2", "1.3", "1.4", "1.5", "1.6", "1.7", "1.8", "1.9", "1.10", "1.11", "1.12", "1.13", "1.14", "1.15", "1.16", "1.17"]`, and the legacy `api: 1`); the plugin answers with
  `protocol`, the highest it shares with Platypus. Platypus checks that the pick is one
  it offered and the manifest declares. A plugin that predates negotiation leaves
  `protocol` out and is taken to speak the highest version the manifest and Platypus
  share. A plugin never relies on a field or method it didn't negotiate.
- **Incompatible plugins are listed, never started.** A transport Platypus doesn't
  have, a `manifestVersion` newer than Platypus reads, or no common protocol version
  show up in the Plugins view as "needs a newer Platypus" (or "too old for this
  Platypus", when the plugin only speaks versions older than any Platypus still
  supports). Nothing is spawned and nothing counts as a crash.
- **Ignore what you don't know, both ways.** Unknown fields in manifests, params,
  results and notifications are ignored. An unknown method is answered `-32601`
  (Platypus probes optional methods once per plugin run and remembers the answer).
  Unknown capability names are ignored with a warning; use `needs` for the ones
  that are hard requirements.
- **Revisions.** 1.0: the first plugin releases. 1.1: the capability registry, runtime `capabilities` and
  `features` in `initialize`, `-32002`. 1.2: the `cloud.*` methods, `cloud.progress` and the `via` parameter of
  `game.launchSpec` (cloud saves). 1.3: the `versions.*` methods, `target` on installs and updates, and
  `installed` on `job.done` (versions). 1.4: the `verify` and `move` capabilities (`game.verify`, `game.move`), `verify`
  on `job.done`, and the optional `game.relocated`. 1.5: `game.options`, and `options` on installs, updates and
  `game.installSize` (install options and DLC). 1.6: the `social` capability (`social.*`: friends and own status).
  1.7: `achievements.list`, `playtime.list` and `redeem.submit` (achievements, playtime and redeem, see
  [Achievements, playtime and redeem](#achievements-playtime-and-redeem-17)). 1.8: the `family` and `settings`
  capabilities (`family.members`, `family.setPreferredCopy`, `settings.schema`, `settings.apply`), `sharedBy`,
  `copies` and `preferredCopy` on library entries and the `family.warning` notification (see [Family](#family-18) and
  [Plugin settings](#plugin-settings-18)). 1.9: the `chat` capability (`chat.history`, `chat.send`, `chat.markRead`) and
  the `chat.*` notifications (see [Chat](#chat-19)). 1.10: the compat fields of `game.details` (`steamAppId`,
  `platforms`, `drm`; see [Compat](#compat-110)). 1.11: the `terms` capability (`terms` on `game.details`, the optional
  `game.acceptTerms`, see [Terms](#terms-111)), and the DLC `kind` and copy `exclude` of `copies` (see [Family](#family-18)).
  1.12: the media and richer facts of `game.details` (`descriptionHtml`, `website`, `storeUrl`, `languages`,
  `controller`, `requirements` (`minimum`, `recommended`), `ageRating`, `tags`, `videos` (`url`, `thumbnail`,
  `title`, `mime`); see [Media](#media-112)). 1.13: the unread snapshot of chat (`chat.unreadCounts`, see
  [Chat](#chat-19)). 1.14: the user's locale (`initialize` `{locale}`, the `locale.changed` notification;
  see [Locale](#locale-114)). 1.15: offline launches (`offline`, `cloudSync`, the `local-art` capability,
  `gameIdPrefix`, the `executable` param of `game.launchSpec`, optional `requires`; see
  [Offline launches](#offline-launches-115)). 1.16: per-game status and notes
  (`LibraryGame.status`, `LibraryGame.userNotes`, a store's suggestion only; see
  [Per-game status and notes](#per-game-status-and-notes-116)). 1.17: the
  `compat-facts` capability (`compat.lookup`, batched compatibility facts) and
  the `runner` capability (`runner.list`, `runner.options`, `runner.wrap`,
  `runner.actions`, `runner.runAction`, `runner.crashHints`; see
  [Compat facts](#compat-facts-117) and [Runner](#runner-117)).
  A capability whose `since` is newer than the negotiated protocol is off for that plugin (Platypus drops it from the
  runtime set and logs it), so a plugin that only agreed to 1.1 never receives a 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9,
  1.10, 1.11, 1.12, 1.13, 1.14, 1.15, 1.16 or 1.17 method.
- **Transport.** Today the only transport is `jsonrpc-stdio` (this document). It
  is an interface in `internal/plugins`: a future transport is a new
  implementation selected by `protocol.transport`, with the same methods,
  capabilities and error codes on top.

## Capabilities

A capability is a name in the manifest that stands for a group of methods and
the UI that depends on it. Platypus keeps them in a registry (`internal/plugins`,
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
| `verify` | 1.4 | `game.verify` | | yes |
| `move` | 1.4 | `game.move` | | yes |
| `social` | 1.6 | `social.start`, `social.stop`, `social.state` | `social.setStatus`, `social.setPlaying` | yes |
| `achievements` | 1.7 | `achievements.list` | | yes |
| `playtime` | 1.7 | `playtime.list` | | yes |
| `redeem` | 1.7 | `redeem.submit` | | yes |
| `family` | 1.8 | `family.members` | `family.setPreferredCopy` | yes |
| `settings` | 1.8 | `settings.schema`, `settings.apply` | | yes |
| `chat` | 1.9 | `chat.history`, `chat.send`, `chat.markRead` | `chat.unreadCounts` (1.13) | yes |
| `terms` | 1.11 | | `game.acceptTerms` | yes |
| `local-art` | 1.15 | | | yes |
| `compat-facts` | 1.17 | `compat.lookup` | | yes |
| `runner` | 1.17 | `runner.list`, `runner.options`, `runner.detectExecutable`, `runner.prefix`, `runner.env`, `runner.wrap`, `runner.actions`, `runner.runAction`, `runner.crashHints` | | yes |

"Not yet" means Platypus recognises the name (no warning, usable in `needs`) but
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
Listing a capability the manifest doesn't declare is a protocol error: Platypus
ignores it and notes it in the plugin's log, and the conformance suite fails.
Platypus shows and gates on this runtime set: it never sends a method of a
capability that is off (the call fails with `-32601` on Platypus's side), and the
Plugins view lists every declared capability, on or off, with its features.

**Per game.** `LibraryGame.features` narrows the capabilities for one game (a
Versions control must not appear for a game with a single build). Absent means
all the plugin offers.

**Surfaces.** Every UI surface that depends on a capability (the registry names
them) renders only when the plugin offers it and, if the game lists `features`,
the game does too. When something is declined or unavailable, the reason is text
the plugin wrote (as `CloudStatus.message` will be for cloud saves); Platypus never
invents one.

**Optional methods.** The first `-32601` for an optional method is remembered
until the plugin process exits; Platypus doesn't ask again in that run. `-32002` is
never remembered.

## Verify and move (1.4)

Capabilities `verify` and `move`, protocol 1.4 and later. They were registered
for 1.1 before they shipped, so a plugin that settled on 1.3 or older is never
sent them. Both work on an installed game's folder: `installDir`, `from` and
`to` are the game's own folder, the one `library.list` reports as `installPath`.

### Verify

```
game.verify { id, installDir, jobId, repair }   -> {}   # job
job.done    { jobId, verify: { checked, damaged, missing, repaired } }
```

A check (`repair: false`) only reads. It counts the files it looked at
(`checked`), those that differ from the store's copies (`damaged`) and those
that are gone (`missing`). Platypus shows that report and asks the user; a repair
(`repair: true`) restores the damaged and missing files from the store and
reports what it restored (`repaired`) and what is still wrong after it
(`damaged`, `missing`). Platypus only sends a repair after a check found damage, so
a plugin may assume the user saw the report. `job.progress` reports the check
(`phase: "checking"`); `job.cancel` stops it, sending neither `job.done` nor
`job.failed`. The game must not be running (Platypus refuses otherwise).

### Move

```
game.move { id, from, to, jobId }   -> {}   # job
job.done  { jobId }
```

`to` does not exist yet; its parent folder does. Platypus has already checked that
the destination has room for the game, that it is not inside `from` and not a
system, home or Games folder, and that the game is not running. The plugin
moves the folder and rebases its own records, and sends `job.progress` while it
works. `job.cancel` before the folder is moved leaves the game where it was.

A plugin without `move` (or with the capability switched off) gets nothing: Platypus
moves the folder itself, then tells the plugin with `game.relocated` if it speaks
1.4. Platypus's copy goes to a staging folder next to `to` (`<to>.platypus-move`),
is checked name by name, kind by kind and size by size, and is renamed into
place. Only then is the old folder removed. A failure or a cancel removes the
staging copy and leaves the game where it was.

### Relocated

```
game.relocated { id, from, to }   -> {}   # optional, no capability
```

Sent after Platypus moved a game's folder itself, to a plugin that speaks 1.4. The
plugin rebases its records (its install path, its launch paths) from `from` to
`to` and answers `{}`; `-32602` means its records don't match `from`. A plugin
that doesn't implement it answers `-32601`, and Platypus keeps the move itself:
it still reports the old `installPath`, the library shows the new folder, and
launch paths are rebased on the way out. The method is not capability-gated.

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
the Plugins view shows; Platypus still handles any empty list.

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
  password was accepted (Platypus shows a password form for the locked ones).
  `current` marks the channel the installed game follows. Both are optional.
- `builds[].source` is `"store"` (the store lists it), `"archive"` (it comes
  from Platypus's install history, `ledger`) or `"manual"`. Platypus marks them in the
  UI as "from the store", "from your install history" and "typed by you".
  `available: false` means the plugin knows the build but can't fetch it now;
  leave it out for `true`. `current` marks the installed build.
- `manual` asks the user to type something the store doesn't list (Steam: one
  manifest id per depot). `label` and `hint` are shown as written next to a
  text box (several lines allowed). Platypus's UI sends the text in
  `target.manual.input`, trimmed and at most 4 KiB, with its line breaks kept;
  parse it as your hint says. Other keys are allowed for callers that build a
  target themselves.
- `links` (optional, at most 10) point at a page that helps the user pick, such
  as a game's depots on SteamDB. Platypus only opens them in the browser (http and
  https only) and never fetches them.
- `updateAvailable` is relative to the pin: with a `pinned` in the request, whether
  the installed version differs from it; without one, whether a newer build
  exists. `pinned` in the answer is ignored: Platypus's own pin is authoritative.
- `ledger` (request) is Platypus's install history for the game, oldest first:
  `[{ store, game, build, channel?, versionName?, date, archiveRef? }]`. List the
  entries your store no longer lists as `source: "archive"` and say whether they
  can still be fetched. `archiveRef` is whatever you put in `installed`; Platypus
  never reads it (Epic: the saved manifest's name; Steam: depot → manifest ids).

**Target.** `game.install` and `game.update` carry `target` when the user (or a
pin) picked a version: `{ channel?, build?, manual? }`. A `channel` alone means
"that branch's current build"; a `build` is exactly that build (with a
`channel`, the build within it); `manual` is the typed values (`{ "input": "<text>" }` from Platypus's UI). Without `target`,
install the store's default. For a build from the install history (`source:
"archive"`) Platypus adds `archiveRef` to the target: the reference it kept from
that build's `installed`, as it was written there. The UI never sends it: Platypus
refuses a target from the UI that already carries one. A pin makes Platypus send the same `target` on every
`game.update`, so `update` may be a downgrade. `game.installSize` takes the
same `target` so the free-space check is for that version. A target for a
locked channel is refused (`-32000` with a message the user can act on). Platypus
resends the same `target` with the same `jobId` when it resumes a job.

**Passwords.** `versions.unlock` validates a password and the plugin keeps it
in `PLATYPUS_PLUGIN_DATA`; answer `{ "unlocked": false }` (or `-32000` with a
message) for a wrong one. Platypus holds a password only for the length of that
call: it is never written to a job, the settings or the install history, never
logged, and a target never names one. A resumed job re-uses the stored unlock.
Don't write the password to your log either. Don't implement the method when the
store has no passwords: it is optional and the first `-32601` is remembered.

**Ledger.** After a `job.done` that carries `installed`, Platypus appends
`{ store, game, build, channel, versionName, date, archiveRef }` to the game's
history (`install-history.json` in Platypus's data folder), so installing the same
build twice in a row refreshes one entry rather than adding two:

```json
{ "jobId": "…", "installed": { "build": "58076094395251196", "channel": "beta",
  "versionName": "1.5", "archiveRef": { "depots": { "481": "7845…" } } } }
```

Send `installed` for every install and update when the capability is on;
`build` is required, the rest optional. `archiveRef` is any JSON, at most 64 KiB.

**Pinning.** Platypus stores the pin per game (a `build` or a `channel`), passes it
as `target` on `game.update` (and on a reinstall), and hides the game's update
prompt while it is set. It does not ask you to know about pins, except through
`versions.list`'s `pinned` request field so you can compute `updateAvailable`.
"Return to latest" clears the pin; the next update installs the default.

**Safety.** Before installing a version other than the store's default, the UI
warns that cloud saves or online play may not match, and asks to confirm. Platypus
backs up nothing itself.

**Errors.** `-32602` for a missing `id`; `-32001` when the store needs a login;
`-32002` (with a message) when the store can't list versions right now.

## Install options (1.5)

The `install-options` capability (protocol 1.5) lets the user choose what an
install includes: a language, add-on content (DLC), optional packs, and on/off
toggles. Platypus shows them in the install dialog before the install starts, and
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
  breaking older Platypus versions.
- **Choices.** `options` has one entry per group: a value id for `select`, a
  list of value ids for `multi`, a bool for `bool`. Platypus sends the complete set
  it resolved (every group, defaults included). A value left out of a `multi`
  list is one the user switched off, so a plugin that can remove an add-on does
  so when it is left out of an update.
- **Owned DLC.** Platypus never installs an unowned DLC: it refuses to queue the
  install, and the dialog says why. A plugin refuses it too (`-32602`) if it is
  asked anyway. An unowned value that is already installed may stay in the list.
- **Remembered.** Platypus keeps each game's last choices and sends them on every
  later install, reinstall and plain update. A job keeps its options; when it
  is sized or starts, Platypus checks them against `game.options` again, so a
  resumed job sends what the store offers now (a DLC refunded while paused is
  left out). Options are sent only while the plugin offers them over 1.5.
- **Changing options on an installed game** is a `game.update` with the new
  `options` (and the pin's `target`, if any).
- **Free space.** `game.installSize` takes the same `options`, so the free-space
  check counts the chosen add-ons and packs.
- **Errors.** `-32602` for a missing `id`, or for a choice the store doesn't
  have. A game the store doesn't install gets an empty `groups` list.

## Social (1.6)

The `social` capability (protocol 1.6) gives the user the friends of the store's account: who is online, away or
playing what, and their own status. Platypus shows them in a small summary in the sidebar and, on click, a friends modal
with one section per store (In game, Online, Offline). Chat is the next section ([Chat](#chat-19)). Groups, friend
requests and rich presence are not part of it.

```
social.start {} -> {}                              turn the friends on: connect, then push as below
social.stop {} -> {}                               disconnect (idempotent)
social.state {} -> { connection, self, friends }   the current snapshot, as the pushes would have it
social.setStatus { state } -> {}                   the user's own status (optional)
social.setPlaying { id | null } -> {}              the game the user plays, null for none (optional)

social.connection { state, message? }              plugin -> Platypus notification
social.self { name, avatarUrl?, state }            the user's own profile and status
social.friends { full, friends: [Friend] }         the whole list (full: true) or changed friends (full: false)
Friend = { id, name, avatarUrl?, state, game?: { id?, title }, lastSeen? }
```

- **States.** A friend's `state` is `online`, `offline`, `away`, `busy`, `snooze`, `looking` or `invisible`. A
  connection is `connecting`, `online`, `reconnecting`, `replaced` (another session took the account), `offline` or
  `loggedOut`. `setStatus` takes `online`, `away`, `busy`, `snooze` or `invisible`; anything else is `-32602`.
- **Pushes.** The first push after `social.start` is `full`. An incremental push replaces the friends it names (by
  `id`) and adds new ones. It never removes a friend: a plugin that drops one sends the full list.
- **Games.** `game.id` is Platypus's key (`p.<plugin id>.<game id>`) when the game is in the user's library, and left out
  otherwise, when only `title` says what the friend plays. Platypus links the game when the key is in the library.
- **Playing.** When a game of this plugin starts, Platypus sends `social.setPlaying { id }` with the store's game id (the
  part after `p.<plugin id>.`); when it stops, `{ id: null }`. Only plugins that offer the capability get it, and only
  for their own games, and only while the user keeps "Show what I'm playing to friends" on for the store (on by
  default). A game started while the friends are off is sent when they turn on. A plugin without the optional method answers `-32601`, which Platypus accepts. After the plugin
  process restarts, Platypus sends the running game again.
- **Lifecycle.** Platypus starts the friends of every plugin that offers `social` at launcher start, unless the user
  switched them off (on by default). It sends `social.stop` when they are switched off or Platypus quits. When the plugin
  process exits, Platypus restarts it after a backoff (2 s, doubling to 5 min) and calls `social.start` again. The wait
  starts again at 2 s only after a session that lasted a minute, so a plugin that crashes on start isn't restarted
  every 2 s. A `replaced` or `loggedOut` connection is not fought: Platypus stops restarting until the plugin reports
  `online` or `connecting` again, or the user restarts the friends (Reconnect, which switches them off and on).
- **Errors.** A store that can't change the status answers `-32601` to `setStatus`, and Platypus tells the user so.
  Before `social.start`, `social.state` answers offline with no friends.

## Achievements, playtime and redeem (1.7)

Capabilities `achievements`, `playtime` and `redeem`, protocol 1.7 and later.
Platypus shows the user's own progress, the playtime the store records and a
code redeemed in the store's own service. Unlocking achievements is never
offered: that is the game's job through the store's SDK.

```
achievements.list { id } -> { total, unlocked, list: [{ id, name, description, icon, hidden, unlockedAt?, globalPercent? }] }
playtime.list {}         -> { games: [{ id, minutes, lastPlayed? }] }
redeem.submit { code }   -> { message, titles?: ["…"] }
```

- **achievements.list** takes the store game id (`id`, as `game.launchSpec` does). `unlockedAt` is unix seconds and
  present only for an unlocked one; a store that can't see progress leaves it out everywhere. `globalPercent` is the
  share of players (0 to 100) and may be left out. `hidden` achievements are masked by core until unlocked, so a plugin
  may send their real name. When the progress is available it replaces the catalogue list of `game.details`.
  The game's `features` must include `achievements` when they are listed. Missing `id` is `-32602`.
- **playtime.list** covers the whole library in one call and is read at plugin start and after a play session
  ends (debounced). Core shows `max(local, store)` in the library and the game page, sorted by it, and never writes
  the store's value back. `lastPlayed` is accepted but not used yet.
- **redeem.submit** takes the code as the user typed it, trimmed, at most 128 characters, with no control
  characters. A rejected code is `-32000` with a message the user can read; a missing `code` is `-32602`. One
  submission runs at a time; Platypus logs only the first two and last two characters and the length.
  The `titles` are the games or items the store granted, when it names them.

## Family (1.8)

The `family` capability is the store's family group: its members, the games they lend
to the user, and a warning while a borrowed game is in use. Steam is the first store
with one; the others leave the capability out.

```
family.members         {}                 -> [Member]          # a list, no params needed
library.list           -> LibraryGame.sharedBy  -> { id, name }     # borrowed from that member
library.list           -> LibraryGame.copies, preferredCopy        # when the members' copies differ
family.setPreferredCopy { id, ownerId }   -> {}                 # the copy the user picks (optional)
game.launchSpec (in use)                  -> -32002 "Family member Ana is playing this game"
family.warning  { id, message, secondsLeft } (notification)
```

A member is `{ "id": "ana", "name": "Ana", "role": "adult", "avatarUrl": "https://…", "state": "online" }`.
`role` is `adult` or `child`; anything else is shown as none. `avatarUrl` must be http(s).
`state` is the store's own words. Platypus shows members as they come; members without an
`id` are dropped. Errors: `-32001` when the user has to log in, `-32002` when the family
can't be read right now.

`sharedBy` marks a borrowed game (the library shows "Shared by Ana" and the "Shared with me"
filter). It names the copy the game is played from now: the one the user picked, or the store's
default. Platypus uses it only while the game's `features` include `family`.

When several members lend the same game and their copies differ (DLC, a note, in use), the
entry also carries `copies` and `preferredCopy`. Each copy is
`{ "ownerId": "ben", "ownerName": "Ben", "dlc": [{ "id": "soundtrack", "name": "Soundtrack", "kind": "soundtrack" }],
"note": "…", "exclude": "…", "available": true, "inUse": false }`. A DLC's `kind` is `dlc` (the default), `soundtrack` or
`other`, and Platypus counts soundtracks apart in the comparison; an unknown kind is read as `dlc`. `exclude` is text the
store gives for what the copy doesn't lend (1.11), shown as it is. `ownerId` is required, `available` says whether the
copy can be borrowed now (an unavailable one is shown greyed out), `inUse` says whether someone is
playing it. `preferredCopy` is the `ownerId` the user picked, or absent/`null` for the store's
default. Copies are read only from a borrowed game: a game the user owns has no `sharedBy`, and
Platypus shows no picker for it, whatever `copies` it carries.

`family.setPreferredCopy { id, ownerId }` records the user's pick. Platypus sends it only for a game
that is borrowed in the library, with a copy from `ownerId` that is listed and `available`; the
plugin answers `-32602` for an owner it doesn't list, and `-32002` for a copy that can't be picked
right now. The store keeps the choice (Echo keeps it in `PLATYPUS_PLUGIN_DATA`), and the next
`library.list` shows it. The method is optional: a plugin that lists `copies` without it gets
`-32601`, which Platypus notes once and stops asking. Platypus refuses the pick, without sending it,
when the family capability is off, the protocol is older than 1.8, or the game's `features` leave
`family` out. A borrowed game
that is in use answers `game.launchSpec` with `-32002` and a message for the user; Platypus
shows it and does not retry.

`family.warning` is raised while the owner is taking a borrowed game back (the store warns
the borrower, then disconnects them). `id` is the store's game id, `message` is shown as
it is (300 characters at most), and `secondsLeft` starts a countdown in the toast. Platypus
doesn't stop the game; the store does. Platypus takes the notification only from a plugin
whose `initialize` has said it offers `family` (before that, the manifest is only a guess).
The toast is shown only for a game in the user's library, and a warning without
`secondsLeft` leaves by itself after 15 seconds.

When a game has two or more copies, Platypus also compares them (computed by core, nothing for the plugin to send): each
copy is set against the preferred copy, or the copy with the most add-ons when none is preferred (ties go to the lower
`ownerId`). The counts say how many add-ons the copy has that the baseline lacks, how many the baseline has that the copy
lacks, how many both have, and the same three for soundtracks.

Owner exclusions, per-copy limits and parental limits are the store's rules. Platypus shows
the state the store reports and enforces none of them.

## Plugin settings (1.8)

The `settings` capability lets the plugin describe its own options and receive the
values the user picked: overlays ("Enable EOS overlay"), launch options, a store's
toggles. Core draws the form, so the plugin needn't write a UI, and it needn't persist
what it is sent.

```
settings.schema {}                -> { groups: [Group] }
settings.apply  { values, gameId? } -> {}
```

```json
{ "groups": [
  { "id": "overlay", "label": "Overlay", "description": "Applied when Echo launches a game.",
    "fields": [
      { "id": "eos-overlay", "kind": "bool", "label": "Enable overlay (example)", "default": false },
      { "id": "region", "kind": "select", "label": "Download region (example)", "default": "eu",
        "options": [{ "id": "eu", "label": "Europe" }, { "id": "us", "label": "United States" }] },
      { "id": "note", "kind": "text", "label": "Extra launch arguments (example)", "default": "", "placeholder": "-windowed" },
      { "id": "volume", "kind": "number", "label": "Bandwidth limit, MB/s (example)", "default": 5, "min": 0, "max": 10 } ] },
  { "id": "per-game", "label": "Per game",
    "fields": [ { "id": "game-overlay", "kind": "bool", "label": "Overlay for this game", "scope": "game", "default": true } ] }
] }
```

- **Groups and fields.** A group has an `id`, a `label`, an optional `description` and its
  `fields`. A field has an `id`, a `kind`, a `label` and an optional `description`. The
  kinds are `bool` (`default` true or false), `select` (`options: [{ id, label }]`, `default`
  an option id), `text` (`default`, `placeholder`) and `number` (`default`, `min`, `max`).
- **Scope.** `scope` is `plugin` (the default) or `game`. Plugin-wide fields show in the
  Plugins view. Game fields show on the game page of each game whose `features` include
  `settings`, and their values go to that game only: `settings.apply` carries `gameId`, the
  store's game id.
- **Unknown kinds are dropped**, with the group left empty, so a plugin can describe kinds
  an older Platypus doesn't show. A default of the wrong type is ignored (the field stays),
  and a repeated field id is dropped (the first one wins). A number default outside
  `min`/`max` is clamped into them; `min` above `max` drops both bounds; a text default
  over 4096 bytes is ignored.
- **Values.** `settings.apply` carries the whole set for its scope: every value the user
  set, and a field left out is at its default. Platypus checks each value against the schema
  first and refuses a save with a message for the user when a value has the wrong kind, is
  not one of the options or is out of range. A value for an id the schema doesn't have is
  dropped. Answer `-32602` for a missing `values` or a value you don't take. A save names
  only the fields the user changed; the others keep their saved values, and a field saved
  as nothing goes back to its default.
- **When Platypus sends them.** After each change, and when the plugin starts (once per run,
  after `initialize`), with the values saved so far that the current schema still takes.
  Saves and starts of one plugin run one after another. A plugin that persists them is fine
  too; it just answers the same way.
- **A plugin that refuses them.** If `settings.apply` answers an error, Platypus keeps the
  values saved and tells the user that they were saved but the plugin didn't take them. The
  form keeps what the user chose; it is not an error on the save.
- **No secrets.** Values are kept in Platypus's settings file as plain JSON. Passwords, tokens
  and anything else secret go through `auth.begin` and `auth.complete`, and stay in
  `PLATYPUS_PLUGIN_DATA`. A field of kind `password` or `secret` is not rendered: core drops
  it, and the conformance suite (`internal/plugtest`) rejects one in a plugin's schema.

## Chat (1.9)

The `chat` capability (protocol 1.9) lets the user read and answer the messages of friends, one to one. It needs the
`social` capability too: Platypus calls chat methods only on a plugin that offers both. Group chats, voice, images and
attachments, and searching the history are not part of it.

```
chat.history { friendId, before?, limit } -> { messages: [Message], more }
chat.send { friendId, text, clientId } -> { id, sentAt }   idempotent per clientId
chat.markRead { friendId, upTo } -> {}                      upTo is a message id
chat.unreadCounts {} -> { counts: [{ friendId, unread }] }  optional, 1.13: the unread snapshot

chat.message { friendId, message: Message }   plugin -> Platypus: a new message from a friend (or sent by the user elsewhere)
chat.typing { friendId }                       plugin -> Platypus: the friend is typing (optional, may be left out)
chat.unread { friendId, count }                plugin -> Platypus: unread messages from a friend, whenever it changes
Message = { id, from: "self" | friendId, text, sentAt }
```

- **Text.** `text` is plain UTF-8 and at most 4096 characters. Platypus refuses an empty or longer message before it reaches
  the plugin, and a plugin answers `-32602` to the same. Platypus shows text as plain text, never as HTML. A link is shown
  as text: only `http` and `https` links open in the browser, other schemes stay plain. Platypus cuts an incoming text down
  to 4096 characters rather than dropping the message.
- **Times.** `sentAt` is Unix time in milliseconds. Platypus shows messages in `sentAt` order, so a notification that
  arrives late is put in its place.
- **History.** `chat.history` answers `messages` of the conversation, oldest first. `before` is a message id: the
  page holds the messages older than it. `more` is true when older messages exist. `limit` defaults to 50; a plugin may
  answer fewer than asked.
- **Sending.** Platypus gives every message a fresh `clientId`. A plugin that gets a `clientId` it has already seen answers
  with the id and time of the first send, and does not send twice. Platypus reuses the `clientId` when it retries.
- **Retries.** A send that fails with a transport error (the plugin went away, the call timed out) is retried once. A
  JSON-RPC error answer is not retried: the message stays on screen as failed, and the user can send it again.
- **Read state.** Platypus clears a conversation's unread count when the user opens it and calls `chat.markRead` with the
  newest message it shows. `chat.unread` is the store's own count and replaces Platypus's.
- **Unread snapshot.** `chat.unreadCounts` answers the unread counts of every conversation at once, so the badges are
  right after a restart instead of waiting for the next message. Platypus calls it with `{}` once a chat-capable plugin
  is ready and logged in (at startup, and after a login), and applies each row like a `chat.unread` notification. It is
  optional and 1.13: Platypus sends it only to a plugin that negotiated 1.13 or later, and a plugin without it answers
  `-32601` (Platypus then asks no more for that run). A store that answers an error keeps today's behaviour: the badges
  still come from the `chat.unread` notifications alone.
- **Storage.** The store keeps the history. Platypus keeps the conversations the user opened in memory only, a bounded
  number of messages each, and nothing is written to disk. Message text never goes to the launcher's logs.
- **Gating.** Platypus sends no chat method when `chat` is off for the plugin, when the negotiated protocol is older than
  1.9, or when the plugin doesn't offer `social`.
- **Errors.** An unknown friend is `-32602`. A store that can't reach the friend answers an error, which Platypus shows.

## Compat (1.10)

Protocol 1.10 adds three optional fields to the answer of `game.details`. They
are metadata for the launcher's compatibility facts: the ProtonDB rating of a
game, its native Linux build, and the DRM facts the store states. No capability
and no new method: a plugin sends the fields it knows, and Platypus reads them only
from a plugin that negotiated 1.10 or later.

```
game.details { id } -> { ..., steamAppId?, platforms?, drm? }
drm = { drmFree?, onlineRequired?, launcher?, denuvoLikely?, notes? }
```

- **steamAppId.** The game's Steam AppID, as a string of digits, when the store knows it (for example a GOG or Epic
  game that is also on Steam). Platypus uses it to look up the ProtonDB rating. Platypus never matches a game by its title:
  without a `steamAppId` a game has no rating, unless the user sets one in the game settings, which wins over this
  hint. A value that isn't digits is ignored.
- **platforms.** The store's platform names: `"linux"`, `"windows"`, `"macos"`. A game with `"linux"` is shown as a
  native Linux game, not with a ProtonDB tier.
- **drm.** Every field is optional, and a missing field is unknown: Platypus shows no badge for it, and never a guessed
  one. `drmFree` true is "DRM-free"; `onlineRequired` true is "Online required"; `launcher` names the store launcher
  the game runs through (`"ubisoft-connect"`, `"ea-app"`, or any other name, shown as written); `denuvoLikely` true is
  "Denuvo likely", never "Denuvo", because it is an inference. `notes` is free text, shown on the game page.

Mapping hints for plugins, from their own store fields:

- Epic: `OwnershipToken` → `denuvoLikely`; `CanRunOffline` false → `onlineRequired`; `ThirdPartyManagedApp` →
  `launcher`.
- GOG: `drmFree` true by policy.

- **Gating.** Platypus reads the compat fields only when the negotiated protocol is 1.10 or later. An older plugin's
  `game.details` is read as before, and its compat fields are ignored.
- **Anti-cheat** is not sent by store plugins. The bundled `linux-compat`
  compat-facts provider reads the AreWeAntiCheatYet catalogue (MIT) and scans
  markers in an installed game's folder.

## Compat facts (1.17)

The `compat-facts` capability supplies store-independent compatibility facts in
batches. The core owns caching and background refresh; rendering never waits for
the plugin. Protocol 1.17 is unpublished, so the online-state extension below
remains part of 1.17; it does not introduce a 1.18 version.

```json
compat.lookup { "games": [{ "id": "game-id", "store": "steam",
  "storeAppId": "620", "steamAppId": "620", "platforms": ["windows"],
  "installDir": "/games/example", "online": true }] }
  -> { "facts": [{ "id": "game-id", "badges": [{ "kind": "rating",
    "label": "Platinum", "tone": "platinum", "link": "https://example.test/",
    "source": "ProtonDB", "checkedAt": 1791589136 }], "links": [],
    "report": { "count": 343, "state": "rated", "tier": "platinum" },
    "notes": [], "ttlSeconds": 604800 }] }
```

Each input has a stable game `id`, its store and store app id, and optional
Steam AppID, platforms and install directory. Results correspond by `id`.
Badges carry a kind (`rating`, `verdict`, `protection`, `note`, or a more
specific source kind such as `anticheat`, `eos`, `drm-free`, `denuvo`,
`launcher`, `online` or `drm-removed`), label, optional detail, tone, link,
source and checked date.
Providers may return links and notes without badges. `wiki`, when present,
contains the PCGamingWiki title, page URL, fetched date and licence URL so the
UI can show the credit alongside those facts. `ttlSeconds` controls how long the core waits before
refreshing that game's answer. Matching to ProtonDB is by Steam AppID only;
titles are never used.

- **`installDir`.** The installed game's directory, when known. Providers may
  use it for local scans; it is not a request to read outside that directory.
- **`storeDRM`.** True when `game.details` supplied DRM facts. A provider can
  use it to avoid adding overlapping third-party DRM facts; removed DRM history
  can still be reported.
- **`online`.** Whether network sources may be queried. The core sends false
  when offline or when "Fetch ratings online" is off. Providers return what
  they know without network access, including their cached network facts, and
  refresh local sources such as installed-file detection. The core stores that
  complete answer as an offline snapshot and asks again once online; providers
  own their source caches and combine cached and fresh facts themselves.

An optional `report` summarizes the provider's report count: `state` is
`rated`, `few` or `none`; `count` is the number of reports when known, and
`tier` is the provider's rating when `state` is `rated`. The core preserves
these fields in its cache so the UI can localize the summary.

## Media (1.12)

Protocol 1.12 adds the game page's media and richer facts to the answer of
`game.details`. No capability and no new method: a plugin sends the fields it
knows, and Platypus reads them only from a plugin that negotiated 1.12 or
later.

```
game.details { id } -> { ..., descriptionHtml?, website?, storeUrl?, languages?, controller?, requirements?, ageRating?, tags?, videos? }
requirements = { minimum?, recommended? }
videos = [{ url, thumbnail?, title?, mime? }]
```

- **descriptionHtml.** The store's long description as HTML. Untrusted: core
  sanitizes it to `p`, `br`, `ul`, `ol`, `li`, `b`, `strong`, `i`, `em`,
  `h2`-`h4` and `a[href https]` — no `img`/`iframe`/`style`/`script` — and the
  UI renders only that. The game page shows it collapsed with a "Show more".
- **requirements.** `minimum` and `recommended` are the store's system
  requirements, sanitized like `descriptionHtml` (or plain text). Shown as
  min/recommended tabs.
- **languages.** The store's supported languages as plain names
  (`["English", "French"]`). Shown in the details grid.
- **controller.** `"full"` or `"partial"` when the store states controller
  support; anything else is ignored.
- **ageRating.** Free text like `"12+"`. Shown in the details grid.
- **tags.** Plain tag names (store categories/genres). Shown with the genres.
- **website**, **storeUrl.** `https` URLs, opened in the browser. The store
  page opens in the embedded store view when available, else the browser.
- **videos.** Trailers/clips. `url` must be `https` (or the local art
  server's loopback http); `thumbnail` the same; `mime` is `video/mp4` or
  `video/webm` (guessed from the suffix when missing). Prefer mp4 480/max or
  webm; HLS-only entries are skipped. The media strip shows thumbnails;
  the lightbox plays with controls, muted, never autoplay with sound.
- **Gating.** Platypus reads these fields only when the negotiated protocol
  is 1.12 or later.
## Terms (1.11)

The `terms` capability (protocol 1.11) lists the agreements a store asks the user to accept before a game is installed:
an end user licence, a privacy notice, a sales note. Platypus shows them with a link to each, and the user accepts them or
doesn't. Platypus never accepts for the user.

```
game.details { id } -> { ..., terms: [{ id, title, url?, version }] }
game.acceptTerms { id, terms: [{ id, version }] } -> {}      optional: the store keeps acceptance too
```

- **Listing.** `terms` is read from `game.details`. Each term has an `id` and a `version`, and an optional `url`
  (`https` or `http`, shown as a link that opens in the browser). Platypus keeps the first of two terms with the same
  `id`, drops a term without an `id` or a `version`, and falls back to the id for a missing title. A game with no
  `terms`, or a store that doesn't answer `game.details` (`-32601`), has none. A `version` changes when the text does:
  a new version is a new term to accept, and an old acceptance doesn't count for it.
- **Installs and updates.** Before a fresh install, and before an update, the install dialog lists the terms the user
  hasn't accepted (in their current version) and asks for "I accept" or Cancel. Platypus checks the same before it queues
  the job: an install or update with a term still unaccepted is refused, and nothing is sent to the store. A verify,
  a move or an uninstall asks for nothing. A job that was queued after its acceptance is not asked again when it starts,
  even if a term has changed since.
- **Acceptance.** Platypus records each accepted `(term id, version)` with the date, per game, in its own settings
  (`settings.json`). The Game settings show the accepted terms. Platypus accepts only the terms the store lists now, at the
  version it lists now: a dialog that is out of date can't accept a version that has changed, and is asked again.
- **Server-side acceptance.** A store that keeps acceptance itself implements `game.acceptTerms`, optional. Platypus calls
  it with the terms the user accepted, before it records them. `-32601` means the store doesn't keep one: Platypus keeps
  the record alone. Any other error stops the acceptance, which is not recorded. A store is never asked to accept a
  term the user didn't see.
- **Gating.** Platypus sends no `game.details` for terms, and no `game.acceptTerms`, when `terms` is off for the plugin,
  when the protocol is older than 1.11, or when the game's `features` leave `terms` out. A game whose terms can't be
  read (the store is down) can't be installed, and the install says so; retry later.
- **Install size.** `game.installSize` doesn't carry terms: Platypus reads them from `game.details` only, so the size
  stays a cheap call.

## Locale (1.14)

Protocol 1.14 gives plugins the user's language (issue #39), so store data
comes back translated where stores support it, and plugin UI text can be
translated. No capability: declaring 1.14 in the manifest is the opt-in.

```
initialize { api, protocols, locale } -> { ... }   locale is BCP 47 ("fr-FR", "fr", "en")
locale.changed { locale }                         notification, core -> plugin, no reply
```

- **initialize.** Platypus sends its supported versions plus `locale`, the
  effective BCP 47 tag (the launcher setting, else the first desktop
  language, else English). An old plugin ignores the unknown field; a new
  one localizes from it. The core also passes the locale to the stores
  itself (Steam `l=french`, GOG `locale=fr-FR`).
- **locale.changed.** When the user switches language, Platypus sends this
  notification to every running plugin that negotiated 1.14 or later.
  Plugins on older protocols get the new locale on their next start; the
  core never restarts a plugin for this. Library titles wait for the next
  rescan; the next game page fetches in the new language.
- **The recommended path** is that the plugin returns already-localized
  text for the locale it was given — the simplest for plugin authors, and
  the same for Go and Python. Keep a per-plugin `locales/<lang>.json`
  catalog the plugin loads itself, falling back to English key by key:
  ```json
  { "details_summary": "A tiny fake game…", "auth_instructions": "Type the code…" }
  ```
  ```go
  //go:embed locales/*.json
  var localeFiles embed.FS
  // text(key) looks up the current locale's table, else English.
  ```
  The echo plugin (`plugins/echo`) is the reference: `locales/en.json`
  plus `locales/fr.json`, two strings (`details_summary`,
  `auth_instructions`). The Steam plugin does the same with a small inline
  dict (`mode_note`, login errors). There is no core-side key resolution
  for plugin text in this phase: the plugin localizes, the core passes
  through.
- **Gating.** `locale` is sent to every plugin (old ones ignore it);
  `locale.changed` goes only to plugins that negotiated 1.14 or later.

## Offline launches (1.15)

Only the store plugin can guarantee that a game launches offline: some stores
need their own runtime, client or a cached licence/token. The core's job is to
never stand in the way (issue #44).

- **`LibraryGame.offline`:** `"ok"`, `"network"` or `"unknown"`. Absent means
  `unknown`. It says whether the game launches without a network. The core
  never disables Play because of it: an `"ok"` or `"unknown"` game shows
  nothing, while a `"network"` game shows a small "Needs a connection to
  <store>" chip while offline — and the plugin's own error is shown verbatim
  if the launch fails. The conformance suite rejects other values.
- **`LibraryGame.cloudSync`** (bool): the store itself syncs this game's
  saves. `gameHasCloudSync` uses it for plugin games, alongside the
  `cloud-saves` capability rule, so automatic backups skip such games.
- **`LibraryGame.lastPlayed`** (unix seconds, optional): when the store last
  saw the game run, for stores whose client launches games outside Platypus.
  The library shows the later of it and Platypus's own sessions.
- **The `local-art` capability.** A plugin offering it may return
  `file:///abs/path` in its art fields (cover, hero, banner, logo,
  screenshots and details art). The core rewrites each to
  `/platypus-art/<plugin-id>/<base64url(abs path)>` and serves it with
  `http.ServeFile`, `Cache-Control: max-age=86400` and `nosniff`. It refuses
  non-absolute paths, extensions other than jpg/jpeg/png/webp, and paths
  outside the plugin's own data folder (`PLATYPUS_PLUGIN_DATA`) and its
  declared `permissions.read` folders (checked after `EvalSymlinks`). Without the capability, `file://` is dropped, as before.
- **The manifest `gameIdPrefix`** (see above): optional, default
  `p.<plugin id>.`, only for bundled/first-party plugins. It lets the built-in
  Steam source move into its own plugin with byte-identical ids (issue #54).
- **`game.launchSpec`'s `executable` param** (see [Launching](#launching)).
- **`requires[].optional`** (see the manifest above): optional launch helpers
  are not reported as missing.
- **The core, offline.** Nothing the core does blocks a launch without a
  network: only `cloudBeforeLaunch` sits on the launch path, and while offline
  `cloud.status` is skipped with the note "cloud saves not checked: offline"
  (while online it is bounded by 5 s, and a network-class failure proceeds
  with the same note — never blocked). Background work skips its fetches
  while offline (compat ratings, online art lookups, game info, the save
  manifest refresh, chat/social/playtime refreshes). Each source's last good
  listing is kept in memory and in `<data>/library-snapshots/<source>.json`,
  so a failed source serves its games with a stale warning instead of hiding
  the library.

## Per-game status and notes (1.16)

A store may suggest where its users stand with a game, but only the user owns
their status and notes: Platypus stores both keyed by game id (issue #51) and
never writes them back — no `settings.apply` equivalent exists for them.

- **`library.list`'s `status`** (optional): the store's suggestion for the
  game, one of `"backlog"`, `"playing"`, `"finished"` or `"abandoned"`.
  Absent or anything else means no suggestion: Platypus drops unknown values.
- **`library.list`'s `userNotes`** (optional): the store's suggested notes,
  plain text (no markdown), at most 8192 bytes. Platypus drops longer ones.
- A hint only fills a field the user left empty: the user's own value or
  cleared field always wins. The conformance suite checks the vocabulary and
  the cap.

## Out of scope

Plugins may only install and launch what the user's own account is entitled to,
through the store's own APIs. No DRM or licence circumvention, and no Steam API
emulators.
