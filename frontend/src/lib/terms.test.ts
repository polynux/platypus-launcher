import { describe, it, expect } from 'vitest';
import { acceptedLabel, pendingTerms, termURL } from './terms';

describe('terms', () => {
  it('a version nobody accepted is pending, the accepted one is not', () => {
    const pending = pendingTerms([
      { id: 'eula', title: 'Licence', version: '2026.2', accepted_at: undefined },
      { id: 'privacy', title: 'Privacy', version: '3', accepted_at: 1700000000 },
    ]);
    expect(pending.map((t) => t.id)).toEqual(['eula']);
  });

  it('only web addresses are links', () => {
    expect(termURL({ url: 'https://example.com/eula' })).toBe('https://example.com/eula');
    expect(termURL({ url: 'http://example.com/x' })).toBe('http://example.com/x');
    expect(termURL({ url: 'javascript:alert(1)' })).toBe('');
    expect(termURL({ url: 'not a url' })).toBe('');
    expect(termURL({})).toBe('');
  });

  it('an unaccepted term has no date', () => {
    expect(acceptedLabel({})).toBe('');
    expect(acceptedLabel({ accepted_at: 1700000000 })).not.toBe('');
  });
});
