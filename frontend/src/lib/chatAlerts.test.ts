import { describe, it, expect, vi, afterEach } from 'vitest';
import { createAlertCoalescer } from './chatAlerts';

afterEach(() => {
  vi.useRealTimers();
});

describe('createAlertCoalescer', () => {
  it('flushes the messages of a store once, after the window, with their count and friends', () => {
    vi.useFakeTimers();
    const flush = vi.fn();
    const c = createAlertCoalescer(flush, 3000);
    c.add('echo', 'Ana');
    vi.advanceTimersByTime(1000);
    c.add('echo', 'Ana');
    c.add('echo', 'Bo');
    expect(flush).not.toHaveBeenCalled();
    vi.advanceTimersByTime(2000);
    expect(flush).toHaveBeenCalledTimes(1);
    expect(flush).toHaveBeenCalledWith('echo', { count: 3, names: ['Ana', 'Bo'] });
  });

  it('keeps the stores apart, and starts a new batch after a flush', () => {
    vi.useFakeTimers();
    const flush = vi.fn();
    const c = createAlertCoalescer(flush, 3000);
    c.add('echo', 'Ana');
    c.add('echo-b', 'Mira');
    vi.advanceTimersByTime(3000);
    expect(flush).toHaveBeenCalledWith('echo', { count: 1, names: ['Ana'] });
    expect(flush).toHaveBeenCalledWith('echo-b', { count: 1, names: ['Mira'] });
    c.add('echo', 'Cy');
    vi.advanceTimersByTime(3000);
    expect(flush).toHaveBeenLastCalledWith('echo', { count: 1, names: ['Cy'] });
  });

  it('drops what is waiting when disposed', () => {
    vi.useFakeTimers();
    const flush = vi.fn();
    const c = createAlertCoalescer(flush, 3000);
    c.add('echo', 'Ana');
    c.dispose();
    vi.advanceTimersByTime(5000);
    expect(flush).not.toHaveBeenCalled();
  });
});
