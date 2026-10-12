import assert from 'node:assert/strict';
import { test } from 'node:test';
import { coreEnv, resolveUserData } from './userdata.ts';

test('userData stays default without overrides, follows config then data', () => {
  assert.equal(resolveUserData({}, '/def'), '/def');
  assert.equal(resolveUserData({ PLATYPUS_CONFIG_DIR: '/cfg', PLATYPUS_DATA_DIR: '/data' }, '/def'), '/cfg/platypus-shell');
  assert.equal(resolveUserData({ PLATYPUS_DATA_DIR: '/data' }, '/def'), '/data/platypus-shell');
});

test('relative overrides are ignored', () => {
  assert.equal(resolveUserData({ PLATYPUS_CONFIG_DIR: 'rel/cfg', PLATYPUS_DATA_DIR: 'rel/data' }, '/def'), '/def');
});

test('the core inherits PLATYPUS_* variables', () => {
  const env = coreEnv({ PATH: '/bin', PLATYPUS_DATA_DIR: '/data', PLATYPUS_PLUGINS: '/plugins' } as NodeJS.ProcessEnv);
  assert.equal(env.PLATYPUS_DATA_DIR, '/data');
  assert.equal(env.PLATYPUS_PLUGINS, '/plugins');
  assert.equal(env.PATH, '/bin');
});
