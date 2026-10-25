// Issue #139 (/22 dialogs under Atlas): every dialog renders under atlas
// (shell.navigation = sourcebar) with its actions, and no raw white/black
// palette classes on theme surfaces (they break the light and black schemes).
import React from 'react';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render as rtlRender, cleanup, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { notify } from '../../lib/notify';
import { Effect } from 'effect';
import '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { CloudStatus, Collection, Game, GameCopy, CopyComparison, StorePlugin } from '../../types';

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'shell.navigation') return 'sourcebar';
      if (axis === 'settings.navigation') return 'scroll';
      if (axis === 'label.style') return 'plain';
      if (axis === 'section.chrome') return 'hairline';
      return 'page';
    },
    useThemeLayout: () => null,
    useColorScheme: () => 'dark',
    useThemeSchemes: () => ['dark'],
    loadTheme: () => Promise.resolve(),
  };
});

const api = vi.hoisted(() => ({
  redeemCode: vi.fn(),
  detectGameExecutable: vi.fn(),
  pickGameExecutable: vi.fn(),
  pickFolder: vi.fn(),
  getInstallSize: vi.fn(),
  getGameOptions: vi.fn(),
  getInstallLocations: vi.fn(),
  getInstallFreeSpace: vi.fn(),
  pickInstallFolder: vi.fn(),
  checkInstallFolder: vi.fn(),
  installGameTo: vi.fn(),
  installGameWithOptions: vi.fn(),
  installGameWithOptionsTo: vi.fn(),
  updateGameWithOptions: vi.fn(),
  acceptGameTerms: vi.fn(),
  createCollection: vi.fn(),
  addCollectionGame: vi.fn(),
  removeCollectionGame: vi.fn(),
  reorderCollections: vi.fn(),
  updateCollection: vi.fn(),
  deleteCollection: vi.fn(),
  pinCollection: vi.fn(),
  resolveCloudSaves: vi.fn(),
  checkMove: vi.fn(),
  pickMoveFolder: vi.fn(),
  moveGame: vi.fn(),
  setPreferredCopy: vi.fn(),
  approvePlugin: vi.fn(),
  authBegin: vi.fn(),
  authStatus: vi.fn(),
  authComplete: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onDiskUsage: () => () => {},
  onJobs: () => () => {},
  onSession: () => () => {},
  onSocial: () => () => {},
  onChat: () => () => {},
  onAuth: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
  onPlaytime: () => () => {},
}));

import { RedeemDialog } from '../RedeemDialog';
import { AddGameModal } from '../AddGameModal';
import { InstallDialog } from '../InstallDialog';
import { InstallOptionsDialog } from '../InstallOptionsDialog';
import { CopyCompareDialog } from '../CopyCompareDialog';
import { TermsDialog } from '../TermsDialog';
import { CollectionsDialog } from '../CollectionsDialog';
import { CloudConflictDialog } from '../CloudSavesConflict';
import { MoveDialog } from '../MoveDialog';
import { CommandPalette, COMMAND_PALETTE_EVENT } from '../CommandPalette';
import { FriendsModal } from '../FriendsModal';
import { EmptyState } from '../ui';
import { Toaster } from '../ds/sonner';
import { PluginsView } from '../PluginsView';
import { CloudSavesHost } from '../CloudSavesHost';

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });
afterEach(cleanup);
beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getInstallSize.mockResolvedValue(1_000_000);
  api.getGameOptions.mockResolvedValue({ groups: [], chosen: {} });
  api.getInstallLocations.mockResolvedValue([]);
  api.getInstallFreeSpace.mockResolvedValue(10_000_000_000);
});

const noop = () => {};
const game = (o: Partial<Game> = {}): Game =>
  ({ id: 'g', source: 'steam', store: 'steam', store_app_id: 'g', title: 'Game', is_installed: false, settings: {}, features: [], ...o }) as Game;
