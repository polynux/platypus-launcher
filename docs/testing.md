# Testing (issue #40)

Machine: dev machine, `nproc` = 16. All measurements with `-count=1`
(no cache). Timing source: `go test -json ./...` and vitest's JSON reporter.

## Baseline (2026-10-13, before)

### Go: 279 s wall, 39 packages, 920 tests

Per-package wall time (packages run in parallel; `internal/app` dominates):

| Package | Wall |
|---|---|
| internal/app | 277.7 s |
| internal/plugtest | 80.0 s |
| plugins/steam-standalone | 85.9 s (incl. 1 load flake, see below) |
| internal/stores | 15.3 s |
| plugins/epic-legendary | 11.2 s |
| plugins/gog-gogdl | 10.9 s |
| internal/steamcm | 8.8 s |
| internal/session | 6.7 s |
| internal/plugins | 6.4 s |
| internal/depot | 4.2 s |
| internal/cloudsync | 3.1 s |
| plugins/amazon-nile | 1.8 s |
| everything else | < 1 s each |

Top 25 slowest tests:

| Test | Time | Cause |
|---|---|---|
| plugtest TestConformanceSteamStandalone | 41.3 s | full protocol suite vs real plugin processes + Python fake CM/CDN, chunk delays (8 extra files × 150 ms) |
| app TestTheStoreClientRunningMeansNoSync | 17.1 s | two echo fixtures → two cold `go build`s (see below) |
| app TestInstallSizeIsCachedAndUnknownWhenTheStoreCantSay | 17.0 s | same: two echo fixtures |
| steam-standalone TestSteamAuth | 15.7 s | full login flow vs Python fake CM (password + guard + polling) |
| steam-standalone TestSteamQRRefresh | 15.5 s | waited out its 15 s poll deadline under load; passes solo in 1.5 s (flake) |
| steam-standalone TestSteamPauseKeepsStagingAndFolder | 13.8 s | download simulation with real chunk pacing |
| steam-standalone TestSteamPasswordTwoStepTOTP | 10.6 s | 2FA login flow vs fake CM |
| steam-standalone TestSteamQRConcurrentBegin | 10.3 s | QR flows vs fake CM |
| app TestInstallGameWithOptionsToUsesTheChosenFolder … TestCloudModesPerPluginAndPerGame (18 tests) | 8.6–9.3 s each | one cold `go build` of `plugins/echo` per fixture (see below) |
| plugtest TestConformanceEpic / Echo / Amazon / Gog | 6.4–7.6 s each | full suite vs real (Python) plugin processes |
| plugtest TestConformanceScaffold | 5.2 s | scaffold build + install + restart |

Root cause of the ~8.6 s per app fixture: every helper (`cloudApp`,
`familyApp`, `versionsApp`, `installEchoApp`, …) runs
`go build platypus/plugins/echo` per test, after `t.Setenv("HOME",
t.TempDir())`. The overridden HOME gives the go command a cold GOCACHE,
so it recompiles echo and its dependencies (~8.2 s; the same build is
0.4 s warm). `internal/plugins` already builds echo once per run
(`buildEcho` with `sync.Once`); `internal/app` and `internal/stores`
do not. `internal/plugtest` and `plugins/steam-standalone` already
prebuild their Go helper binary in `TestMain` with the real
HOME/GOCACHE for exactly this reason.

Other real waits (1–5 s each): steamcm heartbeat intervals (2 s),
session stop-while-starting (4 s), depot CDN body timeout (2.4 s),
pause/resume download pacing (epic/gog/steam 2–3.5 s).

### Vitest (apps/ui): 29.1 s, 106 files, 642 tests

`Duration 29.07s (environment 38%, tests 27%, import 22%, setup 7%,
transform 6%)`. jsdom is created 106 times (149.8 s cumulative across
workers). Top 15 files by assertion time:

