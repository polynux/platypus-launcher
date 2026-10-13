import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const getLauncherSettings = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLauncherSettings: () => getLauncherSettings(),
    setLanguage: vi.fn(),
  },
}));

import { LanguageSetting } from '../LauncherSettingsView';

describe('Language setting', () => {
  beforeEach(() => getLauncherSettings.mockReset());

  // Radix Select shows nothing for an empty value: System ("" in the setting)
  // must still read "System" in the closed picker.
  it('shows System in the closed picker when no language is set', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed({ language: '' }));
    const { getByRole } = render(<LanguageSetting onNotify={vi.fn()} />);
    const trigger = getByRole('combobox', { name: 'Language' });
    await waitFor(() => expect(trigger.textContent).toContain('System'));
  });

  it('shows the chosen language', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed({ language: 'fr' }));
    const { getByRole } = render(<LanguageSetting onNotify={vi.fn()} />);
    await waitFor(() => expect(getByRole('combobox', { name: 'Language' }).textContent).toContain('Français'));
  });
});
