// `node apps/desktop/scripts/package.mjs` (root: `bun run package`):
// builds the UI, the shell and the Go core, then packs a Linux AppImage
// (plus a tar.gz) with electron-builder, configured by
// packaging/electron-builder.yml. Output lands in packaging/dist/.
import { execFileSync } from 'node:child_process';
import { chmodSync, mkdirSync, readFileSync, readdirSync, statSync } from 'node:fs';
import path from 'node:path';

const repo = path.resolve(import.meta.dirname, '..', '..', '..');
const desktop = path.join(repo, 'apps', 'desktop');
const ui = path.join(repo, 'apps', 'ui');
const stage = path.join(repo, 'packaging', 'stage');
const coreOut = path.join(stage, 'platypus-core');

// The pinned Electron both the dev shell and the distributable run:
// apps/desktop's exact dependency (bun's isolated layout hides it from
// electron-builder's auto-detection, so pass it explicitly).
const electronVersion = JSON.parse(readFileSync(path.join(desktop, 'package.json'), 'utf8')).devDependencies.electron;

const run = (cmd, args, cwd, env = {}) => execFileSync(cmd, args, { cwd, stdio: 'inherit', env: { ...process.env, ...env } });

mkdirSync(stage, { recursive: true });

console.log('» building the UI');
run('bun', ['run', 'build'], ui);

console.log('» building the shell');
run('node', ['scripts/build.mjs'], desktop);

console.log('» building the Go core (linux-x64, trimmed, no CGO)');
run('go', ['build', '-trimpath', '-o', coreOut, './cmd/platypus-core'], repo, { CGO_ENABLED: '0', GOOS: 'linux', GOARCH: 'amd64' });
chmodSync(coreOut, 0o755);

console.log('» building the bundled plugins (linux-x64, trimmed, no CGO)');
run('node', ['scripts/bundled-plugins.mjs', path.join(stage, 'bundled-plugins'), '--release'], desktop);

console.log('» packing the AppImage and tar.gz');
// The builder pinned in apps/desktop (never a floating bunx latest).
const builder = path.join(desktop, 'node_modules', '.bin', 'electron-builder');
run(builder, ['--config', 'packaging/electron-builder.yml', '-c.electronVersion=' + electronVersion, '--linux', 'AppImage', 'tar.gz', '--publish', 'never'], repo);

console.log('» artifacts in packaging/dist:');
for (const entry of readdirSync(path.join(repo, 'packaging', 'dist'))) {
  const p = path.join(repo, 'packaging', 'dist', entry);
  if (statSync(p).isFile()) console.log(`  ${(statSync(p).size / 1048576).toFixed(1)} MB  ${entry}`);
}
