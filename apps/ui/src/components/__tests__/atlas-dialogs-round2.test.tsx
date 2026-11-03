// @ts-nocheck — node fs/path guard test, outside the app's DOM tsconfig.
// Round 2 of the Atlas dialog audit (Refs #171): the remaining dialogs use
// the shared header/footer, the smart filter clears back to "any", danger
// stays on Uninstall, and the Kind switch keeps radio semantics.
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within, act, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Effect } from 'effect';
import fs from 'node:fs';
import path from 'node:path';
import '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { CloudStatus, Collection, Game, GameCopy, CopyComparison, Job, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  verifyGame: vi.fn(),
  repairGame: vi.fn(),
  checkMove: vi.fn(),
  pickMoveFolder: vi.fn(),
  moveGame: vi.fn(),
  createCollection: vi.fn(),
  updateCollection: vi.fn(),
  deleteCollection: vi.fn(),
  pinCollection: vi.fn(),
  addCollectionGame: vi.fn(),
  removeCollectionGame: vi.fn(),
  reorderCollections: vi.fn(),
  resolveCloudSaves: vi.fn(),
  launchGameAnyway: vi.fn(),
  launchGameViaClient: vi.fn(),
  launchGame: vi.fn(),
  acceptGameTerms: vi.fn(),
  approvePlugin: vi.fn(),
  uninstallPlugin: vi.fn(),
  listPlugins: vi.fn(),
  rescanPlugins: vi.fn(),
  authStatus: vi.fn(),
}));

const cloudHandlers = vi.hoisted(() => ({ current: new Set<(e: unknown) => void>() }));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onCloud: (cb: (e: unknown) => void) => {
    cloudHandlers.current.add(cb);
    return () => { cloudHandlers.current.delete(cb); };
  },
  onAuth: () => () => {},
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onSocial: () => () => {},
  onFamily: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
  onPlaytime: () => () => {},
  onDiskUsage: () => () => {},
}));

import { VerifyDialog } from '../VerifyDialog';
import { MoveDialog } from '../MoveDialog';
import { CopyCompareDialog } from '../CopyCompareDialog';
import { TermsDialog } from '../TermsDialog';
import { CollectionsDialog } from '../CollectionsDialog';
import { CloudConflictDialog } from '../CloudSavesConflict';
import { CloudSavesHost } from '../CloudSavesHost';
import { FriendsModal } from '../FriendsModal';
import { PluginsView } from '../PluginsView';

const renderDialog = (ui: React.ReactElement) => render(ui, { wrapper: TooltipProvider });
afterEach(cleanup);
beforeEach(() => {
  cloudHandlers.current.clear();
  Object.values(api).forEach((f) => f.mockReset());
  api.verifyGame.mockResolvedValue({ id: 'check-1' });
  api.repairGame.mockResolvedValue({ id: 'repair-1' });
  api.createCollection.mockResolvedValue([]);
  api.updateCollection.mockResolvedValue([]);
  api.resolveCloudSaves.mockResolvedValue({});
  api.acceptGameTerms.mockResolvedValue(undefined);
  api.launchGameAnyway.mockResolvedValue('Started');
  api.launchGameViaClient.mockResolvedValue('Asked Steam');
  api.launchGame.mockResolvedValue('Started');
  api.pickMoveFolder.mockResolvedValue('');
  api.uninstallPlugin.mockResolvedValue([]);
  api.listPlugins.mockResolvedValue([]);
  api.authStatus.mockResolvedValue({ loggedIn: false });
});

const noop = () => {};
const game = (o: Partial<Game> = {}): Game =>
  ({ id: 'g', source: 'steam', store: 'steam', store_app_id: 'g', title: 'Echo Tiny', install_dir: '/g/tiny', is_installed: true, settings: {}, features: [], ...o }) as Game;
