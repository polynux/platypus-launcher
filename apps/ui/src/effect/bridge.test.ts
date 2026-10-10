import { describe, expect, it, vi } from 'vitest';
import { App, backendAvailable, browserOpen, onEvent } from './bridge';
import { coreMethods } from '../gen/api';

describe('backend bridge', () => {
  it('there is no backend in a plain browser', () => {
    delete window.platypus;
    expect(backendAvailable()).toBe(false);
  });

  it('methods are called by name with their arguments as an array', async () => {
    const invoke = vi.fn(async () => 'shell');
    window.platypus = { invoke, on: vi.fn(), openExternal: vi.fn() };
    expect(backendAvailable()).toBe(true);
    await expect(App.GetLibrary()).resolves.toBe('shell');
    await App.SetCustomTitle('g1', 'Title', 'Sort');
    expect(invoke).toHaveBeenLastCalledWith('SetCustomTitle', ['g1', 'Title', 'Sort']);
  });

  it('without the shell, method calls fail loudly instead of faking data', () => {
    delete window.platypus;
    expect(() => App.GetLibrary()).toThrow('Platypus backend not available (GetLibrary)');
  });

  it('the generated method list covers the bridge surface', () => {
    for (const name of ['GetLibrary', 'LaunchGame', 'ListJobs', 'GetGameSettings', 'ChatOpen', 'SocialViews'] as const) {
      expect(coreMethods).toContain(name);
    }
  });

  it('events come from the bridge', () => {
    const off = vi.fn();
    const on = vi.fn(() => off);
    window.platypus = { invoke: vi.fn(), on, openExternal: vi.fn() };
    const cb = vi.fn();
    const unsubscribe = onEvent<number>('jobs', cb);
    expect(on).toHaveBeenCalledWith('jobs', expect.any(Function));
    (on.mock.calls[0] as unknown as [string, (d: unknown) => void])[1](3);
    expect(cb).toHaveBeenCalledWith(3);
    expect(unsubscribe).toBe(off);
  });

  it('without the shell, subscribing is a no-op', () => {
    delete window.platypus;
    expect(onEvent('jobs', () => {})()).toBeUndefined();
  });

  it('openExternal goes through the bridge, and fails loudly without it', () => {
    const openExternal = vi.fn();
    window.platypus = { invoke: vi.fn(), on: vi.fn(), openExternal };
    browserOpen('https://example.com');
    expect(openExternal).toHaveBeenCalledWith('https://example.com');
    delete window.platypus;
    expect(() => browserOpen('https://example.com')).toThrow('Platypus backend not available (openExternal)');
  });
});
