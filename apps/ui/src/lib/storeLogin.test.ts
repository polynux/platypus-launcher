import type React from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { acceptTermsActionFor, isTermsLoginError, loginActionFor, openStoreLogin, setStoreLoginOpener } from './storeLogin';

function coded(code: string): Error {
  const err = new Error(code);
  // Tests build the coded-error shape the core bridge puts on thrown errors.
  const shaped: Error & { data?: unknown } = err;
  shaped.data = { code, params: { title: 'Echo Licensed' } };
  return err;
}

afterEach(() => {
  setStoreLoginOpener(null);
});

describe('isTermsLoginError', () => {
  it('matches only the terms login refusal', () => {
    expect(isTermsLoginError(coded('terms.needsLogin'))).toBe(true);
    expect(isTermsLoginError(coded('terms.acceptFirst'))).toBe(false);
    expect(isTermsLoginError(new Error('no space left'))).toBe(false);
    expect(isTermsLoginError(null)).toBe(false);
  });
});

describe('loginActionFor', () => {
  it('offers no action for other errors', () => {
    expect(loginActionFor(new Error('no space left'), 'echo', 'Log in')).toBeNull();
  });

  it('offers "Log in" for the terms login refusal, opening that store', () => {
    const open = vi.fn();
    setStoreLoginOpener(open);
    const action = loginActionFor(coded('terms.needsLogin'), 'echo', 'Log in');
    expect(action?.label).toBe('Log in');
    action?.onClick({} as React.MouseEvent<HTMLButtonElement>);
    expect(open).toHaveBeenCalledWith('echo');
  });
});

describe('openStoreLogin', () => {
  it('is false with no opener registered', () => {
    expect(openStoreLogin('echo')).toBe(false);
  });
});

describe('acceptTermsActionFor', () => {
  it('offers no action for other errors', () => {
    expect(acceptTermsActionFor(new Error('no space left'), 'Review terms', () => {})).toBeNull();
    expect(acceptTermsActionFor(coded('terms.needsLogin'), 'Review terms', () => {})).toBeNull();
  });

  it('offers "Review terms" for the unaccepted-terms refusal', () => {
    const review = vi.fn();
    const action = acceptTermsActionFor(coded('terms.acceptFirst'), 'Review terms', review);
    expect(action?.label).toBe('Review terms');
    action?.onClick({} as React.MouseEvent<HTMLButtonElement>);
    expect(review).toHaveBeenCalled();
  });
});
