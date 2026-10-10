# Plugin review: epic / gog / amazon / echo, install.sh, plugtest, protocol

## Summary

Reviewed `plugins/epic-legendary`, `plugins/gog-gogdl`, `plugins/amazon-nile`,
`plugins/echo`, `plugins/install.sh` (+ `new.sh` as the template authors copy),
`internal/plugtest`, `docs/plugins/protocol.md` against all plugin
implementations (including `steam-standalone/bin/platypus-steam` for protocol
conformance) and the Go host (`internal/plugins`, `internal/stores`,
`app_plugins.go`, `internal/jobs`).

The good news: the process model is sound (process groups + group kill,
stdin-close exit, restart backoff, hash pinning), Epic's launch-env denylist
matches the host, and Steam does secrets right (stdin, never argv). The bad
news: Amazon repeats the Steam-QR class of bug (global auth state shared by
threads with no lock), GOG/Amazon uninstalls ignore `job.cancel` and delete
anyway, all three non-Steam plugins put one-time secrets on argv, `install.sh`
never cleans stale files and can't install data plugins, the scaffold in
`new.sh` teaches the wrong protocol, the protocol doc is stale (`qr`,
`native`, `mark_needs_client` undocumented), and the conformance suite tests
only the happy path (no cancel, no update, no duplicate jobId, no concurrency,
no negative launchSpec), which is why all of the above pass CI.

## Findings (most severe first)

### PLUG-1 — Amazon `pending_login` race (same class as the Steam QR race)
- severity: high
- category: concurrency
- file:line: `plugins/amazon-nile/bin/platypus-amazon:110-150` (`auth_begin`),
  `:153-170` (`auth_complete`); global at `:150`
- what's wrong: `pending_login = {}` is module-global, written by `auth_begin`
  (`pending_login.update({...})`) and read/cleared by `auth_complete`, with no
  lock. RPC handlers run in one thread per request
  (`main`, `:427`: `threading.Thread(target=handle, ...)`). Two overlapping
  `auth.begin` calls (React StrictMode double-submit, two windows, retry while
  the first dialog is open) overwrite each other's
  `client_id`/`code_verifier`/`serial`. The first dialog's `auth.complete` then
  registers with the second dialog's verifier.
- concrete scenario: open the Amazon login twice (or StrictMode fires
  `auth.begin` twice). Begin A generates verifier Va, begin B generates Vb and
  overwrites the global. User completes A with a valid redirect URL → plugin
  calls `nile register --code <A-code> --code-verifier Vb --serial Sb` → Amazon
  rejects the PKCE challenge → `Amazon rejected the login` even though the user
  did everything right. Worse, completing B after A started can consume A's
  triple and leave A's dialog in `pending_login.clear()` emptiness so a retry
  says `start the login again; it expired`.
- proposed fix: guard with a lock and bind each begin to a session token
  returned to the caller (e.g. a `loginId` field echoed back in `auth.complete`,
  or store triples keyed by a UUID in a dict under `jobs_lock`-style lock with
  expiry). At minimum serialize begin/complete under one lock so a second begin
  either reuses or explicitly invalidates the first with a clear error.
- test that would catch it: fire two `auth.begin` concurrently against the fake
  `nile`, complete the first, assert the `register` argv in
  `$FAKE_NILE_STATE.calls` contains the first begin's verifier (capture it by
  reading the plugin's `pending_login` or by making the fake echo the verifier).
  Today the second verifier wins.
- effort: S (lock + dict keyed by login id)

### PLUG-2 — GOG/Amazon uninstall ignores `job.cancel` and deletes anyway
- severity: high
- category: correctness / data loss
- file:line: `plugins/gog-gogdl/bin/platypus-gog:314-321` (`start_job` uninstall
  never enters `jobs`), `:437-445` (`follow_uninstall` no cancel check);
  `plugins/amazon-nile/bin/platypus-amazon:246-248`, `:297-303` (same shape).
  Contrast `plugins/epic-legendary/bin/platypus-epic:261-277` (uninstall gets a
  `Popen` + `jobs[jobId]` entry) + `:280-310` (`follow_job` checks `canceled`).
