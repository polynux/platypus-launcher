import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { LibraryView } from '../LibraryView';
import type { Game } from '../../types';

vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '/g', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const props = {
  runners: [],
  activity: [],
  search: '',
  onSearch: () => {},
  source: 'all' as const,
  onSource: () => {},
  mode: 'cards' as const,
  onMode: () => {},
  ownership: 'installed' as const,
  onOwnership: () => {},
  warnings: [],
  plugins: [],
  jobs: [],
  lastScan: null,
  onOpenGame: () => {},
  onAddGame: () => {},
  onNavigate: () => {},
};

describe('library live language switching', () => {
  it('re-renders French without a reload', async () => {
    render(<LibraryView games={[mk('Portal 2')]} {...props} />);
    expect(screen.getByRole('button', { name: 'Add game' })).toBeTruthy();
    expect(screen.getByText('All titles')).toBeTruthy();
    await i18n.changeLanguage('fr');
    expect(screen.getByRole('button', { name: 'Ajouter un jeu' })).toBeTruthy();
    expect(screen.getByText('Tous les jeux')).toBeTruthy();
    expect(screen.getByText('Jeux affichés : 1 · installés : 1')).toBeTruthy();
    await i18n.changeLanguage('en');
    expect(screen.getByRole('button', { name: 'Add game' })).toBeTruthy();
    expect(screen.getByText('All titles')).toBeTruthy();
  });
});
