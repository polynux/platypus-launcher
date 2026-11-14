import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import React from 'react';
import type * as ThemeModule from '../lib/theme';
import { TooltipProvider } from './ds/tooltip';
import { StoreView } from './StoreView';

const state = vi.hoisted(() => ({
  variants: {
    'store.toolbar': 'full',
    'store.address': 'shown',
  } as Record<string, string>,
}));

vi.mock('../lib/theme', async (importOriginal) => ({
  ...(await importOriginal<typeof ThemeModule>()),
  useVariant: (axis: string) => state.variants[axis],
}));

afterEach(() => {
  cleanup();
  state.variants['store.toolbar'] = 'full';
  state.variants['store.address'] = 'shown';
  vi.restoreAllMocks();
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
    onEvent: vi.fn(() => () => {}),
  };
}

/** jsdom lays everything out at 0×0, so the area's rect comes from the test:
 *  `top`/`height` follow the toolbar height the variant asks for. */
function stubAreaRect(get: () => { top: number; height: number }) {
  return vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(() => {
    const { top, height } = get();
    return { left: 10, top, width: 300, height, right: 310, bottom: top + height, x: 10, y: top, toJSON: () => ({}) } as DOMRect;
  });
}

describe('store toolbar chrome (issue #115)', () => {
  it('keeps the full /22 shop bar by default', () => {
    const bridge = storeBridgeStub();
    window.platypus = { ...window.platypus!, store: bridge } as typeof window.platypus;
    const rect = stubAreaRect(() => ({ top: 55, height: 400 }));
    const { container } = render(<TooltipProvider><StoreView plugin={plugin as never} onClose={() => {}} onNotify={() => {}} shopBar /></TooltipProvider>);
    expect(container.querySelector('.shop-bar')?.className).toContain('h-[55px]');
    expect(screen.getByTestId('store-address')).toBeInTheDocument();
    expect(bridge.setBounds).toHaveBeenLastCalledWith({ x: 10, y: 55, width: 300, height: 400 });
    rect.mockRestore();
  });

  it('reports the native area rect again after a compact toolbar', () => {
    const bridge = storeBridgeStub();
    window.platypus = { ...window.platypus!, store: bridge } as typeof window.platypus;
    let layout = { top: 55, height: 400 };
    const rect = stubAreaRect(() => layout);
    const view = () => <TooltipProvider><StoreView plugin={plugin as never} onClose={() => {}} onNotify={() => {}} shopBar /></TooltipProvider>;
    const { container, rerender } = render(view());
    expect(bridge.setBounds).toHaveBeenLastCalledWith({ x: 10, y: 55, width: 300, height: 400 });
    // The theme switch re-renders the page; the shorter bar leaves more room.
    state.variants['store.toolbar'] = 'compact';
    layout = { top: 42, height: 413 };
    rerender(view());
    expect(container.querySelector('.shop-bar')?.className).toContain('h-[42px]');
    expect(bridge.setBounds).toHaveBeenLastCalledWith({ x: 10, y: 42, width: 300, height: 413 });
    rect.mockRestore();
  });

  it('hides the address strip but keeps the outside-domain warning', async () => {
    const bridge = storeBridgeStub();
    window.platypus = { ...window.platypus!, store: bridge } as typeof window.platypus;
    state.variants['store.address'] = 'hidden';
    const rect = stubAreaRect(() => ({ top: 55, height: 400 }));
    render(<TooltipProvider><StoreView plugin={plugin as never} initialUrl="https://checkout.example.com/pay" onClose={() => {}} onNotify={() => {}} shopBar /></TooltipProvider>);
    expect(screen.queryByTestId('store-address')).toBeNull();
    expect(screen.getByTestId('store-outside')).toHaveTextContent('checkout.example.com');
    expect(bridge.setBounds).toHaveBeenLastCalledWith({ x: 10, y: 55, width: 300, height: 400 });
    rect.mockRestore();
  });

  it('keeps the trailing buttons at the right edge when the strip is hidden', () => {
    const bridge = storeBridgeStub();
    window.platypus = { ...window.platypus!, store: bridge } as typeof window.platypus;
    state.variants['store.address'] = 'hidden';
    render(<TooltipProvider><StoreView plugin={plugin as never} onClose={() => {}} onNotify={() => {}} shopBar /></TooltipProvider>);
    const external = screen.getByTestId('store-external');
    const spacer = external.parentElement!.querySelector(':scope > span.flex-1');
    expect(spacer).not.toBeNull();
    expect(spacer!.compareDocumentPosition(external) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
