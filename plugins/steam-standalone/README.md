# steam-standalone

A Platypus store plugin for Steam without the Steam client, with its own Go
depot downloader and Steam login. Install, update and launch the games
your own Steam account owns, with Platypus's runner pipeline handling
Proton, wrappers and session tracking.

See [docs/plugins/protocol.md](../../docs/plugins/protocol.md) for the
protocol and [docs/plans/launcher-plugins.md](../../docs/plans/launcher-plugins.md)
for the design ("Steam without the Steam client").

## Requirements

- `python3` from your distribution (the plugin exec).
- The Steam **client** is *not* required, except for games that need it at
  runtime (see launch modes below).

## Install it

```sh
plugins/install.sh steam-standalone
```

Then approve it in Platypus's Plugins view and log in with your Steam account.

## Auth

Login goes through platypus-steam-dl (Go, in the plugin's `bin/`): QR via
`BeginAuthSessionViaQR` + poll, or your account name, password and (when
Steam asks) the Steam Guard code via `GetPasswordRSAPublicKey` +
`BeginAuthSessionViaCredentials` (RSA-encrypted, never stored). The
password is used for that run only and never written anywhere. Steam
issues a refresh token, which the helper persists in the plugin's private
`PLATYPUS_PLUGIN_DATA` folder (`refresh.token` 0600, plus `token.json` /
`session.json`), so you don't log in every time and Platypus never sees
credentials. Logging out wipes that folder.

The library listing calls Steam's Web API (`IPlayerService/GetOwnedGames`).
The stored refresh token isn't accepted there, so `bin/steam_cm.py` logs on to
a Steam CM with it and exchanges it for a short-lived access token (cached in
the plugin's data folder). The list shows everything the account owns,
installed or not.

## Install

Downloads go to `<Games folder>/steam/<game name>/`, one folder per game like
Steam's `steamapps/common`. Sizes for Platypus's free-space check come from the
Go downloader resolving the app's depots over the CM and summing the
manifests (Linux depots first, Windows fallback — the same order an install
uses). Installs are resumable: staged chunks survive a crash under
`PLATYPUS_PLUGIN_DATA/jobs/`, and Platypus re-issues the same job after a crash.
The scheduler spreads chunk fetches over many CDN hosts like the Steam
client ( honouring the Steam client's download region, `PLATYPUS_STEAM_CELLID`
to override).

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
   `SteamAPI_Init` failure or Steam's DRM wrapper). Platypus starts the real
   client hidden (`steam -silent`) and asks it to start the game; the game
   runs against the live client for its licence check. No Steam API
   emulators are used or supported, ever.

Detection between 2 and 3 can't be perfect by inspection. The plugin tries
direct first; if the game dies within seconds of launch, retrying marks it
"needs client" (the game page offers this) and mode 3 is used from then on.
