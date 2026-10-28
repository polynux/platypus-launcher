// Issue #156: page presentations are independent axes, not the shell.
// A theme picks one page presentation without the rest: dashboard.home with
// fixes.rows still shows AtlasHome + the classic fixes rows, and vice versa.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { Dashboard } from '../Dashboard';
import { FixesView } from '../FixesView';
import '../../lib/i18n';

const variants = vi.hoisted(() => ({ map: {} as Record<string, string> }));
vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => variants.map[axis] ?? 'page',
    useThemeLayout: () => null,
  };
});

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {},
}));

afterEach(() => {
  cleanup();
  variants.map = {};
});

const games = [] as never[];

describe('page-presentation axes', () => {
  it('dashboard.home renders AtlasHome while fixes stays rows', () => {
    variants.map['dashboard.presentation'] = 'home';
    variants.map['fixes.presentation'] = 'rows';
    render(
      <Dashboard
        games={[{ id: 'g', title: 'G', store: 'steam', source: 'steam', is_installed: true, settings: {} } as never]}
        runners={[]}
        activity={[]}
        dismissedHero={new Set()}
        onDismissHero={() => {}}
        onOpenGame={() => {}}
        onNavigate={() => {}}
      />,
    );
    // AtlasHome greeting header vs classic blocks
    expect(document.querySelector('header h1')).toBeInTheDocument();
    cleanup();
    render(<FixesView games={games} onOpenGame={() => {}} />);
    expect(screen.getByText(/what needs fixing|needs attention/i)).toBeInTheDocument();
  });

  it('fixes.cards renders Atlas cards while dashboard stays blocks', () => {
    variants.map['dashboard.presentation'] = 'blocks';
    variants.map['fixes.presentation'] = 'cards';
    render(
      <Dashboard
        games={[{ id: 'g', title: 'G', store: 'steam', source: 'steam', is_installed: true, settings: {} } as never]}
        runners={[]}
        activity={[]}
        dismissedHero={new Set()}
        onDismissHero={() => {}}
        onOpenGame={() => {}}
        onNavigate={() => {}}
      />,
    );
    // Classic dashboard blocks, not the Atlas home greeting
    expect(screen.getByText(/your games|continue playing/i)).toBeInTheDocument();
    cleanup();
    render(<FixesView games={[]} onOpenGame={() => {}} />);
    // Atlas attention empty state (cards presentation)
    expect(screen.getByText(/everything works|all clear|no.*attention/i)).toBeInTheDocument();
  });
});
