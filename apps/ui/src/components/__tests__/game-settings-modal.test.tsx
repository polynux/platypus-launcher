import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, StorePlugin, Tweak } from '../../types';

const api = vi.hoisted(() => ({
  getGameDetails: vi.fn(),
  getInstallSize: vi.fn(),
  getGameOptions: vi.fn(),
  getCloudInfo: vi.fn(),
  checkCloudSaves: vi.fn(),
  getGameSettingsFields: vi.fn(),
  getGameVersions: vi.fn(),
  getCustomization: vi.fn(),
  previewCommand: vi.fn(),
  getLauncherSettings: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { GamePage } from '../GamePage';

const ALL = ['install', 'cloud-saves', 'versions', 'verify', 'install-options', 'settings', 'move'];
const plugin = (caps: string[] = ALL): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: caps, install_folder: '/games' }) as StorePlugin;
const game = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '/games/echo/tiny', executable: '', is_installed: true, installable: true, features: null,
  settings: { runner_id: 'auto', env_vars: {}, launch_arguments: '' }, ...o,
}) as Game;
const tweak = { id: 't1', kind: 'recommended', title: 'Use fsync', reason: 'faster', env: { DXVK_ASYNC: '1' } } as unknown as Tweak;

function page(over: Partial<React.ComponentProps<typeof GamePage>> = {}, g: Game = game(), p: StorePlugin | undefined = plugin()) {
  const mocks = { onChangeSettings: vi.fn(), onSaveSettings: vi.fn() };
  const props = {
    runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [] as Tweak[],
    onChangeSettings: mocks.onChangeSettings, onSaveSettings: mocks.onSaveSettings, onJob: vi.fn(), onJobAction() {}, onAllowFolder() {}, onNavigate() {}, jobs: [],
    onNotify: vi.fn(), ...over,
  };
  render(<GamePage game={g} plugin={p} {...props} />);
  return { ...props, ...mocks };
}
const gear = () => screen.getByRole('button', { name: 'Game settings' });
const tabNames = () => screen.getAllByRole('tab').map((t) => t.textContent);

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getInstallSize.mockResolvedValue(0);
  api.getCloudInfo.mockResolvedValue({ mode: 'ask', plugin_mode: 'ask', game_mode: '', running: false, status: { state: 'synced', checkedAt: 1, counts: { same: 1, download: 0, upload: 0, conflict: 0 } } });
  api.getGameSettingsFields.mockResolvedValue({ groups: [], values: {} });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
  api.getGameOptions.mockResolvedValue({ groups: [], chosen: {} });
  api.getCustomization.mockResolvedValue({ title: '', sort_title: '', art: {} });
  api.previewCommand.mockResolvedValue('wine game.exe');
  api.getLauncherSettings.mockResolvedValue({ prefixes_folder: '', default_prefixes_folder: '/home/you/.local/share/platypus/prefixes' });
});

describe('the game page keeps only the essentials', () => {
  it('has a gear for the settings, and none of the settings or secondary actions on the page', () => {
    page({ tweaks: [tweak] });
    expect(gear()).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Play' })).toBeTruthy();
    for (const name of [/Verify files/, /Move…/, /Uninstall/, /Check now/, /Add-ons/, /Launcher settings/]) {
      expect(screen.queryByRole('button', { name })).toBeNull();
    }
    expect(screen.queryByText('Cloud saves')).toBeNull();
    expect(screen.queryByText('Suggested tweaks')).toBeNull();
    expect(screen.getByText('Installation')).toBeTruthy(); // the key facts stay
  });

  it('lets the actions in the hero wrap instead of running off the right edge', () => {
    page({}, game({ update_available: true }));
    expect(screen.getByRole('button', { name: /Update/ })).toBeTruthy();
    const actions = gear().parentElement!;
    expect(actions.className).toMatch(/\bflex-wrap\b/);
    expect(actions.parentElement!.className).toMatch(/\bflex-wrap\b/);
    // The title block can shrink below its text and the hero grows with its content.
    const hero = actions.closest('section')!;
    expect(hero.className).toMatch(/min-h-\[280px\]/);
    expect(hero.className).not.toMatch(/(?<!min-)\bh-\[280px\]/);
  });

  it('draws the user\'s logo in place of the title, keeping the title as its name', () => {
    page({}, game({ logo_url: '/platypus-art/abc/logo.png?v=1', customized: ['logo'] }));
    const h1 = screen.getByRole('heading', { level: 1 });
    expect(within(h1).getByAltText('Echo Tiny').getAttribute('src')).toBe('/platypus-art/abc/logo.png?v=1');
  });

  it('puts the user\'s background behind the page', () => {
    page({}, game({ background_url: '/platypus-art/abc/background.png?v=1' }));
    expect(screen.getByTestId('game-background').getAttribute('style')).toContain('/platypus-art/abc/background.png?v=1');
  });
});

