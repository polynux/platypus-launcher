import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { Modal } from '../Modal';

describe('FE-11 modal focus trap', () => {
  it('wraps Tab from the last focusable back into the dialog', () => {
    const { getByText } = render(
      <Modal label="Test" onClose={() => {}}>
        <button>First</button>
        <button>Last</button>
      </Modal>
    );
    const first = getByText('First');
    const last = getByText('Last');
    last.focus();
    expect(document.activeElement).toBe(last);
    fireEvent.keyDown(last, { key: 'Tab', bubbles: true });
    expect(document.activeElement).toBe(first);
  });

  it('wraps Shift+Tab from the first focusable to the last', () => {
    const { getByText } = render(
      <Modal label="Test" onClose={() => {}}>
        <button>First</button>
        <button>Last</button>
      </Modal>
    );
    const first = getByText('First');
    const last = getByText('Last');
    first.focus();
    fireEvent.keyDown(first, { key: 'Tab', shiftKey: true, bubbles: true });
    expect(document.activeElement).toBe(last);
  });
});