- what's wrong: uninstall runs in a bare thread with no entry in `jobs`, so
  `job.cancel` (`platypus-gog:458-465`, `platypus-amazon:316-323`) finds nothing and
  returns `{}`. `follow_uninstall` then unconditionally removes the game dir
  (GOG: `shutil.rmtree`) or runs `nile uninstall`, and emits `job.done`. The
  queue already marked the job canceled and will ignore the late `job.done`
  (`internal/jobs/queue.go:328-343`), but the files are gone. Epic's uninstall
  is SIGINT-able and aborts; GOG/Amazon diverge from Epic, from each other only
  in that they agree, and from the doc (`protocol.md:144`: "After
  `job.cancel`, stop the work").
- concrete scenario: user clicks Uninstall on a 60 GB GOG game, immediately
  hits Cancel (misclick / wrong game). Queue marks canceled within ms; 200 ms
  later `follow_uninstall` wakes, `rmtree`s the game dir, sends `job.done`
  (ignored). Game deleted despite Cancel. Same for Amazon with a slower `nile
  uninstall` (larger window, no cancel possible at all).
- proposed fix: register uninstalls in `jobs` (`{canceled: False}` + proc when
  there is one), check `canceled` before the destructive step and skip
  `rmtree`/`nile uninstall` + don't emit `job.done` when canceled (mirror
  `follow_job`). For the 0.2 s sleep, wait on an event so cancel wakes it.
- test that would catch it: start uninstall of the conformance game, call
  `job.cancel` immediately, wait 1 s, assert the game dir still exists and the
  library still reports installed. Today the dir is gone.
- effort: S

### PLUG-3 — One-time secrets passed on argv (ps-visible)
- severity: medium
- category: security
- file:line: `plugins/epic-legendary/bin/platypus-epic:122`
  (`legendary("auth", "--code", code, ...)`); `plugins/gog-gogdl/bin/platypus-gog:171`
  (`gogdl("auth", "--code", code, ...)`); `plugins/amazon-nile/bin/platypus-amazon:163-166`
  (`nile("register", "--code", code, "--client-id", ..., "--code-verifier", ...,
  "--serial", ...)`). Contrast `steam-standalone/bin/platypus-steam:653-658,683`
  (password / guard code over stdin JSON, refresh token on stdin, never argv).
- what's wrong: the authorization code (and for Amazon the PKCE verifier +
  client id + serial) appear in the child argv, visible to any local process via
  `ps`/`/proc/<pid>/cmdline` during the 1–120 s auth run. Short-lived, but
  bearer-equivalent until exchanged, and the process table is world-readable by
  default.
- concrete scenario: run `watch -n0.1 'ps aux | grep -E "legendary|gogdl|nile"'`
  while logging in; the full `--code <secret>` line is captured. On a shared /
  streaming / compromised-adjacent box this leaks the login.
- proposed fix: follow the Steam pattern — pass secrets via stdin or env (check
  whether legendary/gogdl/nile accept stdin/env for these; if not, at minimum
  document it and shorten exposure). Never log argv (the plugins already log
  only tails, good).
- test that would catch it: fake CLI asserts no secret appears in `sys.argv`
  (read `/proc/self/cmdline` or have the fake fail if `--code` value looks like
  the GOOD_CODE passed positionally); only stdin/env allowed.
- effort: M (depends on what each CLI accepts; stdin support may need upstream flags)

### PLUG-4 — Conformance suite tests only the happy path, so PLUG-1–3, 6–8 pass CI
- severity: medium
- category: testing
- file:line: `internal/plugtest/plugtest.go:221-327` (`Suite.Conformance`);
  `internal/plugtest/conformance_test.go` (per-plugin wiring)
- what's wrong: `Conformance` drives initialize → auth → library → details →
  install → launchSpec (preview+real) → uninstall, and asserts shapes. It never:
  `auth.logout`; `-32001` when logged out; `job.cancel` (echo ships a `slow`
  game precisely for cancel, but `PickGame` always picks `tiny`); `game.update`;
  duplicate `jobId` resume/idempotency (doc §Jobs requires it);
  concurrent `library.list` + `job.cancel` (doc §Process requires concurrency);
  negative launchSpec (echo ships `escape`/`client`, never picked);
  `redact` prefixes; `initialize` vs manifest capability mismatch;
  `installSize` vs actual progress totals; error-code mapping (`-32001` vs
  `-32000` vs `-32602`). Fakes are incomplete too: the fake `nile`
  (`plugins/amazon-nile/testdata/nile:71-88`) has no `update` branch, so even a
  future update test would fail with `fake nile doesn't know update`.
