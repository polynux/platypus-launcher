import React from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { PlayButton, FORCE_STOP_AFTER_MS } from '../PlayButton';
import type { SessionEvent } from '../../types';

const ev = (state: SessionEvent['state']): SessionEvent => ({ game_id: 'g', state });

function setup(session?: SessionEvent, launching = false) {
  const h = { onPlay: vi.fn(), onStop: vi.fn(), onForceStop: vi.fn() };
  const view = render(<PlayButton session={session} launching={launching} disabled={false} {...h} />);
  return { ...h, ...view, rerender: (s?: SessionEvent) => view.rerender(<PlayButton session={s} launching={false} disabled={false} {...h} />) };
}

afterEach(() => vi.useRealTimers());

describe('PlayButton', () => {
  it('idle: Play launches', () => {
    const { onPlay } = setup();
    fireEvent.click(screen.getByRole('button', { name: /play/i }));
    expect(onPlay).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('button', { name: /stop/i })).toBeNull();
  });

  it('launching: disabled, nothing to stop yet', () => {
    setup(undefined, true);
    expect((screen.getByRole('button', { name: /launching/i }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.queryByRole('button', { name: /stop/i })).toBeNull();
  });

  it('starting: shows a disabled Starting and a Stop', () => {
    const { onStop } = setup(ev('starting'));
    expect((screen.getByRole('button', { name: /starting/i }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole('button', { name: /^stop/i }));
    expect(onStop).toHaveBeenCalledTimes(1);
  });

  it('shows a store client activity and known progress while starting', () => {
    setup({ game_id: 'g', state: 'starting', client_status: { state: 'updating', progress: 0.45 } });
    expect(screen.getByRole('button', { name: /updating 45%/i })).toBeTruthy();
  });

  it('shows the client failure reason and leaves Play available for retry', () => {
    setup({ game_id: 'g', state: 'failed', message: "Steam finished its work but didn't start the game. Press Play again." });
    expect(screen.getByRole('status').textContent).toMatch(/press play again/i);
    expect(screen.getByRole('button', { name: /^play$/i })).toBeTruthy();
  });

  it('running: Playing indicator and a Stop button that stops', () => {
    const { onStop, onPlay } = setup(ev('running'));
    expect(screen.getByText(/playing/i)).toBeTruthy();
    expect(screen.queryByRole('button', { name: /^play$/i })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /^stop/i }));
    expect(onStop).toHaveBeenCalledTimes(1);
    expect(onPlay).not.toHaveBeenCalled();
  });

  it('stopping: pending Stopping…, Force stop only appears after a few seconds', () => {
    vi.useFakeTimers();
    const { onStop, onForceStop } = setup(ev('stopping'));
    const pending = screen.getByRole('button', { name: /stopping/i }) as HTMLButtonElement;
    expect(pending.disabled).toBe(true);
    fireEvent.click(pending);
    expect(onStop).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: /force stop/i })).toBeNull();

    act(() => { vi.advanceTimersByTime(FORCE_STOP_AFTER_MS + 50); });
    fireEvent.click(screen.getByRole('button', { name: /force stop/i }));
    expect(onForceStop).toHaveBeenCalledTimes(1);
  });

  it('the Force stop timer restarts for the next stop', () => {
    vi.useFakeTimers();
    const { rerender } = setup(ev('stopping'));
    act(() => { vi.advanceTimersByTime(FORCE_STOP_AFTER_MS + 50); });
    expect(screen.queryByRole('button', { name: /force stop/i })).not.toBeNull();
    rerender(undefined);
    rerender(ev('stopping'));
    expect(screen.queryByRole('button', { name: /force stop/i })).toBeNull();
  });
});
