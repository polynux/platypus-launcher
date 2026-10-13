import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TooltipProvider } from '../ds/tooltip';
import i18n from '../../lib/i18n';
import { StoreView } from '../StoreView';

const plugin = {
  id: 'echo',
  name: 'Echo (test plugin)',
  storefront_url: 'http://127.0.0.1:18427/store/',
  storefront_domains: ['127.0.0.1'],
};

function bridgeStub() {
  return {
    open: vi.fn(async () => {}),
    setBounds: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    reload: vi.fn(),
    close: vi.fn(),
    onEvent: vi.fn((_cb: (ev: never) => void) => () => {}),
  };
}

describe('store view live language switching', () => {
  it('re-renders French without a reload', async () => {
    window.platypus = { ...window.platypus!, store: bridgeStub() } as typeof window.platypus;
    render(
      <TooltipProvider><StoreView plugin={plugin as never} onClose={() => {}} onNotify={() => {}} /></TooltipProvider>,
    );
    expect(screen.getByRole('button', { name: 'Back' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Home' })).toBeTruthy();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('button', { name: 'Retour' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Accueil' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Fermer Echo (test plugin)' })).toBeTruthy();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('button', { name: 'Back' })).toBeTruthy();
  });
});
