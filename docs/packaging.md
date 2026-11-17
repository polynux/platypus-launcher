# Packaging (issues #41, #82)

Platypus's own folders resolve from the environment, with the platform's
native folders as the default. A packager sets variables; a user overrides
the same way.

## Variables

| Folder | Precedence |
|---|---|
| data | `$PLATYPUS_DATA_DIR`, else the default below |
| config (resolver only) | `$PLATYPUS_CONFIG_DIR`, else the default below |
| cache (re-fetchable only) | `$PLATYPUS_CACHE_DIR`, else the default below |
| plugins | `$PLATYPUS_PLUGINS`, else `<data>/plugins` (the core honours it the same way `plugins/install.sh` does) |

The platform defaults are resolved in `internal/paths` (issue #82):

| Folder | Linux | Windows |
|---|---|---|
| data | `$XDG_DATA_HOME/platypus`, else `~/.local/share/platypus` | `%LOCALAPPDATA%\platypus` (`os.UserCacheDir`) |
| config | `$XDG_CONFIG_HOME/platypus`, else `~/.config/platypus` | `%APPDATA%\platypus` (`os.UserConfigDir`) |
| cache | `$XDG_CACHE_HOME/platypus`, else `~/.cache/platypus` | `%LOCALAPPDATA%\platypus\cache` |

On Windows, config roams with the user's profile and only holds settings;
data is machine-local (backups, the library and prefixes must not follow the
user to another machine), so it lives under the local app data folder. Go
exposes that folder only as `os.UserCacheDir`, and the cache nests under data
so the two never resolve alike.

An XDG variable counts only when set to an absolute path; relative or empty
means unset, per the spec. `settings.json` stays in the data dir; only the
game-info cache (re-fetchable) lives in the cache dir. The Electron shell
keeps its default `userData` unless `PLATYPUS_CONFIG_DIR` (else
`PLATYPUS_DATA_DIR`) is set, in which case it uses a `platypus-shell`
subfolder of it, and passes `PLATYPUS_*` through to the core it spawns.

With no variables set every path is byte-for-byte what it is today on Linux.
When `XDG_DATA_HOME` points elsewhere but only `~/.local/share/platypus`
exists, the legacy folder wins with one info log; data is never moved
automatically (Windows has no legacy folder to fall back to).

## What a packager sets

Flatpak example (config + data + cache redirected, plugins alongside data):

```sh
PLATYPUS_DATA_DIR=/var/data/platypus \
PLATYPUS_CONFIG_DIR=/var/config/platypus \
PLATYPUS_CACHE_DIR=/var/cache/platypus \
platypus
```

`PLATYPUS_PLUGINS` only when the plugin folder must live apart from data.
Steam install roots are not in the generic resolver (issue #82): the Steam
plugins report the roots a game needs (`game.saveContext`) and `internal/steam`,
which the plugins themselves import, finds the install at
`~/.local/share/Steam` first with `$XDG_DATA_HOME/Steam` as an extra
candidate. `PLATYPUS_STEAM_ROOT` pins the Steam lookup to one folder
(packagers, test fixtures).

## Save discovery

Save backup discovery uses the store plugin's roots (`game.saveContext`) plus
the OS-native common folders from `internal/paths.SaveDirsFor`: `home`,
`xdgData` and `xdgConfig` on Linux; `home`, `documents`, `appData`,
`localAppData`, `localAppDataLow`, `public`, `programData` and `windir` on
Windows. Ludusavi's `<win…>` placeholders expand inside a Wine/Proton prefix's
`drive_c` when there is one; a native game on a Windows host uses those
folders directly, and on Linux they stay unresolved.

## Bundled plugins (issue #54)

The distributable ships the Steam client source as a ready plugin folder,
and the core auto-installs it on startup when no `steam` plugin is
installed — existing users lose nothing. The dev run builds it too, so
`bun run dev` behaves like the packaged app.

- Source: `$PLATYPUS_BUNDLED_PLUGINS` (a folder of ready plugin folders),
  else `<dir of the core executable>/plugins`. The dev script
  (`apps/desktop/scripts/run.mjs`) builds into `apps/desktop/.build/bundled-plugins`
  and points the core at it; the packaged build
  (`apps/desktop/scripts/package.mjs` + `packaging/electron-builder.yml`)
  stages `packaging/stage/bundled-plugins` next to the core binary.
  Both go through `apps/desktop/scripts/bundled-plugins.mjs`.
- Install: atomic (temp dir + rename) into `<plugins>/steam`, then approved
  with trust pre-seeded for that manifest hash. It never deletes or
  overwrites a different plugin folder: an existing `steam` folder whose
  manifest isn't the steam plugin is left alone with a warning.
- Updates: when the bundled copy's manifest version is newer than the
  installed one, it replaces it and re-pins the hash, keeping the user's
  choices (stays disabled when disabled, keeps its install folder and
  allowed folders). A user install from `plugins/install.sh` with the same
  version (even a modified one, different hash) or a newer version is left
  alone in dev: bump the bundled version to roll it out.
- Game ids stay `steam_<appid>` (the manifest `gameIdPrefix`) and snapshots
  from before the move have their art URLs mapped, so no data migration.

## umu-launcher (issue #48)

Windows games can run through umu-launcher (see `docs/umu.md`); Platypus
uses it when `umu-run` is installed and falls back to a direct Proton/Wine
launch otherwise. Recommendation: depend on the distro's `umu-launcher`
package (`pacman -S umu-launcher` on Arch, `dnf install umu-launcher` on
Nobara, `pkgs.umu-launcher` on NixOS) rather than bundling it. Where no
distro package exists (e.g. inside a Flatpak), bundle the `umu-run` zipapp
with its Python dependencies instead; Platypus never writes to the
Lutris-bundled copy under `~/.local/share/lutris/runtime/umu`.
