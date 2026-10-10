import assert from 'node:assert/strict';
import { test } from 'node:test';
import { resolvePaths } from './paths.ts';

test('development uses the repo build outputs unless the env says otherwise', () => {
  const p = resolvePaths({ packaged: false, resourcesPath: '/res', exeDir: '/exe', env: {}, repoRoot: '/repo' });
  assert.equal(p.coreBinary, '/repo/apps/desktop/.build/platypus-core');
  assert.equal(p.frontendDist, '/repo/apps/ui/dist');
  assert.equal(p.iconFile, '/repo/packaging/icons/appicon.png');
  const over = resolvePaths({
    packaged: false,
    resourcesPath: '/res',
    exeDir: '/exe',
    env: { PLATYPUS_CORE: '/tmp/core', PLATYPUS_FRONTEND_DIST: '/tmp/dist' },
    repoRoot: '/repo',
  });
  assert.equal(over.coreBinary, '/tmp/core');
  assert.equal(over.frontendDist, '/tmp/dist');
});

test('a packaged app finds the core next to itself, env first', () => {
  const exists = (p: string): boolean => p === '/res/platypus-core';
  const p = resolvePaths({ packaged: true, resourcesPath: '/res', exeDir: '/exe', env: {}, repoRoot: '/repo', exists });
  assert.equal(p.coreBinary, '/res/platypus-core');
  assert.equal(p.frontendDist, '/res/frontend');
  assert.equal(p.iconFile, '/res/icon.png');
  const moved = resolvePaths({
    packaged: true,
    resourcesPath: '/res',
    exeDir: '/exe',
    env: {},
    repoRoot: '/repo',
    exists: (x) => x === '/exe/platypus-core',
  });
  assert.equal(moved.coreBinary, '/exe/platypus-core');
  const over = resolvePaths({ packaged: true, resourcesPath: '/res', exeDir: '/exe', env: { PLATYPUS_CORE: '/tmp/core' }, repoRoot: '/repo', exists });
  assert.equal(over.coreBinary, '/tmp/core');
});