describe('the game settings dialog', () => {
  it('opens on General and lists every tab a full plugin offers', async () => {
    page({ tweaks: [tweak] });
    fireEvent.click(gear());
    expect(await screen.findByRole('dialog', { name: 'Settings for Echo Tiny' })).toBeTruthy();
    expect(tabNames()).toEqual(['General', 'Cloud saves', 'Versions', 'Add-ons & options', 'Plugin settings', 'Suggested tweaks', 'Customization', 'Maintenance']);
    expect(screen.getByRole('tab', { name: 'General', selected: true })).toBeTruthy();
    expect(screen.getByText('runner')).toBeTruthy(); // the general form
  });

  it('shows only the tabs the plugin and the game offer', () => {
    page({}, game({ features: ['install'] }), plugin(['install', 'cloud-saves', 'versions']));
    fireEvent.click(gear());
    expect(tabNames()).toEqual(['General', 'Customization', 'Maintenance']);
  });

  it('gives a Steam game the core tabs plus Maintenance for save backups', () => {
    page({}, game({ source: 'steam', store: 'steam' as never, store_app_id: '10', features: null }), undefined);
    fireEvent.click(gear());
    expect(tabNames()).toEqual(['General', 'Customization', 'Maintenance']);
  });

  it('holds the cloud saves in a tab, with Check now', async () => {
    api.checkCloudSaves.mockResolvedValue({ mode: 'ask', plugin_mode: 'ask', game_mode: '', running: false, status: { state: 'synced', checkedAt: 2, counts: { same: 1, download: 0, upload: 0, conflict: 0 } } });
    page();
    fireEvent.click(gear());
    fireEvent.click(screen.getByRole('tab', { name: 'Cloud saves' }));
    const check = await screen.findByRole('button', { name: /Check now/ });
    fireEvent.click(check);
    await waitFor(() => expect(api.checkCloudSaves).toHaveBeenCalled());
    expect(screen.getByRole('tabpanel', { name: 'Cloud saves' })).toBeTruthy();
  });

  it('says so when the plugin has no settings for the game', async () => {
    page();
    fireEvent.click(gear());
    fireEvent.click(screen.getByRole('tab', { name: 'Plugin settings' }));
    expect(await screen.findByText('Echo has no settings for Echo Tiny.')).toBeTruthy();
  });

  it('switches a tweak from its tab', () => {
    const props = page({ tweaks: [tweak] });
    fireEvent.click(gear());
    fireEvent.click(screen.getByRole('tab', { name: 'Suggested tweaks' }));
    fireEvent.click(screen.getByRole('switch', { name: /Use fsync/ }));
    expect(props.onChangeSettings).toHaveBeenCalled();
    expect(props.onChangeSettings.mock.calls[0][1]).toBe('Applied "Use fsync"');
  });

  it('keeps unsaved general edits while another tab shows, and saves them from the form', async () => {
    const props = page();
    fireEvent.click(gear());
    const args = await screen.findByPlaceholderText('-novid -fullscreen');
    fireEvent.change(args, { target: { value: '-windowed' } });
    expect(screen.getByRole('status').textContent).toMatch(/Unsaved changes/);
    fireEvent.click(screen.getByRole('tab', { name: 'Customization' }));
    expect(screen.getByRole('tab', { name: 'Customization', selected: true })).toBeTruthy();
    fireEvent.click(screen.getByRole('tab', { name: 'General' }));
    expect((screen.getByPlaceholderText('-novid -fullscreen') as HTMLInputElement).value).toBe('-windowed');
    fireEvent.click(screen.getByRole('button', { name: 'Save config' }));
    expect(props.onSaveSettings).toHaveBeenCalledOnce();
    expect(props.onSaveSettings.mock.calls[0][0].launch_arguments).toBe('-windowed');
    // Saved: nothing is unsaved any more, so the dialog can be dismissed again.
    expect(screen.queryByText(/Unsaved changes/)).toBeNull();
  });

  it('moves between tabs with the arrow keys', () => {
    page();
    fireEvent.click(gear());
    const general = screen.getByRole('tab', { name: 'General' });
    fireEvent.keyDown(general, { key: 'ArrowDown' });
    expect(screen.getByRole('tab', { name: 'Cloud saves', selected: true })).toBeTruthy();
    fireEvent.keyDown(screen.getByRole('tab', { name: 'Cloud saves' }), { key: 'ArrowUp' });
    expect(screen.getByRole('tab', { name: 'General', selected: true })).toBeTruthy();
  });

  it('closes with its close button', () => {
    page();
    fireEvent.click(gear());
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('round-trips the working directory for a game that runs itself (issue #33)', async () => {
    const props = page();
    fireEvent.click(gear());
    const work = await screen.findByLabelText('working directory');
    fireEvent.change(work, { target: { value: '/home/you/Games/doom' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save config' }));
    expect(props.onSaveSettings).toHaveBeenCalledOnce();
    expect(props.onSaveSettings.mock.calls[0][0].working_dir).toBe('/home/you/Games/doom');
  });

  it('hides the working directory for a game the Steam client starts', () => {
    page({}, game({ source: 'steam', store: 'steam' as never, store_app_id: '10', executable: '', features: null }), undefined);
    fireEvent.click(gear());
    expect(screen.queryByLabelText('working directory')).toBeNull();
  });

  it('round-trips the Wine prefix and shows the effective path (issue #34)', async () => {
    const props = page();
    fireEvent.click(gear());
    const prefix = await screen.findByLabelText('wine prefix');
    fireEvent.change(prefix, { target: { value: '/home/you/prefixes/echo' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save config' }));
    expect(props.onSaveSettings).toHaveBeenCalledOnce();
    expect(props.onSaveSettings.mock.calls[0][0].wine_prefix).toBe('/home/you/prefixes/echo');
  });

  it('hides the Wine prefix for a native Linux game', () => {
    page({}, game({ native: true }));
    fireEvent.click(gear());
    expect(screen.queryByLabelText('wine prefix')).toBeNull();
    // The working directory still applies.
    expect(screen.getByLabelText('working directory')).toBeInTheDocument();
  });

  it('shows the global prefix folder in the effective-prefix hint (issue #37)', async () => {
    api.getLauncherSettings.mockResolvedValue({ prefixes_folder: '/mnt/big/prefixes', default_prefixes_folder: '/home/you/.local/share/platypus/prefixes' });
    page({}, game({ id: 'p.echo.tiny' }));
    fireEvent.click(gear());
    expect(await screen.findByText(/prefix: \/mnt\/big\/prefixes\/p\.echo\.tiny \(default\)/)).toBeInTheDocument();
  });
});
