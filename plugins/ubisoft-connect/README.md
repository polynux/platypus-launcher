# Ubisoft Connect (phase A skeleton)

Ubisoft games through the official Ubisoft Connect client in one
Platypus-managed Wine prefix. The client owns installs and launches; this
plugin only orchestrates it and reads its on-disk state.
See `docs/research/ubisoft-options.md` and issues
[#10](https://github.com/polynux/platypus-launcher/issues/10),
[#193](https://github.com/polynux/platypus-launcher/issues/193),
[#194](https://github.com/polynux/platypus-launcher/issues/194).

## Verified vs pending (live checks L-U1/L-U2, #195)

Verified without an account (fake `wine` in `testdata/`, no network):

- `client.setup` answers the pinned installer spec (SHA-256 checked).
- `game.launchSpec` answers the `wine-client` delegate
  (`upc.exe uplay://launch/<id>/0`, `find` = the game's exe).
- `game.clientStatus` mapping (not set up / waiting / launching).
- Install/uninstall job lifecycle through the fake client, incl. cancel and
  repeated job ids.

Pending a real prefix and a Ubisoft account (untouched by this skeleton):

- L-U1: the Connect install folder (`upc.exe` path), login webview under our
  Wine/Proton, cache/ownership file locations, `uplay://` reachability
  (direct exe invocation vs `wine start`), process matching for `find`.
- L-U2: one small game install + launch, and the real install-record
  location/format replacing `drive_c/platypus-ubisoft/records/<id>.json`.
- Blocked: the local-cache library reader (#192, needs the licence
  decision #197), phase B web API (needs the product decision Q-U1).

## Pinned installer

`client.setup` downloads
`https://ubistatic3-a.akamaihd.net/orbit/launcher_installer/UbisoftConnectInstaller.exe`
(silent `/S`, Lutris `ubisoft-connect-latest` recipe) and refuses anything
whose SHA-256 differs from the pin in `bin/platypus-ubisoft`
(`INSTALLER_SHA256`, `INSTALLER_SIZE`).

To bump the pin: download the new installer once from the same URL,
`sha256sum` it, replace the hash, size and date in the docstring, delete the
download. Never commit the binary.

The recipe also applies winetricks `d3dcompiler_43` + `corefonts` and disables
the overlay; that is core runner work, not plugin work.

## Test hooks (never set in production)

- `PLATYPUS_UBISOFT_INSTALLER_FILE`: file to use as the installer instead of
  downloading (offline tests).
- `PLATYPUS_UBISOFT_WATCH_TIMEOUT`: install-record watch bound in seconds.
- `FAKE_WINE_DELAY` / `FAKE_WINE_EXE` / `FAKE_WINE_FAIL` / `FAKE_WINE_CALLS`:
  read by `testdata/wine`.
