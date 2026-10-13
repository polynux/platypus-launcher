import assert from 'node:assert/strict';
import { test } from 'node:test';
import { resolveShellLanguage, shellText } from './locale.ts';

test('explicit setting wins, with base fallback (fr-CA ships fr)', () => {
  assert.equal(resolveShellLanguage('fr', ['en']), 'fr');
  assert.equal(resolveShellLanguage('fr-CA', ['en']), 'fr');
  assert.equal(resolveShellLanguage('en', ['fr']), 'en');
});

test('system list resolves in order, exact before base', () => {
  assert.equal(resolveShellLanguage('', ['fr-CA', 'en']), 'fr');
  assert.equal(resolveShellLanguage('', ['en-US']), 'en');
  assert.equal(resolveShellLanguage('', ['de', 'fr']), 'fr');
});

test('unknown languages fall back to English', () => {
  assert.equal(resolveShellLanguage('', ['de', 'ja']), 'en');
  assert.equal(resolveShellLanguage('', []), 'en');
  assert.equal(resolveShellLanguage('de', ['de']), 'en');
});

test('shell strings interpolate and fall back to English', () => {
  assert.equal(shellText('en', 'appName'), 'Platypus');
  assert.match(shellText('en', 'coreCrashed', { code: 1 }), /exit 1/);
  assert.match(shellText('fr', 'coreCrashed', { code: 1 }), /sortie 1/);
  assert.match(shellText('fr', 'coreStartFailed', { error: 'boom' }), /boom/);
  assert.equal(shellText('de', 'appName'), 'Platypus');
  assert.equal(shellText('en', 'no.such.key'), 'no.such.key');
});
