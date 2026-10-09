import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
  getCustomization: vi.fn(),
  setCustomTitle: vi.fn(),
  setCustomArt: vi.fn(),
  resetCustomArt: vi.fn(),
  resetCustomization: vi.fn(),
  pickImage: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
}));

import { CustomizationTab } from '../CustomizationTab';
import type { Customization, Game } from '../../types';

const game = { id: 'echo:1', title: 'Pixel Quest', store_title: 'Pixel Quest', cover_url: 'cover.png', banner_url: '', hero_url: '' } as unknown as Game;
const empty: Customization = { title: '', sort_title: '', art: {} };
const ok = <T,>(v: T) => Effect.succeed(v);

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getCustomization.mockReturnValue(ok(empty));
});

describe('CustomizationTab', () => {
  it('lists every kind of art and starts clean', async () => {
    render(<CustomizationTab game={game} onChanged={() => {}} />);
    await waitFor(() => expect(screen.getByLabelText('Custom title')).not.toBeDisabled());
    for (const k of ['cover', 'banner', 'logo', 'icon', 'background']) {
      expect(screen.getByRole('button', { name: `Choose ${k} image` })).toBeInTheDocument();
    }
    expect(screen.queryByText(/reset everything/i)).toBeNull();
    expect(screen.getByRole('button', { name: 'Save title' })).toBeDisabled();
  });

  it('saves the title and the sort title and tells the owner', async () => {
    const onChanged = vi.fn();
    api.setCustomTitle.mockReturnValue(ok({ ...empty, title: 'PQ', sort_title: 'Pixel' }));
    render(<CustomizationTab game={game} onChanged={onChanged} />);
    await waitFor(() => expect(screen.getByLabelText('Custom title')).not.toBeDisabled());
    fireEvent.change(screen.getByLabelText('Custom title'), { target: { value: 'PQ' } });
    fireEvent.change(screen.getByLabelText('Sort title'), { target: { value: 'Pixel' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save title' }));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
    expect(api.setCustomTitle).toHaveBeenCalledWith('echo:1', 'PQ', 'Pixel');
    expect(screen.getByRole('button', { name: 'Save title' })).toBeDisabled();
    expect(screen.getByText(/reset everything/i)).toBeInTheDocument();
  });

  it('copies a chosen image, marks it as yours and can reset it', async () => {
    const onChanged = vi.fn();
    api.pickImage.mockReturnValue(ok('/home/me/cover.png'));
    api.setCustomArt.mockReturnValue(ok({ ...empty, art: { cover: '/hydra-art/ab/cover.png?v=1' } }));
    api.resetCustomArt.mockReturnValue(ok(empty));
    render(<CustomizationTab game={game} onChanged={onChanged} />);
    await waitFor(() => expect(screen.getByLabelText('Custom title')).not.toBeDisabled());
    fireEvent.click(screen.getByRole('button', { name: 'Choose cover image' }));
    await screen.findByText('yours');
    expect(api.setCustomArt).toHaveBeenCalledWith('echo:1', 'cover', '/home/me/cover.png');
    fireEvent.click(screen.getByRole('button', { name: /reset cover/i }));
    await waitFor(() => expect(screen.queryByText('yours')).toBeNull());
    expect(api.resetCustomArt).toHaveBeenCalledWith('echo:1', 'cover');
    expect(onChanged).toHaveBeenCalledTimes(2);
  });

  it('does nothing when the picker is cancelled', async () => {
    const onChanged = vi.fn();
    api.pickImage.mockReturnValue(ok(''));
    render(<CustomizationTab game={game} onChanged={onChanged} />);
    await waitFor(() => expect(screen.getByLabelText('Custom title')).not.toBeDisabled());
    fireEvent.click(screen.getByRole('button', { name: 'Choose banner image' }));
    await waitFor(() => expect(api.pickImage).toHaveBeenCalled());
    expect(api.setCustomArt).not.toHaveBeenCalled();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it('shows the refusal of a bad image and keeps the form as it was', async () => {
    api.pickImage.mockReturnValue(ok('/x/notes.txt'));
    api.setCustomArt.mockReturnValue(Effect.fail(new Error('not an image: only PNG, JPEG, GIF and WebP are accepted')));
    render(<CustomizationTab game={game} onChanged={() => {}} />);
    await waitFor(() => expect(screen.getByLabelText('Custom title')).not.toBeDisabled());
    fireEvent.click(screen.getByRole('button', { name: 'Choose logo image' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/not an image/);
    expect(screen.queryByText('yours')).toBeNull();
  });

  it('resets everything at once', async () => {
    api.getCustomization.mockReturnValue(ok({ title: 'PQ', sort_title: '', art: { icon: '/hydra-art/ab/icon.png' } }));
    api.resetCustomization.mockReturnValue(ok(empty));
    render(<CustomizationTab game={game} onChanged={() => {}} />);
    fireEvent.click(await screen.findByText(/reset everything/i));
    await waitFor(() => expect(screen.queryByText(/reset everything/i)).toBeNull());
    expect(api.resetCustomization).toHaveBeenCalledWith('echo:1');
    expect((screen.getByLabelText('Custom title') as HTMLInputElement).value).toBe('');
  });
});
