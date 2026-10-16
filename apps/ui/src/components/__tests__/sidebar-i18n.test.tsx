import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { Sidebar } from '../Sidebar';

const props = {
  view: { name: 'library' as const },
  onNavigate: () => {},
  search: '',
  onSearch: () => {},
  storeCounts: {},
  onPickSource: () => {},
  gameCount: 3,
  downloadCount: 0,
  authNeeded: false,
  fixCount: 0,
  runnerCount: 0,
  lastScan: null,
  sessions: [],
  onStop: () => {},
  friends: [],
  friendLabels: {},
  onOpenFriends: () => {},
  canRedeem: false,
  onRedeem: () => {},
};

describe('sidebar live language switching', () => {
  it('re-renders French without a reload', async () => {
    render(<Sidebar {...props} />);
    expect(screen.getByRole('navigation', { name: 'Main' }).textContent).toMatch(/Library/);
    await i18n.changeLanguage('fr');
    expect(screen.getByRole('navigation', { name: 'Principal' }).textContent).toMatch(/Bibliothèque/);
    expect(screen.getByPlaceholderText('Rechercher…')).toBeDefined();
    await i18n.changeLanguage('en');
    expect(screen.getByPlaceholderText('Search…')).toBeDefined();
  });

  it('labels every view a game page can return to, in English and French (#71)', async () => {
    // App passes t(`nav:${view.from}`) as the game page back link, where from
    // is one of these: each must be a translated sidebar string, never the id.
    for (const from of ['dashboard', 'library', 'fixes']) {
      await i18n.changeLanguage('en');
      const en = i18n.t(`nav:${from}`);
      await i18n.changeLanguage('fr');
      const fr = i18n.t(`nav:${from}`);
      expect(en).not.toBe(from);
      expect(fr).not.toBe(from);
      expect(fr).not.toBe(en);
    }
    await i18n.changeLanguage('en');
  });
});
