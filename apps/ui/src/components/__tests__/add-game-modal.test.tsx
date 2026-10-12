import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
  pickGameExecutable: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
}));

import { AddGameModal, suggestTitle } from '../AddGameModal';

const ok = <T,>(v: T) => Effect.succeed(v);

function open(onAdd: (...a: string[]) => void = vi.fn()) {
  const onClose = vi.fn();
  render(<AddGameModal onClose={onClose} onAdd={onAdd} />);
  return { onClose, onAdd };
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
});

describe('AddGameModal Browse (issue #32)', () => {
  it('fills the executable from the picker and suggests a title when empty', async () => {
    api.pickGameExecutable.mockResolvedValue('/home/you/Games/doom/doom.exe');
    open();
    fireEvent.click(screen.getByRole('button', { name: /Browse/ }));
    await waitFor(() => expect(api.pickGameExecutable).toHaveBeenCalled());
    expect((screen.getByLabelText('executable') as HTMLInputElement).value).toBe('/home/you/Games/doom/doom.exe');
    expect((screen.getByPlaceholderText('My Game') as HTMLInputElement).value).toBe('doom');
  });

  it('leaves the field alone when the picker is cancelled', async () => {
    api.pickGameExecutable.mockResolvedValue('');
    open();
    fireEvent.change(screen.getByLabelText('executable'), { target: { value: '/typed/path' } });
    fireEvent.click(screen.getByRole('button', { name: /Browse/ }));
    await waitFor(() => expect(api.pickGameExecutable).toHaveBeenCalled());
    expect((screen.getByLabelText('executable') as HTMLInputElement).value).toBe('/typed/path');
  });

  it('keeps an existing title and degrades when there is no picker', async () => {
    api.pickGameExecutable.mockRejectedValue(new Error('no window to show a file picker'));
    open();
    fireEvent.change(screen.getByPlaceholderText('My Game'), { target: { value: 'Kept' } });
    fireEvent.click(screen.getByRole('button', { name: /Browse/ }));
    await waitFor(() => expect(api.pickGameExecutable).toHaveBeenCalled());
    expect((screen.getByPlaceholderText('My Game') as HTMLInputElement).value).toBe('Kept');
    // The text field still works.
    fireEvent.change(screen.getByLabelText('executable'), { target: { value: '/typed/by/hand' } });
    expect((screen.getByLabelText('executable') as HTMLInputElement).value).toBe('/typed/by/hand');
  });

  it('still submits through validation (the backend refuses a bad path)', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fireEvent.change(screen.getByPlaceholderText('My Game'), { target: { value: 'Bad' } });
    fireEvent.change(screen.getByLabelText('executable'), { target: { value: 'relative/path' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add game' }));
    expect(onAdd).toHaveBeenCalledWith('Bad', 'relative/path');
  });

  it('suggests titles from file names', () => {
    expect(suggestTitle('/g/Half_Life_2.exe')).toBe('Half Life 2');
    expect(suggestTitle('/g/run')).toBe('run');
  });
});