const pluginsReady: StorePlugin = {
  id: 'steam', name: 'Steam', version: '1', store: 'steam', kind: 'store', status: 'ready',
  missing: [], requires: [], capabilities: ['install'], declared: ['install'], offers: [], warnings: [],
  needs: [], network: [], read: [], write: [], dir: '/x', exec: 'run',
  hash: 'h', install_folder: '/g', default_install_folder: '/g', extra_folders: [],
} as unknown as StorePlugin;

/** The shared chrome every Atlas dialog carries: a heading, an × with the
 *  Close label, and a right-aligned foot (ModalFoot) with no equal-width
 *  buttons. Old shells had their own h2, no ×, and flex-1 buttons. */
function expectSharedChrome(dialog: HTMLElement) {
  // Header × (ModalHead always labels it Close); old shells had none.
  expect(dialog.querySelector('button[aria-label="Close"]')).toBeTruthy();
  // ModalFoot: right-aligned with the shared top edge. Old footers had
  // neither (Verify/Move) or the alignment without the edge (Terms etc.).
  const foot = dialog.querySelector('div.justify-end.border-t');
  expect(foot).toBeTruthy();
  // No equal-width buttons (the old Verify/Move/Plugins footers). The Modal
  // itself scrolls in a flex-1 region, so only buttons count.
  expect(dialog.querySelector('button.flex-1')).toBeNull();
}

function expectFooterOrder(dialog: HTMLElement, names: string[]) {
  const buttons = within(dialog).getAllByRole('button');
  const texts = buttons.map((b) => b.textContent?.trim() ?? '');
  let last = -1;
  for (const name of names) {
    const idx = texts.findIndex((t, i) => i > last && t === name);
    expect(idx, `footer order misses ${name} in [${texts.join(' | ')}]`).toBeGreaterThan(last);
    last = idx;
  }
}

const conflictStatus = (): CloudStatus => ({
  state: 'conflict',
  counts: { same: 1, conflict: 1, download: 0, upload: 0, unmapped: 0 },
  conflicts: [{ path: 'save.dat', local: { size: 10, mtime: 20 }, remote: { size: 12, mtime: 18, machine: 'deck' } }],
}) as unknown as CloudStatus;

const COLLECTIONS: Collection[] = [
  { id: 'all', builtin: true, name: 'All', games: ['g'] },
  { id: 'couch', name: 'Couch', pinned: true, games: [] },
] as unknown as Collection[];
const GAMES = [game({ id: 'a', title: 'A' }), game({ id: 'b', title: 'B' })];

