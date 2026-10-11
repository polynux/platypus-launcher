import assert from 'node:assert/strict';
import { test } from 'node:test';
import { csp, isAppURL, isArtPath, safePath, staticFile, webURL } from './security.ts';

test('only http and https addresses are web links', () => {
  assert.equal(webURL('https://protondb.com/app/620'), 'https://protondb.com/app/620');
  assert.equal(webURL(' http://example.com/x '), 'http://example.com/x');
  for (const bad of ['javascript:alert(1)', 'file:///etc/passwd', 'app://platypus/', 'not a url', '', undefined, null, 5, {}]) {
    assert.equal(webURL(bad), '', String(bad));
  }
});

test('art paths are the two prefixes the core serves', () => {
  assert.ok(isArtPath('/steam-art/12/library_600x900.jpg'));
  assert.ok(isArtPath('/platypus-art/abc.png'));
  assert.ok(!isArtPath('/steam-artx/1.jpg'));
  assert.ok(!isArtPath('/assets/index.js'));
});

test('paths that climb out are unsafe', () => {
  assert.ok(safePath('/assets/a.js'));
  assert.ok(!safePath('/assets/../../etc/passwd'));
  assert.ok(!safePath('/..'));
  assert.ok(!safePath('/a\\b'));
  assert.ok(!safePath('/a\0b'));
  assert.ok(safePath('/a..b/c'));
});

test('static files resolve inside the root only', () => {
  const root = '/srv/dist';
  assert.equal(staticFile(root, '/'), '/srv/dist/index.html');
  assert.equal(staticFile(root, '/assets/app.js'), '/srv/dist/assets/app.js');
  assert.equal(staticFile(root, '/assets/%2e%2e/%2e%2e/etc/passwd'), null);
  assert.equal(staticFile(root, '/..%2fetc/passwd'), null);
  assert.equal(staticFile(root, '/%zz'), null);
  assert.equal(staticFile(root, '//etc/passwd'), '/srv/dist/etc/passwd');
});

test('the policy plays videos from https and the local art server, and stays strict otherwise', () => {
  const p = csp();
  const media = p.split(';').map((s) => s.trim()).find((s) => s.startsWith('media-src')) ?? '';
  const tokens = media.split(/\s+/).slice(1);
  for (const want of ['https:', 'http://127.0.0.1:*', 'http://localhost:*']) {
    assert.ok(tokens.includes(want), `${want} not in ${media}`);
  }
  // No bare http: (plain web) and no data:/blob: (no inline media).
  assert.ok(!tokens.includes('http:'), media);
  assert.ok(!tokens.includes('data:'), media);
  assert.ok(!tokens.includes('blob:'), media);
  assert.match(p, /default-src 'none'/);
  assert.match(p, /object-src 'none'/);
  assert.match(p, /script-src 'self'(;|$)/);
});

test('the policy lets scripts in from the app only, and the dev one adds the dev socket', () => {
  const p = csp();
  assert.match(p, /script-src 'self'(;|$)/);
  assert.match(p, /connect-src 'self'(;|$)/);
  assert.doesNotMatch(p, /unsafe-eval/);
  assert.match(csp(5173), /connect-src 'self' ws:\/\/127.0.0.1:5173/);
});

test('only the app page is the app page', () => {
  assert.ok(isAppURL('app://platypus/'));
  assert.ok(isAppURL('app://platypus/library?x=1#y'));
  for (const bad of ['app://evil/', 'https://platypus/', 'app://platypus.evil.com/', 'app://user@evil/', 'file:///x', '', 'nonsense']) {
    assert.ok(!isAppURL(bad), bad);
  }
});
