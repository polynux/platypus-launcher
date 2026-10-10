# Steam stack review — `internal/depot`, `internal/steamcm`, `plugins/steam-standalone`

## Summary

Reviewed the Go depot downloader + Go login (phases 1–6), the Python plugin (`bin/platypus-steam`, `bin/steam_cm.py`), `cmd/platypus-steam-dl/*`, `fakesteam`, `install.sh`, `plugin.json`, and the protocol doc. The crypto/manifest/CM core is solid and well-tested; the risky edges are **install/update data paths**: the downloader holds whole games in RAM, `verify` re-fetches without CDN tokens, the writer allows symlink-escape, branch passwords are dead code, and small-depot slowness is startup serialization (not per-chunk cost). Python threading is mostly correct after the QR-race fix, but two races remain (token-source comparison, password-retry session reuse). 4 high, 10 medium, 5 low findings below. All findings verified against the cited code; no network, logins, or full test suite run (targeted `grep`/reads only).

## Findings (most severe first)

### STEAM-1 — Install/update hold the entire game in RAM (OOM on large games)
- Severity: high · Category: memory / correctness
- `plugins/steam-standalone/cmd/platypus-steam-dl/commands.go:69-110` (`runInstall`), `:158-186` (`runUpdate`), `internal/depot/scheduler.go:114-136,238`
- What's wrong: `Scheduler.Download` returns `map[string][]byte` of **all** chunk plaintexts; `runInstall` accumulates every depot into `merged` and keeps it to the end; `runUpdate` builds `have` by reading **all** installed files into RAM (`readOldChunks`, `:316-338`, one `append([]byte{},...)` copy per chunk). Staging files exist on disk, but RAM still grows with game size. A 4.4 GB game (SUPERHOT, per plan doc) needs ~4.4 GB map + `results` channel refs + staging copies.
- Scenario: install a 4+ GB game on an 8 GB machine → RSS climbs into GBs, OOM-killer or heavy swap; update of the same game reads the whole install into `have` first for the same effect.
- Fix: stream — keep only chunk IDs + staging paths in RAM; write each landed chunk to staging and `WriteChunks` incrementally (or keep per-depot `data` and drop `merged` cross-depot accumulation).
- Test: install/update fixture under `GOMEMLIMIT`/cgroup with peak-RSS assertion, or a test that `runInstall` never holds more than N chunks (count live entries via a wrapped map).
- Effort: L

