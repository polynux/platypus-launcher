// @ts-nocheck — node fs/path fixture test, outside the app's DOM tsconfig.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { resolveLanguage } from './i18n';

interface FixtureCase {
  setting: string;
  system: string[];
  ui: string;
  locale: string;
}

// The shared fixture table with the Go locale resolver
// (internal/locale/testdata/cases.json): the TS side checks the ui column.
describe('resolveLanguage fixtures (shared with Go)', () => {
  it('matches the shared table', () => {
    const file = path.join(__dirname, '../../../../internal/locale/testdata/cases.json');
    const raw = fs.readFileSync(file, 'utf8');
    const cases = JSON.parse(raw.toString()) as FixtureCase[];
    expect(cases.length).toBeGreaterThan(0);
    for (const c of cases) {
      expect(resolveLanguage(c.setting, c.system)).toBe(c.ui);
    }
  });
});
