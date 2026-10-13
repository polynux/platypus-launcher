import { describe, it, expect } from 'vitest';
import i18n from '../../lib/i18n';
import { runnerDisplayName } from '../../lib/settings';

const umu = { id: 'umu', name: 'Proton via umu' };
const proton = { id: 'steam:Proton 11', name: 'Proton 11' };

// Issue #48: the umu runner option is labelled in the UI language.
describe('umu runner option label', () => {
  it('labels the umu option in English and French, others keep the core name', async () => {
    await i18n.changeLanguage('en');
    expect(runnerDisplayName(i18n.t, umu)).toBe('Proton via umu (recommended)');
    expect(runnerDisplayName(i18n.t, proton)).toBe('Proton 11');

    await i18n.changeLanguage('fr');
    expect(runnerDisplayName(i18n.t, umu)).toBe('Proton via umu (recommandé)');
    expect(runnerDisplayName(i18n.t, proton)).toBe('Proton 11');

    await i18n.changeLanguage('en');
    expect(runnerDisplayName(i18n.t, umu)).toBe('Proton via umu (recommended)');
  });
});
