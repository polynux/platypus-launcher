# Store plugin protocol (api 1, draft)

A store plugin is a folder in `~/.local/share/hydra/plugins/<id>/` with a
`plugin.json` and an executable. Hydra starts the executable on demand and talks
JSON-RPC 2.0 over its stdin/stdout, one JSON object per line. Anything the plugin
writes to stderr goes to `~/.local/share/hydra/logs/plugin-<id>.log`.

The protocol is still a draft: it can change until GOG and Amazon plugins work
without core changes (plan milestone 4). Working examples: `plugins/echo` (Go, a
fake store) and `plugins/epic-legendary` (Python, wraps legendary).

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
  "capabilities": ["library", "install", "launch", "auth"],
  "requires": [{ "bin": "legendary", "hint": "pipx install legendary-gl" }],
  "install_dir": "epic",
  "permissions": { "network": ["*.epicgames.com"], "read": ["~/.config/legendary"], "write": ["~/.config/legendary"] }
}
```

- `id` must match the folder name; `steam` and `custom` are reserved.
- `exec` is relative and must stay inside the folder (symlinks included).
- `requires` lists command-line tools looked up on `PATH`. A missing one keeps the
  plugin from starting; the Plugins view shows the hint.
- `permissions` are shown to the user before approval. They aren't enforced yet
  (bubblewrap sandbox, plan milestone 7).

Hydra runs nothing until the user approves the plugin, which pins the SHA-256 of
`exec`. If the file changes, the plugin is disabled until the user approves it again.
To update a plugin, replace the file (write a new one and rename it over the old one);
Linux refuses to overwrite a running executable in place.

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
for people. Use code `-32001` when the user has to log in first.

| Method | Params | Result |
|---|---|---|
| `initialize` | `{ api }` | `{ name, version, api, capabilities, missing }` |
| `auth.status` | | `{ loggedIn, user? }` |
| `auth.begin` | | `{ url?, instructions?, fields: [{ id, label, secret?, placeholder? }] }` |
| `auth.complete` | `{ fields: { <id>: value } }` | `{ loggedIn, user? }` |
| `auth.logout` | | `{ loggedIn }` |
| `library.list` | | `{ games: [LibraryGame] }` |
| `game.installSize` | `{ id }` | `{ downloadBytes, diskBytes }` |
| `game.install` | `{ id, installDir, jobId }` | `{}` once started |
| `game.update` | `{ id, installDir, jobId }` | `{}` once started |
| `game.uninstall` | `{ id, jobId }` | `{}` once started |
| `job.cancel` | `{ jobId }` | `{}` |
| `game.launchSpec` | `{ id, preview }` | `LaunchSpec` |

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
  "coverUrl": "https://…", "heroUrl": "https://…", "bannerUrl": "https://…"
}
```

Hydra keys plugin games as `p.<plugin id>.<game id>`. List every owned game; the
Library shows the uninstalled ones under "owned".

## Jobs

Hydra creates the `jobId`, picks `installDir` (the user's Games folder plus the
plugin's `install_dir`, or the user's override) and checks free space against
`game.installSize` before calling `game.install`. Install into `installDir`, never
anywhere else. Report progress with notifications:

| Notification | Params |
|---|---|
| `job.progress` | `{ jobId, phase?, bytes?, total?, speed?, percent? }` (speed in bytes/s) |
| `job.done` | `{ jobId }` |
| `job.failed` | `{ jobId, error }` |
| `log` | `{ level, message }` (goes to the plugin's log file) |

Hydra runs one job at a time and saves the queue. If Hydra or the plugin dies
mid-install, Hydra calls `game.install` again with the same `jobId`, so installs
must be resumable or at least idempotent. After `job.cancel`, stop the work; Hydra
has already marked the job canceled and ignores later notifications for it.

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
