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
| A, Steam downloader | `fix/steam-dl` / `~/dev/platypus-fix-steam-dl` | STEAM-1, 2, 3, 4 (Go side), 5, 6, 10, 11, 12, 14, 17 | `internal/depot`, `internal/steamcm`, `plugins/steam-standalone/cmd/platypus-steam-dl` |
| B, Plugins (Python and scripts) | `fix/plugins` / `~/dev/platypus-fix-plugins` | STEAM-7, 8, 9, 13, 15, 16, 18; PLUG-1 to 9 | `plugins/*/bin/*`, `plugins/install.sh`, `plugins/new.sh`, `internal/plugtest`, `docs/plugins/protocol.md` |
| C, Core and sessions | `fix/core` / `~/dev/platypus-fix-core` | CORE-1 to 15, FE-2 (the frontend half of CORE-1) | `*.go` at the root, `internal/{plugins,jobs,session,runner,settings,steam,stores,dataplug,fixes,types,jsonrpc}`, plus the session parts of `frontend/src/App.tsx` |
| D, Frontend | `fix/frontend` / `~/dev/platypus-fix-frontend` | FE-1, 3 to 14 | `frontend/src` (not the session wiring that C owns) |

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
- No `wails dev`. Port 34115 belongs to the user's session in `~/dev/platypus-steam-depot-dl`, and that worktree must not be edited.
- No real logins or downloads.
- If a finding turns out to be wrong, say so in the final report and skip it.

## Status (2026-10-08, 07:30)
- **A to D: done.** Merged into `integ/review-fixes` (worktree `~/dev/platypus-integ`). The only conflict was App.tsx loadData (FE-3 combined with CORE-1); resolved in the merge commit.
- **Corrections from the lead's review:**
  - STEAM-12: the job-state flusher marshalled the live map outside its lock, a fatal concurrent map write;
  - CORE-2: no replay of non-idempotent plugin calls after a crash;
  - PLUG-5: install.sh deleted the platypus-steam-dl helper it had just built.
- **Leftovers, done on integ:**
  - Retry button for plugins that gave up (CORE-10);
  - `title` sent in install/update params;
  - toast above modals;
  - dead code removed;
  - deflaked TestInstallFlow, TestSteamCMExchangeAndCache, the crash give-up test and the queue cleanup.
- **Checks:** everything passes. Cold `go test ./...` three times in a row, `-race` on internal and plugins, vet, and in the frontend tsc, vitest (27) and build, plus the production build.
- **Still to do live, with the user:**
  - the STEAM-1 streaming rewrite: install, cancel and resume, update, verify and repair, and peak RAM on a multi-GB depot;
  - STEAM-5 small-depot start time;
  - STEAM-3 on real manifests with symlinked folders;
  - the STEAM-14 token expiry unit;
  - the phase 6 password login.
- **Product decisions for the user:**
  - FE-12: sidebar search now jumps to the Library only on Enter;
  - PLUG-3: legendary, gogdl and nile accept secrets only on argv (tool limitation).
- **Not done:** branch picker UI (beta branches and passwords) and `settings_extra` UI.

## Status — live validation (2026-10-08, 07:45)

Live validation of the rewritten Steam downloader on KAMI (app 272040, single depot 272041, manifest
`6869307445582119673`, ~55 MB compressed / 81 MB disk), branch `integ/review-fixes`, platypus-steam-dl built from
this worktree and run from a temp dir first on PATH (the plugin's sibling-binary preference falls back to PATH
when `bin/platypus-steam-dl` is not shipped). Plugin exec driven over stdin JSON-RPC with the real login
(`PLATYPUS_PLUGIN_DATA=~/.local/share/platypus/plugin-data/steam-standalone`). The cancel landed mid-flight, so no
CDN throttling was needed.

| Step | Result | Numbers |
|---|---|---|
| 1 `game.installSize` | PASS | 55,479,792 download / 81,078,597 disk — exactly the expected numbers |
| 2 fresh install (STEAM-1, STEAM-5) | PASS (after one fix) | TTFB 8.9 s (CM logon, PICS, depot key, CDN list, manifest); 81.0 MB streamed in 1.6 s, avg 51 MB/s, peak ~108 MB/s; 32 connections from the first progress event (STEAM-5 seeding), distinct hosts ramping 1→32 in ~0.5 s; peak RSS (VmHWM) 73.5 MiB (~68 MiB on the resume run, 81.5 MiB on the third install) |
| 3 cancel + same-jobId resume (STEAM-12) | PASS | `job.cancel` at 41.1 % (33.3 MB in); staging kept 35 chunks (30.8 MB) plus a job-state checkpoint; the rerun with the same jobId resumed from 38.1 % (30,885,999 bytes) and finished ~2 s later with `job.done` |
| 4 SHA-1 vs manifest | PASS | independent Python manifest parser (zip container + protobuf + AES filename decryption, hashlib, no Go code): 274 files, 81,078,597 bytes, 0 mismatches, 0 missing, 29 dir entries plus 2 zero-chunk placeholders |
| 5 verify/repair (STEAM-2) | PASS | deleted `puzzles/StageD/SDL8_art2.xml`, corrupted 27 bytes of `fonts/Ubuntu-R.ttf` at offset 1000; `platypus-steam-dl verify` → `bad:2 fixed:2` in 9 s; both files byte-identical to backups afterwards; full SHA-1 sweep clean |
| 6 uninstall + login check | PASS | `~/Games/steam`, `jobs/` (state + staging) and `installed.json` all empty afterwards; `auth.status` still `loggedIn: rUwUe` |

**Fix found live, committed on this branch (test first against the fakes):**
- a156ed2 `live: flusher stop is idempotent; success path stops before RemoveJob` — after a *finished* install
  the plugin's `jobs/<jobId>.json` survived: runInstall's deferred `flusher.stop()` saved state again after
  `depot.RemoveJob`/`clearStaging`, resurrecting the job file (observed twice, including across an uninstall).
  `stop()` now latches (the first call checkpoints and stops the ticker; later calls, `saveNow` and `mark`
  never write again), and runInstall/runUpdate call `flusher.stop()` *before* the cleanup so the removal wins.
  Error and cancel paths keep the STEAM-12 checkpoint: there the deferred stop is the first one and still
  saves. `TestFlusherNoSaveAfterStop` failed on the old code (double stop panicked; the deferred save
  resurrected the file); the existing STEAM-12 checkpoint tests still pass.

**Checks after the fix:** cold `go test ./... -count=1` passes (18 packages). `~/Games/steam` and the plugin's
`jobs/` folder are empty at the end; the dev launcher session (port 34115) was left untouched.

Not covered by this pass: multi-GB peak-RAM ceiling, STEAM-3 symlink folders, STEAM-14 token expiry and the
phase 6 password login — still on the live checklist above.
