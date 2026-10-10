import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from '../select';

describe('Select', () => {
  it('opens, picks an option and calls onValueChange', async () => {
    const onValueChange = vi.fn();
    render(
      <Select onValueChange={onValueChange}>
        <SelectTrigger aria-label="runner">
          <SelectValue placeholder="Pick" />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            <SelectLabel>Proton</SelectLabel>
            <SelectItem value="a">Proton A</SelectItem>
            <SelectItem value="b">Proton B</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>,
    );
    await userEvent.click(screen.getByLabelText('runner'));
    await userEvent.click(await screen.findByRole('option', { name: 'Proton B' }));
    expect(onValueChange).toHaveBeenCalledWith('b');
    expect(screen.getByLabelText('runner')).toHaveTextContent('Proton B');
  });
});
