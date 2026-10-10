# Core backend review (Go)

Scope: `main.go`, `app.go`, `app_jobs.go`, `app_plugins.go`, `internal/plugins`,
`internal/jobs`, `internal/jsonrpc`, `internal/stores`, `internal/runner`,
`internal/settings`, `internal/session`, `internal/steam`, `internal/fixes`,
`internal/types`, `internal/dataplug`.
Background read: `README.md`, `PRODUCT.md`, `DESIGN.md`,
`docs/plans/launcher-plugins.md`, `docs/plans/steam-depot-downloader.md`,
`docs/plugins/protocol.md`.

## Summary

The core is in good shape: plugin hash-pinning, launch-spec validation, the job
queue's single-flight/cancel/resume logic, and settings' atomic writes are all
carefully built and well tested. The review still found 4 high-severity issues:
(1) there is **no session snapshot API**, so any window opened (or reload) after
a launch never shows the running game — this is the root cause of the reported
"window opened after a game launch doesn't show the running game";
(2) a **stale-connection race** in `process.ensure` that explains the flaky
`TestGivesUpAfterRepeatedCrashes` (`want give-up error, got plugin echo exited
during echo`) and can also misroute real user calls right after a crash;
(3) a **claim-then-set race plus unconditional clear** in the session tracker
that allows double launches and can drop live-session tracking;
(4) **launch-secret redaction is bypassed by the game's own stdout**, which lands
verbatim in the per-game log and in failure messages.
Medium items: rescan never syncs data plugins, the install-folder override lacks
the system-folder guard, the `xdg-open` Steam fallback appends launch args as
open targets, previews create Proton prefixes, sideloaded games lose unknown
settings fields in memory, and a gave-up plugin has no recovery path except
toggle/restart.

## Findings

### CORE-1 — high — state-sync — `app.go:63-67`, `internal/session/session.go:42-52`, `frontend/src/App.tsx:174-202`

**What's wrong.** The backend tracks running games (`session.Tracker.active`)
and pushes `session` events, but exposes **no snapshot API**: `App` has
`GetLibrary`, `ListJobs`, `GetProtonVersions`, … and nothing like
`ListSessions`. The frontend only learns running state from the event stream
(`onSession` → `setSessions`). Any window that subscribes after the `running`
event — a second window, a reloaded view, the game page opened after launch —
starts with `sessions = {}` and shows **Play** instead of **Playing**.

**Concrete scenario.** Launch a Steam game (event `starting` → … → `running`
emitted). Open the game's page afterwards (or reload the UI): `currentGame` is
found, but `sessions[currentGame.id]` is empty, so `GamePage` shows `Play`,
not `Playing` (`App.tsx:292,357`, `GamePage.tsx:109-111`). Worse, the
frontend guard `if (launchingId || sessions[game.id]) return` (`App.tsx:161`)
doesn't block there, so the user can press Play again; only the backend's
`a.sessions.Running` check (`app.go:400`) rejects it with "already running" —
confusing after the UI just offered Play.

**Proposed fix.** Add `Tracker.Snapshot() []types.SessionEvent`-ish
(gameID, state, pid, message) and `App.ListSessions()`, and have the frontend
fetch it on startup (and seed `sessions`/`sessionStart`) before subscribing.

**Test that would catch it.** Start a direct game via `Tracker.StartDirect`,
create a *new* subscriber/snapshot (simulating a late window), assert the new
view still reports running; currently there is nothing to call, so the test
fails at the API level.

**Effort:** S (backend) + small frontend change.

---

### CORE-2 — high — race — `internal/plugins/process.go:94-153` (esp. `104-117`), `207-246`

**What's wrong (known flaky test, root-caused).** `ensure()` can hand out a
**dead-but-not-yet-cleaned-up connection**. After the plugin process exits,
the reaper goroutine (`start()`, `go func() { waitErr := cmd.Wait(); … }`)
first waits for the jsonrpc read loop to observe EOF
(`select { case <-conn.Done(): case <-time.After(time.Second) }`) and only
then closes pipes, calls `conn.Close()`, records the crash and closes
`exited`. During that drain window `p.conn != nil`, `ready` is already closed
(from a successful `initialize`), and `conn.Done()` is **not yet closed**, so
`ensure()` takes the `default:` branch and returns the stale conn
(`process.go:115-116`). The subsequent `Call` then fails with `ErrClosed`,
mapped to `plugin echo exited during echo` (`process.go:85-87`) instead of
restarting (or giving up).

