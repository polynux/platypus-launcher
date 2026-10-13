import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { CopyCompareDialog } from '../CopyCompareDialog';
import type { CopyComparison, Game, GameCopy } from '../../types';

const ana: GameCopy = {
  owner_id: 'ana', owner_name: 'Ana', available: true,
  dlc: [{ id: 'expansion', name: 'Expansion One', kind: 'dlc' }],
};
const ben: GameCopy = {
  owner_id: 'ben', owner_name: 'Ben', available: true,
  dlc: [{ id: 'track-pack', name: 'Track pack', kind: 'dlc' }],
};
const compare: CopyComparison = {
  baseline: 'ben',
  diffs: [{ owner_id: 'ana', only_here: 1, only_baseline: 1, shared: 0, soundtracks_only_here: 0, soundtracks_only_baseline: 0, soundtracks_shared: 0 }],
};
const game = { id: 'p.echo.shared', title: 'Echo Shared Racer', preferred_copy: '' } as Game;

describe('copy compare dialog live language switching', () => {
  it('re-renders French without a reload', async () => {
    render(<CopyCompareDialog game={game} copies={[ana, ben]} compare={compare} onClose={() => {}} />);
    expect(screen.getByRole('heading', { name: 'Compare copies' })).toBeDefined();
    expect(screen.getByRole('switch', { name: 'Differences only' })).toBeDefined();
    expect(screen.getByRole('table', { name: 'Add-ons by copy' })).toBeDefined();
    await i18n.changeLanguage('fr');
    expect(screen.getByRole('heading', { name: 'Comparer les copies' })).toBeDefined();
    expect(screen.getByRole('switch', { name: 'Différences uniquement' })).toBeDefined();
    expect(screen.getByRole('table', { name: 'Extensions par copie' })).toBeDefined();
    await i18n.changeLanguage('en');
    expect(screen.getByRole('heading', { name: 'Compare copies' })).toBeDefined();
  });
});
