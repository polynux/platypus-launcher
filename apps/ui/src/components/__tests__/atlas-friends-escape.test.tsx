import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/react';
import { AtlasFriendsPanel } from '../AtlasFriendsPanel';

afterEach(cleanup);

describe('Atlas friends panel Escape handling', () => {
  it('leaves Escape to a dialog above the panel', () => {
    const onClose = vi.fn();
    render(<AtlasFriendsPanel views={[]} labels={{}} plugins={[]} inLibrary={() => false} onClose={onClose} onSetEnabled={() => {}} onSetStatus={() => {}} onSetPlaying={() => {}} onReconnect={() => {}} onOpenGame={() => {}} />);
    const dialog = document.createElement('div');
    dialog.setAttribute('role', 'dialog');
    document.body.append(dialog);
    fireEvent.keyDown(document.body, { key: 'Escape' });
    expect(onClose).not.toHaveBeenCalled();
    dialog.remove();
  });
});
