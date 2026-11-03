import React from 'react';
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Input } from './input';

describe('Input placeholder styling', () => {
  it('uses the muted text token for placeholders', () => {
    render(<Input aria-label="Program" placeholder="Path to program" />);
    expect(screen.getByRole('textbox', { name: 'Program' })).toHaveClass('placeholder:text-text-muted');
  });
});
