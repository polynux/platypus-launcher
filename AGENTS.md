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
- `docs/`: user and contributor docs (`docs/electron.md` describes the shell).

## Agent skills

### Issue tracker

Issues live in GitHub Issues of polynux/platypus-launcher, used through the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default triage roles, label string = role name (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` and `docs/adr/` at the repo root (created when first needed). See `docs/agents/domain.md`.