### STEAM-2 — `verify` repair re-fetches without CDN tokens
- Severity: high · Category: logic bug
- `plugins/steam-standalone/cmd/platypus-steam-dl/commands.go:236` vs `:176,218-219`
- What's wrong: install/update download with `d.hosts` (per-depot hosts **with** CDN auth tokens attached by `withTokens`/`hostsForDepot`). `runVerify`'s repair loop passes `ss.hosts` — the bare server list **without** tokens.
- Scenario: host type requiring `GetCDNAuthToken` (third-party CDNs in the plan's risk list) → install succeeds (tokened URLs), a file corrupts, verify detects `bad:2` then the repair `Download` gets 401/403 on every host and returns "download failed", leaving the game broken although the bytes are fetchable.
- Fix: pass `d.hosts` in the verify repair loop (one-line change).
- Test: fake CDN that 403s chunk URLs without `?token=`; install passes, corrupt-a-file + verify passes only with the fix.
- Effort: S

### STEAM-3 — Writer symlink escape outside the install dir
- Severity: high · Category: security (path traversal)
- `internal/depot/writer.go:28-43`
- What's wrong: entries are sorted by path and applied in order; a symlink entry (`os.Symlink`) followed by a file whose name traverses through it (`filepath.Join(dir, rel)` with no symlink check) writes outside `dir`. The `strings.Contains(rel, "..")` check blocks `..` but not `link/evil` where `link` is a just-created symlink. Manifests come from Steam but are publisher-influenced content.
- Scenario: manifest with `link → /tmp/evil` (symlink entry) + `link/pwned` (file entry, `"link" < "link/pwned"` so symlink lands first) → `OpenFile(dir/link/pwned)` follows the symlink and writes outside the game folder as the user.
- Fix: write files before symlinks, or reject any file whose resolved parent (`EvalSymlinks` on `Dir`) escapes `dir`, or open with `O_NOFOLLOW` and refuse symlink components.
- Test: manifest with symlink + file-through-symlink asserts `writeOne` errors and no outside file is created.
- Effort: M

### STEAM-4 — Branch passwords are dead code (never sent)
- Severity: high · Category: dead code / missing feature (explicitly not live-validated)
- `plugins/steam-standalone/cmd/platypus-steam-dl/pics.go:124-128` (`betaHash`), `internal/steamcm/content.go:321-355` (`CheckBetaPassword`), `plugins/steam-standalone/cmd/platypus-steam-dl/flow.go:158`
- What's wrong: `betaHash` has no callers, `CheckBetaPassword` has no callers (`grep` confirms both are define-only), and `ManifestRequestCode` is always called with `""` for `branchPasswordHash`. The request struct (`main.go:78-84`) has no password field, and the Python `dl_request` never sends one.
- Scenario: user selects a password-protected branch → downloader requests the branch gid with no hash (wrong manifest or "no depots") instead of `CheckAppBetaPassword` + hashed request code; there is no path in the code that could succeed.
- Fix: plumb `branch_password` from plugin params → `request` → `CheckBetaPassword` then `betaHash` into `ManifestRequestCode`; surface "wrong branch password" distinctly.
- Test: fake CM asserting the request-code call carries `SHA1(password)` and the beta-check precedes it; wrong-password fake returns the CM error.
- Effort: M

### STEAM-5 — Small-depot "tail" is serial startup, not per-chunk cost
- Severity: medium · Category: performance
- `plugins/steam-standalone/cmd/platypus-steam-dl/flow.go:149-180` (per-depot serial `DepotKey` → `ManifestRequestCode` → `fetchManifest`), `:216-248` (`hostsForDepot` loops hosts serially, each `CDNAuthToken` up to 15 s), `internal/depot/scheduler.go:185-214` (`GrowEvery: 3 s`)
- What's wrong: before the first byte, one job does O(depots) serial CM calls plus O(depots × hosts) serial token calls (32 hosts in the SUPERHOT live run = 32 sequential service calls). A 55 MB depot (~55 chunks) then runs at the `InitialConns: 8` floor because it finishes before the 3 s growth tick fires. The plan doc's "per-chunk overhead" attribution is wrong; chunk fetch itself is one GET per ~1 MB chunk with keep-alive.
- Scenario: KAMI-sized depot on 32 hosts → seconds of CM/token round trips + 8-conn download of a few dozen chunks; tail looks like "~1 MB/s" while the scheduler ceiling (44 MB/s peak live) is fine.
- Fix: fetch depot keys/codes/manifests concurrently (bounded worker pool), fetch CDN tokens in parallel and lazily (only for hosts actually used, or on 401), and seed `InitialConns = min(MaxConns, hosts*MaxPerHost, chunks)`.
- Test: fake CM with 50 ms injected latency per service call; assert time-to-first-chunk drops ~N× with the fix; small-fixture download uses more than 8 conns when hosts allow.
- Effort: M

### STEAM-6 — Connection growth ignores host count; progress `Hosts` is fake
- Severity: medium · Category: logic bug / dead code
- `internal/depot/scheduler.go:190-213`, `:240-243`, `:251`, `:276`
- What's wrong: growth adds workers up to `MaxConns: 32` regardless of `len(hosts)*MaxPerHost` slots (2 hosts × 4 = 8 slots; workers 9–32 spin in `fetchOne`'s wait loop). `hostsUsed["x"] = true` + `_ = hostsUsed` is dead; `Progress.Hosts` reports `len(hosts)` (total) not hosts actually used.
- Scenario: 2-host test/fake setup grows to 32 workers → 24 threads wake every 20–500 ms finding all hosts full, burning CPU and polluting the `connections` number shown in `job.progress`.
- Fix: cap growth at `min(MaxConns, len(hosts)*MaxPerHost)`; track real used hosts in `fetchOne` success path; delete `hostsUsed`.
- Test: 2-host download with `MaxConns: 32` asserts peak `Progress.Connections ≤ 8` and `Hosts ≤ 2`.
- Effort: S

