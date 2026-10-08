import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { Modal } from '../Modal';

describe('S1 modal dismissable', () => {
  it('Esc and backdrop click close when dismissable (default)', () => {
    const onClose = vi.fn();
    const { getByRole } = render(<Modal label="T" onClose={onClose}><button>x</button></Modal>);
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
    fireEvent.mouseDown(getByRole('dialog').parentElement!);
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it('Esc and backdrop click are ignored when dismissable=false; own buttons still work', () => {
    const onClose = vi.fn();
    const { getByRole, getByText } = render(
      <Modal label="T" onClose={onClose} dismissable={false}><button onClick={onClose}>Cancel</button></Modal>
    );
    fireEvent.keyDown(window, { key: 'Escape' });
    fireEvent.mouseDown(getByRole('dialog').parentElement!);
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.click(getByText('Cancel'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
