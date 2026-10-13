import { describe, expect, it, afterEach } from 'vitest';
import i18n from './i18n';
import { coreErrMsg, coreErrorDataOf } from './coreError';

function coded(message: string, code: string, params?: Record<string, string>): Error {
  const err = new Error(message) as Error & { data: unknown };
  err.data = params === undefined ? { code } : { code, params };
  return err;
}

afterEach(async () => {
  await i18n.changeLanguage('en');
});

describe('coreErrMsg', () => {
  it('renders English for a coded error under en', () => {
    expect(coreErrMsg(coded('Doom is running', 'game.running', { title: 'Doom' }))).toBe('Doom is already running.');
  });

  it('renders French for a coded error under fr', async () => {
    await i18n.changeLanguage('fr');
    expect(coreErrMsg(coded('Doom is running', 'game.running', { title: 'Doom' }))).toBe(
      "« Doom » est déjà en cours d'exécution.",
    );
  });

  it('an unknown code falls back to the English message', async () => {
    await i18n.changeLanguage('fr');
    expect(coreErrMsg(coded('mystery failure', 'game.somethingNew'))).toBe('mystery failure');
  });

  it('a plain error is unchanged, in any language', async () => {
    expect(coreErrMsg(new Error('nope'))).toBe('nope');
    expect(coreErrMsg('just a string')).toBe('just a string');
    await i18n.changeLanguage('fr');
    expect(coreErrMsg(new Error('nope'))).toBe('nope');
  });

  it('a malformed payload falls back to the message', () => {
    const err = new Error('raw text') as Error & { data: unknown };
    err.data = { code: 42 };
    expect(coreErrMsg(err)).toBe('raw text');
    err.data = { code: 'game.running', params: { title: 7 } };
    expect(coreErrMsg(err)).toBe('raw text');
    err.data = null;
    expect(coreErrMsg(err)).toBe('raw text');
    expect(coreErrorDataOf(null)).toBeNull();
  });
});
