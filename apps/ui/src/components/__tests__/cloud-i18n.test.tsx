import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';
import { CloudConflictDialog } from '../CloudSavesConflict';
import { CloudSavesDefaults } from '../CloudSavesSetting';
import type { CloudStatus, Game, StorePlugin } from '../../types';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getCloudSettings: () => Effect.succeed({ plugins: {} }),
    setCloudPluginMode: () => Effect.succeed(undefined),
  },
}));

const game = { id: 'p.echo.tiny', title: 'Echo Tiny' } as Game;

const status = {
  state: 'conflict',
  counts: { same: 0, download: 0, upload: 0, conflict: 1, unmapped: 0 },
  conflicts: [
    {
      path: 'Profile/save.dat',
      local: { size: 3705, mtime: 1_780_000_000 },
      remote: { size: 3710, mtime: 1_780_000_100, machine: 'Bo-PC' },
    },
  ],
  backupDir: '/home/u/backups/tiny',
} as unknown as CloudStatus;

const plugins = [
  { id: 'echo', name: 'Echo', status: 'ready', capabilities: ['cloud-saves'], declared: ['cloud-saves'] },
] as unknown as StorePlugin[];

describe('cloud live language switching', () => {
  it('re-renders the conflict dialog in French without a reload', async () => {
    render(<CloudConflictDialog game={game} status={status} onClose={() => {}} onResolved={() => {}} />);
    expect(screen.getByRole('heading', { name: 'Saves changed on both sides' })).toBeInTheDocument();
    expect(screen.getByText(/has a save file that changed/)).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Choice for Profile/save.dat' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Decide later' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Apply choices' })).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('heading', { name: 'Sauvegardes modifiées des deux côtés' })).toBeInTheDocument();
    expect(screen.getByText(/a un fichier de sauvegarde modifié/)).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Choix pour Profile/save.dat' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Décider plus tard' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Appliquer les choix' })).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('heading', { name: 'Saves changed on both sides' })).toBeInTheDocument();
  });

  it('re-renders the cloud settings in French without a reload', async () => {
    render(<CloudSavesDefaults plugins={plugins} />);
    expect(screen.getByRole('heading', { name: 'Cloud saves' })).toBeInTheDocument();
    expect(screen.getByText(/Whether Platypus syncs/)).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('heading', { name: 'Sauvegardes cloud' })).toBeInTheDocument();
    expect(screen.getByText(/Si Platypus synchronise/)).toBeInTheDocument();
    expect(await screen.findByRole('radio', { name: 'Désactivé' })).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('heading', { name: 'Cloud saves' })).toBeInTheDocument();
  });
});
