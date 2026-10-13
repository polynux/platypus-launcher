import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
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
const tweak = { id: 't1', kind: 'recommended', title: 'Use async shaders', reason: 'faster', env: { DXVK_ASYNC: '1' } } as unknown as Tweak;

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
const generalTab = () => screen.getByRole('tab', { name: 'General' });
const tweaksTab = () => screen.getByRole('tab', { name: 'Suggested tweaks' });

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

describe('the shared settings draft (issue #59)', () => {
  it('keeps a tweak toggle when General saves', async () => {
    const props = page({ tweaks: [tweak] });
    fireEvent.click(gear());
    fireEvent.click(tweaksTab());
    fireEvent.click(screen.getByRole('switch', { name: /Use async shaders/ }));
    expect(props.onChangeSettings).toHaveBeenCalledOnce();
    expect(props.onChangeSettings.mock.calls[0][0].env_vars.DXVK_ASYNC).toBe('1');
    fireEvent.click(generalTab());
    fireEvent.click(screen.getByRole('button', { name: 'Save config' }));
    expect(props.onSaveSettings).toHaveBeenCalledOnce();
    expect(props.onSaveSettings.mock.calls[0][0].env_vars.DXVK_ASYNC).toBe('1');
  });

  it('keeps a General edit when a tweak toggles', async () => {
    const props = page({ tweaks: [tweak] });
    fireEvent.click(gear());
    const args = await screen.findByPlaceholderText('-novid -fullscreen');
    fireEvent.change(args, { target: { value: '-windowed' } });
    fireEvent.click(tweaksTab());
    fireEvent.click(screen.getByRole('switch', { name: /Use async shaders/ }));
    expect(props.onChangeSettings).toHaveBeenCalledOnce();
    const saved = props.onChangeSettings.mock.calls[0][0];
    expect(saved.launch_arguments).toBe('-windowed');
    expect(saved.env_vars.DXVK_ASYNC).toBe('1');
  });

  it('keeps both in the reverse order: toggle, then a General edit, then save', async () => {
    const props = page({ tweaks: [tweak] });
    fireEvent.click(gear());
    fireEvent.click(tweaksTab());
    fireEvent.click(screen.getByRole('switch', { name: /Use async shaders/ }));
    fireEvent.click(generalTab());
    const args = await screen.findByPlaceholderText('-novid -fullscreen');
    fireEvent.change(args, { target: { value: '-windowed' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save config' }));
    expect(props.onSaveSettings).toHaveBeenCalledOnce();
    const saved = props.onSaveSettings.mock.calls[0][0];
    expect(saved.env_vars.DXVK_ASYNC).toBe('1');
    expect(saved.launch_arguments).toBe('-windowed');
  });

  it('shows the saved values in both tabs after a save', async () => {
    const props = page({ tweaks: [tweak] });
    fireEvent.click(gear());
    fireEvent.click(tweaksTab());
    fireEvent.click(screen.getByRole('switch', { name: /Use async shaders/ }));
    fireEvent.click(generalTab());
    const args = await screen.findByPlaceholderText('-novid -fullscreen');
    fireEvent.change(args, { target: { value: '-windowed' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save config' }));
    expect(props.onSaveSettings).toHaveBeenCalledOnce();
    // The tweak still reads on…
    fireEvent.click(tweaksTab());
    expect(screen.getByRole('switch', { name: /Use async shaders/ })).toHaveAttribute('aria-checked', 'true');
    // …and General still shows both the edit and the tweak's env var.
    fireEvent.click(generalTab());
    expect((screen.getByPlaceholderText('-novid -fullscreen') as HTMLInputElement).value).toBe('-windowed');
    for (const advanced of screen.getAllByRole('button', { name: /^Advanced/ })) fireEvent.click(advanced);
    expect(screen.getByDisplayValue('DXVK_ASYNC')).toBeInTheDocument();
    expect(screen.queryByText(/Unsaved changes/)).toBeNull();
  });

  it('keeps the one scroll container while switching tabs', () => {
    page({ tweaks: [tweak] });
    fireEvent.click(gear());
    const scroll = screen.getByTestId('gs-tab-scroll');
    fireEvent.click(tweaksTab());
    expect(screen.getByTestId('gs-tab-scroll')).toBe(scroll);
    fireEvent.click(generalTab());
    expect(screen.getByTestId('gs-tab-scroll')).toBe(scroll);
  });
});
