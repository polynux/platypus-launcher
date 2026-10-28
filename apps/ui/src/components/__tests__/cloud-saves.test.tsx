import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { Effect } from 'effect';

const h = vi.hoisted(() => ({
  handlers: new Set<(e: unknown) => void>(),
  api: {} as Record<string, ReturnType<typeof vi.fn>>,
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, name: string) => h.api[name] }),
  onCloud: (cb: (e: unknown) => void) => {
    h.handlers.add(cb);
    return () => h.handlers.delete(cb);
  },
  onActivity: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { CloudSavesPanel } from '../CloudSavesPanel';
import { CloudConflictDialog } from '../CloudSavesConflict';
import { CloudSavesHost } from '../CloudSavesHost';
import { CloudSavesDefaults } from '../CloudSavesSetting';
import type { CloudEvent, CloudInfo, CloudStatus, Game, StorePlugin } from '../../types';

const emit = (e: Partial<CloudEvent>) => act(() => { h.handlers.forEach((cb) => cb({ game_id: 'p.echo.tiny', kind: 'status', ...e })); });

const ok = <T,>(v: T) => vi.fn(() => Effect.succeed(v));
const fail = (msg: string) => vi.fn(() => Effect.fail(new Error(msg)));

const counts = (o: Partial<CloudStatus['counts']> = {}) => ({ same: 3, download: 0, upload: 0, conflict: 0, unmapped: 0, ...o });
const NOW = Math.floor(Date.now() / 1000);

const status = (o: Partial<CloudStatus> = {}): CloudStatus => ({ state: 'synced', counts: counts(), checkedAt: NOW - 120, ...o });
const info = (o: Partial<CloudInfo> = {}): CloudInfo => ({ mode: 'ask', plugin_mode: 'ask', game_mode: '', status: status(), running: false, ...o });

const conflictStatus = (): CloudStatus => status({
  state: 'conflict', message: '2 files changed on both sides', counts: counts({ same: 1, conflict: 2 }),
  backupDir: '/home/u/backups/tiny',
  conflicts: [
    { path: 'Profile/save.dat', local: { size: 3705, mtime: NOW - 3600 }, remote: { size: 3718, mtime: NOW - 86400 * 30, machine: 'Deck' } },
    { path: 'Settings/options.cfg', local: { size: 120, mtime: NOW - 99999 }, remote: { size: 0, mtime: NOW - 10 } },
  ],
});

function game(o: Partial<Game> = {}): Game {
  return {
    id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '', hero_url: '',
    install_dir: '/g/tiny', executable: '', is_installed: true, settings: {} as Game['settings'], features: null, ...o,
  };
}

function plugin(caps: string[] = ['library', 'cloud-saves'], declared = caps): StorePlugin {
  return {
    id: 'echo', name: 'Echo', version: '1', author: '', description: '', store: 'echo', kind: 'store', status: 'ready',
    missing: [], requires: [], capabilities: caps, declared, offers: [], needs: [], warnings: [], initialized: true,
    dir: '/x', exec: 'run', hash: 'h', network: [], read: [], write: [], running: true,
    install_folder: '/g', default_install_folder: '/g', extra_folders: [],
  };
}

beforeEach(() => {
  h.handlers.clear();
  for (const k of Object.keys(h.api)) delete h.api[k];
  Object.assign(h.api, {
    getCloudInfo: ok(info()),
    checkCloudSaves: ok(info()),
    resolveCloudSaves: ok(info()),
    cancelCloudSync: ok(true),
    launchGameAnyway: ok('Started'),
    launchGameViaClient: ok('Asked Steam'),
    launchGame: ok('Started'),
    getCloudSettings: ok({ plugins: {}, games: {} }),
    setCloudPluginMode: ok(undefined),
    setCloudGameMode: ok(undefined),
  });
});

// A fresh game id per test keeps the once-per-session auto check from leaking between tests.
let n = 0;
const fresh = (o: Partial<Game> = {}) => game({ id: `p.echo.g${++n}`, ...o });

