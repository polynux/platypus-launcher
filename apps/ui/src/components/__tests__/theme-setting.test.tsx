import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
  listThemes: vi.fn(),
  getLauncherSettings: vi.fn(),
  setTheme: vi.fn(),
  getTheme: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
}));

import { ThemeSetting } from '../LauncherSettingsView';

const themes = [
  { id: 'atlas', name: 'Atlas', schema: 1, supports: ['tokens', 'variants'], status: 'ok', warnings: [] },
  { id: 'warm', name: 'Warm', schema: 1, supports: ['tokens'], status: 'ok', warnings: [] },
  { id: 'cool', name: 'Cool dark', schema: 1, supports: ['tokens'], status: 'ok', warnings: ['unknown token "glow", ignored'] },
  { id: 'future', name: 'Future', schema: 2, supports: ['tokens'], status: 'newer', warnings: ['needs schema 2, this Platypus reads schema 1'] },
  { id: 'fixture', name: 'Fixture', schema: 1, supports: ['tokens'], status: 'ok', warnings: [], source: 'fixture-theme' },
];
const coolTokens = {
  id: 'cool',
  tokens: { accent: '#7dd3fc', ground: '#0b1118' },
  wash: [],
};

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.listThemes.mockReturnValue(Effect.succeed(themes));
  api.getLauncherSettings.mockReturnValue(Effect.succeed({ theme: '' }));
  api.setTheme.mockReturnValue(Effect.succeed({ theme: 'cool' }));
  api.getTheme.mockReturnValue(Effect.succeed(coolTokens));
  window.localStorage.clear();
});

afterEach(() => {
  document.documentElement.style.cssText = '';
});

describe('ThemeSetting', () => {
  it('shows the default theme when none is picked, and lists themes that cannot be picked, with the reason', async () => {
    const { getByRole, findByRole } = render(<ThemeSetting onNotify={() => {}} />);
    await waitFor(() => expect(getByRole('combobox', { name: 'Theme' }).textContent).toContain('Atlas'));
    fireEvent.click(getByRole('combobox', { name: 'Theme' }));
    const future = await findByRole('option', { name: /Future/ });
    expect(future).toHaveAttribute('aria-disabled', 'true');
    expect(future.textContent).toContain('needs a newer Platypus');
    expect(getByRole('option', { name: 'Cool dark' })).not.toHaveAttribute('aria-disabled', 'true');
  });

  it('names the plugin a theme comes from', async () => {
    const { getByRole, findByRole } = render(<ThemeSetting onNotify={() => {}} />);
    await waitFor(() => expect(getByRole('combobox', { name: 'Theme' }).textContent).toContain('Atlas'));
    fireEvent.click(getByRole('combobox', { name: 'Theme' }));
    expect(await findByRole('option', { name: /Fixture.*fixture-theme/ })).not.toHaveAttribute('aria-disabled', 'true');
  });

  it('lists the warnings of each theme where they can be read', async () => {
    const { findByRole } = render(<ThemeSetting onNotify={() => {}} />);
    const list = await findByRole('list', { name: 'Theme problems' });
    expect(list).toHaveTextContent('Cool dark: unknown token "glow", ignored');
    expect(list).toHaveTextContent('Future: needs schema 2, this Platypus reads schema 1');
  });

  it('applies a picked theme at once and says so', async () => {
    const onNotify = vi.fn();
    const { getByRole, findByRole } = render(<ThemeSetting onNotify={onNotify} />);
    await waitFor(() => expect(getByRole('combobox', { name: 'Theme' }).textContent).toContain('Atlas'));
    fireEvent.click(getByRole('combobox', { name: 'Theme' }));
    fireEvent.click(await findByRole('option', { name: 'Cool dark' }));
    await waitFor(() => expect(api.setTheme).toHaveBeenCalledWith('cool'));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('Theme applied'));
    expect(api.getTheme).toHaveBeenCalledWith('');
    expect(document.documentElement.style.getPropertyValue('--color-accent')).toBe('#7dd3fc');
  });
});
