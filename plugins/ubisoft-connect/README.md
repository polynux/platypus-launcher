# Ubisoft Connect (library + setup + launch + client-run installs, protocol 1.25)

Ubisoft games through the official Ubisoft Connect client in one
Platypus-managed Wine prefix. The client owns installs and launches; this
plugin answers the installer spec, the wine-client launch spec, the
client-run install/uninstall commands, the standalone client spec for the
plugin card's Open button, and reads the client's on-disk state:
the local-cache library reader is issue #192, written clean-room from
`docs/research/ubisoft-library-format.md`. See `docs/research/ubisoft-options.md`
and issues
[#193](https://github.com/polynux/platypus-launcher/issues/193),
[#194](https://github.com/polynux/platypus-launcher/issues/194),
[#212](https://github.com/polynux/platypus-launcher/issues/212),
[#213](https://github.com/polynux/platypus-launcher/issues/213).

The plugin starts no process of its own: only the core runs Wine (installer,
client, client commands), from the specs this plugin answers. It speaks
protocol 1.20, 1.23 and 1.25 (the `install` capability is on only over 1.23).

## Verified vs pending (live checks L-U1/L-U2, #195)

Verified with fakes and loopback (no network, no Wine, no account):

- `client.setup` downloads the pinned installer through the real fetch path
  (loopback server, patched pin), checks size + SHA-256, caches it under
  `PLATYPUS_PLUGIN_DATA`, and answers the prefix copy spec.
- `game.launchSpec` answers the `wine-client` delegate without the client
  installed (so the core reports `client.notSetUp`), with the preferred
  `UbisoftConnect.exe` path, `uplay://launch/<id>/0`, `find` = the game's
  exe, and `startTimeout` 900.
- `game.install`/`game.update`/`game.uninstall` answer the `clientRun`
  command (`uplay://install/<id>` / `uplay://uninstall/<id>`) with
  `clientOwnsLocation`, and end the job from the client's on-disk state
  (the install registry gaining `InstallDir`, the catalogue cache being
  rewritten, or the install key disappearing; bounded wait, readable failure
  on timeout, silent on cancel).
- `library.list` reads the client's caches in the prefix: the product
  catalogue (`cache/configuration/configurations`), the account's owned ids
  (`cache/ownership/<userId>`) and the install registry in Wine's
  `system.reg` (`HKLM\Software\Wow6432Node\Ubisoft\Launcher\Installs\<id>\InstallDir`,
  plus the unredirected key). Owned games are listed once; DLC/ULC products,
  other stores' copies (`third_party_platform`) and products with no game
  executable are filtered out; an installed game carries the registry path.
  A game's launch follows `uplay://launch/<id>/0` and tracks the catalogue's
  executable. A cache that is missing, truncated/garbage, or the October 2026
  encrypted `.vdat` container answers one clear, localized status (-32002):
  "Start Ubisoft Connect once and sign in so Platypus can read your games"
  ("Lancez Ubisoft Connect une fois et connectez-vous…" in French); it is not
  an error per call and never crashes on garbage.

The library reader is written only from the public formats in
`docs/research/ubisoft-library-format.md` (issue #197 leaves the project
licence open, so no code was ported from any project).

Pending a real prefix and a Ubisoft account (untouched by this plugin):

- L-U1: the Connect install folder (`UbisoftConnect.exe` vs `upc.exe` is a
  guess until then), login webview under our Wine/Proton, the cache paths
  and ownership file name on a real account, whether `uplay://launch` takes
  the catalogue's product id, and process matching for `find`.
- L-U2: one small game install + launch, and whether the client rewrites the
  catalogue cache on install/update (the watch also keys on the install
  registry, so an install still completes without it).
- Blocked: phase B web API (needs product decision Q-U1); reading the
  encrypted `.vdat` cache (needs the prefix's DPAPI user secrets, so the
  entitlement endpoint is the fallback).

## Pinned installer

`client.setup` downloads
`https://ubistatic3-a.akamaihd.net/orbit/launcher_installer/UbisoftConnectInstaller.exe`
(silent `/S`, Lutris `ubisoft-connect-latest` recipe) and refuses anything
whose size or SHA-256 differs from the pin in `bin/platypus-ubisoft`
(`INSTALLER_SHA256`, `INSTALLER_SIZE`).

To bump the pin: download the new installer once from the same URL,
`sha256sum` it, replace the hash, size and date in the docstring, delete the
download. Never commit the binary.

Nothing applies the rest of the Lutris recipe yet: winetricks
`d3dcompiler_43` + `corefonts`, and the overlay-off `settings.yaml`, stay as
noted in #195.

## Test hooks (never set in production)

- `PLATYPUS_UBISOFT_INSTALLER_URL` / `_SHA256` / `_SIZE`: override the
  installer pin, so tests drive the real fetch path through a loopback
  server.
- `PLATYPUS_UBISOFT_WATCH_TIMEOUT` / `_WATCH_POLL`: bound the
  install/uninstall watch.
- `PLATYPUS_UBISOFT_SIGNIN_GRACE`: overrides the signin grace.