describe('Cloud saves row: when it appears', () => {
  it('renders nothing for a plugin without the capability', () => {
    const { container } = render(<CloudSavesPanel game={fresh()} plugin={plugin(['library'])} />);
    expect(container).toBeEmptyDOMElement();
    expect(h.api.getCloudInfo).not.toHaveBeenCalled();
  });

  it('renders nothing when the capability is declared but switched off', () => {
    const { container } = render(<CloudSavesPanel game={fresh()} plugin={plugin(['library'], ['library', 'cloud-saves'])} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing for a game whose features leave it out', () => {
    const { container } = render(<CloudSavesPanel game={fresh({ features: ['install'] })} plugin={plugin()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing without a plugin (Steam, sideloaded)', () => {
    const { container } = render(<CloudSavesPanel game={fresh()} plugin={undefined} />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe('Cloud saves row: state', () => {
  it('shows the chip, the counts and when it was checked', async () => {
    h.api.getCloudInfo = ok(info({ status: status({ state: 'pending', counts: counts({ download: 1, upload: 2 }) }) }));
    render(<CloudSavesPanel game={fresh()} plugin={plugin()} />);
    expect(await screen.findByTestId('cloud-state')).toHaveTextContent('changes waiting');
    expect(screen.getByTestId('cloud-counts')).toHaveTextContent('3 in sync · 1 to download · 2 to upload');
    expect(screen.getByText(/checked 2 min ago/)).toBeInTheDocument();
  });

  it('gives the plugin\'s reason when a game can\'t be synced', async () => {
    h.api.getCloudInfo = ok(info({ status: status({ state: 'unsupported', message: 'no Auto-Cloud rules', counts: counts({ same: 0 }) }) }));
    render(<CloudSavesPanel game={fresh()} plugin={plugin()} />);
    expect(await screen.findByTestId('cloud-state')).toHaveTextContent('not supported');
    expect(screen.getByTestId('cloud-message')).toHaveTextContent('no Auto-Cloud rules');
    expect(screen.queryByRole('button', { name: /choose which saves/i })).toBeNull();
  });

  it('shows an unreachable store as offline, with the plugin\'s words', async () => {
    h.api.getCloudInfo = ok(info({ status: status({ state: 'offline', message: 'The store is not reachable', counts: counts({ same: 0 }) }) }));
    render(<CloudSavesPanel game={fresh()} plugin={plugin()} />);
    expect(await screen.findByTestId('cloud-state')).toHaveTextContent('offline');
    expect(screen.getByTestId('cloud-message')).toHaveTextContent('The store is not reachable');
  });

  it('looks once on its own when nothing is known yet, then leaves it to Check now', async () => {
    h.api.getCloudInfo = ok(info({ status: null }));
    const g = fresh();
    const first = render(<CloudSavesPanel game={g} plugin={plugin()} />);
    await waitFor(() => expect(h.api.checkCloudSaves).toHaveBeenCalledTimes(1));
    first.unmount();
    render(<CloudSavesPanel game={g} plugin={plugin()} />);
    await screen.findByTestId('cloud-state');
    expect(h.api.checkCloudSaves).toHaveBeenCalledTimes(1);
  });

  it('does not look on its own when the mode is off', async () => {
    h.api.getCloudInfo = ok(info({ mode: 'off', game_mode: 'off', status: null }));
    render(<CloudSavesPanel game={fresh()} plugin={plugin()} />);
    expect(await screen.findByTestId('cloud-state')).toHaveTextContent('off');
    expect(h.api.checkCloudSaves).not.toHaveBeenCalled();
  });

  it('Check now asks the backend and shows the answer', async () => {
    const g = fresh();
    h.api.checkCloudSaves = ok(info({ status: status({ state: 'pending', counts: counts({ upload: 1 }) }) }));
    render(<CloudSavesPanel game={g} plugin={plugin()} />);
    await screen.findByTestId('cloud-state');
    fireEvent.click(screen.getByRole('button', { name: /check now/i }));
    await waitFor(() => expect(screen.getByTestId('cloud-state')).toHaveTextContent('changes waiting'));
    expect(h.api.checkCloudSaves).toHaveBeenCalledWith(expect.objectContaining({ id: g.id }));
  });

  it('shows a failed check as an alert', async () => {
    h.api.checkCloudSaves = fail('plugin echo is stopped');
    render(<CloudSavesPanel game={fresh()} plugin={plugin()} />);
    await screen.findByTestId('cloud-state');
    fireEvent.click(screen.getByRole('button', { name: /check now/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent('plugin echo is stopped');
  });
});

describe('Cloud saves row: working', () => {
  it('shows progress from the plugin and cancels the sync', async () => {
    const g = fresh();
    render(<CloudSavesPanel game={g} plugin={plugin()} />);
    await screen.findByTestId('cloud-state');
    emit({ game_id: g.id, kind: 'progress', phase: 'download', done: 1, total: 4 });
    expect(await screen.findByRole('status')).toHaveTextContent('download · 1/4');
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '1');
    expect(screen.getByRole('button', { name: /check now/i })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: /cancel cloud sync/i }));
    expect(h.api.cancelCloudSync).toHaveBeenCalledWith(g.id);
    emit({ game_id: g.id, kind: 'canceled' });
    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
  });

  it('ignores events of other games', async () => {
    render(<CloudSavesPanel game={fresh()} plugin={plugin()} />);
    await screen.findByTestId('cloud-state');
    emit({ game_id: 'p.echo.other', kind: 'progress', phase: 'upload', done: 1, total: 2 });
    expect(screen.queryByRole('status')).toBeNull();
  });
});

describe('Cloud saves row: running game', () => {
  it('does not offer to settle conflicts while the game runs', async () => {
    const g = fresh();
    h.api.getCloudInfo = ok(info({ status: conflictStatus(), running: true }));
    render(<CloudSavesPanel game={g} plugin={plugin()} />);
    expect(await screen.findByRole('button', { name: /choose which saves to keep/i })).toBeDisabled();
    expect(screen.getByText(/choose its saves after it exits/i)).toBeInTheDocument();
  });
});

describe('Cloud saves row: official client', () => {
  it('explains, and offers launching through the client when the plugin can', async () => {
    const g = fresh();
    h.api.getCloudInfo = ok(info({ status: status({ state: 'pending', counts: counts({ download: 1 }), client: { running: true, name: 'Steam', canLaunch: true } }) }));
    render(<CloudSavesPanel game={g} plugin={plugin()} />);
    const note = await screen.findByRole('note');
    expect(note).toHaveTextContent('Steam is running');
    fireEvent.click(within(note).getByRole('button', { name: /launch through steam/i }));
    expect(h.api.launchGameViaClient).toHaveBeenCalledWith(expect.objectContaining({ id: g.id }));
  });

  it('only explains when the plugin has no client launch', async () => {
    h.api.getCloudInfo = ok(info({ status: status({ client: { running: true, name: 'Steam' } }) }));
    render(<CloudSavesPanel game={fresh()} plugin={plugin()} />);
    const note = await screen.findByRole('note');
    expect(note).toHaveTextContent('Close Steam to sync');
    expect(within(note).queryByRole('button')).toBeNull();
  });

  it('shows no banner when the client is not running', async () => {
    h.api.getCloudInfo = ok(info({ status: status({ client: { running: false, name: 'Steam', canLaunch: true } }) }));
    render(<CloudSavesPanel game={fresh()} plugin={plugin()} />);
    await screen.findByTestId('cloud-state');
    expect(screen.queryByRole('note')).toBeNull();
  });
});

describe('Cloud saves row: mode', () => {
  it('saves the game\'s own mode and goes back to the plugin default', async () => {
    const g = fresh();
    render(<CloudSavesPanel game={g} plugin={plugin()} />);
    await screen.findByTestId('cloud-state');
    const group = screen.getByRole('radiogroup', { name: /sync mode for this game/i });
    fireEvent.click(within(group).getByRole('radio', { name: 'Off' }));
    await waitFor(() => expect(h.api.setCloudGameMode).toHaveBeenCalledWith(g.id, 'off'));
    h.api.getCloudInfo = ok(info({ game_mode: 'off', mode: 'off' }));
    emit({ game_id: g.id, kind: 'status' });
    await waitFor(() => expect(within(group).getByRole('radio', { name: 'Off' })).toBeChecked());
    fireEvent.click(within(group).getByRole('radio', { name: /default · ask/i }));
    await waitFor(() => expect(h.api.setCloudGameMode).toHaveBeenLastCalledWith(g.id, ''));
  });
});

describe('Cloud conflict dialog', () => {
  const open = (o: Partial<React.ComponentProps<typeof CloudConflictDialog>> = {}) => {
    const props = { game: game(), status: conflictStatus(), onClose: vi.fn(), onResolved: vi.fn(), ...o };
    render(<CloudConflictDialog {...props} />);
    return props;
  };

  it('lists each file with both sides\' sizes and dates, and which is newer', () => {
    open();
    const rows = within(screen.getByRole('list', { name: /conflicting files/i })).getAllByRole('listitem');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent('Profile/save.dat');
    expect(rows[0].textContent).toMatch(/3\D?705 B/); // this device
    expect(rows[0].textContent).toMatch(/3\D?718 B/); // the cloud
    expect(rows[0]).toHaveTextContent('from Deck');
    // The device copy is newer in the first file, the cloud copy in the second.
    expect(within(rows[0]).getAllByText('newer')).toHaveLength(1);
    expect(rows[1]).toHaveTextContent('0 B');
    expect(screen.getByText(/backups: \/home\/u\/backups\/tiny/)).toBeInTheDocument();
  });

  it('offers Keep this device, Keep cloud and Decide later per file; nothing is chosen for the user', () => {
    open();
    const group = screen.getByRole('radiogroup', { name: 'Choice for Profile/save.dat' });
    expect(within(group).getAllByRole('radio').map((r) => r.textContent)).toEqual(['Keep this device', 'Keep cloud', 'Decide later']);
    expect(within(group).getByRole('radio', { name: 'Decide later' })).toBeChecked();
    expect(screen.getByRole('button', { name: /apply choices/i })).toBeDisabled();
  });

  it('sends only the files the user decided, then reports the result', async () => {
    const g = game();
    h.api.resolveCloudSaves = ok(info({ status: status({ state: 'pending', counts: counts({ upload: 1 }) }) }));
    const props = open({ game: g });
    fireEvent.click(within(screen.getByRole('radiogroup', { name: 'Choice for Profile/save.dat' })).getByRole('radio', { name: 'Keep cloud' }));
    fireEvent.click(screen.getByRole('button', { name: /apply 1 choice$/i }));
    await waitFor(() => expect(props.onResolved).toHaveBeenCalled());
    expect(h.api.resolveCloudSaves).toHaveBeenCalledWith(g, { 'Profile/save.dat': 'remote' });
  });

  it('applies mixed choices for several files, and "all files" sets every one', async () => {
    open();
    // The link-style "all files" button, not one of the per-file radios.
    fireEvent.click(screen.getByRole('button', { name: /^keep this device$/i }));
    fireEvent.click(within(screen.getByRole('radiogroup', { name: 'Choice for Settings/options.cfg' })).getByRole('radio', { name: 'Keep cloud' }));
    fireEvent.click(screen.getByRole('button', { name: /apply 2 choices/i }));
    await waitFor(() => expect(h.api.resolveCloudSaves).toHaveBeenCalled());
    expect(h.api.resolveCloudSaves.mock.calls[0][1]).toEqual({ 'Profile/save.dat': 'local', 'Settings/options.cfg': 'remote' });
  });

  it('keeps the dialog and shows the error when resolving fails', async () => {
    h.api.resolveCloudSaves = fail('the store refused');
    const props = open();
    fireEvent.click(within(screen.getByRole('radiogroup', { name: 'Choice for Profile/save.dat' })).getByRole('radio', { name: 'Keep this device' }));
    fireEvent.click(screen.getByRole('button', { name: /apply 1 choice$/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent('the store refused');
    expect(props.onResolved).not.toHaveBeenCalled();
  });

  it('offers "Launch without resolving" only while a launch waits', () => {
    const { rerender } = render(<CloudConflictDialog game={game()} status={conflictStatus()} onClose={() => {}} onResolved={() => {}} />);
    expect(screen.queryByRole('button', { name: /launch without resolving/i })).toBeNull();
    const onLaunchAnyway = vi.fn();
    rerender(<CloudConflictDialog game={game()} status={conflictStatus()} onClose={() => {}} onResolved={() => {}} onLaunchAnyway={onLaunchAnyway} />);
    fireEvent.click(screen.getByRole('button', { name: /launch without resolving/i }));
    expect(onLaunchAnyway).toHaveBeenCalled();
  });
});

describe('Row and dialog together', () => {
  it('opens the dialog from the row and closes it once nothing conflicts', async () => {
    const g = fresh();
    h.api.getCloudInfo = ok(info({ status: conflictStatus() }));
    h.api.resolveCloudSaves = ok(info({ status: status() }));
    render(<CloudSavesPanel game={g} plugin={plugin()} />);
    fireEvent.click(await screen.findByRole('button', { name: /choose which saves to keep/i }));
    const dialog = await screen.findByRole('dialog', { name: /cloud save conflict/i });
    fireEvent.click(within(dialog).getByRole('button', { name: /^keep this device$/i }));
    fireEvent.click(within(dialog).getByRole('button', { name: /apply 2 choices/i }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(screen.getByTestId('cloud-state')).toHaveTextContent('in sync');
  });
});

describe('CloudSavesHost', () => {
  const setup = (games: Game[] = [game()]) => {
    const onNotify = vi.fn();
    const onLog = vi.fn();
    render(<CloudSavesHost games={games} onNotify={onNotify} onLog={onLog} />);
    return { onNotify, onLog };
  };

  it('toasts and logs a finished push with the game\'s title', () => {
    const { onNotify, onLog } = setup();
    emit({ kind: 'pushed', message: 'Saved 2 files to the cloud' });
    expect(onNotify).toHaveBeenCalledWith('Saved 2 files to the cloud · Echo Tiny', 'ok');
    expect(onLog).toHaveBeenCalledWith(expect.objectContaining({ type: 'cloud', game: 'p.echo.tiny' }));
  });

  it('shows a conflict prompt as the conflict dialog; resolving launches the game', async () => {
    const g = game();
    h.api.resolveCloudSaves = ok(info({ status: status() }));
    const { onNotify } = setup([g]);
    emit({ kind: 'prompt', prompt: 'conflict', message: 'x', status: conflictStatus() });
    const dialog = await screen.findByRole('dialog', { name: /cloud save conflict/i });
    fireEvent.click(within(dialog).getByRole('button', { name: /^keep cloud$/i }));
    fireEvent.click(within(dialog).getByRole('button', { name: /apply 2 choices/i }));
    await waitFor(() => expect(h.api.launchGame).toHaveBeenCalledWith(g));
    expect(onNotify).toHaveBeenCalledWith('Saves settled', 'ok');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('"Launch without resolving" launches anyway for a conflict', async () => {
    const g = game();
    setup([g]);
    emit({ kind: 'prompt', prompt: 'conflict', status: conflictStatus() });
    fireEvent.click(await screen.findByRole('button', { name: /launch without resolving/i }));
    expect(h.api.launchGameAnyway).toHaveBeenCalledWith(g, 'conflict');
  });

  it('asks before launching with an unreachable store', async () => {
    const g = game();
    setup([g]);
    emit({ kind: 'prompt', prompt: 'offline', message: 'The store is not reachable' });
    const dialog = await screen.findByRole('dialog', { name: /cloud saves not synced/i });
    expect(dialog).toHaveTextContent('The store is not reachable');
    fireEvent.click(within(dialog).getByRole('button', { name: /launch anyway/i }));
    expect(h.api.launchGameAnyway).toHaveBeenCalledWith(g, 'offline');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('cancelling an offline prompt launches nothing', async () => {
    setup();
    emit({ kind: 'prompt', prompt: 'offline', message: 'offline' });
    fireEvent.click(await screen.findByRole('button', { name: 'Cancel' }));
    expect(h.api.launchGameAnyway).not.toHaveBeenCalled();
    expect(h.api.launchGame).not.toHaveBeenCalled();
  });

  it('still asks about a game the library no longer lists, under its title', async () => {
    setup([]);
    emit({ kind: 'prompt', prompt: 'offline', message: 'The store is not reachable', title: 'Echo Tiny' });
    const dialog = await screen.findByRole('dialog', { name: /cloud saves need a decision/i });
    expect(dialog).toHaveTextContent('Echo Tiny');
    expect(dialog).toHaveTextContent('The store is not reachable');
    expect(h.api.launchGameAnyway).not.toHaveBeenCalled();
  });

  it('offers launching through the store client, or without cloud sync', async () => {
    const g = game();
    setup([g]);
    emit({ kind: 'prompt', prompt: 'client-running', message: 'Steam is running.', client: 'Steam' });
    const dialog = await screen.findByRole('dialog', { name: /store client running/i });
    expect(dialog).toHaveTextContent('Steam is running');
    fireEvent.click(within(dialog).getByRole('button', { name: /launch without cloud sync/i }));
    expect(h.api.launchGameAnyway).toHaveBeenCalledWith(g, 'client-running');
  });

  it('launches through the client when asked', async () => {
    const g = game();
    setup([g]);
    emit({ kind: 'prompt', prompt: 'client-running', message: 'Steam is running.', client: 'Steam' });
    fireEvent.click(await screen.findByRole('button', { name: /launch through steam/i }));
    expect(h.api.launchGameViaClient).toHaveBeenCalledWith(g);
  });
});

describe('Cloud saves defaults in Settings', () => {
  it('shows the saved default of each store and saves a change', async () => {
    h.api.getCloudSettings = ok({ plugins: { echo: 'auto' }, games: {} });
    render(<CloudSavesDefaults plugins={[plugin()]} />);
    const group = screen.getByRole('radiogroup', { name: /cloud saves for echo/i });
    await waitFor(() => expect(within(group).getByRole('radio', { name: 'Auto' })).toBeChecked());
    fireEvent.click(within(group).getByRole('radio', { name: 'Off' }));
    await waitFor(() => expect(h.api.setCloudPluginMode).toHaveBeenCalledWith('echo', 'off'));
  });

  it('has one row per ready store that offers cloud saves, and nothing else', () => {
    const other = { ...plugin(), id: 'echo-b', name: 'Echo B' };
    const { container } = render(<CloudSavesDefaults plugins={[plugin(), other, plugin(['library'])]} />);
    expect(screen.getAllByRole('radiogroup')).toHaveLength(2);
    expect(screen.getByRole('radiogroup', { name: /cloud saves for echo b/i })).toBeTruthy();
    expect(container.textContent).toContain('Echo B');
  });

  it('is absent when no store offers cloud saves', () => {
    const { container } = render(<CloudSavesDefaults plugins={[plugin(['library'])]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('goes back to the saved value when saving fails', async () => {
    h.api.setCloudPluginMode = fail('disk full');
    render(<CloudSavesDefaults plugins={[plugin()]} />);
    const group = screen.getByRole('radiogroup', { name: /cloud saves for echo/i });
    await waitFor(() => expect(within(group).getByRole('radio', { name: 'Ask' })).toBeChecked());
    fireEvent.click(within(group).getByRole('radio', { name: 'Off' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('disk full');
    expect(within(group).getByRole('radio', { name: 'Ask' })).toBeChecked();
  });
});
