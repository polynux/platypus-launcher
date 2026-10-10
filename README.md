# Platypus

Platypus is a game launcher for Linux. It puts the games of several stores in one library, gives each game its
own Proton/Wine runner settings, community fixes and cloud-save handling, and talks to the stores through
plugins: a store plugin is a small program that speaks the JSON-RPC protocol in
[docs/plugins/protocol.md](docs/plugins/protocol.md). The repository is
https://github.com/polynux/platypus-launcher.

Platypus keeps its data in `~/.local/share/platypus`.

## Development

The app is built with [Wails](https://wails.io) (Go backend, React + TypeScript frontend).

```sh
cd frontend && bun install && bun run build && cd ..   # the Go tests embed frontend/dist
wails dev -tags webkit2_41                             # live development
go test ./...                                          # backend and plugins
cd frontend && bunx tsc --noEmit && bunx vitest run    # frontend
wails build -tags webkit2_41                           # a redistributable build
```

Store plugins live in `plugins/`; `plugins/install.sh <name>` installs one into
`~/.local/share/platypus/plugins/` (set `PLATYPUS_PLUGINS` to install somewhere else), and `plugins/new.sh`
scaffolds a new one. Platypus lists a new or changed plugin as unapproved until you approve it in the Plugins
view.

## License

Platypus is released under the [Apache License 2.0](LICENSE). Copyright 2026 Guillaume Dorce.

## Third-party data

Platypus shows compatibility and protection facts it reads from other projects. Their data stays theirs:

- [ProtonDB](https://www.protondb.com): community compatibility reports, read from its public pages.
- [AreWeAntiCheatYet](https://areweanticheatyet.com): anti-cheat status of games, MIT licensed.
- [PCGamingWiki](https://www.pcgamingwiki.com): DRM and anti-cheat facts, licensed
  [CC BY-NC-SA 3.0](https://creativecommons.org/licenses/by-nc-sa/3.0/) and used here non-commercially; the
  facts are credited in the interface where they are shown.