### STEAM-7 — Legacy token-source check never matches (list vs tuple + int vs float)
- Severity: medium · Category: logic bug
- `plugins/steam-standalone/bin/platypus-steam:193-196` vs `:234-256`
- What's wrong: `migrate_token` stores `source` as JSON list `[path, mtime-int]`; `refresh_token` compares it to Python tuple `(path, getmtime()-float)`. List ≠ tuple always, and int ≠ float mtime, so the "already migrated" fast path never hits and every `refresh_token()` call re-walks `home/`, re-decompresses `account.config`, re-runs `JWT_RE`/`refresh_pairs`, and rewrites `token.json` + `refresh.token`.
- Scenario: pre-Go login present → each `library.list`/`access_token` call does the full deflate+regex+rewrite cycle instead of one migration; benign today (same token) but wasteful and racy under concurrent `library.list` threads.
- Fix: compare `list(st["source"]) == [path, mtime]` with normalized types (or store/compare only the token + account and drop the mtime check).
- Test: legacy-migration test asserting second `refresh_token()` call does no filesystem writes (mock `os.walk`/`open` counts).
- Effort: S

### STEAM-8 — Password retry with a corrected password reuses the wrong session
- Severity: medium · Category: concurrency / UX
- `plugins/steam-standalone/bin/platypus-steam:570-584` (`run_auth_password`), `636-661` (`PWSession`)
- What's wrong: sessions are keyed by username only; the password from the **first** submit is baked into the live `PWSession`. A fast second submit with a corrected password attaches to the still-alive first session (`sent_code=True`, only `guard` forwarded) and fails with "wrong account name or password" even though the new password is right.
- Scenario: typo password → immediately resubmit corrected password + guard before first session errors → login fails once more; user thinks the correct password is wrong.
- Fix: key `pw_sessions` by `(username, password)` or, on resubmit with a different password, `stop()` the old session and start fresh.
- Test: two rapid `run_auth_password` calls, same user different passwords, fake helper failing the first; second must spawn a new `Popen` (assert argv/stdin payload), not reuse.
- Effort: S

### STEAM-9 — Executable search only scans the install root
- Severity: medium · Category: UX / logic bug
- `plugins/steam-standalone/bin/platypus-steam:1129-1187` (`find_executable`, `find_native_executable`)
- What's wrong: both use `os.scandir(game_dir)` (root only). Many depot layouts put the binary in a subdir (`bin/game`, `game/run.sh`, Unity `Build/Game.x86_64` is root but others aren't). `record_install` then stores `""`, and `launch_spec` fails at launch time with "couldn't find a game executable".
- Scenario: successful install + verify-clean game whose entry point is one level down → game page shows installed but Launch errors.
- Fix: `os.walk` with the same ranking (Unity pair → script → biggest ELF / biggest non-installer `.exe`), bounded depth, skipping `*_Data`/redist dirs.
- Test: fixture installs with exe in `bin/` only; `find_executable` must return it.
- Effort: S

