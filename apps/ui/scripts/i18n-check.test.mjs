// Fixture test for the i18n:check script (issue #39): missing keys fail,
// unused keys warn, per-locale completeness is reported. Black-box through a
// temp fixture so the script's real CLI surface is what is tested.
import { describe, it, expect, afterEach } from 'vitest';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

const script = path.resolve(import.meta.dirname, './i18n-check.mjs');
const dirs = [];

afterEach(() => {
  while (dirs.length) rmSync(dirs.pop(), { recursive: true, force: true });
});

function fixture({ locales, src }) {
  const dir = mkdtempSync(path.join(tmpdir(), 'i18n-check-'));
  dirs.push(dir);
  for (const [rel, content] of Object.entries(locales)) {
    const p = path.join(dir, 'locales', rel);
    mkdirSync(path.dirname(p), { recursive: true });
    writeFileSync(p, typeof content === 'string' ? content : JSON.stringify(content));
  }
  for (const [rel, content] of Object.entries(src)) {
    const p = path.join(dir, 'src', rel);
    mkdirSync(path.dirname(p), { recursive: true });
    writeFileSync(p, content);
  }
  return dir;
}

function run(dir) {
  try {
    const out = execFileSync(process.execPath, [script, '--locales', path.join(dir, 'locales'), '--src', path.join(dir, 'src')], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    return { exit: 0, stdout: out.toString(), stderr: '' };
  } catch (err) {
    return { exit: err.status ?? 1, stdout: (err.stdout ?? '').toString(), stderr: (err.stderr ?? '').toString() };
  }
}

describe('i18n:check', () => {
  it('fails on keys used in code but missing from en', () => {
    const dir = fixture({
      locales: {
        'en/a.json': { ok: 'fine', unused: 'never used', items_one: 'one', items_other: 'many' },
        'fr/a.json': { ok: 'bon' },
      },
      src: {
        'f.ts': `import { t } from './x';\nconst a = t('a:ok');\nconst b = t('a:missing');\nconst c = t('a:items', { count: 2 });\n`,
      },
    });
    const r = run(dir);
    expect(r.exit).toBe(1);
    expect(r.stderr).toMatch(/missing key in en: a:missing/);
    expect(r.stderr).toMatch(/warning: unused key in en: a:unused/);
    expect(r.stdout).toMatch(/locale fr: 1\/3 keys/);
  });

  it('passes when every used key exists', () => {
    const dir = fixture({
      locales: { 'en/a.json': { ok: 'fine' }, 'fr/a.json': { ok: 'bon' } },
      src: { 'f.ts': `const a = t('a:ok');\n` },
    });
    const r = run(dir);
    expect(r.exit).toBe(0);
    expect(r.stdout).toMatch(/i18n:check ok/);
  });

  it('resolves bare keys against the file hook default', () => {
    const dir = fixture({
      locales: { 'en/a.json': { heading: 'Hi' } },
      src: { 'f.tsx': `const { t } = useTranslation('a');\nconst h = <h1>{t('heading')}</h1>;\n` },
    });
    expect(run(dir).exit).toBe(0);
  });
});
