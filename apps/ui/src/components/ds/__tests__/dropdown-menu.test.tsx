import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../dropdown-menu';

describe('DropdownMenu', () => {
  it('opens the portal panel, picks an item and closes with Escape', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <div data-testid="host">
        <DropdownMenu>
          <DropdownMenuTrigger>Tools</DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem onSelect={onSelect}>Plugins</DropdownMenuItem>
            <DropdownMenuItem>Storage</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>,
    );
    expect(screen.queryByRole('menu')).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Tools' }));
    const menu = screen.getByRole('menu');
    // The panel portals outside the trigger's container: an overflow
    // ancestor can never clip it.
    expect(screen.getByTestId('host').contains(menu)).toBe(false);
    await user.click(await screen.findByRole('menuitem', { name: 'Plugins' }));
    expect(onSelect).toHaveBeenCalledOnce();
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('moves focus into the menu and returns it on Escape', async () => {
    const user = userEvent.setup();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Tools</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem>Fixes</DropdownMenuItem>
          <DropdownMenuItem>Plugins</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>,
    );
    screen.getByRole('button', { name: 'Tools' }).focus();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('menuitem', { name: 'Fixes' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: 'Plugins' })).toHaveFocus();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).toBeNull();
    expect(screen.getByRole('button', { name: 'Tools' })).toHaveFocus();
  });
});
