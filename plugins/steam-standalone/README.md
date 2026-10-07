# steam-standalone

A Hydra store plugin for Steam without the Steam client, wrapping
[DepotDownloader](https://github.com/SteamRE/DepotDownloader). Install,
update and launch the games your own Steam account owns, with Hydra's runner
pipeline handling Proton, wrappers and session tracking.

See [docs/plugins/protocol.md](../../docs/plugins/protocol.md) for the
protocol and [docs/plans/launcher-plugins.md](../../docs/plans/launcher-plugins.md)
for the design ("Steam without the Steam client").

## Requirements

- `depotdownloader` on PATH — get the linux-x64 binary from the
  [DepotDownloader releases](https://github.com/SteamRE/DepotDownloader/releases)
  and put it on your PATH as `depotdownloader`.
- `dotnet` runtime 8 or newer (DepotDownloader needs it).
- The Steam **client** is *not* required, except for games that need it at
  runtime (see launch modes below).

## Install it

```sh
plugins/install.sh steam-standalone
```

Then approve it in Hydra's Plugins view and log in with your Steam account.

## Auth

Login goes through DepotDownloader itself: the plugin feeds your account
name, password and (when Steam asks) the Steam Guard code to one
DepotDownloader run with `-remember-password`. The password is used for that
run only and never written anywhere. Steam issues a login key, which
DepotDownloader persists in .NET isolated storage created under the plugin's
private `HYDRA_PLUGIN_DATA` folder (the plugin points `HOME` there), so you
don't log in every time and Hydra never sees credentials. Logging out wipes
that folder.

The library listing calls Steam's Web API (`IPlayerService/GetOwnedGames`).
The stored refresh token isn't accepted there, so `bin/steam_cm.py` logs on to
a Steam CM with it and exchanges it for a short-lived access token (cached in
the plugin's data folder). The list shows everything the account owns,
installed or not. DepotDownloader runs always pass `-username` with the
account name, because it keys the stored login by name.

## Install

Downloads go to `<Games folder>/steam/<game name>/`, one folder per game like
Steam's `steamapps/common`. Sizes for Hydra's free-space check come from a
`-manifest-only` run (manifests only, no game data). Installs are resumable:
DepotDownloader skips chunks it already has, and Hydra re-issues the same
job after a crash. Downloads use 24 parallel connections
(`HYDRA_STEAM_MAX_DOWNLOADS` to change) and the Steam client's download
region when one is set (`HYDRA_STEAM_CELLID` to override).

## Launch modes

Steam games fall into three cases, detected per game from the installed
files, shown on the game page:

1. **No Steamworks** — no `steam_api*.dll` / `libsteam_api.so` in the
   install. The game launches directly; fully launcher-free.
2. **Steamworks optional** — Steamworks files exist, but most such games
   only use them for achievements or overlay and run fine when
   `SteamAPI_Init` fails. The game launches directly with `SteamAppId` /
   `SteamGameId` set so it doesn't try to relaunch itself through Steam.
3. **Client required** — the game needs the real Steam client (fatal
   `SteamAPI_Init` failure or Steam's DRM wrapper). Hydra starts the real
   client hidden (`steam -silent`) and asks it to start the game; the game
   runs against the live client for its licence check. No Steam API
   emulators are used or supported, ever.

Detection between 2 and 3 can't be perfect by inspection. The plugin tries
direct first; if the game dies within seconds of launch, retrying marks it
"needs client" (the game page offers this) and mode 3 is used from then on.