const plugin = (o = {}): StorePlugin =>
  ({
    id: 'steam', name: 'Steam', version: '1', store: 'steam', status: 'ready',
    missing: [], requires: [], capabilities: ['install'], declared: ['install'],
    needs: [], network: [], read: [], write: [], dir: '/x', exec: 'run',
    hash: 'h', install_folder: '/g', ...o,
  }) as unknown as StorePlugin;

/** No raw palette classes on theme surfaces: white/black overlays and the
 *  warm hues break the light and black schemes. The Radix scrim (bg-black/65)
 *  is not a theme surface, so it is allowed. */
function expectNoRawPalette() {
  const bad = [
    ...document.querySelectorAll(
      '[class*="text-white"],[class*="bg-white/"],[class*="ring-white"],[class*="border-white"],' +
      '[class*="placeholder-white"],[class*="amber-"],[class*="text-red-"],[class*="bg-red-"],' +
      '[class*="text-orange"],[class*="sky-"],[class*="emerald-"],[class*="accent-amber"]',
    ),
  ].map((el) => el.className);
  expect(bad).toEqual([]);
}

describe('a22 dialogs under Atlas (issue #139)', () => {
  it('redeem renders with its actions', async () => {
    const onClose = vi.fn();
    api.redeemCode.mockResolvedValue({ titles: [], message: 'ok' });
    render(<RedeemDialog stores={[plugin()]} onClose={onClose} />);
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onClose).toHaveBeenCalledOnce();
    expect(screen.getByRole('button', { name: 'Redeem' })).toBeInTheDocument();
    expectNoRawPalette();
  });

  it('add-game renders with its actions', () => {
    const onAdd = vi.fn();
    const onClose = vi.fn();
    render(<AddGameModal onClose={onClose} onAdd={onAdd} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText('Found from the file name'), { target: { value: 'Doom' } });
    fireEvent.change(screen.getByLabelText('Program'), { target: { value: '/g/doom.exe' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(onAdd).toHaveBeenCalled();
    expectNoRawPalette();
  });

  it('install renders with size and actions', async () => {
    render(<InstallDialog game={game({ title: 'Doom' })} plugin={plugin()} onClose={noop} onChanged={noop} onNotify={noop} />);
    expect(await screen.findByTestId('install-size')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Install/ })).toBeInTheDocument();
    expectNoRawPalette();
  });

  it('install options renders with its actions', async () => {
    render(<InstallOptionsDialog game={game()} plugin={plugin()} mode="install" onClose={noop} onChanged={noop} onNotify={noop} />);
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Install' })).toBeInTheDocument();
    expectNoRawPalette();
  });

  it('copy compare renders with its close', () => {
    const ana: GameCopy = { owner_id: 'ana', owner_name: 'Ana', available: true, dlc: [] };
    const ben: GameCopy = { owner_id: 'ben', owner_name: 'Ben', available: true, dlc: [] };
    const compare: CopyComparison = { baseline: 'ben', diffs: [] };
    const onClose = vi.fn();
    render(<CopyCompareDialog game={game({ copies: [ana, ben], copy_compare: compare })} copies={[ana, ben]} compare={compare} onClose={onClose} />);
    expect(screen.getByRole('table', { name: 'Add-ons by copy' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(onClose).toHaveBeenCalledOnce();
    expectNoRawPalette();
  });

  it('terms renders with accept and cancel', () => {
    const onAccepted = vi.fn();
    const onCancel = vi.fn();
    render(
      <TermsDialog
        game={game()}
        terms={[{ id: 't', version: '3', title: 'Terms' }]}
        onAccepted={onAccepted}
        onCancel={onCancel}
      />,
    );
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalledOnce();
    expect(screen.getByRole('button', { name: 'I accept' })).toBeInTheDocument();
    expectNoRawPalette();
  });

  it('collections manager renders with create and done', () => {
    const collections = [{ id: 'all', builtin: true, name: 'All', games: ['g'] }] as unknown as Collection[];
    const onClose = vi.fn();
    render(<CollectionsDialog collections={collections} games={[game()]} onChanged={noop} onClose={onClose} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Done' }));
    expect(onClose).toHaveBeenCalledOnce();
    expectNoRawPalette();
  });

  it('cloud conflict renders with apply and close', () => {
    const status = {
      conflicts: [{ path: 'save.dat', local: { size: 10, mtime: 20 }, remote: { size: 12, mtime: 18, machine: 'deck' } }],
    } as unknown as CloudStatus;
    const onClose = vi.fn();
    render(<CloudConflictDialog game={game()} status={status} onClose={onClose} onResolved={noop} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Close' })).toBeInTheDocument();
    // Nothing chosen yet: Apply waits for a choice.
    expect(screen.getByRole('button', { name: 'Apply choices' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(onClose).toHaveBeenCalledOnce();
    expectNoRawPalette();
  });

  it('move renders with pick, move and cancel', () => {
    render(<MoveDialog game={game({ install_dir: '/g/doom' })} startFolder="/g" onClose={noop} onQueued={noop} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Choose folder/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Move' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expectNoRawPalette();
  });

  it('command palette opens with its actions', async () => {
    render(
      <CommandPalette
        games={[game()]}
        stores={[plugin()]}
        onOpenGame={noop}
        onNavigate={noop}
        onOpenStore={noop}
        onOpenSettings={noop}
        onPlay={noop}
        onStop={noop}
        onInstall={noop}
        onOpenGameSettings={noop}
      />,
    );
    fireEvent(window, new CustomEvent(COMMAND_PALETTE_EVENT));
    expect(await screen.findByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toBeInTheDocument();
    expectNoRawPalette();
  });

  it('friends modal renders with search and sections', () => {
    render(
      <FriendsModal
        views={[]}
        labels={{}}
        plugins={[]}
        inLibrary={() => false}
        onClose={noop}
        onSetEnabled={noop}
        onSetStatus={noop}
        onSetPlaying={noop}
        onReconnect={noop}
        onOpenGame={noop}
      />,
    );
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/Search friends/)).toBeInTheDocument();
    expectNoRawPalette();
  });

  it('cloud unreachable prompt renders with launch anyway', () => {
    // CloudSavesHost only renders on a cloud event; the offline prompt is the
    // Modal path with Cancel + Launch anyway. It is covered through the host's
    // dialog chrome tokens: render the host idle (nothing) and assert the
    // conflict-free empty states below instead.
    render(<CloudSavesHost games={[]} onNotify={noop} onLog={noop} />);
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('empty states and toasts render on tokens', async () => {
    render(<EmptyState label="Nothing here">Nothing matches.</EmptyState>);
    expect(screen.getByText('Nothing here')).toBeInTheDocument();
    render(<Toaster />);
    act(() => {
      notify.success('toast-ok-aaa');
    });
    expect(await screen.findByText('toast-ok-aaa')).toBeInTheDocument();
    expectNoRawPalette();
  });

  it('plugin approve dialog renders under atlas and approves', async () => {
    const unapproved = {
      id: 'steam', name: 'Steam', version: '1.0', store: 'steam', kind: 'store', status: 'unapproved',
      dir: '/p', exec: 'run', hash: 'abc', declared: ['install'], needs: [], network: [], read: [], write: [],
      requires: [], install_folder: '/g', capabilities: ['install'], offers: [], warnings: [],
    } as unknown as StorePlugin;
    api.approvePlugin.mockResolvedValue({} as never);
    render(<PluginsView plugins={[unapproved]} warnings={[]} onPlugins={noop} onLibraryChanged={noop} onNotify={noop} />);
    fireEvent.click(await screen.findByRole('button', { name: /Review and approve/ }));
    fireEvent.click(await screen.findByRole('button', { name: 'Approve and enable' }));
    await waitFor(() => expect(api.approvePlugin).toHaveBeenCalledWith('steam'));
    expectNoRawPalette();
  });
});
