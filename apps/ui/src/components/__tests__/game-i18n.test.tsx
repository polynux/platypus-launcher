import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { PlayButton } from '../PlayButton';
import { CompatChip, DeckBadge } from '../CompatBadges';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

const noop = () => {};
const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;
const pdb = (tier: string) => [{ kind: 'rating', label: tier[0].toUpperCase() + tier.slice(1), tone: tier, source: 'ProtonDB', checked_at: 0 }] as Game['compat_badges'];

describe('game area live language switching', () => {
  it('re-renders the play button in French without a reload', async () => {
    render(<PlayButton launching={false} disabled={false} onPlay={noop} onStop={noop} onForceStop={noop} />);
    expect(screen.getByRole('button', { name: 'Play' })).toBeTruthy();
    await i18n.changeLanguage('fr');
    expect(screen.getByRole('button', { name: 'Jouer' })).toBeTruthy();
    await i18n.changeLanguage('en');
    expect(screen.getByRole('button', { name: 'Play' })).toBeTruthy();
  });

  it('re-renders compat chips in French without a reload', async () => {
    const { unmount } = render(<CompatChip game={mk('a', { compat_badges: pdb('platinum') })} />, { wrapper: TooltipProvider });
    expect(screen.getByText('ProtonDB · Platinum')).toBeTruthy();
    await i18n.changeLanguage('fr');
    expect(screen.getByText('ProtonDB · Platine')).toBeTruthy();
    unmount();
    const { unmount: unmount2 } = render(<DeckBadge rating="verified" />, { wrapper: TooltipProvider });
    expect(screen.getByText('Deck · Vérifié')).toBeTruthy();
    unmount2();
    await i18n.changeLanguage('en');
    const { unmount: unmount3 } = render(<CompatChip game={mk('a', { compat_badges: pdb('platinum') })} />, { wrapper: TooltipProvider });
    expect(screen.getByText('ProtonDB · Platinum')).toBeTruthy();
    unmount3();
  });
});
