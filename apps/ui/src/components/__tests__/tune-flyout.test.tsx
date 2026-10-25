import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Effect } from 'effect';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { axe } from 'vitest-axe';
import type { Game, GameSettings, SettingField, SettingGroup } from '../../types';

const api = vi.hoisted(() => ({ getRunnerOptions: vi.fn(), saveGameSettings: vi.fn() }));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
}));

import {
  TuneFlyout,
  applyTunePreset,
  gamescopeLine,
  toggleTune,
  tunePreset,
  tuneSwitches,
  type TunePreset,
} from '../TuneFlyout';

expect.extend(matchers);

const settings = (o: Partial<GameSettings> = {}): GameSettings => ({
  runner_id: 'auto',
  wine_prefix: '',
  launch_arguments: '',
  env_vars: {},
  use_gamemode: true,
  use_mangohud: false,
  use_gamescope: true,
  gamescope_w: 0,
  gamescope_h: 0,
  gamescope_r: 0,
  gamescope_args: '',
  eac_runtime: false,
  battleye_runtime: false,
  working_dir: '',
  ...o,
});

const game = (o: Partial<Game> = {}, s: Partial<GameSettings> = {}): Game =>
  ({
    id: 'g1',
    source: 'steam',
    store: 'steam',
    store_app_id: '1',
    title: 'Tuned Game',
    cover_url: '',
    banner_url: '',
    hero_url: '',
    install_dir: '/g',
    executable: '/g/game.exe',
    is_installed: true,
    features: [],
    settings: settings(s),
    compat_badges: [{ kind: 'rating', label: 'Gold', tone: 'gold', source: 'ProtonDB', checked_at: 0 }],
    ...o,
  }) as Game;

const field = (id: string): SettingField => ({ id, kind: 'select', label: id, scope: 'game', default: '' });
const groups = (): SettingGroup[] => [
  {
    id: 'runner',
    label: 'Runner',
    fields: [{ ...field('runner_id'), default: 'auto', options: [{ id: 'auto', label: 'Automatic' }, { id: 'ge', label: 'GE-Proton' }] }],
  },
  { id: 'graphics', label: 'Graphics', fields: [field('upscaler'), field('mangohud_preset')] },
  { id: 'wrappers', label: 'Wrappers', fields: [field('use_gamemode'), field('use_gamescope')] },
];

function mount(g: Game, onClose = vi.fn(), onAllSettings = vi.fn()) {
  return render(<TuneFlyout game={g} open onClose={onClose} onAllSettings={onAllSettings} />);
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getRunnerOptions.mockReturnValue(Effect.succeed(groups()));
  api.saveGameSettings.mockReturnValue(Effect.succeed(undefined));
});

afterEach(() => cleanup());

describe('the preset mapping', () => {
  const cases: [TunePreset, Partial<GameSettings>][] = [
    ['quality', { use_gamemode: true, use_gamescope: false, upscaler: 'native' }],
    ['balanced', { use_gamemode: true, use_gamescope: true, upscaler: 'native' }],
    ['performance', { use_gamemode: true, use_gamescope: true, upscaler: 'fsr1' }],
  ];

  it('reads each preset back from its switches', () => {
    for (const [id, s] of cases) expect(tunePreset(tuneSwitches(settings(s)))).toBe(id);
    expect(tunePreset(tuneSwitches(settings({ use_gamemode: false, use_gamescope: true, upscaler: 'native' })))).toBeNull();
    // A stale FSR under a lock that is off reads as the switch shows it: off.
    expect(tunePreset(tuneSwitches(settings({ use_gamemode: true, use_gamescope: false, upscaler: 'fsr1' })))).toBe('quality');
  });

  it('writes the three switches and leaves the FPS overlay alone', () => {
    for (const [id, s] of cases) {
      const next = applyTunePreset(settings({ mangohud_preset: 'full' }), id);
      expect(tuneSwitches(next)).toEqual(tuneSwitches(settings({ ...s, mangohud_preset: 'full' })));
      expect(next.mangohud_preset).toBe('full');
    }
  });

  it('counts FSR as off while the lock is off, and clears it when the lock goes', () => {
    expect(tuneSwitches(settings({ use_gamescope: false, upscaler: 'fsr1' })).fsr).toBe(false);
    expect(toggleTune(settings({ use_gamescope: true, upscaler: 'fsr1' }), 'gamescope').upscaler).toBe('native');
  });

  it('names the lock resolution when the game has one', () => {
    const t = (key: string, o?: Record<string, unknown>) => (o ? `${key} ${JSON.stringify(o)}` : key);
    expect(gamescopeLine(settings({ gamescope_resolution: '2560x1440', gamescope_rate: '165' }), t)).toBe('tune:gamescope.known {"resolution":"2560×1440","rate":"165"}');
    expect(gamescopeLine(settings({ gamescope_w: 1920, gamescope_h: 1080, gamescope_r: 120 }), t)).toBe('tune:gamescope.known {"resolution":"1920×1080","rate":"120"}');
    expect(gamescopeLine(settings(), t)).toBe('tune:gamescope.sub');
  });
});

