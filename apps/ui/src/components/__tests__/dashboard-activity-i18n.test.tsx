import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render as rtlRender, screen } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { Dashboard } from '../Dashboard';
import { ActivityFeed, ActivityPanel } from '../ActivityFeed';
import { MigrationNotice } from '../MigrationNotice';
import { TooltipProvider } from '../ds/tooltip';
import type { ActivityEvent, Game } from '../../types';

vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const dashProps = (games: Game[]) => ({
  games, runners: [], activity: [], dismissedHero: new Set<string>(), onDismissHero: () => {}, onOpenGame: () => {}, onNavigate: () => {},
});

describe('dashboard and activity live language switching', () => {
  it('re-renders French without a reload', async () => {
    render(<Dashboard {...dashProps([mk('a'), mk('c', { store: 'custom', title: 'Gem' })])} ownedCount={5} />);
    expect(screen.getByText('Your games')).toBeTruthy();
    expect(screen.getByText(/2 installed · 5 owned/)).toBeTruthy();
    expect(screen.getByText('Recently added')).toBeTruthy();
    expect(screen.getByText('Waiting on you')).toBeTruthy();
    expect(screen.getByText('Activity')).toBeTruthy();

    await i18n.changeLanguage('fr');
    expect(screen.getByText('Vos jeux')).toBeTruthy();
    expect(screen.getByText(/2 jeux installés · 5 en tout/)).toBeTruthy();
    expect(screen.getByText('Ajoutés récemment')).toBeTruthy();
    expect(screen.getByText('En attente de vous')).toBeTruthy();
    expect(screen.getByText('Activité')).toBeTruthy();

    await i18n.changeLanguage('en');
    expect(screen.getByText('Your games')).toBeTruthy();
    expect(screen.getByText(/2 installed · 5 owned/)).toBeTruthy();
  });

  it('translates activity rows and the empty feed', async () => {
    const events = [{ id: 1, ts: Date.now(), type: 'download', detail: 'installed Gem', meta: 'echo' }] as ActivityEvent[];
    render(<ActivityPanel events={events} onFullLog={() => {}} />);
    expect(screen.getByText('download')).toBeTruthy();
    expect(screen.getByText('full log →')).toBeTruthy();

    await i18n.changeLanguage('fr');
    expect(screen.getByText('téléchargement')).toBeTruthy();
    expect(screen.getByText('journal complet →')).toBeTruthy();

    await i18n.changeLanguage('en');
    expect(screen.getByText('download')).toBeTruthy();
  });

  it('translates the migration notice', async () => {
    const notice = { moved: true, copied: false, set_aside: ['echo'], warnings: [] };
    render(<MigrationNotice notice={notice as never} onOpenPlugins={() => {}} onDismiss={() => {}} />);
    expect(screen.getByRole('status', { name: /hydra to platypus/i })).toBeTruthy();
    expect(screen.getByText(/moved from the old hydra folder/i)).toBeTruthy();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('status', { name: /hydra vers platypus/i })).toBeTruthy();
    expect(screen.getByText(/déplacées de l'ancien dossier hydra/i)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Ignorer' })).toBeTruthy();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('button', { name: 'Dismiss' })).toBeTruthy();
  });
});