### STEAM-10 — CDN body timeout leaks a reader goroutine; 64 MiB cap truncates silently
- Severity: medium · Category: resource leak / correctness
- `internal/depot/cdn.go:53-84`
- What's wrong: body is read in a spawned goroutine; on the 60 s `bodyCtx` timeout the function returns but the goroutine keeps `ReadAll`ing (leak per timed-out chunk). `io.LimitReader(..., 64<<20)` + `ReadAll` without an overflow check truncates over-long bodies with no error.
- Scenario: flaky host stalling 60+ s per chunk leaks one goroutine + connection per chunk; a >64 MiB manifest/chunk (unexpected today, attacker-controlled tomorrow) parses as corrupt instead of erroring clearly.
- Fix: read synchronously with `http.Client{Timeout}` / `SetReadDeadline`, or cancel the read via `CancelRequest`; after `ReadAll`, probe one more byte and error on overflow.
- Test: hanging-body fake asserts no goroutine growth (`runtime.NumGoroutine`) after timeouts; oversized-body fake asserts an error, not truncated bytes.
- Effort: S

### STEAM-11 — Multi-depot progress resets per depot
- Severity: medium · Category: UI/UX bug
- `plugins/steam-standalone/cmd/platypus-steam-dl/commands.go:70-72,79-92` and `flow.go:307-314`
- What's wrong: `total = totalBytes(all depots)` but each `sched.Download` reports `DoneBytes` from **zero** for its own depot; the wrapped callback forwards it unchanged. The UI bar jumps backwards at every depot boundary.
- Scenario: 2-depot game, 50/50 split → progress climbs to ~50%, drops to ~0%, climbs again; `speed` (`done/elapsed`, see STEAM-14) dips correspondingly.
- Fix: accumulate a `base` offset per depot (`onProg` adds `base`; `base += depotBytes` after each depot).
- Test: two-depot fake install asserting emitted `bytes` sequence is monotonically non-decreasing.
- Effort: S