describe('smart filter clears back to any (F10)', () => {
  it('sets Installed then clears it: the created filter has no installed key', async () => {
    const user = userEvent.setup();
    api.createCollection.mockResolvedValue(COLLECTIONS);
    renderDialog(<CollectionsDialog collections={COLLECTIONS} games={GAMES} onChanged={noop} onClose={noop} />);
    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'Night' } });
    await user.click(screen.getByRole('radio', { name: 'Smart filter' }));
    await user.click(screen.getByRole('combobox', { name: 'Installed' }));
    await user.click(await screen.findByRole('option', { name: 'Yes' }));
    // Back to "any": the sentinel maps to ''.
    await user.click(screen.getByRole('combobox', { name: 'Installed' }));
    await user.click(await screen.findByRole('option', { name: 'Any' }));
    // One remaining condition, so Create reaches the core without installed.
    await user.click(screen.getByRole('combobox', { name: 'Played' }));
    await user.click(await screen.findByRole('option', { name: 'Yes' }));
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(api.createCollection).toHaveBeenCalledWith(expect.any(String), { played: true }));
    expect(api.createCollection.mock.calls[0][1]).not.toHaveProperty('installed');
  });

  it('edits a smart collection back to any: the saved filter drops the key', async () => {
    const user = userEvent.setup();
    const smartCollection = { id: 'played', name: 'Played', pinned: true, smart: { installed: true, played: true } } as unknown as Collection;
    api.updateCollection.mockResolvedValue(COLLECTIONS);
    renderDialog(<CollectionsDialog collections={[COLLECTIONS[0], smartCollection]} games={GAMES} onChanged={noop} onClose={noop} />);
    fireEvent.click(screen.getByRole('button', { name: 'Edit conditions for Played' }));
    await user.click(screen.getByRole('combobox', { name: 'Installed' }));
    await user.click(await screen.findByRole('option', { name: 'Any' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(api.updateCollection).toHaveBeenCalledWith('played', 'Played', { played: true }));
    expect(api.updateCollection.mock.calls[0][2]).not.toHaveProperty('installed');
  });

  it('clearing the badge kind drops its value too', async () => {
    const user = userEvent.setup();
    api.createCollection.mockResolvedValue(COLLECTIONS);
    renderDialog(<CollectionsDialog collections={COLLECTIONS} games={GAMES} onChanged={noop} onClose={noop} />);
    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'Night' } });
    await user.click(screen.getByRole('radio', { name: 'Smart filter' }));
    await user.click(screen.getByRole('combobox', { name: 'Badge type' }));
    await user.click(await screen.findByRole('option', { name: 'Rating' }));
    await user.click(screen.getByRole('combobox', { name: 'Badge value' }));
    await user.click(await screen.findByRole('option', { name: 'Native' }));
    await user.click(screen.getByRole('combobox', { name: 'Badge type' }));
    await user.click(await screen.findByRole('option', { name: 'Any' }));
    await user.click(screen.getByRole('combobox', { name: 'Played' }));
    await user.click(await screen.findByRole('option', { name: 'Yes' }));
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(api.createCollection).toHaveBeenCalled());
    const filter = api.createCollection.mock.calls[0][1] as Record<string, unknown>;
    expect(filter).not.toHaveProperty('badge_kind');
    expect(filter).not.toHaveProperty('badge_value');
    expect(filter).toEqual({ played: true });
  });

  it('uses the ds Select, not a native select', async () => {
    const user = userEvent.setup();
    const { container } = renderDialog(<CollectionsDialog collections={COLLECTIONS} games={GAMES} onChanged={noop} onClose={noop} />);
    await user.click(screen.getByRole('radio', { name: 'Smart filter' }));
    expect(container.querySelector('select')).toBeNull();
    expect(screen.getByRole('combobox', { name: 'Installed' })).toBeInTheDocument();
    const src = fs.readFileSync(path.join(__dirname, '../CollectionsDialog.tsx'), 'utf8');
    expect(src).toContain("from './ds/select'");
    expect(src).not.toContain('<select');
  });

  it('the Kind switch is one radiogroup with the selected kind marked', async () => {
    const user = userEvent.setup();
    renderDialog(<CollectionsDialog collections={COLLECTIONS} games={GAMES} onChanged={noop} onClose={noop} />);
    const group = screen.getByRole('radiogroup', { name: 'Kind' });
    expect(within(group).getByRole('radio', { name: 'Manual list' })).toHaveAttribute('aria-checked', 'true');
    await user.click(within(group).getByRole('radio', { name: 'Smart filter' }));
    expect(within(group).getByRole('radio', { name: 'Smart filter' })).toHaveAttribute('aria-checked', 'true');
    expect(within(group).getByRole('radio', { name: 'Manual list' })).toHaveAttribute('aria-checked', 'false');
  });
});

