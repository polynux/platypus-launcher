import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';
import { RedeemDialog } from '../RedeemDialog';
import { Lightbox } from '../Lightbox';
import { TermsDialog } from '../TermsDialog';
import { LegalSection } from '../LegalSection';
import { ErrorNotice } from '../ErrorNotice';
import { GameFoldersSettings } from '../GameFoldersSettings';
import type { Game, GameTerm, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({ getGameTerms: vi.fn(), getGameFolders: vi.fn() }));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameTerms: () => Effect.promise(() => api.getGameTerms()),
    getGameFolders: () => Effect.promise(() => api.getGameFolders()),
  },
}));

const game = { id: 'p.echo.licensed', source: 'echo', title: 'Echo Licensed' } as Game;
const stores = [{ id: 'echo', name: 'Echo' }] as unknown as StorePlugin[];
const terms: GameTerm[] = [
  { id: 'eula', title: 'Echo end user licence', url: 'https://example.com/echo/eula', version: '2026.2' },
];
const plugin = { id: 'echo', capabilities: ['terms'], declared: ['terms'] } as unknown as StorePlugin;

beforeEach(() => {
  vi.clearAllMocks();
  api.getGameFolders.mockResolvedValue([
    { path: '/home/u/Games', is_default: true, exists: true, free_bytes: 120_000_000_000 },
  ]);
});

describe('misc dialogs live language switching', () => {
  it('re-renders the redeem dialog in French without a reload', async () => {
    render(<RedeemDialog stores={stores} onClose={() => {}} />);
    expect(screen.getByRole('heading', { name: 'Redeem a code' })).toBeInTheDocument();
    expect(screen.getByText('Store')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Redeem' })).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('heading', { name: 'Utiliser un code' })).toBeInTheDocument();
    expect(screen.getByText('Boutique')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Utiliser' })).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('heading', { name: 'Redeem a code' })).toBeInTheDocument();
  });

  it('re-renders the terms dialog in French without a reload', async () => {
    render(<TermsDialog game={game} terms={terms} onAccepted={() => {}} onCancel={() => {}} />);
    expect(screen.getByRole('heading', { name: 'Terms for Echo Licensed' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'I accept' })).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('heading', { name: 'Conditions pour Echo Licensed' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: "J'accepte" })).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('button', { name: 'I accept' })).toBeInTheDocument();
  });

  it('re-renders the legal section in French without a reload', async () => {
    api.getGameTerms.mockResolvedValue([{ ...terms[0], accepted_at: 1_700_000_000 }]);
    render(<LegalSection game={game} plugin={plugin} />);
    expect(await screen.findByText(/^accepted /)).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(await screen.findByText(/^accepté le /)).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(await screen.findByText(/^accepted /)).toBeInTheDocument();
  });

  it('re-renders the lightbox in French without a reload', async () => {
    render(
      <Lightbox images={['https://example.com/1.jpg']} index={0} alt="Echo Tiny" onIndex={() => {}} onClose={() => {}} />,
    );
    expect(screen.getByRole('dialog', { name: 'Echo Tiny media viewer' })).toBeInTheDocument();
    expect(screen.getByLabelText('Close viewer')).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('dialog', { name: 'Visionneuse média de Echo Tiny' })).toBeInTheDocument();
    expect(screen.getByLabelText('Fermer la visionneuse')).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('dialog', { name: 'Echo Tiny media viewer' })).toBeInTheDocument();
  });

  it('re-renders error notices in French without a reload', async () => {
    const onRetry = () => {};
    render(<ErrorNotice message="panic: nil pointer at foo.go:42" fallback="unused here" onRetry={onRetry} />);
    // A custom fallback wins over the catalog default.
    expect(screen.getByText('unused here')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Try again/ })).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('button', { name: /Réessayer/ })).toBeInTheDocument();
    expect(screen.getByText('Détails')).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('button', { name: /Try again/ })).toBeInTheDocument();
  });

  it('renders the folders description as one French sentence with the path inline', async () => {
    render(<GameFoldersSettings onNotify={() => {}} />);
    expect(await screen.findByText(/Where games install/)).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    const fr = await screen.findByText(/Où s'installent les jeux/);
    expect(fr.textContent).toMatch(/\/epic/);
    expect(fr.textContent).toMatch(/Steam garde ses propres dossiers/);

    await i18n.changeLanguage('en');
    expect(await screen.findByText(/Where games install/)).toBeInTheDocument();
  });
});