**Concrete scenario.** Matches the reported flake exactly:
`TestGivesUpAfterRepeatedCrashes` (`manager_test.go:248-260`) does
crash,crash,crash then `echo`. If the final `Call("echo")` wins the race and
runs `ensure()` inside the drain window of the third crash, it gets the stale
conn and returns `plugin echo exited during echo` instead of the expected
`crashed 3 times` give-up error. Load widens the window (read-loop EOF vs.
scheduler), hence ~1-in-3 under load. I ran the test 60+× unloaded (plus
`-race`) without reproducing — the code path above plus the exact error
string is the evidence; the race is real whether or not it fires on a given
run. The same race affects users: the first call after any plugin crash can
spuriously report `exited during <method>` instead of transparently
restarting.

**Proposed fix.** Either (a) close a `waitDone` channel immediately after
`cmd.Wait()` returns and have `ensure()` treat that as dead (wait on `exited`
/ back off instead of returning the conn), or (b) make `call()` retry once
through `ensure()` on `ErrClosed` unless the manager already gave up. (a) is
more precise; (b) is a smaller patch. Also consider shortening the
always-up-to-1s drain: it exists to let the reader drain output, but it is
exactly the stale-conn window.

**Test that would catch it.** Deterministic unit test: stub a backend whose
process dies but whose `conn.Done()` stays open for ~200ms (simulating the
drain), call `ensure`/`call` in that window, assert it blocks until `exited`
and restarts rather than returning `ErrClosed`. Plus run the existing test
with `-race -count=50` under `stress`/parallel load in CI.

**Effort:** M.

---

### CORE-3 — high — race — `internal/session/session.go:46-64`, `112-153`; `app.go:399-402`

**What's wrong.** claiming a game and clearing it are not atomic:

1. `LaunchGame` checks `a.sessions.Running(game.ID)` (`app.go:400`) and
   *later* `WatchClient`/`StartDirect` calls `t.set(gameID, …)`. Two
   concurrent `LaunchGame` calls for the same game (double-click, two
   windows — note CORE-1 makes the second window likely) can both pass the
   check and start two processes.
2. `set()` unconditionally overwrites; `clear()` unconditionally deletes. If
   run A is overwritten by run B, A's exit goroutine deletes B's entry:
   the UI flips to stopped while B is still running, and `Running()` returns
   false so a third launch is allowed.
3. `WatchClient`'s `defer t.clear(gameID)` has the same flaw across
   start→run→stop transitions.

**Concrete scenario.** Direct game G: `LaunchGame(G)` twice in quick
succession (or from two windows). Both see `Running == false`; both
`exec.Start` and `set(G, cmdN)`. Whichever `Wait` returns first clears `G`
even though the other process is alive. Frontend shows stopped; pressing Play
starts a third copy.

**Proposed fix.** Add an atomic claim, e.g.
`TryClaim(gameID, cmd) bool` (set only if absent), use it in both
`StartDirect` and `WatchClient`, and make `clear` compare-and-delete
(only delete if the stored cmd matches). Return "already running" from the
claim, not from a separate check.

**Test that would catch it.** Concurrent `StartDirect` (or `LaunchGame`
against a fake store) × N for one game ID with a blocking process; assert
exactly one succeeds and the tracker still reports running after the losers'
processes exit.

**Effort:** S.

---

### CORE-4 — high — security — `internal/session/session.go:112-153`, `app.go:374-386`, `plugins/echo/main.go:362-363`