### STEAM-12 — Update is not resumable; install resume loses up to 50 chunks on SIGINT-cancel
- Severity: medium · Category: resume / cancellation
- `plugins/steam-standalone/cmd/platypus-steam-dl/commands.go:124-195` (no `SaveJob`/staging vs install's `:74-92,96,111-112`), `bin/platypus-steam:1330-1352` (`stop_proc` sends `SIGINT`; Go has no signal handler)
- What's wrong: `runUpdate` never writes job state or staged chunks, so cancelling/killing an update restarts it from zero (re-reading all old chunks). `runInstall` flushes `JobState` only every 50 chunks; `job.cancel`'s `SIGINT` kills `platypus-steam-dl` immediately (default disposition), so the last <50 landed chunks exist as staging files but are unrecorded in `Done` (re-downloaded next run; staging files themselves are never fsynced either).
- Scenario: cancel a 90%-done update → next update re-fetches everything; SIGINT-cancel an install at chunk 49/4300 → 49 chunks re-downloaded despite being on disk.
- Fix: share the install staging/job path in update; flush job state on a timer (e.g. 2 s) as well as every N chunks; `fsync` staged files; handle `SIGINT` in Go to flush before exit.
- Test: kill `-INT` mid-install/update fake, assert resumed run fetches only missing chunks (count CDN hits).
- Effort: M

### STEAM-13 — Uninstall ignores `job.cancel` (deletes anyway)
- Severity: medium · Category: cancellation
- `plugins/steam-standalone/bin/platypus-steam:1050-1056` (`start_job`), `1320-1327` (`follow_uninstall`), `1342-1352` (`job_cancel`)
- What's wrong: uninstall threads never register in `jobs`, so `job.cancel` finds nothing and returns `{}`; `follow_uninstall` unconditionally `rmtree`s and emits `job.done` (which Platypus ignores post-cancel per protocol, but the files are already gone).
- Scenario: user cancels an uninstall mid-`rmtree` → game directory still deleted.
- Fix: register uninstall jobs in `jobs` and check `canceled` before/after `rmtree` (or document uninstall as non-cancellable and reject `job.cancel` for it).
- Test: start uninstall of a large fixture tree, cancel immediately, assert directory survives.
- Effort: S

### STEAM-14 — CDN token expiry discarded; long jobs go stale
- Severity: low · Category: correctness
- `plugins/steam-standalone/cmd/platypus-steam-dl/flow.go:225-248` (`tok, _, err := ...CDNAuthToken`)
- What's wrong: `expiration_time` is discarded (`_`). Tokens are cached per (host, depot) for the whole job; a multi-hour job keeps using the first token.
- Scenario: 40+ GB install outliving token TTL → late chunks 403, scheduler penalises healthy hosts and may fail the job although a fresh token would work.
- Fix: store `(token, expiry)` and refresh when `now > expiry - margin`.
- Test: fake CM issuing 1-second tokens; long fake download asserts a second token fetch occurs.
- Effort: S

### STEAM-15 — `dl_run` keeps all output lines; speed is total-average, not current
- Severity: low · Category: performance / UX
- `plugins/steam-standalone/bin/platypus-steam:993-1028` (`text` list), `307-314` of `flow.go` + `scheduler.go:241-243` (`BytesPerSec = done/elapsed`)
- What's wrong: `text` appends every stdout line for the whole job only to render the last 3 (`tail()`); 50k-chunk jobs hold 50k strings. Speed divides total bytes by total elapsed, so the tail always reads "~1 MB/s" even at full rate — likely the source of the "slow tail" report alongside STEAM-5.
- Fix: ring buffer (last 20 lines) for `text`; emit windowed speed (bytes since last tick) or an EWMA.
- Test: large fake install asserting bounded `text` length; speed-field unit test on synthetic progress.
- Effort: S

### STEAM-16 — `game_dir_name` can collide; `install.sh` needs Go but doesn't say so
- Severity: low · Category: consistency / docs
- `plugins/steam-standalone/bin/platypus-steam:1068-1074`, `plugins/install.sh:31-39`, `plugins/steam-standalone/plugin.json:11-14`
- What's wrong: dir name is the sanitized store name only (`"A/B"` and `"A_B"` → `"A_B"`), so two apps can share a folder and overwrite each other. `install.sh` `go build`s helpers, but `plugin.json:requires` lists only `python3` (+ optional `steam`), so a fresh user install without a Go toolchain fails with an unhinted error.
- Scenario: two games sanitizing identically → second install lands in the first's folder; Go-less machine → `install.sh steam-standalone` errors on `go: command not found` with no hint.
- Fix: suffix the dir with the app id (`<safe>_<appid>`, migrating existing installs via `dir_name` already stored); add `{ "bin": "go", "hint": ... }` or ship prebuilt `platypus-steam-dl`.
- Test: unit test on colliding names; install dry-run without `go` on `PATH` asserting the hint.
- Effort: S

### STEAM-17 — `gone`-file removal swallows errors; dirs left behind
- Severity: low · Category: error handling
- `plugins/steam-standalone/cmd/platypus-steam-dl/commands.go:173-175`
- What's wrong: `os.Remove` (not `RemoveAll`) return value ignored; a removed file that is actually a (now-empty or non-empty) directory silently survives, and permission errors vanish.
- Scenario: update removing a directory-typed entry → stale empty dirs accumulate; read-only file → update reports success but old file remains.
- Fix: `RemoveAll` + collect errors into the result.
- Test: update fixture with a gone dir asserting it disappears.
- Effort: S

### STEAM-18 — Python QR `rc` written without the lock; `store_cache` stampede
- Severity: low · Category: threading
- `plugins/steam-standalone/bin/platypus-steam:366,378,389` (`qr_proc["rc"] = ...` outside `qr_lock`), `:489-510` (`store_app` fetch outside `store_lock`)
- What's wrong: the QR watcher assigns `qr_proc["rc"]` without holding `qr_lock` while `stop_qr_login`/readers mutate the same dict (CPython makes it atomic but ordering with `ready.set()` is仍 racy; a superseded watcher overwrites the live session's `rc`). Concurrent `library.list` threads miss the same uncached Store entry and issue duplicate HTTP fetches (harmless, just N× traffic on rescans).
- Scenario: duplicate `auth.begin` (StrictMode) → stale watcher clobbers `rc` after the fresh session started; 2 concurrent library scans → 2× Store requests per uncached app.
- Fix: move the `rc` assignment inside the `ours()`/`qr_lock` block; add singleflight (in-flight dict) around `store_app` fetches.
- Test: threaded duplicate-begin test asserting `rc` belongs to the live proc; Store-fetch counter test with concurrent callers.
- Effort: S

