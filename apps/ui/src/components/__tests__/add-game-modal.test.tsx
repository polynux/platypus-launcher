import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
  pickGameExecutable: vi.fn(),
  pickFolder: vi.fn(),
  detectGameExecutable: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
}));

import { AddGameModal, suggestTitle, type AddGameOptions } from '../AddGameModal';

const ok = <T,>(v: T) => Effect.succeed(v);

function open(onAdd: (title: string, exe: string, opts: AddGameOptions) => Promise<void> | void = vi.fn()) {
  const onClose = vi.fn();
  render(<AddGameModal onClose={onClose} onAdd={onAdd} />);
  return { onClose, onAdd };
}

function fillValid() {
  fireEvent.change(screen.getByPlaceholderText('Found from the file name'), { target: { value: 'Doom' } });
  fireEvent.change(screen.getByLabelText('Program'), { target: { value: '/home/you/Games/doom/doom.exe' } });
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
    expect((screen.getByLabelText('Program') as HTMLInputElement).value).toBe('/home/you/Games/doom/doom.exe');
    expect((screen.getByPlaceholderText('Found from the file name') as HTMLInputElement).value).toBe('doom');
  });

  it('leaves the field alone when the picker is cancelled', async () => {
    api.pickGameExecutable.mockResolvedValue('');
    open();
    fireEvent.change(screen.getByLabelText('Program'), { target: { value: '/typed/path' } });
    fireEvent.click(screen.getByRole('button', { name: /Browse/ }));
    await waitFor(() => expect(api.pickGameExecutable).toHaveBeenCalled());
    expect((screen.getByLabelText('Program') as HTMLInputElement).value).toBe('/typed/path');
  });

  it('keeps an existing title and degrades when there is no picker', async () => {
    api.pickGameExecutable.mockRejectedValue(new Error('no window to show a file picker'));
    open();
    fireEvent.change(screen.getByPlaceholderText('Found from the file name'), { target: { value: 'Kept' } });
    fireEvent.click(screen.getByRole('button', { name: /Browse/ }));
    await waitFor(() => expect(api.pickGameExecutable).toHaveBeenCalled());
    expect((screen.getByPlaceholderText('Found from the file name') as HTMLInputElement).value).toBe('Kept');
    // The text field still works.
    fireEvent.change(screen.getByLabelText('Program'), { target: { value: '/typed/by/hand' } });
    expect((screen.getByLabelText('Program') as HTMLInputElement).value).toBe('/typed/by/hand');
  });

  it('still submits through validation (the backend refuses a bad path)', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fireEvent.change(screen.getByPlaceholderText('Found from the file name'), { target: { value: 'Bad' } });
    fireEvent.change(screen.getByLabelText('Program'), { target: { value: 'relative/path' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(onAdd).toHaveBeenCalledWith('Bad', 'relative/path', { workingDir: '', winePrefix: '' });
  });

  it('suggests titles from file names', () => {
    expect(suggestTitle('/g/Half_Life_2.exe')).toBe('Half Life 2');
    expect(suggestTitle('/g/run')).toBe('run');
  });
});