| File | Time |
|---|---|
| components/__tests__/maintenance.test.tsx | 11.1 s |
| components/__tests__/game-settings-modal.test.tsx | 7.4 s |
| components/__tests__/versions-gamepage.test.tsx | 4.0 s |
| components/__tests__/add-game-modal.test.tsx | 4.0 s |
| components/__tests__/versions-dialog.test.tsx | 3.6 s |
| components/__tests__/install-options-dialog.test.tsx | 3.6 s |
| components/__tests__/install-dialog.test.tsx | 2.2 s |
| components/__tests__/cloud-saves.test.tsx | 3.0 s |
| components/__tests__/family-settings.test.tsx | 2.7 s |
| components/__tests__/friends.test.tsx | 2.7 s |
| components/__tests__/library-sort.test.tsx | 2.4 s |
| components/__tests__/ui-home.test.tsx | 2.4 s |
| components/__tests__/online-art.test.tsx | 1.9 s |
| components/__tests__/settings-env-rows.test.tsx | 1.7 s |
| components/__tests__/custom-year.test.tsx | 3.3 s |

Cause: heavy component renders (GamePage, modals) with real timers —
`findBy*`/`waitFor` with real timeouts — plus a fresh jsdom per file.
Vitest itself suggests `pool: 'vmThreads'` (reuses one jsdom per
worker, keeps per-file isolation): 29.1 s → 13.0 s, same 642 tests.

Desktop shell (`bun run test:desktop`, `node --test`): 31 tests, 0.7 s.

### Network

No test in the default suite dials the real internet as far as reading
goes: onlineart/compat/gameinfo/artwork tests use `httptest` servers,
the gog/epic/amazon/steam plugins answer from fixture API files
(`PLATYPUS_*_API_FILE`), and the fake CM/CDN helpers bind localhost.
Nothing enforces this yet — see "Categories" below for the guard.

### Pre-existing failure under load

`plugins/steam-standalone TestSteamQRRefresh` failed in the full run
("auth.status never served the refreshed QR code" after its 15 s
deadline) and passes alone in 1.5 s: a load flake, not a logic failure.

## After (2026-10-13)

Default run, `nproc` = 16, `-count=1`: **Go 28 s wall + vitest ~13 s +
desktop 0.7 s ≈ 42 s serial, 0 failures** (was: Go 279 s wall with 1
load flake + vitest 29 s ≈ 310 s). No test in the default run touches
the network (refused by guards, see below).

### Go: 28 s wall (was 279 s)

