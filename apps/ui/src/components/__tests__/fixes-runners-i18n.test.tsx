import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { FixesView } from '../FixesView';
import { SystemView } from '../SystemView';
import type { Game, ProtonVersion } from '../../types';

const games = [
  {
    id: 'p.echo.tiny',
    title: 'Echo Tiny',
    store: 'echo',
    fix_info: { tier: 'bronze', source: 'protondb', description: 'Needs tweaks.' },
  },
] as unknown as Game[];

const runners = [
  { id: 'proton-9', name: 'Proton 9', path: '/home/u/.steam/root/compatibilitytools.d/Proton-9', is_custom: true, is_system: false },
] as unknown as ProtonVersion[];

describe('fixes and runners live language switching', () => {
  it('re-renders the fixes view in French without a reload', async () => {
    render(<FixesView games={games} onOpenGame={() => {}} />);
    expect(screen.getByText(/Curated recipes/)).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'needs attention' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'all rated' })).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByText(/Recettes préparées/)).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'à surveiller' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'tous notés' })).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('radio', { name: 'needs attention' })).toBeInTheDocument();
  });

  it('re-renders the fixes empty state in French without a reload', async () => {
    render(<FixesView games={[]} onOpenGame={() => {}} />);
    expect(screen.getByText('no fix data yet')).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByText('pas encore de données de compatibilité')).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByText('no fix data yet')).toBeInTheDocument();
  });

  it('re-renders the system view in French without a reload', async () => {
    const { unmount } = render(<SystemView runners={[]} onRefresh={() => {}} scanning={false} />);
    expect(screen.getByText('no runners')).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByText('aucun runner')).toBeInTheDocument();
    unmount();

    render(<SystemView runners={runners} onRefresh={() => {}} scanning={false} />);
    expect(screen.getByText('personnalisé')).toBeInTheDocument();
    expect(screen.getByText('trouvé')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Réanalyser/ })).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByText('custom')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Rescan/ })).toBeInTheDocument();
  });
});

describe('classic fixes attention signals (issue #144)', () => {
  const signal = (id: string, o: object) =>
    ({ id, title: id, store: 'echo', cover_url: 'https://example.com/cover.jpg', install_dir: '/games/test', is_installed: true, settings: {}, ...o }) as unknown as Game;

  it('lists a save conflict with its reason and opens the game', async () => {
    await i18n.changeLanguage('en');
    const onOpenGame = vi.fn();
    render(<FixesView games={[signal('conflicted', { has_save_conflict: true })]} onOpenGame={onOpenGame} />);
    expect(screen.getByText(/Saves changed on this device and in the cloud/)).toBeInTheDocument();
    fireEvent.click(screen.getByText('conflicted').closest('button')!);
    expect(onOpenGame).toHaveBeenCalledWith('conflicted');
  });

  it('lists a damaged prefix with its reason and opens the game', async () => {
    await i18n.changeLanguage('en');
    const onOpenGame = vi.fn();
    render(<FixesView games={[signal('broken', { prefix_damaged: true })]} onOpenGame={onOpenGame} />);
    expect(screen.getByText(/The Wine prefix looks damaged/)).toBeInTheDocument();
    fireEvent.click(screen.getByText('broken').closest('button')!);
    expect(onOpenGame).toHaveBeenCalledWith('broken');
  });
});