describe('AddGameModal advanced working directory (issue #33)', () => {
  it('saves the working directory with the game', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fireEvent.change(screen.getByPlaceholderText('Found from the file name'), { target: { value: 'Doom' } });
    fireEvent.change(screen.getByLabelText('Program'), { target: { value: '/home/you/Games/doom/doom.exe' } });
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    fireEvent.change(screen.getByLabelText('working directory'), { target: { value: '/home/you/Games/doom' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(onAdd).toHaveBeenCalledWith('Doom', '/home/you/Games/doom/doom.exe', { workingDir: '/home/you/Games/doom', winePrefix: '' });
  });

  it('fills the working directory from the folder picker and leaves it on cancel', async () => {
    api.pickFolder.mockResolvedValue('/home/you/Games/doom/data');
    open();
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    fireEvent.click(screen.getAllByRole('button', { name: /Browse/ })[1]);
    await waitFor(() => expect(api.pickFolder).toHaveBeenCalled());
    expect((screen.getByLabelText('working directory') as HTMLInputElement).value).toBe('/home/you/Games/doom/data');

    api.pickFolder.mockResolvedValue('');
    fireEvent.click(screen.getAllByRole('button', { name: /Browse/ })[1]);
    await waitFor(() => expect(api.pickFolder).toHaveBeenCalledTimes(2));
    expect((screen.getByLabelText('working directory') as HTMLInputElement).value).toBe('/home/you/Games/doom/data');
  });

  it('refuses a relative working directory before submitting', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fireEvent.change(screen.getByPlaceholderText('Found from the file name'), { target: { value: 'Doom' } });
    fireEvent.change(screen.getByLabelText('Program'), { target: { value: '/home/you/Games/doom/doom.exe' } });
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    fireEvent.change(screen.getByLabelText('working directory'), { target: { value: 'relative/dir' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/absolute path/);
    expect(onAdd).not.toHaveBeenCalled();
  });
});

describe('AddGameModal advanced Wine prefix (issue #34)', () => {
  it('saves the prefix with the game', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fireEvent.change(screen.getByPlaceholderText('Found from the file name'), { target: { value: 'Doom' } });
    fireEvent.change(screen.getByLabelText('Program'), { target: { value: '/home/you/Games/doom/doom.exe' } });
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    fireEvent.change(screen.getByLabelText('wine prefix'), { target: { value: '/home/you/prefixes/doom' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(onAdd).toHaveBeenCalledWith('Doom', '/home/you/Games/doom/doom.exe', { workingDir: '', winePrefix: '/home/you/prefixes/doom' });
  });

  it('fills the prefix from the folder picker and leaves it on cancel', async () => {
    api.pickFolder.mockResolvedValue('/home/you/prefixes/doom');
    open();
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    fireEvent.click(screen.getAllByRole('button', { name: /Browse/ })[2]);
    await waitFor(() => expect(api.pickFolder).toHaveBeenCalled());
    expect((screen.getByLabelText('wine prefix') as HTMLInputElement).value).toBe('/home/you/prefixes/doom');

    api.pickFolder.mockResolvedValue('');
    fireEvent.click(screen.getAllByRole('button', { name: /Browse/ })[2]);
    await waitFor(() => expect(api.pickFolder).toHaveBeenCalledTimes(2));
    expect((screen.getByLabelText('wine prefix') as HTMLInputElement).value).toBe('/home/you/prefixes/doom');
  });

  it('refuses a relative prefix before submitting', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fireEvent.change(screen.getByPlaceholderText('Found from the file name'), { target: { value: 'Doom' } });
    fireEvent.change(screen.getByLabelText('Program'), { target: { value: '/home/you/Games/doom/doom.exe' } });
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    fireEvent.change(screen.getByLabelText('wine prefix'), { target: { value: 'prefixes/doom' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/absolute path/);
    expect(onAdd).not.toHaveBeenCalled();
  });
});

describe('AddGameModal all-or-nothing add (review fix 3)', () => {
  it('closes the dialog once the add resolves', async () => {
    const onAdd = vi.fn(async () => {});
    const { onClose } = open(onAdd);
    fillValid();
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
  });

  it('keeps the dialog open with the error under the working directory field', async () => {
    const onAdd = vi.fn(async () => { throw new Error('working directory: no such file or directory'); });
    const { onClose } = open(onAdd);
    fillValid();
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    // The dialog stays open and the error shows where it belongs.
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(/no such file/);
    expect(screen.getByLabelText('working directory').closest('div')!.parentElement!).toContainElement(alert);
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Add to library' })).toBeInTheDocument();
    // Fixing the field clears it and retrying works.
    fireEvent.change(screen.getByLabelText('working directory'), { target: { value: '/home/you/Games/doom' } });
    expect(screen.queryByText(/no such file/)).toBeNull();
  });

  it('shows a backend refusal of the executable as a dialog-level alert', async () => {
    const onAdd = vi.fn(async () => { throw new Error('/x is not a file'); });
    const { onClose } = open(onAdd);
    fillValid();
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/not a file/);
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe('AddGameModal release year (issue #35)', () => {
  it('saves the year with the game', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fireEvent.change(screen.getByPlaceholderText('Found from the file name'), { target: { value: 'Doom' } });
    fireEvent.change(screen.getByLabelText('Program'), { target: { value: '/home/you/Games/doom/doom.exe' } });
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    fireEvent.change(screen.getByLabelText('release year'), { target: { value: '1993' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(onAdd).toHaveBeenCalledWith('Doom', '/home/you/Games/doom/doom.exe', { workingDir: '', winePrefix: '', releaseYear: 1993 });
  });

  it('leaves the year out when empty', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fillValid();
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(onAdd).toHaveBeenCalledWith('Doom', '/home/you/Games/doom/doom.exe', { workingDir: '', winePrefix: '' });
  });

  it('refuses a bad year before submitting', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fillValid();
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    fireEvent.change(screen.getByLabelText('release year'), { target: { value: '1899' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/between 1970/);
    expect(onAdd).not.toHaveBeenCalled();
  });

  it('keeps the dialog open with a backend year refusal under the year field', async () => {
    const onAdd = vi.fn(async () => { throw new Error('release year must be between 1970 and 2027, got 1800'); });
    const { onClose } = open(onAdd);
    fillValid();
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(/release year/);
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe('AddGameModal executable detection (issue #154)', () => {
  it('debounces keystrokes and keeps the latest path when an older answer is slower', async () => {
    vi.useFakeTimers();
    try {
      let resolveFirst!: (v: { native: boolean; kind: string }) => void;
      const first = new Promise<{ native: boolean; kind: string }>((r) => { resolveFirst = r; });
      api.detectGameExecutable.mockImplementation((path: string) =>
        path === '/g/first.exe' ? first : Promise.resolve({ native: true, kind: 'elf' }),
      );
      open();
      fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
      const program = screen.getByLabelText('Program');
      fireEvent.change(program, { target: { value: '/g/first.exe' } });
      // Still debouncing: no RPC yet for the intermediate keystroke.
      expect(api.detectGameExecutable).not.toHaveBeenCalled();
      await vi.advanceTimersByTimeAsync(250);
      expect(api.detectGameExecutable).toHaveBeenCalledTimes(1);

      fireEvent.change(program, { target: { value: '/g/second' } });
      await vi.advanceTimersByTimeAsync(250);
      expect(api.detectGameExecutable).toHaveBeenCalledTimes(2);

      // The stale first answer resolves last and must not win.
      resolveFirst({ native: false, kind: 'windows-exe' });
      await vi.runAllTimersAsync();
      // Flush the pending Effect promises.
      await Promise.resolve();
      await Promise.resolve();
      vi.useRealTimers();
      // Latest path is native: the run choice names the native auto default,
      // not the stale Wine answer.
      await waitFor(() =>
        expect(screen.getByRole('combobox', { name: 'Run with' })).toHaveTextContent('detected: native'),
      );
    } finally {
      vi.useRealTimers();
    }
  });
});
