import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import React from 'react';
import type { Game, StorePlugin } from '../types';
import type * as ThemeModule from '../lib/theme';
import { InstallDialog } from './InstallDialog';
import { Modal } from './Modal';
import { MoveDialog } from './MoveDialog';

const state = vi.hoisted(() => ({
  variants: {
    'dialog.presentation': 'dialog',
    'dialog.size': 'auto',
  } as Record<string, string>,
}));

vi.mock('../lib/theme', async (importOriginal) => ({
  ...(await importOriginal<typeof ThemeModule>()),
  useVariant: (axis: string) => state.variants[axis],
}));
vi.mock('../effect/runtime', () => ({ LauncherApi: {} }));
// The install dialog's own data calls are not under test: only its chrome is.
vi.mock('../lib/installSize', () => ({ useInstallSize: () => ({ state: 'unknown' }) }));
vi.mock('./InstallLocationPicker', () => ({ InstallLocationPicker: () => null }));

afterEach(() => {
  cleanup();
  state.variants['dialog.presentation'] = 'dialog';
  state.variants['dialog.size'] = 'auto';
});

const game = { id: 'g1', title: 'Echo', install_dir: '/games/echo' } as Game;

describe('dialogs follow the dialog variants (issue #114)', () => {
  it('turns the move dialog into a sheet under dialog.presentation = sheet', () => {
    state.variants['dialog.presentation'] = 'sheet';
    render(<MoveDialog game={game} startFolder="/games" onClose={() => {}} onQueued={() => {}} />);
    const dialog = screen.getByRole('dialog', { name: /Move Echo/i });
    expect(dialog.className).toContain('w-[min(42rem,92vw)]');
    expect(dialog.className).not.toContain('max-w-[560px]');
  });

  it('turns the install dialog into a sheet too', () => {
    state.variants['dialog.presentation'] = 'sheet';
    render(<InstallDialog game={game} plugin={{ name: 'Echo', capabilities: [] } as unknown as StorePlugin} onClose={() => {}} onChanged={() => {}} onNotify={() => {}} />);
    expect(screen.getByRole('dialog', { name: /Install Echo/i }).className).toContain('w-[min(42rem,92vw)]');
  });

  it('keeps each dialog built-in size under dialog.size = auto', () => {
    render(<MoveDialog game={game} startFolder="/games" onClose={() => {}} onQueued={() => {}} />);
    expect(screen.getByRole('dialog', { name: /Move Echo/i }).className).toContain('max-w-[560px]');
  });

  it('widens a dialog with no use-site override under dialog.size = lg', () => {
    state.variants['dialog.size'] = 'lg';
    render(<MoveDialog game={game} startFolder="/games" onClose={() => {}} onQueued={() => {}} />);
    const dialog = screen.getByRole('dialog', { name: /Move Echo/i });
    expect(dialog.className).toContain('max-w-[720px]');
    expect(dialog.className).not.toContain('max-w-[560px]');
  });

  it('widens a dialog that names no size at all (default sm) under dialog.size = lg', () => {
    render(<Modal label="Plain" onClose={() => {}}><p>Body</p></Modal>);
    expect(screen.getByRole('dialog', { name: 'Plain' }).className).toContain('max-w-[440px]');
    cleanup();
    state.variants['dialog.size'] = 'lg';
    render(<Modal label="Plain" onClose={() => {}}><p>Body</p></Modal>);
    const dialog = screen.getByRole('dialog', { name: 'Plain' });
    expect(dialog.className).toContain('max-w-[720px]');
    expect(dialog.className).not.toContain('max-w-[440px]');
  });

  it('applies a use-site size when the theme leaves the global one auto', () => {
    render(<Modal label="Collection" size="xl" onClose={() => {}}><p>Body</p></Modal>);
    expect(screen.getByRole('dialog', { name: 'Collection' }).className).toContain('max-w-[920px]');
  });

  it('treats dialog.size as a minimum over a use site size, and never a sheet width', () => {
    // A smaller theme size never shrinks a larger dialog; a larger one
    // widens a small dialog.
    state.variants['dialog.size'] = 'md';
    render(<Modal label="Settings" size="xl" onClose={() => {}}><p>Body</p></Modal>);
    expect(screen.getByRole('dialog', { name: 'Settings' }).className).toContain('max-w-[920px]');
    cleanup();
    state.variants['dialog.size'] = 'lg';
    render(<Modal label="Plain" onClose={() => {}}><p>Body</p></Modal>);
    const dialog = screen.getByRole('dialog', { name: 'Plain' });
    expect(dialog.className).toContain('max-w-[720px]');
    expect(dialog.className).not.toContain('max-w-[440px]');
    cleanup();
    state.variants['dialog.size'] = 'xl';
    render(<Modal label="Settings sheet" presentation="sheet" onClose={() => {}}><p>Body</p></Modal>);
    const sheet = screen.getByRole('dialog', { name: 'Settings sheet' });
    expect(sheet.className).toContain('w-[min(42rem,92vw)]');
    expect(sheet.className).not.toContain('max-w-[920px]');
  });
});
