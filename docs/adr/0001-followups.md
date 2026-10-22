# Follow-up issue drafts for ADR 0001 / #75

These drafts are intentionally local; they have not been filed on GitHub.
Suggested labels use labels that already exist in `gh label list`.

## 1. Move Steam client metadata, artwork, and save discovery behind the Steam plugin

**Suggested labels:** `core`, `plugin:steam`, `enhancement`, `priority:P2`

**Body:**

### What

Remove the core's `internal/steam` imports from `internal/app` and
`internal/savebackup`. Move Steam client launch detection, app ID metadata,
Steam library artwork URL mapping, store artwork lookup, and Steam save-root
discovery behind the Steam plugin's existing protocol capabilities. Keep the
core's library, customization, and backup APIs store-neutral.

### Why

`go list -deps ./cmd/platypus-core` currently lists `internal/steam`, and the
call sites in `app_gameinfo.go`, `app_presets.go`, `app_library_snapshot.go`,
`app_artwork.go`, `app_backups.go`, and `savebackup/resolve.go` make core
behavior depend directly on Steam's client layout and API.

### Acceptance

- `go list -deps ./cmd/platypus-core | grep 'platypus/internal/steam'` prints nothing.
- The Steam plugin supplies the same artwork, metadata, client-launch, and save-location behavior through versioned capabilities.
- Tests cover a Steam plugin and a non-Steam plugin with no Steam installation present.
- `go test ./internal/app ./internal/savebackup ./plugins/steam ./plugins/steam-standalone` passes.

## 2. Make native script launches host-neutral

**Suggested labels:** `core`, `enhancement`, `priority:P2`

**Body:**

### What

Replace the direct `/bin/sh` fallback in `internal/app/app.go` with a small
host capability that selects a script interpreter for the current OS. Keep
ELF/PE/AppImage detection in `plugins/runner` and preserve the runner's
`runner.detectExecutable` contract.

### Why

Core direct launches still execute `/bin/sh` for non-executable scripts and
shebang files, so the Windows build does not imply those launches work on
Windows. This behavior belongs at the generic process-launch boundary.

### Acceptance

- No core production path references `/bin/sh`.
- Unix script launches retain their current command, arguments, and error behavior.
- Windows scripts use an available Windows interpreter or return a clear unsupported-script error.
- Linux native launch tests and `GOOS=windows go build ./cmd/platypus-core` pass.

## 3. Add platform-native paths and save-location providers

**Suggested labels:** `core`, `enhancement`, `priority:P2`

**Body:**

### What

Keep `internal/paths` as the single resolver, but provide OS-specific defaults
and known save locations. Preserve `PLATYPUS_*` overrides and XDG handling on
Linux; use native user data/config/cache folders on Windows. Remove Steam
roots from the generic resolver as part of the Steam plugin move.

### Why

`internal/paths/paths.go`, `internal/cloudsync/context.go`, and
`internal/savebackup/resolve.go` currently derive defaults and save locations
from XDG and Linux directory conventions.

### Acceptance

- Linux defaults and XDG precedence remain unchanged.
- Windows defaults resolve under the current user's native folders.
- Backup discovery is driven by plugin save locations plus OS-native common folders.
- Unit tests cover environment overrides, defaults, and legacy Linux fallback.

## 4. Move disk and protected-path checks behind platform helpers

**Suggested labels:** `core`, `enhancement`, `priority:P2`

**Body:**

### What

Replace `/proc/self/mountinfo` disk grouping and Linux system-directory string
lists in `internal/app` and `internal/plugins` with OS-specific helpers for
volume identity, filesystem space, roots, and protected install/removal paths.

### Why

`app_diskusage.go`, `jobs/space_unix.go`, `app_remove.go`, `app_discard.go`,
`app_installdir.go`, and `plugins/manager.go` encode Linux mounts, `/`, and
`/usr` assumptions. Windows currently has a build-only unavailable response
for free-space reporting and a mode-bit approximation for writability.

### Acceptance

- Windows disk usage and install space use the selected volume's actual capacity.
- Remove/install/plugin safety checks cover Windows drive roots and protected OS folders.
- Linux mount grouping, free-space values, and protected-directory behavior remain unchanged.
- Unit tests cover paths on different volumes and all platform root cases.

## 5. Route opening files, folders, and URLs through the desktop host

**Suggested labels:** `core`, `enhancement`, `priority:P2`

**Body:**

### What

Use the existing `app.Host` open-file/open-directory boundary for core actions
that currently run `xdg-open` in `app_plugins.go` and `app_crashhints.go`.
Keep Steam URI launching in the Steam plugin and route its open request through
the host where the protocol permits it.

### Why

The desktop shell already uses Electron `shell.openExternal` for URLs, but
these core paths directly invoke the Linux-only `xdg-open` executable.

### Acceptance

- Core production code contains no `exec.Command("xdg-open", ...)` calls.
- Linux opens the same paths and URLs as before.
- Windows opens selected files/folders and external URLs through Electron.
- Host tests cover success and unavailable-host errors.

## 6. Complete Windows process supervision and core stdio support

**Suggested labels:** `core`, `bug`, `priority:P1`

**Body:**

### What

Complete the Windows implementations introduced for `session`, plugin
processes, and `cmd/platypus-core`: use job objects for descendant cleanup,
close inherited handles without closing protocol stdio, and preserve a clean
JSON-RPC stdout channel. Replace the current single-process kill and stdout
fallback limitations.

### Why

The core and plugins cross-compile, but Windows process trees, parent-death
cleanup, inherited-handle cleanup, and protocol stdout separation are not
implemented by the current Windows files.

### Acceptance

- Stopping a direct game or plugin terminates its child processes without killing the launcher.
- The core protocol stream remains isolated from logs and child output.
- Windows-specific tests cover child cleanup, graceful shutdown, forced shutdown, and inherited handles.
- `GOOS=windows go build ./cmd/platypus-core ./plugins/...`, `go test ./internal/session ./internal/plugins`, and `bun run check:windows` pass.

