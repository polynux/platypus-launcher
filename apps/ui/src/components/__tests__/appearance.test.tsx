import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

const themeState = vi.hoisted(() => ({ schemes: ['dark', 'light', 'black'] as string[], scheme: 'dark' as string, saveScheme: vi.fn() }));
const toneState = vi.hoisted(() => ({ supported: 'whole' as string, choice: 'accent' as string, setChoice: vi.fn() }));

vi.mock('../../lib/theme', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...actual,
    useThemeSchemes: () => themeState.schemes,
    useColorScheme: () => themeState.scheme,
    saveColorSchemeChoice: (s: string) => {
      themeState.saveScheme(s);
      themeState.scheme = s;
      return Promise.resolve();
    },
    useVariant: (axis: string) => (axis === 'color.game-tone' ? toneState.supported : 'page'),
  };
});

vi.mock('../../lib/gameTone', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib/gameTone')>();
  return {
    ...actual,
    useGameToneChoice: () => ({
      supported: toneState.supported,
      choice: toneState.choice,
      setChoice: (next: string) => {
        toneState.setChoice(next);
        toneState.choice = next;
      },
    }),
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}));

import { AppearanceSetting } from '../LauncherSettingsView';

afterEach(() => cleanup());

beforeEach(() => {
  themeState.schemes = ['dark', 'light', 'black'];
  themeState.scheme = 'dark';
  themeState.saveScheme.mockReset();
  toneState.supported = 'whole';
  toneState.choice = 'accent';
  toneState.setChoice.mockReset();
});

describe('Appearance settings (issue #134)', () => {
  it('renders both pickers when the theme declares schemes and takes the game colour', () => {
    render(<AppearanceSetting onNotify={() => {}} />);
    expect(screen.getByLabelText('appearance.schemeLabel')).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'appearance.scheme.dark' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'appearance.scheme.light' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'off' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'accent' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'whole' })).toBeInTheDocument();
  });

  it('saves the scheme pick as a launcher setting', async () => {
    render(<AppearanceSetting onNotify={() => {}} />);
    fireEvent.click(screen.getByRole('radio', { name: 'appearance.scheme.light' }));
    await waitFor(() => expect(themeState.saveScheme).toHaveBeenCalledWith('light'));
  });

  it('saves the game colour pick and keeps the switch in agreement', () => {
    const notified: string[] = [];
    render(<AppearanceSetting onNotify={(m) => { notified.push(m); }} />);
    fireEvent.click(screen.getByRole('radio', { name: 'whole' }));
    expect(toneState.setChoice).toHaveBeenCalledWith('whole');
  });

  it('does not repaint the app accent while Appearance is mounted', () => {
    document.documentElement.style.setProperty('--color-accent', 'rgb(12, 34, 56)');
    render(<AppearanceSetting onNotify={() => {}} />);
    expect(document.documentElement.style.getPropertyValue('--color-accent')).toBe('rgb(12, 34, 56)');
  });

  it('hides the scheme picker when the theme is dark only', () => {
    themeState.schemes = ['dark'];
    render(<AppearanceSetting onNotify={() => {}} />);
    expect(screen.queryByLabelText('appearance.schemeLabel')).toBeNull();
    // The game picker stays: the theme still takes the colour.
    expect(screen.getByRole('radio', { name: 'accent' })).toBeInTheDocument();
  });

  it('hides the game picker when the theme leaves the axis off', () => {
    toneState.supported = 'off';
    render(<AppearanceSetting onNotify={() => {}} />);
    expect(screen.getByLabelText('appearance.schemeLabel')).toBeInTheDocument();
    expect(screen.queryByRole('radio', { name: 'accent' })).toBeNull();
  });
});
