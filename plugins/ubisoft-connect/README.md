# Ubisoft Connect (phase A skeleton: setup + launch)

Ubisoft games through the official Ubisoft Connect client in one
Platypus-managed Wine prefix. The client owns installs and launches; this
plugin answers the installer spec and the wine-client launch spec, and reads
the client's on-disk state.
See `docs/research/ubisoft-options.md` and issues
[#10](https://github.com/polynux/platypus-launcher/issues/10),
[#193](https://github.com/polynux/platypus-launcher/issues/193),
[#194](https://github.com/polynux/platypus-launcher/issues/194).

The plugin starts no process of its own: only the core runs Wine (installer
and client), from the specs this plugin answers.

## Verified vs pending (live checks L-U1/L-U2, #195)

Verified with fakes and loopback (no network, no Wine, no account):

- `client.setup` downloads the pinned installer through the real fetch path
  (loopback server, patched pin), checks size + SHA-256, caches it under
  `PLATYPUS_PLUGIN_DATA`, and answers the prefix copy spec.
- `game.launchSpec` answers the `wine-client` delegate without the client
  installed (so the core reports `client.notSetUp`), with the preferred
  `UbisoftConnect.exe` path, `uplay://launch/<id>/0`, `find` = the game's
  exe, and `startTimeout` 900.
- `game.clientStatus` mapping: `idle` (no client) / `updating` (waiting) /
  `launching` (installed), incl. the Proton layout and the `upc.exe`
  fallback.
- The pin-mismatch paths (wrong SHA-256, wrong size) fail instead of
  installing anything.

Pending a real prefix and a Ubisoft account (untouched by this skeleton):

- L-U1: the Connect install folder (`UbisoftConnect.exe` vs `upc.exe` is a
  guess until then), login webview under our Wine/Proton, cache/ownership
  file locations, `uplay://` reachability, process matching for `find`.
- L-U2: one small game install + launch, and the real install-record
  location/format replacing `drive_c/platypus-ubisoft/records/<id>.json`.
- Blocked: the local-cache library reader (#192, needs the licence
  decision #197); installs/uninstalls through the client (core-run
  client-commands issue below); phase B web API (needs product decision
  Q-U1); a waiting-for-signin `clientStatus` state (protocol proposal,
  see below).

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
`d3dcompiler_43` + `corefonts`, and the overlay-off `settings.yaml`, have no
home — they belong to the core-run setup/client work, tracked in the
 core-run client-commands issue #212 and on #195.

## Installs, uninstalls, and protocol proposals

`game.install`/`game.uninstall` are deliberately absent: the plugin must not
run Wine itself. They wait on the core-run client-commands issue
 ("Core-run client commands for wine-client stores" #212),
which lets those methods answer a wine-client spec the core runs as part of
the plugin's job. Likewise, while a first-run client self-updates or waits
for sign-in, no current `clientStatus` state fits: that waits on the
 protocol issue #213 (refs #191 #195).

## Test hooks (never set in production)

- `PLATYPUS_UBISOFT_INSTALLER_URL` / `_SHA256` / `_SIZE`: override the
  installer pin, so tests drive the real fetch path through a loopback
  server.
