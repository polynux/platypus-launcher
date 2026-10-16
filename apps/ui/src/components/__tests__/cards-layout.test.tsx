import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render as rtlRender } from '@testing-library/react';
import { Dashboard } from '../Dashboard';
import { LibraryView } from '../LibraryView';
import { CompatChip, DeckBadge } from '../CompatBadges';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;
const pdb = (tier: string) => ({ kind: 'rating', label: tier[0].toUpperCase() + tier.slice(1), tone: tier, source: 'ProtonDB', checked_at: 1791589136 });
const two = (tier: string, deck: string) => ({ compat_badges: [pdb(tier)], fix_info: { source: 'steamdeck', tier: '', description: '', winetricks: [], dll_overrides: {}, launch_args: '', deck, deck_notes: [] } } as Partial<Game>);

const dashGames = () => [
  mk('plain', { title: 'No badges' }),
  mk('one', { title: 'One badge', compat_badges: [pdb('gold')] }),
  mk('two', { title: 'Two badges', ...two('platinum', 'verified') }),
];

const dash = (games: Game[]) =>
  render(<Dashboard games={games} runners={[]} activity={[]} dismissedHero={new Set(games.map((g) => g.id))} onDismissHero={() => {}} onOpenGame={() => {}} onNavigate={() => {}} />);

const lib = (games: Game[]) =>
  render(
    <LibraryView games={games} runners={[]} activity={[]} search="" onSearch={() => {}} source="all" onSource={() => {}} mode="cards" onMode={() => {}}
      ownership="owned" onOwnership={() => {}} warnings={[]} plugins={[]} jobs={[]} lastScan={null} onOpenGame={() => {}} onAddGame={() => {}} onNavigate={() => {}} />
  );

describe('card layout (#61, #62)', () => {
  it('dashboard: cards keep equal heights with the text block top-aligned', () => {
    const { container } = dash(dashGames());
    const grid = container.querySelector('.cover-grid');
    expect(grid?.className).toContain('items-stretch');
    const cards = [...(container.querySelectorAll('.cover-grid > button') as NodeListOf<HTMLElement>)];
    expect(cards.length).toBe(3);
    for (const card of cards) {
      // The card fills its grid area (equal heights) and stacks art over text.
      expect(card.className).toContain('card-compat');
      expect(card.className).toContain('flex-col');
      expect(card.className).toContain('h-full');
      // The text block hugs the art: empty space stays at the bottom.
      const body = card.children[1] as HTMLElement;
      expect(body.className).toContain('justify-start');
      expect(body.className).toContain('flex-1');
      // The badge row reserves its height even with no badges.
      const slot = body.children[2] as HTMLElement;
      expect(slot.className).toMatch(/min-h-\[/);
    }
  });

  it('library: cards are top-aligned with a stable badge row', () => {
    const { container } = lib(dashGames());
    const cards = [...(container.querySelectorAll('.cover-grid-sm > button') as NodeListOf<HTMLElement>)];
    expect(cards.length).toBeGreaterThanOrEqual(3);
    for (const card of cards.slice(0, 3)) {
      expect(card.className).toContain('card-compat');
      expect(card.className).toContain('flex-col');
      expect(card.className).toContain('h-full');
      expect(card.className).toContain('min-w-0');
      const body = card.children[1] as HTMLElement;
      expect(body.className).toContain('justify-start');
      const slot = body.children[2] as HTMLElement;
      expect(slot.className).toMatch(/min-h-\[/);
      // The meta line truncates inside the card, with the whole line as tooltip.
      const meta = body.children[1] as HTMLElement;
      expect(meta.className).toContain('truncate');
      expect(meta.getAttribute('title')).toBeTruthy();
    }
  });

  it('badges wrap, never overflow, and carry a compact label for narrow cards', () => {
    const { getByText, container } = render(<CompatChip game={mk('x', { ...two('platinum', 'verified') })} />);
    const row = container.firstElementChild as HTMLElement;
    expect(row.className).toContain('flex-wrap');
    expect(row.className).toContain('max-w-full');
    expect(row.className).toContain('min-w-0');
    for (const badge of [...row.querySelectorAll('[data-slot="badge"]')] as HTMLElement[]) {
      // No fixed width that could spill past the card; ellipsis is the last resort.
      expect(badge.className).toContain('max-w-full');
      expect(badge.className).toContain('min-w-0');
      expect(badge.className).toContain('truncate');
    }
    // Full labels for wide cards, tier/verdict only for narrow ones.
    expect(getByText('ProtonDB · Platinum')).toBeTruthy();
    expect(getByText('Platinum').className).toContain('badge-short');
    expect(getByText('Deck · Verified')).toBeTruthy();
    expect(getByText('Verified').className).toContain('badge-short');
    // The full text stays in the tooltips.
    expect(container.querySelector('[title="ProtonDB · Platinum · 2026-10-09"]')).toBeTruthy();
  });

  it('deck badge alone clips inside the card instead of overflowing', () => {
    const deckOnly = mk('d', { fix_info: { source: 'steamdeck', tier: '', description: '', winetricks: [], dll_overrides: {}, launch_args: '', deck: 'unsupported', deck_notes: [] } });
    const { getByText, container } = render(<CompatChip game={deckOnly} />);
    expect(getByText('Deck · Unsupported')).toBeTruthy();
    const badge = container.querySelector('[data-slot="badge"]') as HTMLElement;
    expect(badge.className).toContain('max-w-full');
    expect(badge.className).toContain('truncate');
  });

  it('DeckBadge exposes the compact verdict alongside the full label', () => {
    const { getByText } = render(<DeckBadge rating="playable" />);
    expect(getByText('Deck · Playable')).toBeTruthy();
    expect(getByText('Playable').className).toContain('badge-short');
  });
});

// jsdom doesn't apply style.css, so guard the rule itself: the compact label
// is hidden everywhere by default, not only inside cards (issue #63: the game
// page showed "JOUABLEJOUABLE").
describe('badge label CSS', () => {
  it('hides the compact label outside narrow cards', async () => {
    // The UI's tsconfig has no Node types: name the two calls this needs.
    const fsName = 'node:fs';
    const { readFileSync, existsSync } = (await import(/* @vite-ignore */ fsName)) as {
      readFileSync(path: string, encoding: 'utf8'): string;
      existsSync(path: string): boolean;
    };
    const cwd = (globalThis as unknown as { process: { cwd(): string } }).process.cwd();
    // Vitest runs from apps/ui (bun run test:ui) or the repo root.
    const file = ['src/style.css', 'apps/ui/src/style.css'].map((p) => `${cwd}/${p}`).find(existsSync);
    expect(file).toBeTruthy();
    const css = readFileSync(file!, 'utf8');
    expect(css).toMatch(/^\.badge-short\s*\{\s*display:\s*none;\s*\}/m);
    expect(css).toMatch(/@container \(max-width: 190px\)[\s\S]*\.card-compat \.badge-short\s*\{\s*display:\s*inline;/);
  });
});