| Package | Before | After | How |
|---|---|---|---|
| internal/app | 277.7 s | 25.1 s | echo binary built once per run, not per test (cold GOCACHE was the 8.6 s/fixture); build runs with the real HOME/GOCACHE |
| internal/plugtest | 80.0 s | 4.5–5.9 s | 6 full protocol conformance suites → slow (`RequireSlow`) |
| plugins/steam-standalone | 85.9 s | ~11 s | 8 slowest auth/QR/download-pacing tests → slow |
| internal/stores | 15.3 s | 9.6 s | echo binary built once per run |
| plugins/epic-legendary | 11.2 s | 10.6 s | unchanged (real Python plugin, kept in default) |
| plugins/gog-gogdl | 10.9 s | 10.6 s | unchanged (same) |
| internal/steamcm | 8.8 s | 8.8 s | unchanged (heartbeat intervals are the product's) |
| internal/session | 6.7 s | 6.7 s | unchanged |
| everything else | — | ≤ 6 s | — |

Slow suites, opt-in (`PLATYPUS_TEST_SLOW=1`, measured): plugtest 78 s,
steam-standalone 67 s — both pass.

### Vitest: ~13 s (was 29.1 s), same 642 tests, 0 errors

`pool: 'vmThreads'` (one jsdom per worker, per-file isolation kept):
29.1 s → 12.4–13.0 s. The pool switch exposed a leaked 300 ms debounce
timer in `versions-gamepage.test.tsx` (`previewCommand` missing from the
file's `LauncherApi` mock → unhandled error after teardown); the mock
now stubs it, like `game-settings-modal.test.tsx` does.

Desktop shell: 31 tests, 0.7 s, unchanged.

### Flakes

- `go test -race -count=5 ./internal/app/`: pass, 210 s.
- `TestSocialWithTheEchoPlugin` ("not connected to the store" under
  load): the friends session can flap between the connected-wait and
  `SetSocialStatus` (the manager reconnects on its own). The test now
  retries a transient "not connected" with a 15 s deadline instead of
  failing; wrong statuses and missing methods still fail at once. Same
  treatment for echo-b in `TestTwoStoresHaveTheirOwnFriends`.
- `TestSteamQRRefresh` (15 s deadline waited out under full-suite load;
  passes solo in 1.5 s): moved to slow with the other QR/auth
  polling tests. `TestSteamLegacyInstallFolderKeepsWorking` varied
  0.9–10.9 s with load (paced downloads): moved to slow too.

### Follow-ups (not done)

- `TestSteamAuth` takes ~15 s even solo against the fake CM; the wait
  is in the login polling path, not the test. Worth profiling separately.
- steamcm heartbeat tests (2 s each) and the 2 s "no duplicate refresh"
  stabilize windows in app auth/stats tests use the product's real
  intervals; shortening them means injecting test clocks.
- Mass `t.Parallel()` in `internal/app` is done (issue #41, below). Still
  serial: the `ECHO_*` plugin-knob tests (~20 `t.Setenv` sites; per-test
  values would need threading through `plugins.Manager.Env` at each call
  site) and the overlap-sensitive `TestAuthStatusRefreshSingleFlight`.

### Follow-up: XDG paths (issue #41, 2026-10-13)

Folders resolve from `PLATYPUS_*` / XDG variables (`internal/paths`), the
App takes explicit dirs (`app.Options`, resolved once in `cmd/platypus-core`
main), and all 43 `t.Setenv("HOME", …)` sites are explicit dirs (41 in
`internal/…`, 2 steam-standalone cell-id tests via `PLATYPUS_STEAM_ROOT`), so 230
env-free `internal/app` tests plus the `internal/settings` tests run with
`t.Parallel()` (`-parallel` caps at GOMAXPROCS = 16). Method as above,
`-count=1`, `nproc` = 16:

| Package | Before (#40) | After (#41) | How |
|---|---|---|---|
| internal/app | 25.1 s | ~8–9 s | parallel env-free tests (serial: `ECHO_*` knobs, singleflight overlap test) |
| default `go test ./...` wall | 28 s | ~13–30 s | app no longer dominates; wall is now `steam-standalone` (~11–29 s run variance, pre-existing pacing/polling waits) + epic/gog (~10 s) |
| everything else | — | unchanged | vitest ~13 s, desktop 0.7 s |

`go test -race -count=3 ./internal/app/ ./internal/settings/ ./internal/paths/`
passes (app ~45 s, settings/paths ~1 s). Full default `go test ./...`:
35 packages, 0 failures. Two flakes fixed along the way, not ignored:
`TestInstallLocationsListDefaultLibrariesAndRecents` encoded the old
temp-`HOME` quirk (a stale default lingering as a library); it now asserts
the clean list. `TestAuthStatusRefreshSingleFlight` needs all 8 readers to
overlap one flight, which load breaks (a straggler correctly starts another
refresh); it stays serial.

## Categories

- Default (`go test ./...`, `bun run test`): unit + in-process tests
  only. No network (refused by a test guard), no slow runtimes.
- Slow (`PLATYPUS_TEST_SLOW=1`, or `go test -short` where noted): real
  plugin processes, Python plugins, full protocol conformance suites,
  download pacing, polling timeouts.
- Network (`PLATYPUS_TEST_NETWORK=1`): tests that dial non-loopback
  hosts. None exist yet; the guard fails the default run if one appears.
- Scripts (root package.json): `test:fast` (default, no network, no
  slow), `test:slow`, `test:network`, `test:all`. `bun run test` keeps
  meaning what CI and agents use today (UI + desktop only, fast set) —
  deliberate: CI already runs Go separately, and agents keep a short
  command for the UI while iterating.

Helpers: `internal/testenv` (`RequireNetwork(t)`, `RequireSlow(t)`).
Go network guard: `internal/testenv` installs an `http.DefaultTransport`
+ dialer refusing non-loopback in `TestMain`-driven packages (or via the
helper). Vitest network guard: `apps/ui/src/test/setup.ts` stubs
`fetch` to refuse non-localhost.