**What's wrong.** Secrets listed in a launch spec's `redact` prefixes are
hidden only in the *command line string* (`prepared.shown()`). The game's
actual stdout/stderr are written **verbatim** to the per-game log and kept in
`tailBuffer`, whose content becomes the user-visible `failed` message. Any
game that echoes its arguments (extremely common with `--help`-style logging,
and exactly what the echo test plugin's `game.sh` does: `echo "echo game $*"`)
leaks the real secret (e.g. `-AUTH_PASSWORD=s3cret-…`) into
`~/.local/share/platypus/logs/<game>.log` (mode `0644`) and into the session
`failed` event shown in the UI.

**Concrete scenario.** Install and launch echo `tiny` for real
(`preview=false` mints `s3cret-<nanos>`, `plugins/echo/main.go:425-428`).
`game.sh` prints its argv, including the token. Fail the launch quickly (or
just inspect the log): the log file and any `failed` message contain the raw
token, while the preview correctly showed `-AUTH_PASSWORD=***`.

**Proposed fix.** Thread the `redact` prefixes into `StartDirect` (or into a
redacting `io.Writer`) and filter both the log file and the tail buffer,
e.g. replace `prefix + rest-of-token` with `prefix***` per line. Alternatively
document that plugins must never echo secrets — but Platypus already promises
redaction in "previews and logs" (`protocol.md:172-173`), so the log side
must be fixed in core.

**Test that would catch it.** `StartDirect` a script that echoes
`-AUTH_PASSWORD=s3cret-xyz` with redact `["-AUTH_PASSWORD="]`; force a quick
failure; assert neither the log file nor the emitted message contains
`s3cret-xyz`.

**Effort:** M.

---

### CORE-5 — medium — state-sync — `app.go:109-115`, `app_plugins.go:76-84`, `internal/dataplug/registry.go:29-50`

**What's wrong.** Data plugins are loaded once at startup
(`a.data.Add(...)` for each discovered plugin + builtin). `RescanPlugins`
re-runs `Discover` but never syncs `a.data`, and `Registry` has `Add` only
(no `Reset`/`Remove`). Installing (or removing) a data plugin followed by
Rescan shows it as `ready` in the Plugins view while its fixes/tweaks are
silently inactive (or stale) until restart. `GetTweaks` is additionally cached
behind `tweaksOnce` (`app.go:293-296`), so even a resync of fixes wouldn't
refresh tweaks.

**Concrete scenario.** Drop in a `community-fixes` data plugin, Rescan,
open an affected game's page: `GetGameFix` still returns nil/builtin until
the app restarts, contradicting the Plugins view. Removing a data plugin
likewise keeps serving its fixes until restart.

**Proposed fix.** On `Discover`/`Rescan`, rebuild the data registry from
`DataPlugins()` + builtin (add `Registry.Reset` or rebuild a fresh
`Registry`), and reset `tweaksOnce`/recompute tweaks.

**Test that would catch it.** Discover with a valid data plugin → `Fix()`
found; remove it → `Discover` → `Fix()` gone, without restart.

**Effort:** M.

---

### CORE-6 — medium — security/UX — `internal/plugins/manager.go:372-386` vs `390-428`

**What's wrong.** `AllowFolder` rejects `/`, `$HOME`, and system dirs via
`checkAllowable`, but `SetInstallFolder` (the plugin's install root — which
is *always* a valid launch location in `ValidateSpec`) only requires an
absolute path. A user can point a plugin's install folder at `/`, `/etc`,
`/usr`, `/usr/bin`, or their whole home via the folder picker, after which
that plugin's specs may launch binaries from there.

**Concrete scenario.** `SetPluginInstallFolder("echo", "/usr/bin")` succeeds
today; a subsequent `ValidateSpec` with `exe=/usr/bin/true` passes because it
is inside the install folder. The equivalent `AllowFolder("/usr/bin")` is
correctly refused.

**Proposed fix.** Reuse `checkAllowable` (or a shared superset) in
`SetInstallFolder`.

**Test that would catch it.** `SetInstallFolder` with `/`, home, `/usr/bin`,
`/etc` must error; a temp-dir Games subfolder must succeed.

**Effort:** S.

---

### CORE-7 — medium — wrong-behaviour — `internal/steam/launch.go:20-35`

**What's wrong.** The comment on the fallback says the `steam://` handler
"can't forward launch arguments", yet `LaunchArguments` are appended
unconditionally — including to the `xdg-open steam://rungameid/<id>` command.
`xdg-open` then treats `-novid` etc. as additional files/URLs to open.

**Concrete scenario.** No `steam`/`flatpak` binary present, game has launch
args `-novid -fullscreen`: Platypus runs
`xdg-open steam://rungameid/620 -novid -fullscreen`. The game starts without
its args (or the open fails / opens junk), depending on the handler.

**Proposed fix.** Append `LaunchArguments` only in the `steam` and
`flatpak` branches, not the `xdg-open` fallback.

**Test that would catch it.** With `PATH` lacking `steam`/`flatpak`,
`Command(game with LaunchArguments)` must equal exactly
`[xdg-open, steam://rungameid/<id>]`.

**Effort:** S.

---

### CORE-8 — medium — side-effect — `app.go:389-395` → `322-372` → `internal/runner/launcher.go:42-56`

**What's wrong.** `PreviewLaunchCommand(game)` is called "on every change"
in the settings dialog (`protocol.md:175`), but the preview path runs the
full `BuildCommand`, which `MkdirAll`s
`~/.local/share/platypus/prefixes/<gameID>` for every non-native game. Mere
previewing creates on-disk state.

**Concrete scenario.** Open settings for a never-launched Windows game,
tweak a dropdown a few times (each keystroke re-previews): empty prefix
folders accumulate under `prefixes/` although the game was never launched.

**Proposed fix.** Pass `preview` into the build step (or split
resolve-vs-create) and skip `MkdirAll` on previews; create the prefix
lazily at real launch.

**Test that would catch it.** Call `BuildCommand`/preview for a game with a
fresh `HOME`, assert no `prefixes/` dir is created; launch path still
creates it.

**Effort:** S.

---

### CORE-9 — medium — state-sync — `app.go:172-196`

**What's wrong.** `customGames()` merges saved per-game settings via
`json.Marshal(saved) → json.Unmarshal(&g.Settings struct)`, which **drops
every key the struct doesn't model** (`sync_mode`, `winetricks`, …).
`settings.go` deliberately preserves those keys on disk
(`app.go:…` / `settings.go:34-36`), and library games keep them through the
frontend merge (`App.tsx:95-98`), but sideloaded games lose them in memory
after every restart.

**Concrete scenario.** Sideload a game, set an extra setting the backend
doesn't model (e.g. `sync_mode: ntsync`), restart: the value is still in
`settings.json` but the UI/backend game object no longer carries it until the
user saves again.

**Proposed fix.** Keep the raw map alongside (or merge at the map level into
the emitted game), consistent with `GetGameSettings`/frontend merging.

**Test that would catch it.** Save `{"runner_id":"auto","sync_mode":"ntsync"}`
for a custom game, reload via `customGames()`, assert the compound settings
still contain `sync_mode`.

**Effort:** S.

---

### CORE-10 — medium — recovery — `internal/plugins/process.go:130-134`, `250-259`

**What's wrong.** `crashes` only resets after one run survives `stableAfter`
(1 minute). After `maxCrashes` (5 by default) quick crashes, *every* later
call fails with `crashed 5 times` forever — including long after the cause
was fixed — until the process record is dropped (disable/enable cycle,
re-approval, or restart). `Discover` does not reset it, and the UI offers no
"retry plugin" action, so a transient breakage (e.g. a missing helper
installed later) looks permanent.

**Concrete scenario.** Plugin crashes 5× while `legendary` is missing;
install it; calls still fail with `crashed 5 times …` an hour later.

**Proposed fix.** Reset (or decay) the crash count on successful
`Approve`/`SetEnabled(true)`/`Discover`-fresh-start, and/or surface a
backend `RetryPlugin(id)` the Plugins view can offer after a give-up.

**Test that would catch it.** Crash to give-up, then `Approve` again (same
hash) or call the new reset; assert the next call restarts the plugin.

**Effort:** S–M.

---

### CORE-11 — low — data-hygiene — `internal/settings/settings.go:233-249`

**What's wrong.** `RemoveCustomGame` deletes the entry from `CustomGames`
but leaves its `Games[gameID]` settings behind. Orphaned per-game settings
accumulate and could even reattach if an ID ever collided (IDs are
`custom-<nanos>`, so collision is unlikely — this is clutter, not a hijack).

**Scenario.** Add game → save settings → remove game → `GameSettings()`
still contains the ID.

**Fix.** Delete `s.data.Games[id]` in the same locked section (with the same
rollback-on-save-error pattern). **Test:** add/settings/remove → key gone.
**Effort:** S.

---

### CORE-12 — low — durability — `internal/jobs/queue.go:475-504` vs `internal/settings/settings.go:281-308`

**What's wrong.** `jobs.save()` uses tmp-file + rename (good) but no `Sync`
before rename, and `changed()` discards the error (`_ = q.save()`). A crash
or full disk can silently lose progress/persistence, whereas `settings.save()`
does `CreateTemp → Write → Sync → Close → Rename` and rolls back in-memory
state on error. Progress saves are also throttled (`saveEvery = 5s`), so up
to 5s of progress is always only in memory — fine by design, but a failed
*state* save should at least be noticed.

**Scenario.** Disk-full during an install: queue transitions still emit to
the UI while persistence silently fails; after restart the job resumes from
an older snapshot.

**Fix.** Mirror settings: `Sync` before rename in `jobs.save()`; surface
save errors (log/counter — `Queue` currently has no logger; at minimum don't
silently swallow state-save errors).

**Test:** save with an unwritable dir returns error; progress survives
`Open` after `save`. **Effort:** S.

---

### CORE-13 — low — security/hygiene — `internal/plugins/process.go:177-184`, `314-334`

**What's wrong.** Plugin log files are `0644` world-readable and
append-only with no rotation (`plugin-<id>.log` gets stderr plus every
`log` notification). Plugin output may include tokens/paths; combined with
CORE-4, secrets can sit in a world-readable file that grows forever.

**Fix.** Create/keep logs at `0600` (like `PLATYPUS_PLUGIN_DATA/state.json`
in the echo plugin) and add size-based rotation/truncation.
**Test:** log file mode `0600`; 10k log notifications keep the file under
the cap. **Effort:** S.

---

### CORE-14 — low — docs — `internal/plugins/manifest.go:77-78` vs `docs/plugins/protocol.md:40`

**What's wrong.** The code comment says permissions "are declared now and
enforced by the bubblewrap sandbox later (plan milestone 7)", the protocol
doc says "(bubblewrap sandbox, plan milestone 8)". One of them is stale.

**Fix.** Pick the right milestone number in both places. **Effort:** S.

---

### CORE-15 — low — correctness — `internal/jobs/queue.go:203-207`, `350-358`

**What's wrong (two small progress nits).** (a) `prepare()` only fills
`j.Total` from the size probe `if j.Total == 0`, so a retried job keeps a
stale total even when the plugin now reports a different size. (b) `Notify`
ignores `Bytes == 0` (`if u.Bytes > 0`), so a plugin can never reset a
resumed job's byte counter to zero — Platypus keeps the pre-crash value.

**Scenario.** Flaky install reports 50% → crash → plugin restarts from 0
and reports `bytes: 0`: UI keeps showing the old bytes until the counter
passes it.

**Fix.** Always refresh `Total` from a successful size probe (unless a newer
progress total already arrived); accept `Bytes >= 0` and clamp into
`[0, Total]`.

**Test:** extend `TestPercentOnlyProgressEstimatesBytes`-style coverage with
a zero-bytes progress and a changed-size retry. **Effort:** S.

---

## Improvements (not bugs)

Ranked by value/effort (highest first):

1. **Add `ListSessions` + seed-on-startup (CORE-1 fix).** Small backend
   method, big UX win: reloads/second windows become correct and the
   double-launch path mostly disappears. S.
2. **Atomic session claim + compare-and-delete clear (CORE-3 fix).** Kills a
   whole class of double-launch/stale-UI races. S.
3. **Redact secrets in game output/logs (CORE-4 fix).** Reuse the spec's
   `redact` prefixes in a filtering writer in `StartDirect`. M.
4. **Resync data registry on Rescan (CORE-5 fix).** Rebuild `dataplug.Registry`
   from `DataPlugins()` + builtin; drop the `tweaksOnce` cache or invalidate
   it there. M.
5. **Guard `SetInstallFolder` with `checkAllowable` (CORE-6 fix).** One-line
   reuse of the existing guard. S.
6. **Don't append launch args to the `xdg-open` fallback (CORE-7 fix).**
   Two-line branch fix. S.
7. **Skip prefix creation on preview (CORE-8 fix).** Thread `preview` through
   or split mkdir out of `BuildCommand`. S.
8. **Reset plugin crash count on approve/enable/rediscover (CORE-10 fix).**
   Makes transient plugin breakage recoverable from the UI. S.
9. **`0600` + rotation for plugin logs (CORE-13).** Matches the `0600`
   already used for plugin state files. S.
10. **Bound the ProtonDB response body (`fixes/service.go:57-61`).**
    `io.ReadAll` with no cap (`http.MaxBytesReader`-style `io.LimitReader`,
    e.g. 1–2 MB); tiny risk, trivial fix. S.
11. **Avoid FS work under `Manager.mu` (`manager.go:290-306`).**
    `MkdirAll` for log/data dirs while holding the manager lock can stall
    `Discover`/`Call`; compute paths under lock, create outside. S.
12. **Deduplicate `prepare` size-probe refresh + accept zero-byte progress
    (CORE-15).** S.

## Checked and fine

- `main.go:48-64` (`steamArt`): `Clean("/"+rel)`, extension gate
  (`.jpg`/`.png`), root-empty → 404, files served under
  `appcache/librarycache` only — no traversal.
- `plugins/manifest.go`: id/store regexps, reserved ids, id==folder
  enforcement, `exec` must be local, `ExecPath` symlink resolution +
  containment + executable-regular-file check, `install_dir`/permission path
  validation, whole-folder SHA-256 pinning with `__pycache__`/`*.pyc`
  exclusion and `PYTHONDONTWRITEBYTECODE=1`.
- `plugins/process.go` lifecycle intent: verify-before-every-start,
  own process group + `Pdeathsig`, stdin-close shutdown with grace then
  group kill, `initialize` handshake with API check, backoff + give-up
  design (modulo CORE-2/CORE-10).
- `jsonrpc/conn.go`: 32 MB line cap, stray non-JSON lines skipped without
  dropping the connection, plugin→Platypus requests rejected with
  `MethodNotFound`, concurrent calls with per-call channels, pending cleanup
  on cancel/close, `Notify` without response.
- `stores/launchspec.go` direct path: absolute exe, symlink resolution,
  containment in install/allowed folders, regular-file + exec-bit checks,
  cwd containment, env denylist (`PATH`/`HOME`/`LD_*`/`WINE*`/`PROTON_*`/
  `STEAM_COMPAT_*`/`VK_*`/interpreters), delegate path (`requires`
  membership, `LookPath`, `find.cmdline` length ≥ 3).
- `stores/plugin.go`: `GameID` sanitization + hash suffix (no Steam-id
  collision — covered by `TestGameID`), `webURL` http(s)-only gate,
  `game.details` method-not-found fallback to empty details.
- `jobs/queue.go`: one-job-at-a-time single flight, queued/running
  dedup per game, cancel-then-ignore-late-notifications, crash requeue with
  attempt cap (`TestCrashRestartsJob`), restart requeue from file
  (`TestResumeAfterRestart`), cross-plugin notification isolation,
  `MethodNotFound` size-probe skip, 1.02× space margin, tmp+rename
  persistence, corrupt-file move-aside (mirrored in settings).
- `settings/settings.go`: atomic `CreateTemp → Write → Sync → Rename`,
  rollback of in-memory state on save error, corrupt-file move-aside with
  usable defaults, `~` expansion + absolute + is-dir validation for the
  Games folder, custom-game dedup through `EvalSymlinks` (symlink
  duplicates rejected — covered by `TestCustomGames`).
- `runner/select.go` + `scanner.go`: GE-Proton > Experimental > numbered
  Proton > Wine ranking with highest-version-wins and `-rtsp` de-preference;
  fixed-directory scan, no network.
- `steam` client-launch path: `IsClientLaunch` gating, reaper detection via
  `SteamLaunch` + `AppId=<id>\x00` (NUL terminator avoids `12`-vs-`123`
  prefix collision), `FindByCmdline` skips self and unreadable pids.
- `app_plugins.go:158-185` (`PluginAuthBegin`): http(s)-only login URLs,
  field normalization with `code` default. `app.go:226-257` (`GetLibrary`):
  per-store timeouts, one bad plugin degrades to a warning (Steam still
  fails hard, as documented).
