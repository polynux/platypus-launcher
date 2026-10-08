# Review fixes (2026-10-08)

Full code review of the launcher and plugins, run on `feat/steam-depot-dl` at b0d32c4. Four reviewers, one report per area:
- [core](../reviews/core.md) (CORE-*)
- [frontend](../reviews/frontend.md) (FE-*)
- [steam](../reviews/steam.md) (STEAM-*)
- [plugins](../reviews/plugins.md) (PLUG-*)

Each finding names its file and line, a scenario, the fix and a test. This plan only groups them into workstreams that don't share files, so they can run in parallel worktrees.

## Workstreams

| WS | Branch / worktree | Findings | Files owned |
|---|---|---|---|
| A, Steam downloader | `fix/steam-dl` / `~/dev/hydra-fix-steam-dl` | STEAM-1, 2, 3, 4 (Go side), 5, 6, 10, 11, 12, 14, 17 | `internal/depot`, `internal/steamcm`, `plugins/steam-standalone/cmd/hydra-steam-dl` |
| B, Plugins (Python and scripts) | `fix/plugins` / `~/dev/hydra-fix-plugins` | STEAM-7, 8, 9, 13, 15, 16, 18; PLUG-1 to 9 | `plugins/*/bin/*`, `plugins/install.sh`, `plugins/new.sh`, `internal/plugtest`, `docs/plugins/protocol.md` |
| C, Core and sessions | `fix/core` / `~/dev/hydra-fix-core` | CORE-1 to 15, FE-2 (the frontend half of CORE-1) | `*.go` at the root, `internal/{plugins,jobs,session,runner,settings,steam,stores,dataplug,fixes,types,jsonrpc}`, plus the session parts of `frontend/src/App.tsx` |
| D, Frontend | `fix/frontend` / `~/dev/hydra-fix-frontend` | FE-1, 3 to 14 | `frontend/src` (not the session wiring that C owns) |

Shared edges:
- **CORE-1 / FE-2.** C adds the backend snapshot API, regenerates the wailsjs bindings, and wires the frontend for it. D doesn't touch the session code.
- **STEAM-4 (branch passwords).** A owns the Go side. If the fix needs a plugin parameter, B passes it. Otherwise A removes the dead path.
- **steam-standalone `plugin.json` and `install.sh`.** B owns them. A states any needs in its report.
- **Tests.** Each workstream runs its own packages, plus a final cold `go test ./... -count=1`. Known flake (CORE-2): internal/plugins TestGivesUpAfterRepeatedCrashes, which C fixes.

## Merge order
1. `feat/steam-depot-dl`, once the live password login passes.
2. A, B, C and D, rebased in that order, with the full checks after each.

## Rules for every workstream
- No DRM or licence bypass.
- Never log or print secrets, and never put them in argv.
- Never `git add -A`; one commit per finding or tight group.
- Tests go first where the report names one: it must fail on the old code.
- No `wails dev`. Port 34115 belongs to the user's session in `~/dev/hydra-steam-depot-dl`, and that worktree must not be edited.
- No real logins or downloads.
- If a finding turns out to be wrong, say so in the final report and skip it.
