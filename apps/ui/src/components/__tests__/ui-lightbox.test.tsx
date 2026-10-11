import React, { useState } from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Lightbox } from '../Lightbox';

const imgs = ['a.png', 'b.png', 'c.png'];

function Harness() {
  const [open, setOpen] = useState(false);
  const [i, setI] = useState(0);
  return (
    <>
      <button onClick={() => setOpen(true)}>open</button>
      {open && <Lightbox images={imgs} index={i} alt="Game" onIndex={setI} onClose={() => setOpen(false)} />}
    </>
  );
}

describe('Lightbox', () => {
  it('opens, navigates with wrap, closes on Esc and restores focus', () => {
    render(<Harness />);
    const opener = screen.getByText('open');
    opener.focus();
    fireEvent.click(opener);
    const dialog = screen.getByRole('dialog');
    expect(dialog.getAttribute('aria-modal')).toBe('true');
    expect(dialog.contains(document.activeElement)).toBe(true);
    expect(screen.getByText('1 / 3')).toBeTruthy();
    fireEvent.keyDown(dialog, { key: 'ArrowRight' });
    expect(screen.getByText('2 / 3')).toBeTruthy();
    fireEvent.keyDown(dialog, { key: 'ArrowLeft' });
    fireEvent.keyDown(dialog, { key: 'ArrowLeft' });
    expect(screen.getByText('3 / 3')).toBeTruthy();
    fireEvent.keyDown(dialog, { key: 'ArrowRight' });
    expect(screen.getByText('1 / 3')).toBeTruthy();
    fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(document.activeElement).toBe(opener);
  });

  it('closes on backdrop click but not on image click', () => {
    render(<Harness />);
    fireEvent.click(screen.getByText('open'));
    fireEvent.click(screen.getByAltText('Game screenshot 1'));
    expect(screen.getByRole('dialog')).toBeTruthy();
    fireEvent.click(screen.getByRole('dialog'));
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('traps Tab inside the dialog', () => {
    render(<Harness />);
    fireEvent.click(screen.getByText('open'));
    const dialog = screen.getByRole('dialog');
    screen.getByLabelText('Next media').focus();
    fireEvent.keyDown(dialog, { key: 'Tab' });
    expect(document.activeElement).toBe(screen.getByLabelText('Close viewer'));
  });
});
