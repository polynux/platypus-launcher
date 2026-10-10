import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { Modal } from '../Modal';

// Radix attaches its outside-pointer listener on the next tick.
const tick = () => new Promise((r) => setTimeout(r, 10));

// Radix defers outside-dismiss to the click that follows the primary pointerdown.
const clickBackdrop = (el: Element) => {
  fireEvent.pointerDown(el, { pointerType: 'mouse', button: 0 });
  fireEvent.click(el);
};

describe('S1 modal dismissable', () => {
  it('Esc and backdrop click close when dismissable (default)', async () => {
    const onClose = vi.fn();
    const { getByRole } = render(<Modal label="T" onClose={onClose}><button>x</button></Modal>);
    await tick();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
    clickBackdrop(getByRole('dialog').parentElement!);
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it('Esc and backdrop click are ignored when dismissable=false; own buttons still work', async () => {
    const onClose = vi.fn();
    const { getByRole, getByText } = render(
      <Modal label="T" onClose={onClose} dismissable={false}><button onClick={onClose}>Cancel</button></Modal>
    );
    await tick();
    fireEvent.keyDown(document, { key: 'Escape' });
    clickBackdrop(getByRole('dialog').parentElement!);
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.click(getByText('Cancel'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
