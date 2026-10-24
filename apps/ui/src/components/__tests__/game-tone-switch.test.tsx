import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({ getLauncherSettings: vi.fn(), setGameTone: vi.fn() }));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
}));

// The theme's color.game-tone axis: what the switch reads (lib/theme.ts).
const axis = vi.hoisted(() => ({ support: 'whole' as string }));
vi.mock('../../lib/theme', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib/theme')>();
  const { VARIANT_DEFAULTS } = await import('../../gen/catalog');
  return {
    ...actual,
    useVariant: (name: keyof typeof VARIANT_DEFAULTS) =>
      name === 'color.game-tone' ? axis.support : VARIANT_DEFAULTS[name],
  };
});

import { GameToneSwitch } from '../GameToneSwitch';
import { useGameTone } from '../../lib/gameTone';

// App owns the hook and hands the tone to the switch.
function Harness() {
  return <GameToneSwitch tone={useGameTone(null)} />;
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getLauncherSettings.mockReturnValue(Effect.succeed({ game_tone: '' }));
  api.setGameTone.mockReturnValue(Effect.succeed({ game_tone: 'whole' }));
  axis.support = 'whole';
  document.documentElement.style.cssText = '';
});

afterEach(() => {
  document.documentElement.style.cssText = '';
  document.documentElement.className = '';
});

describe('GameToneSwitch', () => {
  it('offers Accent and Whole app where the theme supports both, starting on Accent', async () => {
    const { getByRole } = render(<Harness />);
    const accent = getByRole('radio', { name: 'Accent' });
    await waitFor(() => expect(accent).toHaveAttribute('aria-checked', 'true'));
    expect(getByRole('radio', { name: 'Whole app' })).toHaveAttribute('aria-checked', 'false');
    expect(getByRole('radiogroup', { name: 'Game colour' })).toBeInTheDocument();
  });

  it('keeps the stored choice across a mount', async () => {
    api.getLauncherSettings.mockReturnValue(Effect.succeed({ game_tone: 'whole' }));
    const { getByRole } = render(<Harness />);
    await waitFor(() => expect(getByRole('radio', { name: 'Whole app' })).toHaveAttribute('aria-checked', 'true'));
    expect(getByRole('radio', { name: 'Accent' })).toHaveAttribute('aria-checked', 'false');
  });

  it('stores the pick as a launcher setting and shows it at once', async () => {
    const { getByRole } = render(<Harness />);
    await waitFor(() => expect(getByRole('radio', { name: 'Accent' })).toHaveAttribute('aria-checked', 'true'));
    fireEvent.click(getByRole('radio', { name: 'Whole app' }));
    expect(getByRole('radio', { name: 'Whole app' })).toHaveAttribute('aria-checked', 'true');
    await waitFor(() => expect(api.setGameTone).toHaveBeenCalledWith('whole'));
  });

  it('is hidden on a theme that takes only the accent, or none of it', () => {
    axis.support = 'accent';
    expect(render(<Harness />).container).toBeEmptyDOMElement();
    axis.support = 'off';
    expect(render(<Harness />).container).toBeEmptyDOMElement();
  });
});
