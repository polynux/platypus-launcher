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
});