describe('converted dialogs share the header and footer (F3)', () => {
  it('verify renders its title, Close ×, and Not now before Repair', async () => {
    const jobs = [{ id: 'check-1', kind: 'verify', source: 's', game_id: 'g', title: 'Echo Tiny', state: 'done', verify: { checked: 12, damaged: 3, missing: 0, repaired: 0 } }] as unknown as Job[];
    renderDialog(<VerifyDialog game={game()} jobs={jobs} onClose={noop} />);
    const dialog = await screen.findByRole('dialog', { name: 'Verify Echo Tiny' });
    expectSharedChrome(dialog);
    expect(within(dialog).getByRole('heading', { name: 'Verify Echo Tiny' })).toBeInTheDocument();
    expectFooterOrder(dialog, ['Not now', 'Repair']);
    const repair = within(dialog).getByRole('button', { name: 'Repair' });
    expect(repair.className).toMatch(/bg-accent/);
    expect(repair.className).not.toMatch(/sky-/);
  });

  it('cloud conflict renders its heading, Close ×, and Close before Apply', () => {
    const onClose = vi.fn();
    renderDialog(<CloudConflictDialog game={game()} status={conflictStatus()} onClose={onClose} onResolved={noop} />);
    const dialog = screen.getByRole('dialog', { name: /cloud save conflict/i });
    expectSharedChrome(dialog);
    expect(within(dialog).getByRole('heading', { name: 'Saves changed on both sides' })).toBeInTheDocument();
    // Header × plus footer Close: both named Close.
    expect(within(dialog).getAllByRole('button', { name: 'Close' })).toHaveLength(2);
    expectFooterOrder(dialog, ['Close', 'Apply choices']);
    fireEvent.click(within(dialog).getAllByRole('button', { name: 'Close' })[1]);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('copy compare renders its title, sub, Close ×, and a single Close foot', () => {
    const ana: GameCopy = { owner_id: 'ana', owner_name: 'Ana', available: true, dlc: [] };
    const ben: GameCopy = { owner_id: 'ben', owner_name: 'Ben', available: true, dlc: [] };
    const compare: CopyComparison = { baseline: 'ben', diffs: [] };
    const onClose = vi.fn();
    renderDialog(<CopyCompareDialog game={game({ copies: [ana, ben], copy_compare: compare })} copies={[ana, ben]} compare={compare} onClose={onClose} />);
    const dialog = screen.getByRole('dialog', { name: /compare copies of/i });
    expectSharedChrome(dialog);
    expect(within(dialog).getByRole('heading', { name: 'Compare copies' })).toBeInTheDocument();
    expect(within(dialog).getAllByRole('button', { name: 'Close' })).toHaveLength(2);
    fireEvent.click(within(dialog).getAllByRole('button', { name: 'Close' })[1]);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('terms renders its heading with Cancel before I accept', () => {
    const onCancel = vi.fn();
    renderDialog(<TermsDialog game={game()} terms={[{ id: 't', version: '3', title: 'Terms' }]} onAccepted={noop} onCancel={onCancel} />);
    const dialog = screen.getByRole('dialog', { name: /terms for/i });
    expectSharedChrome(dialog);
    expect(within(dialog).getByRole('heading', { name: 'Terms for Echo Tiny' })).toBeInTheDocument();
    expectFooterOrder(dialog, ['Cancel', 'I accept']);
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalledOnce();
  });

  it('move renders its title with Cancel before Move', () => {
    renderDialog(<MoveDialog game={game()} startFolder="/g" onClose={noop} onQueued={noop} />);
    const dialog = screen.getByRole('dialog', { name: 'Move Echo Tiny' });
    expectSharedChrome(dialog);
    expect(within(dialog).getByRole('heading', { name: 'Move Echo Tiny' })).toBeInTheDocument();
    expectFooterOrder(dialog, ['Cancel', 'Move']);
  });

  it('collections manager renders its title with Done in the foot', () => {
    const onClose = vi.fn();
    renderDialog(<CollectionsDialog collections={COLLECTIONS} games={GAMES} onChanged={noop} onClose={onClose} />);
    const dialog = screen.getByRole('dialog', { name: 'Collections' });
    expectSharedChrome(dialog);
    expect(within(dialog).getByRole('heading', { name: 'Collections' })).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Done' }));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('friends list renders its title, search, and Close foot', () => {
    const onClose = vi.fn();
    renderDialog(
      <FriendsModal views={[]} labels={{}} plugins={[]} inLibrary={() => false} onClose={onClose}
        onSetEnabled={noop} onSetStatus={noop} onSetPlaying={noop} onReconnect={noop} onOpenGame={noop} />,
    );
    const dialog = screen.getByRole('dialog', { name: 'Friends' });
    expectSharedChrome(dialog);
    expect(within(dialog).getByRole('heading', { name: 'Friends' })).toBeInTheDocument();
    expect(within(dialog).getByPlaceholderText(/search friends/i)).toBeInTheDocument();
    expect(within(dialog).getAllByRole('button', { name: 'Close' })).toHaveLength(2);
    fireEvent.click(within(dialog).getAllByRole('button', { name: 'Close' })[1]);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('friends chat renders its title with a Close foot', () => {
    renderDialog(
      <FriendsModal views={[]} labels={{}} plugins={[]} inLibrary={() => false} onClose={noop}
        onSetEnabled={noop} onSetStatus={noop} onSetPlaying={noop} onReconnect={noop} onOpenGame={noop}
        chat={<div>chat body</div>} onBackFromChat={noop} />,
    );
    const dialog = screen.getByRole('dialog', { name: 'Friends' });
    expectSharedChrome(dialog);
    expect(dialog).toHaveTextContent('chat body');
  });

  it('offline cloud prompt renders Cancel before Launch anyway', async () => {
    const g = game();
    renderDialog(<CloudSavesHost games={[g]} onNotify={noop} onLog={noop} />);
    act(() => { cloudHandlers.current.forEach((cb) => cb({ game_id: g.id, kind: 'prompt', prompt: 'offline', message: 'The store is not reachable' })); });
    const dialog = await screen.findByRole('dialog', { name: 'Cloud saves not synced' });
    expectSharedChrome(dialog);
    expectFooterOrder(dialog, ['Cancel', 'Launch anyway']);
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(api.launchGameAnyway).not.toHaveBeenCalled();
  });

  it('client-running cloud prompt renders Cancel, sync-less launch, then client launch', async () => {
    const g = game();
    renderDialog(<CloudSavesHost games={[g]} onNotify={noop} onLog={noop} />);
    act(() => { cloudHandlers.current.forEach((cb) => cb({ game_id: g.id, kind: 'prompt', prompt: 'client-running', message: 'Steam is running.', client: 'Steam' })); });
    const dialog = await screen.findByRole('dialog', { name: 'Store client running' });
    expectSharedChrome(dialog);
    expectFooterOrder(dialog, ['Cancel', 'Launch without cloud sync', 'Launch through Steam']);
  });

  it('decision cloud prompt for a missing game renders a Close foot', async () => {
    renderDialog(<CloudSavesHost games={[]} onNotify={noop} onLog={noop} />);
    act(() => { cloudHandlers.current.forEach((cb) => cb({ game_id: 'missing', title: 'Echo Tiny', kind: 'prompt', prompt: 'offline', message: 'The store is not reachable' })); });
    const dialog = await screen.findByRole('dialog', { name: 'Cloud saves need a decision' });
    expectSharedChrome(dialog);
    expect(within(dialog).getAllByRole('button', { name: 'Close' })).toHaveLength(2);
  });

  it('plugin uninstall confirms with the danger style after Cancel', async () => {
    const user = userEvent.setup();
    renderDialog(<PluginsView plugins={[pluginsReady]} warnings={[]} onPlugins={noop} onLibraryChanged={noop} onNotify={noop} />);
    await user.click(screen.getByRole('button', { name: 'Uninstall' }));
    const dialog = await screen.findByRole('dialog', { name: 'Uninstall Steam?' });
    expectSharedChrome(dialog);
    expectFooterOrder(dialog, ['Cancel', 'Uninstall']);
    const confirm = within(dialog).getByRole('button', { name: 'Uninstall' });
    expect(confirm.className).toMatch(/bg-danger/);
    expect(confirm.className).toMatch(/text-danger/);
  });
});

describe('shared dialog edge (F11)', () => {
  it('keeps the strong hairline ring on every scheme', async () => {
    const fsMod = await import('node:fs');
    const dialog = fsMod.readFileSync(path.join(__dirname, '../ds/dialog.tsx'), 'utf8');
    expect(dialog).toContain('ring-hairline-strong');
  });
});
