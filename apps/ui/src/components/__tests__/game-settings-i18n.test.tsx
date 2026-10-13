import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { TweaksPanel } from '../TweaksPanel';
import type { Game, Tweak } from '../../types';

const game = { id: 'p.echo.tiny', title: 'Echo Tiny', settings: { env_vars: {} } } as Game;
const tweaks = [
  { id: 't1', title: 'Use fsync', reason: 'Helps on this kernel', kind: 'recommended', env: {} },
  { id: 't2', title: 'Disable esync', reason: 'When the game stutters', kind: 'troubleshoot', env: {} },
] as Tweak[];

describe('tweaks panel live language switching', () => {
  it('re-renders French without a reload', async () => {
    render(
      <TweaksPanel game={game} tweaks={tweaks} viaSteam onChange={() => {}} onOpenSettings={() => {}} />,
    );
    expect(screen.getByRole('heading', { name: 'Suggested tweaks' })).toBeDefined();
    expect(screen.getByText('recommended')).toBeDefined();
    expect(screen.getByText('if something breaks')).toBeDefined();
    await i18n.changeLanguage('fr');
    expect(screen.getByRole('heading', { name: 'Ajustements suggérés' })).toBeDefined();
    expect(screen.getByText('recommandés')).toBeDefined();
    expect(screen.getByText('si quelque chose casse')).toBeDefined();
    // The Steam sentence keeps its link in French.
    expect(screen.getByRole('button', { name: 'options de lancement' })).toBeDefined();
    await i18n.changeLanguage('en');
    expect(screen.getByRole('heading', { name: 'Suggested tweaks' })).toBeDefined();
  });
});
