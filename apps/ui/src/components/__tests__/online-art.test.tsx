import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { ArtCandidate, Game } from '../../types';

const api = vi.hoisted(() => ({
  getCustomization: vi.fn(),
  searchArtOnline: vi.fn(),
  applyOnlineArt: vi.fn(),
  getLauncherSettings: vi.fn(),
  setArtworkOnline: vi.fn(),
}));
const libraryCbs = vi.hoisted(() => ({ list: [] as Array<(id: string) => void> }));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onLibrary: (cb: (id: string) => void) => {
    libraryCbs.list.push(cb);
    return () => {};
  },
}));

import { CustomizationTab } from '../CustomizationTab';
import { ArtworkSetting } from '../LauncherSettingsView';

const game = (o: Partial<Game> = {}): Game => ({
  id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom', title: 'Hollow Knight',
  cover_url: '', banner_url: '', install_dir: '/g/hk', executable: '/g/hk/hk.exe',
  is_installed: true, features: null,
  settings: { runner_id: 'auto', env_vars: {} }, ...o,
}) as Game;

const emptyCustom = { title: '', sort_title: '', art: {}, art_source: '', art_source_name: '' };
const cands: ArtCandidate[] = [
  { app_id: 123, name: 'Hollow Knight', thumb_url: 'https://cdn/123.jpg' },
  { app_id: 124, name: 'Hollow Knight: Silksong', thumb_url: 'https://cdn/124.jpg' },
];

function tab(g: Game = game()) {
  const onChanged = vi.fn();
  render(<CustomizationTab game={g} onChanged={onChanged} />);
  return { onChanged };
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  libraryCbs.list.length = 0;
  api.getCustomization.mockResolvedValue(emptyCustom);
});

describe('OnlineArtFinder (issue #36)', () => {
  it('offers finding art online for sideloaded games, prefilled with the title', async () => {
    tab();
    await waitFor(() => expect(api.getCustomization).toHaveBeenCalled());
    expect((screen.getByLabelText('Search for art online') as HTMLInputElement).value).toBe('Hollow Knight');
  });

  it('hides the finder for store games', async () => {
    tab(game({ source: 'steam', store: 'steam', store_app_id: '123', executable: '' }));
    await waitFor(() => expect(api.getCustomization).toHaveBeenCalled());
    expect(screen.queryByLabelText('Search for art online')).toBeNull();
  });

  it('searches, lists candidates with thumbnails, and applies a pick', async () => {
    api.searchArtOnline.mockResolvedValue(cands);
    api.applyOnlineArt.mockResolvedValue({ ...emptyCustom, art: { cover: '/platypus-art/x/cover.png' }, art_source: 'steam:123', art_source_name: 'Hollow Knight' });
    const { onChanged } = tab();
    await waitFor(() => expect(api.getCustomization).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: 'Search' }));
    await waitFor(() => expect(api.searchArtOnline).toHaveBeenCalledWith('Hollow Knight'));
    expect(screen.getByText('Hollow Knight: Silksong')).toBeInTheDocument();
    const thumbs = document.querySelectorAll('img[src="https://cdn/124.jpg"]');
    expect(thumbs.length).toBe(1);
    fireEvent.click(screen.getAllByRole('button', { name: 'Apply' })[0]);
    await waitFor(() => expect(api.applyOnlineArt).toHaveBeenCalledWith('custom-1', 123));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('says so when nothing is found, and shows search errors', async () => {
    api.searchArtOnline.mockResolvedValue([]);
    tab();
    await waitFor(() => expect(api.getCustomization).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: 'Search' }));
    expect(await screen.findByText(/No artwork found/)).toBeInTheDocument();

    api.searchArtOnline.mockRejectedValue(new Error('finding artwork online is switched off'));
    fireEvent.click(screen.getByRole('button', { name: 'Search' }));
    expect(await screen.findByText(/switched off/)).toBeInTheDocument();
  });

  it('shows an apply error', async () => {
    api.searchArtOnline.mockResolvedValue(cands);
    api.applyOnlineArt.mockRejectedValue(new Error('Steam has no cover or banner art for Hollow Knight'));
    tab();
    await waitFor(() => expect(api.getCustomization).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: 'Search' }));
    await screen.findByText('Hollow Knight: Silksong');
    fireEvent.click(screen.getAllByRole('button', { name: 'Apply' })[0]);
    expect(await screen.findByText(/no cover or banner/)).toBeInTheDocument();
  });

  it('shows the attribution line when art came from the lookup', async () => {
    api.getCustomization.mockResolvedValue({ ...emptyCustom, art: { cover: '/platypus-art/x/cover.png' }, art_source: 'steam:123', art_source_name: 'Hollow Knight' });
    tab();
    const line = await screen.findByTestId('art-attribution');
    expect(line.textContent).toMatch(/Art from Steam/);
    expect(line.textContent).toMatch(/Hollow Knight/);
  });

  it('shows no attribution line without a source', async () => {
    tab();
    await waitFor(() => expect(api.getCustomization).toHaveBeenCalled());
    expect(screen.queryByTestId('art-attribution')).toBeNull();
  });

  it('reloads the panel when the library event arrives for its game (AppID auto-apply)', async () => {
    tab();
    await waitFor(() => expect(api.getCustomization).toHaveBeenCalled());
    expect(screen.queryByTestId('art-attribution')).toBeNull();
    expect(libraryCbs.list.length).toBeGreaterThan(0);
    api.getCustomization.mockResolvedValue({
      ...emptyCustom,
      art: { cover: '/platypus-art/x/cover.png' },
      art_source: 'steam:4094660',
      art_source_name: 'Rivage',
    });
    libraryCbs.list.forEach((cb) => cb('custom-1'));
    expect(await screen.findByTestId('art-attribution')).toBeTruthy();
    // An event for another game leaves this panel alone.
    api.getCustomization.mockClear();
    libraryCbs.list.forEach((cb) => cb('other-game'));
    await new Promise((r) => setTimeout(r, 50));
    expect(api.getCustomization).not.toHaveBeenCalled();
  });
});

describe('ArtworkSetting (issue #36)', () => {
  it('toggles the artwork switch', async () => {
    api.getLauncherSettings.mockResolvedValue({ artwork_online: true });
    api.setArtworkOnline.mockResolvedValue({ artwork_online: false });
    const onNotify = vi.fn();
    render(<ArtworkSetting onNotify={onNotify} />);
    const toggle = await screen.findByLabelText('Find artwork online');
    expect(toggle).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(toggle);
    await waitFor(() => expect(api.setArtworkOnline).toHaveBeenCalledWith(false));
  });
});
