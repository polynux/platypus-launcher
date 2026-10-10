// Where the shell finds what it runs and shows. In development the core is
// the freshly built binary and the page is apps/ui/dist; in a packaged app
// both travel with the app (electron-builder puts them in resources/) so the
// AppImage runs without a repo checkout around it. Explicit env overrides win
// in both modes, which is what the tests and run.mjs use.
import { existsSync } from 'node:fs';
import path from 'node:path';

export interface ResolvedPaths {
  /** The Go core binary the shell spawns. */
  coreBinary: string;
  /** The built UI the app:// handler serves. */
  frontendDist: string;
  /** The window icon (packaging/icons/appicon.png in the repo). */
  iconFile: string;
}

export function resolvePaths(opts: {
  packaged: boolean;
  resourcesPath: string;
  /** Directory of the app executable (packaged fallback when resources move). */
  exeDir: string;
  env: NodeJS.ProcessEnv;
  /** Repo root, for development only. */
  repoRoot: string;
  exists?: (p: string) => boolean;
}): ResolvedPaths {
  const exists = opts.exists ?? existsSync;
  const pick = (first: string, second: string): string => (exists(first) ? first : second);
  return {
    coreBinary:
      opts.env.PLATYPUS_CORE ??
      (opts.packaged
        ? pick(path.join(opts.resourcesPath, 'platypus-core'), path.join(opts.exeDir, 'platypus-core'))
        : path.join(opts.repoRoot, 'apps', 'desktop', '.build', 'platypus-core')),
    frontendDist: path.resolve(
      opts.env.PLATYPUS_FRONTEND_DIST ??
        (opts.packaged ? path.join(opts.resourcesPath, 'frontend') : path.join(opts.repoRoot, 'apps', 'ui', 'dist')),
    ),
    iconFile: opts.packaged
      ? path.join(opts.resourcesPath, 'icon.png')
      : path.join(opts.repoRoot, 'packaging', 'icons', 'appicon.png'),
  };
}
