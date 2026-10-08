import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { Sidebar } from '../Sidebar';

const base = {
  search: '',
  onSearch: () => {},
  storeCounts: {},
  onPickSource: () => {},
  gameCount: 1,
  downloadCount: 0,
  authNeeded: false,
  fixCount: 0,
  runnerCount: 0,
  lastScan: null,
  sessions: [],
  onStop: () => {},
};

describe('FE-12 sidebar search does not hijack navigation', () => {
  it('stays on the game view on change, navigates on Enter', () => {
    const onNavigate = vi.fn();
    const onSearch = vi.fn();
    const { getByPlaceholderText } = render(
      <Sidebar
        {...base}
        view={{ name: 'game', id: 'g1', from: 'library' }}
        onNavigate={onNavigate}
        onSearch={onSearch}
      />
    );
    const input = getByPlaceholderText('Search…');
    fireEvent.change(input, { target: { value: 'had' } });
    expect(onSearch).toHaveBeenCalledWith('had');
    expect(onNavigate).not.toHaveBeenCalled();
    fireEvent.keyDown(input, { key: 'Enter', bubbles: true });
    expect(onNavigate).toHaveBeenCalledWith({ name: 'library' });
  });
});
