// Builds the bundled store plugins (issue #54) into <outdir>: one ready
// plugin folder per bundled plugin, currently just `steam` (a Go plugin in
// this module, like the other Go plugins plugins/install.sh builds).
//
//   node scripts/bundled-plugins.mjs <outdir> [--release]
//
// <outdir>/steam holds bin/platypus-steam, plugin.json and icon.svg.
// --release builds trimmed for linux-x64 without CGO (packaged builds);
// without it the build is for the current machine (dev runs).
import { execFileSync } from 'node:child_process';
import { chmodSync, copyFileSync, mkdirSync } from 'node:fs';
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

const dest = path.join(path.resolve(outdir), 'steam');
mkdirSync(path.join(dest, 'bin'), { recursive: true });
const bin = path.join(dest, 'bin', 'platypus-steam');
if (release) {
  run('go', ['build', '-trimpath', '-o', bin, './plugins/steam'], repo,
    { CGO_ENABLED: '0', GOOS: 'linux', GOARCH: 'amd64' });
} else {
  run('go', ['build', '-o', bin, './plugins/steam'], repo);
}
chmodSync(bin, 0o755);
for (const file of ['plugin.json', 'icon.svg']) {
  copyFileSync(path.join(repo, 'plugins', 'steam', file), path.join(dest, file));
}
console.log(`» bundled plugins in ${path.resolve(outdir)}`);