## Improvements (not bugs)

1. **Stream the writer (biggest value).** Write each depot's chunks as they land (or per-file as its chunks complete) instead of buffering whole depots; combines STEAM-1/12 fixes and bounds memory to `conns × chunk`. Effort M, value high.
2. **Parallelize all startup RPCs.** One bounded pool for depot keys, request codes, manifests, and CDN tokens; lazy token fetch on first use per host. Turns small-depot installs from seconds of serial round trips to ~one round trip. Effort M, value high.
3. **Monotonic multi-depot progress + windowed speed.** Base-offset accumulation (STEAM-11) and per-second speed; makes the tail report trustworthy. Effort S, value medium.
4. **Recursive executable ranking with tests.** Walk up to depth 3, keep Unity-pair > script > biggest-ELF ordering, skip `*_Data`/redist; fixes a class of "installed but won't launch". Effort S, value medium.
5. **Wire branch passwords end-to-end (or remove).** Either plumb `branch_password` (STEAM-4) or delete `betaHash`/`CheckBetaPassword` so the next reader doesn't assume it works. Effort S–M, value medium.
6. **Plan/code alignment.** `docs/plans/steam-depot-downloader.md` promises "one `http.Transport` per host" and "CPU pool (GOMAXPROCS)" — code has one shared `Transport` and inline decrypt. Either implement or amend the doc so reviewers stop flagging it. Effort S, value low.

## Checked and fine

- Chunk crypto path (AES ECB-IV + CBC/PKCS7, VSZa/VZip/zip magics, Adler-0, CRC/size enforcement) + `TestChunkRoundTrip*`, `TestZstdContainerValidation` — correct and well-pinned.
- Manifest zip/section parse, `DecryptFilenames`, flag constants pinned by `TestFileFlagValues` — correct.
- Scheduler spread/retry/resume/verify against fake CDN (`TestSchedulerDownloadsAcrossHosts`, `SlowAndFailing`, `ResumeAndVerify`, `MultiBeatsSingle`) — core algorithm fine; issues above are bounds/reporting, not the fetch loop.
- Depot selection filters (oslist/osarch/language/lowviolence, shared-entry-first) — matches live byte-exact sizes per plan doc.
- `steamcm` wire: `9804` pre-logon fix, Multi/gzip unwrap with pending queue, heartbeat/license side messages, jobid matching — correct.
- Auth: QR refresh (`new_challenge_url`), password two-submit same-session, `GuardNeeded` email-preferred, EResult 84 no-retry, RSA PKCS#1 v1.5 + base64 — fake-tested paths fine (live password/branch-password still pending per brief).
- Secrets: refresh token on stdin only, never argv/logged; `refresh.token`/`token.json`/`steam-access.json` 0600 + tighten-on-touch; `session.json` 0644 holds no secret — correct.
- QR duplicate-begin serialization + ownership checks + `_reap` (no `<defunct>`), `job.cancel` SIGINT→kill with canceled-suppression, `Verify` skipping chunkless placeholders — correct as far as reviewed.
- `fakesteam` shared-harness design (one fixture → both Go and Python tests) and `TestMain` prebuilt-binary Go-env fix — sound; fake CDN ignoring gid/code is a known test gap, not a product bug.
- Protocol consistency: method table, `CODE_AUTH_REQUIRED=-32001`, `job.progress/done/failed` shapes, `SteamAppId`/`SteamGameId` env (not on denylist), `store-client` spec with `requires: steam` — matches `docs/plugins/protocol.md`.
