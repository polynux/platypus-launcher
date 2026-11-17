import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { PlayButton } from '../PlayButton';

const noop = () => {};
const btn = (session?: React.ComponentProps<typeof PlayButton>['session']) =>
  render(<PlayButton session={session} launching={false} disabled={false} onPlay={noop} onStop={noop} onForceStop={noop} />);

describe('signin client status', () => {
  it('shows "Waiting for you in <client>" (en), with the plugin label when given', () => {
    const { unmount } = btn({ game_id: 'g', state: 'starting', message: 'Steam is signin', client_status: { state: 'signin' } });
    expect(screen.getByRole('button', { name: /waiting for you in steam/i })).toBeTruthy();
    unmount();
    btn({ game_id: 'g', state: 'starting', message: 'Ubisoft Connect is signin', client_status: { state: 'signin', label: 'Waiting for you in Ubisoft Connect' } });
    expect(screen.getByRole('button', { name: /waiting for you in ubisoft connect/i })).toBeTruthy();
  });

  it('renders in French without a reload', async () => {
    const { unmount } = btn({ game_id: 'g', state: 'starting', message: 'Steam is signin', client_status: { state: 'signin' } });
    await i18n.changeLanguage('fr');
    expect(screen.getByRole('button', { name: /en attente de vous dans steam/i })).toBeTruthy();
    unmount();
    await i18n.changeLanguage('en');
  });
});
