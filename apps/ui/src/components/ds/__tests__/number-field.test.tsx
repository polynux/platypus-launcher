import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NumberField } from '../number-field';

describe('NumberField', () => {
  it('clamps to min/max on blur', () => {
    const onChange = vi.fn();
    render(<NumberField aria-label="n" value={5} min={1} max={10} onChange={onChange} />);
    const input = screen.getByLabelText('n');
    fireEvent.change(input, { target: { value: '99' } });
    fireEvent.blur(input);
    expect(onChange).toHaveBeenLastCalledWith(10);
    fireEvent.change(input, { target: { value: '-4' } });
    fireEvent.blur(input);
    expect(onChange).toHaveBeenLastCalledWith(1);
  });

  it('an empty value means unset', () => {
    const onChange = vi.fn();
    render(<NumberField aria-label="n" value={5} onChange={onChange} />);
    const input = screen.getByLabelText('n');
    fireEvent.change(input, { target: { value: '' } });
    fireEvent.blur(input);
    expect(onChange).toHaveBeenLastCalledWith(undefined);
  });

  it('renders unset as an empty input', () => {
    render(<NumberField aria-label="n" value={undefined} placeholder="auto" onChange={() => {}} />);
    expect(screen.getByLabelText('n')).toHaveValue('');
  });

  it('step buttons add/subtract step and respect bounds', async () => {
    const onChange = vi.fn();
    const { rerender } = render(<NumberField aria-label="n" value={5} min={0} max={6} step={2} onChange={onChange} />);
    await userEvent.click(screen.getByLabelText('Increase'));
    expect(onChange).toHaveBeenLastCalledWith(6);
    await userEvent.click(screen.getByLabelText('Decrease'));
    expect(onChange).toHaveBeenLastCalledWith(3);
    rerender(<NumberField aria-label="n" value={6} min={0} max={6} step={2} onChange={onChange} />);
    expect(screen.getByLabelText('Increase')).toBeDisabled();
  });
});
