// Bundles the main process and the preload script into dist/. Both are CommonJS:
// a sandboxed preload can't load ES modules, and Electron's main is simplest that way.
import { build } from 'esbuild';
import path from 'node:path';

const root = path.resolve(import.meta.dirname, '..');
const common = { bundle: true, platform: 'node', target: 'node22', external: ['electron'], format: 'cjs', logLevel: 'info', sourcemap: 'linked' };

await build({ ...common, entryPoints: [path.join(root, 'src/main.ts')], outfile: path.join(root, 'dist/main.cjs'), define: { 'import.meta.dirname': '__dirname' } });
await build({ ...common, entryPoints: [path.join(root, 'src/preload.ts')], outfile: path.join(root, 'dist/preload.cjs') });
