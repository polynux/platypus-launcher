import { afterEach, describe, expect, it, vi } from 'vitest';
import { App, backendAvailable, onEvent } from './bridge';
import { coreMethods } from '../gen/api';

vi.mock('../../wailsjs/go/main/App', () => ({ GetLibrary: vi.fn(async () => 'wails') }));
vi.mock('../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: vi.fn(), EventsOn: vi.fn(() => () => {}) }));

afterEach(() => {
  delete window.platypus;
});

describe('backend bridge', () => {
  it('there is no backend in a plain browser', () => {
    expect(backendAvailable()).toBe(false);
  });

  it('under Electron, methods are called by name with their arguments as an array', async () => {
    const invoke = vi.fn(async () => 'electron');
    window.platypus = { invoke, on: vi.fn(), openExternal: vi.fn() };
    expect(backendAvailable()).toBe(true);
    await expect(App.GetLibrary()).resolves.toBe('electron');
    await App.SetCustomTitle('g1', 'Title', 'Sort');
    expect(invoke).toHaveBeenLastCalledWith('SetCustomTitle', ['g1', 'Title', 'Sort']);
  });

  it('without the bridge, methods are the Wails bindings', async () => {
    await expect(App.GetLibrary()).resolves.toBe('wails');
  });

  it('the generated method list covers the bridge surface', () => {
    for (const name of ['GetLibrary', 'LaunchGame', 'ListJobs', 'GetGameSettings', 'ChatOpen', 'SocialViews'] as const) {
      expect(coreMethods).toContain(name);
    }
  });

  it('under Electron, events come from the bridge', () => {
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
});
