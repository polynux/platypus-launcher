# Platypus

Platypus is a game launcher for Linux. It puts the games of several stores in one library, gives each game its
own Proton/Wine runner settings, community fixes and cloud-save handling, and talks to the stores through
plugins: a store plugin is a small program that speaks the JSON-RPC protocol in
[docs/plugins/protocol.md](docs/plugins/protocol.md). The repository is
https://github.com/polynux/platypus-launcher.

Platypus keeps its data in `~/.local/share/platypus`.

## Development

The app is a Go core (`cmd/platypus-core`) with a React + TypeScript UI (`apps/ui`) inside a desktop shell
(`apps/desktop`, Electron + Chromium). The UI talks to the core over stdio JSON-RPC through `window.platypus`;
the typed bridge is generated with `go run ./cmd/gen-ts` into `apps/ui/src/gen` (see
[docs/electron.md](docs/electron.md)).

```sh
bun install                           # once: the apps/ui and apps/desktop workspaces
bun run dev                           # desktop app with the UI from Vite (hot reload)
bun run start                         # desktop app with the built UI (builds it if missing)
go test ./...                         # backend and plugins
bun run test:ui                       # UI tests (vitest in apps/ui)
bun run test:desktop                  # shell tests (node --test in apps/desktop)
bun run typecheck                     # tsc in both apps
bun run gen-ts                        # regenerate apps/ui/src/gen after changing the core surface
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