- concrete scenario: every finding in this report except the happy-path shapes
  passes `go test ./internal/plugtest/`: Amazon double-begin race, GOG/Amazon
  cancel-ignored uninstall, argv secrets, scaffold `game.update → install`,
  GOG `native`/`platform` extra keys, Amazon missing `speed`, GOG empty title
  for orphans — none exercised.
- proposed fix: extend `Suite.Conformance` with: cancel-install (pick `slow`-like
  game or cancel mid-download and assert `canceled`, no `job.done`); update
  round-trip; same-`jobId` re-`install` idempotency; 5× parallel `library.list`
  while a job runs; `auth.logout` + logged-out `-32001`; launchSpec negative
  (outside folder refused, tested via echo `escape` in a dedicated case or a
  fake installed-outside entry); assert `job.progress` carries `bytes`+`total`
  and `speed` when claimed. Add `update` to the fake `nile`.
- test that would catch it: the new subtests themselves (they fail today).
- effort: M

### PLUG-5 — `install.sh`: stale files never removed, data plugins uninstallable, fragile `exec` parse, silent missing deps
- severity: medium
- category: installer
- file:line: `plugins/install.sh:17-18` (sed parse + `has no exec` rejection);
  `:20-28` (build/copy); `:31-40` (helpers); `:44-54` (module copy, no delete)
- what's wrong:
  1. No deletion: the module loop copies `src/bin/*` over `dest/bin/*` but
     never removes files deleted upstream. A removed helper (e.g. old
     `steam_cm.py`) stays in the installed plugin, still importable
     (`sys.path.insert(0, dirname(...))`), still hashed (so the pinned hash
     covers a file upstream no longer ships). Updates can leave zombie code.
  2. Data plugins (`kind: data`, no `exec`) are rejected (`has no exec`), so
     there is no scripted way to install the very plugin kind the doc says
     "needs no approval".
  3. `exec_rel` comes from `sed -n 's/.*"exec": *"\([^"]*\)".*/\1/p'`: matches
     any line containing `"exec":`, including inside `description`/comments, and
     takes the last match on multi-match lines. A description mentioning
     `"exec": "bin/x"` would hijack the install target.
  4. Missing-dependency messages: `go`/`python3`/`cp` absence surfaces as raw
     `command not found` / `go build` errors, not the manifest `hint` the
     Plugins view would show. `set -eu` aborts mid-copy leaving `.new` files
     behind (harmless but untidy).
- concrete scenario: upstream deletes `bin/old_helper.py`; run `install.sh`;
  `dest/bin/old_helper.py` remains, `import old_helper` still succeeds, hash
  differs from a clean install of the same version. Or: try
  `install.sh my-data-pack` → `has no exec`, dead end. Or: a `description`
  containing the string `"exec": "bin/evil"` on one line redirects the binary
  write (mitigated: repo-owned manifests, but the parser is still wrong).
- proposed fix: parse `plugin.json` with `python3 -c json.load` (or `jq` if
  guaranteed) instead of sed; support `kind: data` (copy listed `data` files +
  manifest); sync the exec dir (delete dest files absent from src, excluding
  the running exec which is replaced by rename); print manifest `hint`s when
  `go`/`python3` is missing; clean `*.new` on failure via trap.
- test that would catch it: install, delete a module from src, reinstall,
  assert dest no longer contains it; install a fixture data plugin; feed a
  manifest whose description contains `"exec":` and assert the right binary wins.
- effort: S-M

### PLUG-6 — `new.sh` scaffold teaches the wrong protocol (copied by every new plugin)
- severity: medium
- category: docs / template correctness
- file:line: `plugins/new.sh:58` (`code=1` default, should be `-32000` family);
  `:66-74` (`send` omits `"jsonrpc": "2.0"`); `:156-200` (`jobs` dict unused,
  `follow_install` un cancellable); `:211-212` (`install_path` in-memory only);
  `:245` (`"game.update": lambda p: start_job(p, "install")`); `:252-271`
  (`main` replies to notifications, no `id` guard, no `jsonrpc` on responses,
  generic `code: 1`); `:126-138` (`library_list` lists while logged out)
