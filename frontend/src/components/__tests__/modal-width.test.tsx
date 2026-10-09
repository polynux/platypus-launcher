import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Modal } from '../Modal';

describe('modal width', () => {
  it('is narrow by default, so a small dialog never spans the window', () => {
    render(<Modal label="Small" onClose={() => {}}><p>body</p></Modal>);
    const dialog = screen.getByRole('dialog', { name: 'Small' });
    expect(dialog.className).toContain('max-w-md');
  });

  it('a dialog that passes its own max width replaces the default', () => {
    render(<Modal label="Wide" onClose={() => {}} className="max-w-2xl"><p>body</p></Modal>);
    const dialog = screen.getByRole('dialog', { name: 'Wide' });
    expect(dialog.className).toContain('max-w-2xl');
    expect(dialog.className).not.toContain('max-w-md');
  });
});
