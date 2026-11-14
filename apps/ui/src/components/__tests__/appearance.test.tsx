import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

const themeState = vi.hoisted(() => ({ schemes: ['dark', 'light', 'black'] as string[], scheme: 'dark' as string, saveScheme: vi.fn(), accent: '#9db0ff' }));
const toneState = vi.hoisted(() => ({ supported: 'whole' as string, choice: 'accent' as string, setChoice: vi.fn() }));
const accentState = vi.hoisted(() => ({ value: '', set: vi.fn(), save: vi.fn(async () => {}) }));

vi.mock('../../lib/theme', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...actual,
    useThemeSchemes: () => themeState.schemes,
    useColorScheme: () => themeState.scheme,
    currentThemeVariables: () => ({ '--color-accent': themeState.accent }),
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
    useDefaultAccent: () => [accentState.value, (next: string) => { accentState.value = next; accentState.set(next); }, accentState.save],
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}));

import { AppearanceSetting } from '../LauncherSettingsView';

afterEach(() => { cleanup(); vi.useRealTimers(); });

beforeEach(() => {
  themeState.schemes = ['dark', 'light', 'black'];
  themeState.scheme = 'dark';
  themeState.accent = '#9db0ff';
  themeState.saveScheme.mockReset();
  toneState.supported = 'whole';
  toneState.choice = 'accent';
  toneState.setChoice.mockReset();
  accentState.value = '';
  accentState.set.mockReset();
  accentState.save.mockReset();
});

describe('Appearance settings (issue #134)', () => {
  it('shows the theme accent in the default swatch while a custom accent is active', () => {
    accentState.value = '#D34F68';
    render(<AppearanceSetting onNotify={() => {}} />);
    expect(screen.getByRole('button', { name: 'appearance.accent.themeDefault' }).style.backgroundColor).toBe('rgb(157, 176, 255)');
  });

  it('offers the theme accent first and saves a custom colour when the picker closes', () => {
    const view = render(<AppearanceSetting onNotify={() => {}} />);
    expect(screen.getByRole('group', { name: 'appearance.accentLabel' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'appearance.accent.themeDefault' }));
    expect(accentState.save).toHaveBeenCalledWith('', '');
    const picker = screen.getByLabelText('appearance.accent.custom');
    fireEvent.change(picker, { target: { value: '#9944CC' } });
    expect(accentState.set).toHaveBeenCalledWith('#9944CC');
    expect(accentState.save).toHaveBeenCalledTimes(1);
    view.rerender(<AppearanceSetting onNotify={() => {}} />);
    fireEvent.blur(picker);
    expect(accentState.save).toHaveBeenLastCalledWith('#9944CC', '');
  });
  it('debounces custom colour persistence while dragging the picker', async () => {
    vi.useFakeTimers();
    render(<AppearanceSetting onNotify={() => {}} />);
    fireEvent.change(screen.getByLabelText('appearance.accent.custom'), { target: { value: '#9944CC' } });
    expect(accentState.save).not.toHaveBeenCalled();
    await act(async () => { vi.advanceTimersByTime(299); });
    expect(accentState.save).not.toHaveBeenCalled();
    await act(async () => { vi.advanceTimersByTime(1); });
    expect(accentState.save).toHaveBeenCalledWith('#9944CC', '');
  });
  it('flushes a pending custom colour save when Appearance unmounts', () => {
    const view = render(<AppearanceSetting onNotify={() => {}} />);
    fireEvent.change(screen.getByLabelText('appearance.accent.custom'), { target: { value: '#9944CC' } });
    expect(accentState.save).not.toHaveBeenCalled();
    view.unmount();
    expect(accentState.save).toHaveBeenCalledWith('#9944CC', '');
  });
  it('shows save errors for default accent changes', async () => {
    const notify = vi.fn();
    accentState.save.mockRejectedValueOnce(new Error('settings read-only'));
    render(<AppearanceSetting onNotify={notify} />);
    fireEvent.click(screen.getByRole('button', { name: 'appearance.accent.rose' }));
    await waitFor(() => expect(notify).toHaveBeenCalledWith(expect.any(String), 'err'));
  });
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
