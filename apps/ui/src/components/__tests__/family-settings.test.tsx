import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, waitFor, act } from '@testing-library/react';
import { Effect } from 'effect';
import type { FamilyWarning, Game, SettingsSaved, SettingsView, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  getFamilyMembers: vi.fn(),
  getPluginSettings: vi.fn(),
  savePluginSettings: vi.fn(),
  getGameSettingsFields: vi.fn(),
  saveGameSettingsFields: vi.fn(),
}));
const family = vi.hoisted(() => ({ onWarning: null as null | ((w: unknown) => void) }));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    // Each call is an Effect of whatever the test's mock resolves to.
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onFamily: (cb: (w: unknown) => void) => {
    family.onWarning = cb;
    return () => { family.onWarning = null; };
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
}));

import { FamiliesPanel } from '../FamiliesPanel';
import { PluginSettingsPanel, GameSettingsPanel } from '../PluginSettings';
import { FamilyWarningHost } from '../FamilyWarningHost';
import { LibraryView } from '../LibraryView';
import { TooltipProvider } from '../ds/tooltip';

const plugin = (o: Partial<StorePlugin> = {}): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: ['family', 'settings'], declared: ['family', 'settings'], ...o }) as StorePlugin;

const game = (o: Partial<Game> = {}): Game =>
  ({ id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', is_installed: true, settings: {}, features: ['settings'], ...o }) as Game;

const view = (o: Partial<SettingsView> = {}): SettingsView => ({
  groups: [{
    id: 'overlay', label: 'Overlay', description: 'Applied when Echo launches a game.',
    fields: [
      { id: 'eos', kind: 'bool', label: 'Enable EOS overlay', scope: 'plugin', default: false },
      { id: 'region', kind: 'select', label: 'Region', scope: 'plugin', default: 'eu', options: [{ id: 'eu', label: 'Europe' }, { id: 'us', label: 'United States' }] },
      { id: 'note', kind: 'text', label: 'Launch note', scope: 'plugin', default: '', placeholder: '-windowed' },
      { id: 'volume', kind: 'number', label: 'Volume', scope: 'plugin', default: 5, min: 0, max: 10 },
    ],
  }],
  values: {},
  ...o,
});

beforeEach(() => {
  vi.clearAllMocks();
  api.getFamilyMembers.mockResolvedValue([]);
  api.getPluginSettings.mockResolvedValue(view());
  api.savePluginSettings.mockResolvedValue({ applied: true });
  api.getGameSettingsFields.mockResolvedValue({ groups: [], values: {} });
  api.saveGameSettingsFields.mockResolvedValue({ applied: true });
});

afterEach(() => {
  vi.useRealTimers();
});

describe('Families', () => {
  it('lists the members with their role and the state the store reports', async () => {
    api.getFamilyMembers.mockResolvedValue([
      { id: 'ana', name: 'Ana', role: 'adult', avatar_url: '', state: 'online' },
      { id: 'ben', name: 'Ben', role: 'child', state: 'offline' },
    ]);
    render(<FamiliesPanel plugin={plugin()} />);
    const list = await screen.findByRole('list', { name: 'Family members' });
    expect(list.textContent).toContain('Ana');
    expect(list.textContent).toContain('adult');
    expect(list.textContent).toContain('online');
    expect(list.textContent).toContain('child');
    expect(api.getFamilyMembers).toHaveBeenCalledWith('echo');
  });

  it('is not shown, and asks nothing, for a plugin without family', () => {
    render(<FamiliesPanel plugin={plugin({ capabilities: ['settings'], declared: ['settings'] })} />);
    expect(screen.queryByLabelText('Families')).toBeNull();
    expect(api.getFamilyMembers).not.toHaveBeenCalled();
  });

  it('says why when the family cannot be read', async () => {
    api.getFamilyMembers.mockRejectedValue(new Error('the store is down'));
    render(<FamiliesPanel plugin={plugin()} />);
    expect(await screen.findByRole('alert')).toHaveTextContent('the store is down');
  });
});

describe('plugin settings', () => {
  it('renders the fields of each kind, with the saved value or the default', async () => {
    api.getPluginSettings.mockResolvedValue(view({ values: { eos: true } }));
    render(<PluginSettingsPanel plugin={plugin()} />);
    expect(await screen.findByRole('switch', { name: 'Enable EOS overlay' })).toHaveAttribute('aria-checked', 'true');
    expect(screen.getByText('Europe')).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Launch note' })).toHaveAttribute('placeholder', '-windowed');
    expect(screen.getByRole('spinbutton', { name: 'Volume' })).toHaveValue(5);
    expect(screen.getByText('Applied when Echo launches a game.')).toBeInTheDocument();
  });

  it('saves only the field that changed; the defaults of the others are not user choices', async () => {
    render(<PluginSettingsPanel plugin={plugin()} />);
    fireEvent.click(await screen.findByRole('switch', { name: 'Enable EOS overlay' }));
    await waitFor(() => expect(api.savePluginSettings).toHaveBeenCalledTimes(1));
    expect(api.savePluginSettings).toHaveBeenCalledWith('echo', { eos: true });
  });

  it('runs the saves of a scope one after the other, and the last change wins', async () => {
    let finishFirst: (s: SettingsSaved) => void = () => {};
    api.savePluginSettings
      .mockImplementationOnce(() => new Promise<SettingsSaved>((resolve) => { finishFirst = resolve; }))
      .mockResolvedValue({ applied: true });
    render(<PluginSettingsPanel plugin={plugin()} />);
    fireEvent.click(await screen.findByRole('switch', { name: 'Enable EOS overlay' }));
    fireEvent.click(screen.getByRole('switch', { name: 'Enable EOS overlay' }));
    await waitFor(() => expect(api.savePluginSettings).toHaveBeenCalledTimes(1));
    finishFirst({ applied: true });
    await waitFor(() => expect(api.savePluginSettings).toHaveBeenCalledTimes(2));
    expect(api.savePluginSettings).toHaveBeenLastCalledWith('echo', { eos: false });
  });

  it('after a failed save the form shows what the backend holds', async () => {
    api.savePluginSettings.mockRejectedValue(new Error('store is down'));
    api.getPluginSettings.mockResolvedValueOnce(view()).mockResolvedValueOnce(view({ values: { eos: false, volume: 3 } }));
    render(<PluginSettingsPanel plugin={plugin()} />);
    fireEvent.click(await screen.findByRole('switch', { name: 'Enable EOS overlay' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('store is down');
    await waitFor(() => expect(api.getPluginSettings).toHaveBeenCalledTimes(2));
    expect(await screen.findByRole('spinbutton', { name: 'Volume' })).toHaveValue(3);
  });

  it('keeps a value the plugin did not take, and says so', async () => {
    api.savePluginSettings.mockResolvedValue({ applied: false, message: "saved, but Echo didn't take them: Echo can't take settings right now" });
    render(<PluginSettingsPanel plugin={plugin()} />);
    fireEvent.click(await screen.findByRole('switch', { name: 'Enable EOS overlay' }));
    expect(await screen.findByRole('status')).toHaveTextContent("Echo didn't take them");
    expect(screen.getByRole('switch', { name: 'Enable EOS overlay' })).toHaveAttribute('aria-checked', 'true');
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('commits a number when the box is left, not on every key', async () => {
    render(<PluginSettingsPanel plugin={plugin()} />);
    const volume = await screen.findByRole('spinbutton', { name: 'Volume' });
    fireEvent.change(volume, { target: { value: '7' } });
    expect(api.savePluginSettings).not.toHaveBeenCalled();
    fireEvent.blur(volume);
    await waitFor(() => expect(api.savePluginSettings).toHaveBeenCalledWith('echo', expect.objectContaining({ volume: 7 })));
  });

  it('puts a refused value back and says why', async () => {
    api.savePluginSettings.mockRejectedValue(new Error('volume: number is out of range'));
    render(<PluginSettingsPanel plugin={plugin()} />);
    const eos = await screen.findByRole('switch', { name: 'Enable EOS overlay' });
    fireEvent.click(eos);
    expect(await screen.findByRole('alert')).toHaveTextContent('volume: number is out of range');
    expect(screen.getByRole('switch', { name: 'Enable EOS overlay' })).toHaveAttribute('aria-checked', 'false');
  });

  it('is not shown for a plugin that does not offer settings', () => {
    render(<PluginSettingsPanel plugin={plugin({ capabilities: ['family'], declared: ['family'] })} />);
    expect(api.getPluginSettings).not.toHaveBeenCalled();
  });
});

describe('game settings', () => {
  const perGame: SettingsView = {
    groups: [{ id: 'per-game', label: 'Per game', fields: [{ id: 'game-overlay', kind: 'bool', label: 'Overlay for this game', scope: 'game', default: true }] }],
    values: {},
  };

  it('shows the game-scope fields on a game whose features include settings', async () => {
    api.getGameSettingsFields.mockResolvedValue(perGame);
    render(<GameSettingsPanel game={game()} plugin={plugin()} />);
    expect(await screen.findByRole('switch', { name: 'Overlay for this game' })).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(screen.getByRole('switch', { name: 'Overlay for this game' }));
    await waitFor(() => expect(api.saveGameSettingsFields).toHaveBeenCalledWith(expect.objectContaining({ id: 'p.echo.tiny' }), { 'game-overlay': false }));
  });

  it('is not shown when the game does not offer settings', () => {
    render(<GameSettingsPanel game={game({ features: ['install'] })} plugin={plugin()} />);
    expect(api.getGameSettingsFields).not.toHaveBeenCalled();
  });

  it('shows nothing when the plugin has no game-scope field', async () => {
    const { container } = render(<GameSettingsPanel game={game()} plugin={plugin()} />);
    await waitFor(() => expect(api.getGameSettingsFields).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });
});

describe('family warning toast', () => {
  const warning = (o: Partial<FamilyWarning> = {}): FamilyWarning =>
    ({ source: 'echo', game_id: 'p.echo.ana', message: 'Ana wants to play this game.', seconds_left: 3, ...o });
  const games = [game({ id: 'p.echo.ana', title: 'Echo Borrowed Quest' })];

  it('counts down and leaves when the time is up', () => {
    vi.useFakeTimers();
    render(<FamilyWarningHost games={games} />);
    act(() => family.onWarning?.(warning()));
    expect(screen.getByRole('alert')).toHaveTextContent('Echo Borrowed Quest');
    expect(screen.getByRole('alert')).toHaveTextContent('Ana wants to play this game.');
    expect(screen.getByText('3 s left')).toBeInTheDocument();
    // One second at a time: each tick is a state change the next timer waits on.
    act(() => { vi.advanceTimersByTime(1000); });
    expect(screen.getByText('2 s left')).toBeInTheDocument();
    act(() => { vi.advanceTimersByTime(1000); });
    expect(screen.getByText('1 s left')).toBeInTheDocument();
    act(() => { vi.advanceTimersByTime(1000); });
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('stays without a countdown when the plugin gave no time', () => {
    vi.useFakeTimers();
    render(<FamilyWarningHost games={games} />);
    act(() => family.onWarning?.(warning({ seconds_left: 0 })));
    act(() => { vi.advanceTimersByTime(10000); });
    expect(screen.getByRole('alert')).toHaveTextContent('Ana wants to play this game.');
    expect(screen.queryByText(/s left/)).toBeNull();
  });

  it('leaves by itself after a while when it has no countdown', () => {
    vi.useFakeTimers();
    render(<FamilyWarningHost games={games} />);
    act(() => family.onWarning?.(warning({ seconds_left: 0 })));
    act(() => { vi.advanceTimersByTime(15000); });
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('is not shown for a game that is not in the library', () => {
    render(<FamilyWarningHost games={games} />);
    act(() => family.onWarning?.(warning({ game_id: 'p.echo.gone', seconds_left: 0 })));
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('can be dismissed', () => {
    render(<FamilyWarningHost games={games} />);
    act(() => family.onWarning?.(warning({ seconds_left: 0 })));
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

describe('library family filter and badge', () => {
  const mk = (id: string, o: Partial<Game> = {}): Game =>
    ({ id, source: 'echo', store: 'echo', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

  const libraryView = (games: Game[]) => (
    <LibraryView games={games} runners={[]} activity={[]} search="" onSearch={() => {}} source="all" onSource={() => {}} mode="cards" onMode={() => {}}
      ownership="owned" onOwnership={() => {}} warnings={[]} plugins={[]} jobs={[]} lastScan={null} onOpenGame={() => {}} onAddGame={() => {}} onNavigate={() => {}} />
  );
  const library = (games: Game[]) => render(libraryView(games), { wrapper: TooltipProvider });

  it('marks borrowed games and filters to the ones shared with the user', () => {
    library([mk('own'), mk('lent', { shared_by: { id: 'ana', name: 'Ana' } })]);
    expect(screen.getByText('Shared by Ana')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^Filters/ }));
    fireEvent.click(screen.getByRole('radio', { name: 'shared with me' }));
    expect(screen.queryByText('own')).toBeNull();
    expect(screen.getAllByText('lent').length).toBeGreaterThan(0);
  });

  it('has no family filter when nothing is borrowed', () => {
    library([mk('own')]);
    expect(screen.queryByRole('radiogroup', { name: 'Family' })).toBeNull();
  });

  it('does not keep the shared filter once no game is borrowed', () => {
    const r = library([mk('own'), mk('lent', { shared_by: { id: 'ana', name: 'Ana' } })]);
    fireEvent.click(screen.getByRole('button', { name: /^Filters/ }));
    fireEvent.click(screen.getByRole('radio', { name: 'shared with me' }));
    expect(screen.queryByText('own')).toBeNull();
    r.rerender(libraryView([mk('own')]));
    expect(screen.queryByRole('radiogroup', { name: 'Family' })).toBeNull();
    expect(screen.getAllByText('own').length).toBeGreaterThan(0);
  });
});
