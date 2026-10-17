// Builds the bundled plugins into <outdir>: one ready plugin folder per
// bundled plugin (Steam store and Linux compatibility facts).
//
//   node scripts/bundled-plugins.mjs <outdir> [--release]
//
// <outdir>/steam holds bin/platypus-steam, plugin.json and icon.svg.
// --release builds trimmed for linux-x64 without CGO (packaged builds);
// without it the build is for the current machine (dev runs).
import { execFileSync } from 'node:child_process';
import { chmodSync, copyFileSync, existsSync, mkdirSync } from 'node:fs';
import path from 'node:path';

const outdir = process.argv[2];
const release = process.argv.includes('--release');
if (!outdir) {
  console.error('usage: bundled-plugins.mjs <outdir> [--release]');
  process.exit(2);
}
const repo = path.resolve(import.meta.dirname, '..', '..', '..');
const run = (cmd, args, cwd, env = {}) =>
  execFileSync(cmd, args, { cwd, stdio: 'inherit', env: { ...process.env, ...env } });

for (const [id, command] of [['steam', 'platypus-steam'], ['linux-compat', 'platypus-linux-compat'], ['runner', 'platypus-runner']]) {
  const dest = path.join(path.resolve(outdir), id);
  mkdirSync(path.join(dest, 'bin'), { recursive: true });
  const bin = path.join(dest, 'bin', command);
  if (release) {
    run('go', ['build', '-trimpath', '-o', bin, `./plugins/${id}`], repo,
      { CGO_ENABLED: '0', GOOS: 'linux', GOARCH: 'amd64' });
  } else {
    run('go', ['build', '-o', bin, `./plugins/${id}`], repo);
  }
  chmodSync(bin, 0o755);
  copyFileSync(path.join(repo, 'plugins', id, 'plugin.json'), path.join(dest, 'plugin.json'));
  const icon = path.join(repo, 'plugins', id, 'icon.svg');
  if (existsSync(icon)) copyFileSync(icon, path.join(dest, 'icon.svg'));
}
console.log(`» bundled plugins in ${path.resolve(outdir)}`);
