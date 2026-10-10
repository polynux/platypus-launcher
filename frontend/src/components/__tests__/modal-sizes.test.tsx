import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Effect } from 'effect';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    verifyGame: () => Effect.never,
    repairGame: () => Effect.never,
    checkMove: () => Effect.never,
    moveGame: () => Effect.never,
    pickMoveFolder: () => Effect.succeed(''),
  },
}));

import { Modal, MODAL_SIZES } from '../Modal';
import { VerifyDialog } from '../VerifyDialog';
import { MoveDialog } from '../MoveDialog';
import type { Game } from '../../types';

const game = { id: 'g', title: 'G', install_dir: '/g/G', source: 'echo' } as unknown as Game;

describe('shared dialog sizes', () => {
  it('a Modal without a size keeps the width its owner gives it', () => {
    render(<Modal label="plain" onClose={() => {}} className="max-w-[300px]">x</Modal>);
    const d = screen.getByRole('dialog', { name: 'plain' });
    expect(d.className).toMatch(/max-w-\[300px\]/);
  });

  it('a size sets the maximum width', () => {
    render(<Modal label="sized" onClose={() => {}} size="sm">x</Modal>);
    expect(screen.getByRole('dialog', { name: 'sized' }).className).toContain(MODAL_SIZES.sm);
  });

  it('the verify and move dialogs are not full width', () => {
    const { unmount } = render(<VerifyDialog game={game} jobs={[]} onClose={() => {}} />);
    expect(screen.getByRole('dialog', { name: 'Verify G' }).className).toContain(MODAL_SIZES.md);
    unmount();
    render(<MoveDialog game={game} startFolder="/g" onClose={() => {}} onQueued={() => {}} />);
    expect(screen.getByRole('dialog', { name: 'Move G' }).className).toContain(MODAL_SIZES.md);
  });
});
