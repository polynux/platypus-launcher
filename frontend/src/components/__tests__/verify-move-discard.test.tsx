import React from 'react';
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { DownloadsView } from '../DownloadsView';
import type { Job, JobKind } from '../../types';

function job(over: Partial<Job> = {}): Job {
  return {
    id: 'j1',
    kind: 'install',
    source: 's',
    game_id: 'g',
    store_game_id: 'a',
    title: 'Goose',
    state: 'canceled',
    bytes: 0,
    total: 0,
    speed: 0,
    percent: -1,
    attempts: 1,
    created_at: 0,
    updated_at: 0,
    canceled_at: 1,
    ...over,
  };
}

const noop = () => {};

describe('a canceled check or move has no data to discard', () => {
  it.each<JobKind>(['verify', 'move'])('hides Discard data for a canceled %s', (kind) => {
    const { queryByLabelText } = render(
      <DownloadsView jobs={[job({ kind })]} onAction={noop} onClear={noop} onOpenGame={noop} />
    );
    expect(queryByLabelText('Discard downloaded data of Goose')).toBeNull();
  });

  it('still offers Discard data for a canceled install', () => {
    const { getByLabelText } = render(
      <DownloadsView jobs={[job({ kind: 'install' })]} onAction={noop} onClear={noop} onOpenGame={noop} />
    );
    expect(getByLabelText('Discard downloaded data of Goose')).toBeTruthy();
  });
});