describe('TuneFlyout', () => {
  it('focuses the dialog itself and marks the preset the switches match', async () => {
    const { getByRole } = mount(game({}, { use_gamemode: true, use_gamescope: true, upscaler: 'native' }));
    const balanced = getByRole('button', { name: 'Balanced' });
    expect(balanced).toHaveAttribute('aria-pressed', 'true');
    // A mouse open must not paint a focus ring on the first preset: focus
    // lands on the dialog root, and keyboard users tab into the controls.
    expect(document.activeElement).toBe(getByRole('dialog', { name: 'Quick settings for Tuned Game' }));
    expect(getByRole('dialog', { name: 'Quick settings for Tuned Game' })).toBeInTheDocument();
    await waitFor(() => expect(api.getRunnerOptions).toHaveBeenCalled());
  });

  it('saves a preset through the settings API and follows it', async () => {
    const { getByRole } = mount(game({}, { use_gamemode: true, use_gamescope: true, upscaler: 'native' }));
    fireEvent.click(getByRole('button', { name: 'Performance' }));
    expect(getByRole('button', { name: 'Performance' })).toHaveAttribute('aria-pressed', 'true');
    expect(getByRole('switch', { name: /Sharper upscaling/ })).toHaveAttribute('aria-checked', 'true');
    await waitFor(() => expect(api.saveGameSettings).toHaveBeenCalledWith('g1', expect.objectContaining({ use_gamemode: true, use_gamescope: true, upscaler: 'fsr1' })));
  });

  it('saves a switch through the settings API', async () => {
    const { getByRole } = mount(game());
    const mode = getByRole('switch', { name: /Performance mode/ });
    expect(mode).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(mode);
    expect(mode).toHaveAttribute('aria-checked', 'false');
    await waitFor(() => expect(api.saveGameSettings).toHaveBeenCalledWith('g1', expect.objectContaining({ use_gamemode: false })));
  });

  it('disables the FSR switch until Lock resolution is on, and says why', () => {
    const { getByRole } = mount(game({}, { use_gamemode: true, use_gamescope: false, upscaler: 'fsr1' }));
    const fsr = getByRole('switch', { name: /Sharper upscaling/ });
    expect(fsr).toBeDisabled();
    expect(fsr).toHaveAttribute('aria-checked', 'false');
    expect(fsr).toHaveTextContent('Turn on Lock resolution first');

    fireEvent.click(getByRole('switch', { name: /^Lock resolution/ }));
    expect(getByRole('switch', { name: /Sharper upscaling/ })).toBeEnabled();
  });

  it('disables a switch the runner does not offer, with the reason', () => {
    api.getRunnerOptions.mockReturnValue(Effect.succeed(groups().filter((g) => g.id !== 'graphics')));
    const { getByRole } = mount(game(), vi.fn(), vi.fn());
    return waitFor(() => {
      const overlay = getByRole('switch', { name: /FPS overlay/ });
      expect(overlay).toBeDisabled();
      expect(overlay).toHaveTextContent("This runner doesn't offer it");
    });
  });

  it('shows the ProtonDB tier and picks the runner through the schema', async () => {
    const { getByRole, getByText } = mount(game());
    expect(getByText('Gold on ProtonDB')).toBeInTheDocument();
    const pick = await waitFor(() => getByRole('combobox', { name: 'Compatibility layer' }));
    expect(pick).toHaveTextContent('Automatic');
  });

  it('picks the runner from the portaled list without closing', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const { getByRole } = mount(game(), onClose);
    await user.click(await waitFor(() => getByRole('combobox', { name: 'Compatibility layer' })));
    const option = await screen.findByRole('option', { name: 'GE-Proton' });
    // The pointer down on the portaled listbox is not an outside click.
    fireEvent.pointerDown(option);
    expect(onClose).not.toHaveBeenCalled();
    await user.click(option);
    await waitFor(() => expect(api.saveGameSettings).toHaveBeenCalledWith('g1', expect.objectContaining({ runner_id: 'ge' })));
    expect(onClose).not.toHaveBeenCalled();
  });

  it('returns focus to the Tune button on Esc and on an outside click', () => {
    const anchor = document.createElement('button');
    anchor.setAttribute('data-tune-anchor', '');
    document.body.appendChild(anchor);
    const onClose = vi.fn();
    mount(game(), onClose);
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(anchor);
    fireEvent.pointerDown(document.body);
    expect(onClose).toHaveBeenCalledTimes(2);
    expect(document.activeElement).toBe(anchor);
    anchor.remove();
  });

  it('says Steam chooses the compatibility layer for a client launch', () => {
    const { getByText, getByRole, queryByRole } = mount(game({ client_launch: true }));
    expect(getByText('chosen in Steam')).toBeInTheDocument();
    expect(queryByRole('combobox', { name: 'Compatibility layer' })).toBeNull();
    // The switches stay: they ride along in the Steam launch options.
    expect(getByRole('switch', { name: /Performance mode/ })).toBeInTheDocument();
  });

  it('closes on Escape and on a click outside, and not on a click inside or on the anchor', () => {
    const onClose = vi.fn();
    const { getByRole } = mount(game(), onClose);
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);

    fireEvent.pointerDown(getByRole('dialog'));
    expect(onClose).toHaveBeenCalledTimes(1);

    const outside = document.createElement('span');
    document.body.appendChild(outside);
    fireEvent.pointerDown(outside);
    expect(onClose).toHaveBeenCalledTimes(2);
    outside.remove();

    const anchor = document.createElement('button');
    anchor.setAttribute('data-tune-anchor', '');
    document.body.appendChild(anchor);
    fireEvent.pointerDown(anchor);
    expect(onClose).toHaveBeenCalledTimes(2);
    anchor.remove();
  });

  it('opens the game settings from "All settings", closing first', () => {
    const onClose = vi.fn();
    const onAllSettings = vi.fn();
    const { getByRole } = mount(game(), onClose, onAllSettings);
    fireEvent.click(getByRole('button', { name: /All settings/ }));
    expect(onAllSettings).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('has no accessibility violations', async () => {
    const { container } = mount(game());
    (expect(await axe(container)) as any).toHaveNoViolations();
  });

  it('has no accessibility violations with FSR locked out', async () => {
    const { container } = mount(game({}, { use_gamescope: false }));
    (expect(await axe(container)) as any).toHaveNoViolations();
  });
});