- what's wrong: the template every new store plugin starts from: maps
  `game.update` to a full `install` (update semantics lost from day one);
  answers notifications (`{"id": None}` responses, violating JSON-RPC — the Go
  `jsonrpc.Conn` tolerates it, but strict hosts won't); omits `jsonrpc: 2.0` on
  responses; keeps install state in process memory (`INSTALLS`) instead of
  `$PLATYPUS_PLUGIN_DATA`, so a restart forgets installs (doc requires
  resumable/idempotent same-`jobId`); `job.cancel` pops from a dict
  `start_job` never fills (no-op); library lists without login (all four real
  plugins gate on auth). Authors who fill only the STORE sections ship these.
- concrete scenario: scaffold → `install.sh` → install stub game → restart
  Platypus (plugin process exits) → library shows uninstalled (state lost).
  Or: queue an update → plugin does a full reinstall into a fresh dir.
  Or: cancel the stub install → nothing stops (`follow_install` has no handle).
- proposed fix: fix the template: `update → update`, guard `if "id" not in msg:
  continue`, always set `jsonrpc`, persist `INSTALLS` under
  `$PLATYPUS_PLUGIN_DATA/installed.json`, register jobs + honor cancel, gate
  `library_list` on login with `-32001`. Copy the `out_lock`/`jobs_lock`
  pattern from the real plugins.
- test that would catch it: run the scaffold through `Suite.Conformance` plus a
  restart test (kill plugin, re-`library.list`, assert still installed) and a
  cancel test. Today all three fail.
- effort: S

### PLUG-7 — Cross-plugin divergences the suite should pin (auth/size/launch details)
- severity: medium
- category: protocol consistency
- file:line (a) no-`speed`: `platypus-amazon:267-294` (`follow_job` never sets
  `speed`; doc table `protocol.md:134-139` says speed in bytes/s; Epic `:292-301`
  and GOG `:343-356` send it); (b) `installSize` auth-mapping:
  `platypus-gog:277` (`"auth" in err.lower()` — any stderr containing "auth",
  e.g. "authoritative", becomes `-32001`); `platypus-amazon:229`
  (`"logged" in err.lower()`); Epic `:84-85` (exact `Login failed`/`not logged
  in`); (c) `game.details` unknown-id: Epic `:208` raises, GOG `:500-553`
  swallows to `{}` (typo'd id shows empty page, no error); Amazon has no
  `game.details` at all (`:379-392` METHODS) — allowed, but then `game.details`
  coverage in `Conformance` (`plugtest.go:278-282`) asserts nothing, so the
  three behaviors are never compared; (d) Amazon `find_exe` blocklist
  (`platypus-amazon:355-369`) excludes `uninst/setup/redist/dxsetup` but not
  `vcredist/dotnet`, which Steam's picker (`platypus-steam:1145-1148,1175`)
  excludes — Amazon can pick `vcredist_x64.exe` as the game exe.
- concrete scenario: (a) Amazon progress bars never show speed (queue sets
  `Speed=0`, UI shows --). (b) A GOG `info` failure mentioning "auth" in an
  unrelated word (e.g. "authoritative server error") reports "log in first"
  instead of the real error. (c) `game.details {"id": "typo"}` → Epic errors,
  GOG returns `{}`, Amazon `-32601`; the game page shows three different
  things. (d) An Amazon game shipping `vcredist_x64.exe` larger than the real
  game exe launches the redist.
- proposed fix: add `speed` to Amazon progress (parse the `25.0 MiB/s` the fake
  already prints); narrow auth-match to the CLIs' real logged-out strings
  (assert in tests); make unknown-id `game.details` return a real error
  everywhere (or document `{}` and conform); extend Amazon's exe blocklist to
  `vcredist/dot NET` like Steam.
- test that would catch it: conformance asserts `speed >= 0` present on at least
  one progress per install; unknown-id details returns error (or documented
  empty) identically; fake Amazon game dir containing a big `vcredist.exe`
  launches the real game.
- effort: S

### PLUG-8 — Protocol doc stale vs implementations + host
- severity: medium
- category: docs
- file:line: `docs/plugins/protocol.md:68-82` (methods table), `:84-86`
  (auth.begin), `:127-144` (jobs), `:146-191` (launch); vs
  `platypus-steam:519-553` (`qr` in `auth.status`/`auth.begin`, `wait_qr_login`),
  `platypus-gog:483-490` + `platypus-steam:1246-1260` (`native` inside launchSpec —
  not in the doc's LaunchSpec, which only defines `native` on library entries
  `:101-103`), `platypus-gog:264-270` (`platform`/`executable` extra keys on
  library entries), `platypus-steam:1273-1282` (`mark_needs_client`, not in the
  table at all), `internal/types/models.go:140-149` (`AuthPrompt.QR`,
  `AuthStatus.QR` — implemented, undocumented in `protocol.md`)
- what's wrong: the doc new plugin authors read doesn't mention the QR flow
  (Steam's primary login), the `native` key two plugins return in launchSpec
  (host ignores unknown JSON keys, so harmless today, but a strict future host
  or a new author copying GOG will propagate it), the extra library keys
  (`platform`, `executable`, `genres`-as-debug on Amazon `:218-220`), or the
  `mark_needs_client` out-of-band method. It also says installs go "into
  `installDir`, never anywhere else" (`:131`) while Steam creates
  `installDir/<game>` (`platypus-steam:1061`) and GOG falls back to `path` itself
  (`platypus-gog:388-402`) — both compliant in spirit, but the doc never says
  subdirs are expected, so `find_game_dir`'s `return path` fallback (two games
  sharing one dir) looks legal.
- concrete scenario: author implements strictly to the doc, omits `qr`,
  wonders why Steam-style login can't be expressed; or copies GOG's
  `platform`/`executable`/`native` keys assuming they're required, then a
  future strict host rejects them.
- proposed fix: document `qr` on `auth.status`/`auth.begin`, `native` (or
  forbid it) on launchSpec, `mark_needs_client`, and the install-subdir
  convention (one dir per game under `installDir`); either bless or ban the
  extra library keys. Add a host-side strict-mode test that fails on unknown
  keys so doc and wire stay in sync.
- test that would catch it: a doc-vs-code test asserting every key the four
  plugins emit appears in the doc (snapshot the METHODS tables + sample
  payloads).
- effort: S (doc) / M (strict test)

### PLUG-9 — Small correctness lows (grouped; each S to fix)
- severity: low
- category: correctness
- file:line:
  - GOG orphan title: `platypus-gog:369-385` (`record_install` stores
    `"title": job.get("title", "")`) but `start_job` (`:337-338`) never stores
    `title` in `jobs[jobId]` → orphan entries (`:239-243`,
    `title: ig.get("title") or app`) show the numeric id. Scenario: uninstall a
    game from GOG Galaxy (owned list no longer contains it) → Platypus lists `1234`
    instead of `Super Game`. Fix: pass `title` through `start_job` params (the
    queue knows it) or keep the pre-install library title.
  - GOG `playTasks` filter inverted for manifests without `isPrimary`:
    `platypus-gog:420-426` (`if rel and not task.get("isPrimary"): continue`)
    skips every task lacking the flag, falling through to the `.sh` walk even
    when `playTasks[0].path` names the real exe. Scenario: Windows game whose
    `goggame-*.info` has no `isPrimary` → launcher picks an arbitrary `.sh`
    or `""` → `couldn't find a start script`. Fix: prefer primary, fall back to
    any task path.
  - GOG empty-Bearer header: `platypus-gog:115-125` (`api_get` always sends
    `Authorization: Bearer ` even when `token == ""`, e.g. `platform_of`
    `:302`). Scenario: `products/{id}?expand=downloads` rejects empty bearer
    where anonymous succeeds → Linux games install as Windows. Fix: omit the
    header when token is empty.
  - Amazon `uuid1` serial: `platypus-amazon:113` (`uuid.uuid1().hex`) embeds MAC +
    timestamp, sent to Amazon and logged in calls files. Scenario: state/calls
    files shared in a bug report leak the MAC. Fix: `uuid4`.
  - Shutdown fragility: epic/gog/amazon `main` builds
    `[j["proc"] for j in jobs.values()]` (`platypus-epic:429-430`,
    `platypus-gog:607-608`, `platypus-amazon:428-429`); a future entry without `proc`
    raises `KeyError` during shutdown. Steam already uses `j.get("proc")` +
    `None` guard (`platypus-steam:1412-1415`, `stop_proc(None)` safe). Fix: copy
    Steam's form.
  - `amazon setup_env()` (`platypus-amazon:87-91`) mutates `os.environ` on every
    call with no lock; benign today (same value) but races with any future
    per-login value. Fix: set once at startup.
- test that would catch them: orphan-title round-trip; fixture
  `goggame-*.info` without `isPrimary`; header capture in fake API server;
  `grep uuid1`; kill-plugin-during-uninstall shutdown test.
- effort: S each

## Improvements (not bugs)

1. Share one Python RPC skeleton across epic/gog/amazon/steam (today four
   copies of `send`/`notify`/`RPCError`/`handle`/`main` + three copies of
   `stop_proc`/`job_cancel`). A `platypus_plugin.py` next to each exec (shipped by
   `install.sh`'s module loop) would kill PLUG-6-class drift. Value high,
   effort S-M.
2. Unify child-CLI error mapping in one helper (`auth-required` iff the CLI's
   documented logged-out string, else verbatim tail). Today three regexes with
   different false-positive rates (PLUG-7b). Value medium, effort S.
3. Make `install.sh` print the manifest `hint` for the missing tool and exit 2
   with the plugin id, so headless users get the same guidance as the Plugins
   view. Value medium, effort S.
4. Record `installSize` in the job record and assert `progress.total ==
   installSize.downloadBytes` in conformance (catches unit mixups like MiB vs
   bytes; Amazon's `float(...) * MIB` string-parse is exactly the shape that
   breaks). Value medium, effort S.
5. Have fakes log argv to `.calls` *without* secrets (or with redaction like
   `RedactArgs`), so call-logs can be pasted into issues. Today
   `$FAKE_*_STATE.calls` contains real GOOD_CODEs. Value low-medium, effort S.

## Checked and fine

- Epic launch-env denylist matches host `deniedEnv`/`deniedEnvPrefixes`
  (`platypus-epic:29-35` vs `internal/stores/launchspec.go:20-31`).
- Plugin process lifecycle: own process group + `Pdeathsig`, group kill on
  wait, stdin-close exit, restart backoff + 5-crash give-up
  (`internal/plugins/process.go:156-248`).
- Hash pinning covers manifest + all regular files, skips `__pycache__`/`*.pyc`,
  counts symlink targets (`internal/plugins/manifest.go:247-292`); exec
  confined to folder + must be executable (`:199-219`).
- `install.sh` atomic replace-by-rename for exec/helpers/modules + manifest
  (`:28,38,53,55`), so a running plugin keeps its old binary until restart.
- Free-space gate uses `installSize` before `game.install`, margin included,
  `update` uses download bytes (`internal/jobs/queue.go:177-215`).
- LaunchSpec path validation resolves symlinks and confines exe/cwd to
  install/allowed folders; delegate command must be a manifest `requires` bin
  (`internal/stores/launchspec.go:52-155`); `outside` behavior covered by
  Epic's `Pre` test.
- `auth.begin` URL scheme restricted to http/https in `app_plugins.go:165-169`;
  empty-field fallback to `code` field (`:171-184`).
- Echo reference behavior: crash-resume (`flaky`), huge-game space refusal,
  `escape`/`client` launch paths, `redact` preview tokens
  (`plugins/echo/main.go:313-436`).
- Steam secrets hygiene: password/guard/refresh token over stdin, never argv
  or logs; token files `0600` (`platypus-steam:653-658,949-972,121-136`).
- GOG `install_size` max-language selection with `*` all-zeros noted
  (`platypus-gog:284-291`); Epic offline `status --json --offline` for auth checks
  (`platypus-epic:96-100`).
- `plugtest` wiring (copy tree, fake PATH, approve-then-ready, queue through
  real `jobs.Queue`) is faithful to `app.go` wiring; per-plugin tests assert
  call shapes (`--base-path`, `--skip-sdl`, redact) beyond conformance.
