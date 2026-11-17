# Ubisoft Connect (setup + launch + client-run installs, protocol 1.23)

Ubisoft games through the official Ubisoft Connect client in one
Platypus-managed Wine prefix. The client owns installs and launches; this
plugin answers the installer spec, the wine-client launch spec, and the
client-run install/uninstall commands, and reads the client's on-disk state.
See `docs/research/ubisoft-options.md` and issues
[#193](https://github.com/polynux/platypus-launcher/issues/193),
[#194](https://github.com/polynux/platypus-launcher/issues/194),
[#212](https://github.com/polynux/platypus-launcher/issues/212),
[#213](https://github.com/polynux/platypus-launcher/issues/213).

The plugin starts no process of its own: only the core runs Wine (installer,
client, client commands), from the specs this plugin answers. It speaks
protocol 1.20 and 1.23 (the `install` capability is on only over 1.23).

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
  `clientOwnsLocation`, and end the job from the client's on-disk records
  (bounded wait, readable failure on timeout, silent on cancel). Installed
  games appear in the library only once the local-cache reader lands
  (#192): until then a finished install has nowhere to show.
Pending a real prefix and a Ubisoft account (untouched by this plugin):

- L-U1: the Connect install folder (`UbisoftConnect.exe` vs `upc.exe` is a
  guess until then), login webview under our Wine/Proton, cache/ownership
  file locations, `uplay://` reachability, process matching for `find`.
- L-U2: one small game install + launch, and the real install-record
  location/format replacing `drive_c/platypus-ubisoft/records/<id>.json`.
- Blocked: the local-cache library reader (#192, needs the licence
  decision #197); phase B web API (needs product decision Q-U1).

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
