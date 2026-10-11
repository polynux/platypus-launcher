import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, act } from '@testing-library/react';
import { TooltipProvider } from '../ds/tooltip';
import { Sidebar } from '../Sidebar';
import { StoreView, storeHostOf, storeHostOutside } from '../StoreView';

const base = {
  view: { name: 'library' as const },
  onNavigate: () => {},
  search: '',
  onSearch: () => {},
  storeCounts: {},
  onPickSource: () => {},
  gameCount: 0,
  downloadCount: 0,
  authNeeded: false,
  fixCount: 0,
  runnerCount: 0,
  lastScan: null,
  sessions: [],
  onStop: () => {},
  friends: [],
  friendLabels: {},
  onOpenFriends: () => {},
  canRedeem: false,
  onRedeem: () => {},
};

const side = (extra: Partial<React.ComponentProps<typeof Sidebar>>) =>
  render(<TooltipProvider><Sidebar {...base} {...extra} /></TooltipProvider>);

const echo = { id: 'echo', name: 'Echo (test plugin)', icon_url: 'app://platypus/icon' };

describe('sidebar Store section', () => {
  it('lists enabled plugins that declare a storefront, and opens them', () => {
    const onOpenStore = vi.fn();
    const { getByRole } = side({ stores: [echo], onOpenStore });
    const btn = getByRole('button', { name: /Echo \(test plugin\)/ });
    fireEvent.click(btn);
    expect(onOpenStore).toHaveBeenCalledWith('echo');
  });

  it('is hidden when no plugin declares a storefront', () => {
    const { queryByRole, queryByText } = side({ stores: [] });
    expect(queryByText('store')).toBeNull();
    expect(queryByRole('button', { name: /Echo/ })).toBeNull();
  });

  it('marks the open store current', () => {
    const { getByRole } = side({ stores: [echo], activeStore: 'echo' });
    expect(getByRole('button', { name: /Echo \(test plugin\)/ }).getAttribute('aria-current')).toBe('page');
  });
});

describe('store address strip', () => {
  it('names the host and flags outside domains', () => {
    expect(storeHostOf('https://store.steampowered.com/app/620/')).toBe('store.steampowered.com');
    expect(storeHostOf('nonsense')).toBe('');
    expect(storeHostOutside('https://store.steampowered.com/app/620/', ['steampowered.com'])).toBe(false);
    expect(storeHostOutside('https://checkout.stripe.com/pay', ['steampowered.com'])).toBe(true);
    expect(storeHostOutside('nonsense', ['steampowered.com'])).toBe(false);
  });
});

const plugin = {
  id: 'echo',
  name: 'Echo (test plugin)',
  storefront_url: 'http://127.0.0.1:18427/store/',
  storefront_domains: ['127.0.0.1'],
};

function storeBridgeStub() {
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

describe('StoreView toolbar', () => {
  it('opens the storefront and reports its bounds', () => {
    const bridge = storeBridgeStub();
    window.platypus = { ...window.platypus!, store: bridge } as typeof window.platypus;
    const { getByTestId } = render(
      <TooltipProvider><StoreView plugin={plugin as never} onClose={() => {}} onNotify={() => {}} /></TooltipProvider>,
    );
    expect(bridge.open).toHaveBeenCalledWith('echo', undefined);
    expect(bridge.setBounds).toHaveBeenCalled();
    expect(getByTestId('store-back').hasAttribute('disabled')).toBe(true);
    expect(getByTestId('store-forward').hasAttribute('disabled')).toBe(true);
    expect(getByTestId('store-address').textContent).toContain('127.0.0.1');
  });

  it('opens a deep link when the game page gave one', () => {
    const bridge = storeBridgeStub();
    window.platypus = { ...window.platypus!, store: bridge } as typeof window.platypus;
    render(
      <TooltipProvider>
        <StoreView plugin={plugin as never} initialUrl="http://127.0.0.1:18427/store/game/tiny" onClose={() => {}} onNotify={() => {}} />
      </TooltipProvider>,
    );
    expect(bridge.open).toHaveBeenCalledWith('echo', 'http://127.0.0.1:18427/store/game/tiny');
  });

  it('warns when the view wandered outside the domains', () => {
    const bridge = storeBridgeStub();
    let listener!: (ev: never) => void;
    bridge.onEvent = vi.fn((cb: (ev: never) => void) => {
      listener = cb;
      return () => {};
    });
    window.platypus = { ...window.platypus!, store: bridge } as typeof window.platypus;
    const { getByTestId, getByRole } = render(
      <TooltipProvider><StoreView plugin={plugin as never} onClose={() => {}} onNotify={() => {}} /></TooltipProvider>,
    );
    act(() => {
      listener({
        kind: 'navigated', pluginId: 'echo', url: 'https://checkout.example/pay',
        canGoBack: true, canGoForward: false, title: 'Pay', loading: false,
      } as never);
    });
    expect(getByTestId('store-address').textContent).toContain('checkout.example');
    expect(getByTestId('store-address').textContent).toMatch(/outside/);
    expect(getByRole('button', { name: 'Back' }).hasAttribute('disabled')).toBe(false);
    fireEvent.click(getByRole('button', { name: 'Back' }));
    expect(bridge.back).toHaveBeenCalledTimes(1);
  });

  it('closes through the toolbar', () => {
    const bridge = storeBridgeStub();
    window.platypus = { ...window.platypus!, store: bridge } as typeof window.platypus;
    const onClose = vi.fn();
    const { getByTestId, unmount } = render(
      <TooltipProvider><StoreView plugin={plugin as never} onClose={onClose} onNotify={() => {}} /></TooltipProvider>,
    );
    fireEvent.click(getByTestId('store-close'));
    expect(onClose).toHaveBeenCalledTimes(1);
    unmount();
    expect(bridge.close).toHaveBeenCalled();
  });
});
