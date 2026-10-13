import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import { DownloadsView } from '../DownloadsView';
import { JobProgress } from '../JobProgress';
import type { Job } from '../../types';

const wrap = (ui: React.ReactElement) => render(ui, { wrapper: TooltipProvider });

function job(over: Partial<Job> = {}): Job {
  return {
    id: 'j1',
    kind: 'install',
    source: 's',
    game_id: 'g',
    store_game_id: 'a',
    title: 'Goose',
    state: 'running',
    bytes: 3_700_000,
    total: 30_000_000_000,
    speed: 1_000_000,
    percent: 42,
    attempts: 1,
    created_at: 0,
    updated_at: 0,
    ...over,
  };
}

const noop = () => {};

describe('downloads live language switching', () => {
  it('re-renders French without a reload', async () => {
    wrap(<DownloadsView jobs={[job(), job({ id: 'j2', state: 'paused' })]} onAction={noop} onClear={noop} onOpenGame={noop} />);
    expect(screen.getByLabelText('Pause Goose')).toBeTruthy();
    expect(screen.getByText('Resume all')).toBeTruthy();
    expect(screen.getByText('queue')).toBeTruthy();

    await i18n.changeLanguage('fr');
    expect(screen.getByLabelText('Mettre Goose en pause')).toBeTruthy();
    expect(screen.getByText('Tout reprendre')).toBeTruthy();
    expect(screen.getByText("file d'attente")).toBeTruthy();
    expect(screen.getByText(/la reprise continue où elle s'est arrêtée/)).toBeTruthy();

    await i18n.changeLanguage('en');
    expect(screen.getByLabelText('Pause Goose')).toBeTruthy();
    expect(screen.getByText('Resume all')).toBeTruthy();
  });

  it('localizes the job progress bytes line', async () => {
    wrap(<JobProgress job={job()} onCancel={noop} />);
    expect(screen.getByText(/3\.7 MB of 30\.0 GB/)).toBeTruthy();

    await i18n.changeLanguage('fr');
    expect(screen.getByText(/3,7 MB sur 30,0 GB/)).toBeTruthy();

    await i18n.changeLanguage('en');
    expect(screen.getByText(/3\.7 MB of 30\.0 GB/)).toBeTruthy();
  });
});
