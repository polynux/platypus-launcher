# Packaging (issue #41)

Platypus's own folders resolve from the environment, with today's paths as
the default. A packager sets variables; a user overrides the same way.

## Variables

| Folder | Precedence |
|---|---|
| data | `$PLATYPUS_DATA_DIR`, else `$XDG_DATA_HOME/platypus`, else `~/.local/share/platypus` |
| config (resolver only) | `$PLATYPUS_CONFIG_DIR`, else `$XDG_CONFIG_HOME/platypus`, else `~/.config/platypus` |
| cache (re-fetchable only) | `$PLATYPUS_CACHE_DIR`, else `$XDG_CACHE_HOME/platypus`, else `~/.cache/platypus` |
| plugins | `$PLATYPUS_PLUGINS`, else `<data>/plugins` (the core honours it the same way `plugins/install.sh` does) |

An XDG variable counts only when set to an absolute path; relative or empty
means unset, per the spec. `settings.json` stays in the data dir; only the
game-info cache (re-fetchable) lives in the cache dir. The Electron shell
keeps its default `userData` unless `PLATYPUS_CONFIG_DIR` (else
`PLATYPUS_DATA_DIR`) is set, in which case it uses a `platypus-shell`
subfolder of it, and passes `PLATYPUS_*` through to the core it spawns.

With no variables set every path is byte-for-byte what it is today. When
`XDG_DATA_HOME` points elsewhere but only `~/.local/share/platypus` exists,
the legacy folder wins with one info log; data is never moved automatically.

## What a packager sets

Flatpak example (config + data + cache redirected, plugins alongside data):

```sh
PLATYPUS_DATA_DIR=/var/data/platypus \
PLATYPUS_CONFIG_DIR=/var/config/platypus \
PLATYPUS_CACHE_DIR=/var/cache/platypus \
platypus
```

`PLATYPUS_PLUGINS` only when the plugin folder must live apart from data.
Steam installs are still found at `~/.local/share/Steam` first;
`$XDG_DATA_HOME/Steam` joins as an extra candidate. `PLATYPUS_STEAM_ROOT`
pins the Steam lookup to one folder (packagers, test fixtures).
