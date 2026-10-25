# Platypus

Linux game launcher (Go core, React UI in a desktop shell) with store plugins over a versioned JSON-RPC protocol.

## Repository layout

- `cmd/platypus-core`: the core (the App without a window, over stdio JSON-RPC).
- `cmd/gen-ts`: regenerates the UI's typed core bridge (`apps/ui/src/gen`).
- `internal/…`: the App and its packages (plugins, jobs, sessions, runners, …).
- `apps/ui`: the React + TypeScript UI (Effect-TS, Tailwind, Vite).
- `apps/desktop`: the desktop shell (Electron + Chromium) around the core.
- `plugins/`: store plugins and their install/scaffold scripts.
- `packaging/`: icons and (later) the distributable build config.
- `tools/design-lab`: the UI design mockup (dev server, port 5175).
- `tools/design-lab-2`: redesign explorations (port 5176); `/22` is the main design.
- `docs/`: user and contributor docs (`docs/electron.md` describes the shell).
- Folders: `internal/paths` resolves data/config/cache/plugins from
  `PLATYPUS_*` / XDG variables (defaults are today's `~/.local/share/platypus`
  paths); see `docs/packaging.md`.

## Agent skills

### Issue tracker

Issues live in GitHub Issues of polynux/platypus-launcher, used through the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default triage roles, label string = role name (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` and `docs/adr/` at the repo root (created when first needed). See `docs/agents/domain.md`.

### Tests (issue #40)

- While iterating: `go test <pkg>` for what you touch, `bun run test:ui` for the UI. Both default to the fast set: no network, no slow runtimes.
- Before committing: `bun run test:fast` (default Go + UI + desktop, no network, no slow).
- Full suites: `bun run test:slow`, `bun run test:network`, `bun run test:all`.
- `bun run test` keeps meaning what it always did (UI + desktop only, fast set).
- Labels: slow integration tests (real plugin processes, Python plugins, full protocol conformance, download pacing) call `testenv.RequireSlow(t)` / use `*.slow.test.tsx`, and run only with `PLATYPUS_TEST_SLOW=1`. Network tests call `testenv.RequireNetwork(t)` and run only with `PLATYPUS_TEST_NETWORK=1`; the default run refuses non-loopback HTTP/fetch outright. Details: `docs/testing.md`.
