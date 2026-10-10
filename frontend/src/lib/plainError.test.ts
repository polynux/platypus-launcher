import { describe, expect, it } from 'vitest';
import { hasDetails, plainError } from './plainError';

const FALLBACK = 'The store plugin didn\'t answer. Try again.';

describe('plainError', () => {
  it('keeps a sentence core wrote for a person', () => {
    expect(plainError('Echo Client is not installed', FALLBACK)).toBe('Echo Client is not installed');
  });

  it('hides JSON-RPC and parameter wording behind the fallback', () => {
    expect(plainError('invalid params: ctx needs os (linux or windows), an absolute installDir', FALLBACK)).toBe(FALLBACK);
    expect(plainError('Cloud saves couldn\'t be checked: -32602 bad request', FALLBACK)).toBe(FALLBACK);
    expect(plainError('json: cannot unmarshal string into Go value', FALLBACK)).toBe(FALLBACK);
    expect(plainError('TypeError: Cannot read properties of undefined', FALLBACK)).toBe(FALLBACK);
  });

  it('uses the fallback for an empty message', () => {
    expect(plainError('   ', FALLBACK)).toBe(FALLBACK);
    expect(plainError(undefined, FALLBACK)).toBe(FALLBACK);
  });
});

describe('hasDetails', () => {
  it('offers Details only when the shown sentence differs from the raw text', () => {
    expect(hasDetails('Echo Client is not installed', FALLBACK)).toBe(false);
    expect(hasDetails('invalid params: ctx needs os', FALLBACK)).toBe(true);
    expect(hasDetails('', FALLBACK)).toBe(false);
  });
});
