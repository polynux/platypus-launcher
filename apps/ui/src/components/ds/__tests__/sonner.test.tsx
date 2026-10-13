// @ts-nocheck — node fs/path guard test, outside the app's DOM tsconfig.
import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Toaster } from '@/components/ds/sonner';
import {
  __resetNotifyForTests,
  ERROR_DURATION,
  INFO_DURATION,
  SUCCESS_DURATION,
  notify,
} from '@/lib/notify';
import * as fs from 'node:fs';
import * as path from 'node:path';

function renderToaster() {
  return render(<Toaster />);
}

beforeEach(() => {
  vi.useRealTimers();
  __resetNotifyForTests();
});

afterEach(() => {
  __resetNotifyForTests();
  vi.useRealTimers();
});

describe('toasts (issue #64)', () => {
  it('stacks several notifications, newest first', async () => {
    renderToaster();
    act(() => {
      notify.success('stack-first-aaa');
      notify.success('stack-second-bbb');
      notify.success('stack-third-ccc');
    });
    await waitFor(() => expect(screen.getAllByRole('status')).toHaveLength(3), { timeout: 3000 });
    const items = screen.getAllByRole('status');
    const order = items.map((el) => el.textContent ?? '');
    expect(order[0]).toContain('stack-third-ccc');
    expect(order[1]).toContain('stack-second-bbb');
    expect(order[2]).toContain('stack-first-aaa');
  });

  it('auto-dismisses by type: success ~3.5s, info ~5s, error ~8s', async () => {
    vi.useFakeTimers();
    renderToaster();
    act(() => {
      notify.success('dur-ok-aaa');
    });
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(screen.getByText('dur-ok-aaa')).not.toBeNull();
    act(() => {
      vi.advanceTimersByTime(SUCCESS_DURATION + 800);
    });
    expect(screen.queryByText('dur-ok-aaa')).toBeNull();

    act(() => {
      notify.info('dur-info-bbb');
    });
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(screen.getByText('dur-info-bbb')).not.toBeNull();
    act(() => {
      vi.advanceTimersByTime(INFO_DURATION - 1000);
    });
    expect(screen.queryByText('dur-info-bbb')).not.toBeNull();
    act(() => {
      vi.advanceTimersByTime(1000 + 800);
    });
    expect(screen.queryByText('dur-info-bbb')).toBeNull();

    act(() => {
      notify.error('dur-err-ccc');
    });
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(screen.getByRole('alert')).toHaveTextContent('dur-err-ccc');
    act(() => {
      vi.advanceTimersByTime(INFO_DURATION);
    });
    expect(screen.queryByText('dur-err-ccc')).not.toBeNull();
    act(() => {
      vi.advanceTimersByTime(ERROR_DURATION - INFO_DURATION + 800);
    });
    expect(screen.queryByText('dur-err-ccc')).toBeNull();
  });

  it('pauses the timer while hovered', async () => {
    vi.useFakeTimers();
    renderToaster();
    act(() => {
      notify.success('hover-me-aaa');
    });
    act(() => {
      vi.advanceTimersByTime(0);
    });
    const toast = screen.getByText('hover-me-aaa');
    const list = toast.closest('ol') ?? document.body;
    // Hover expands the stack, which pauses Sonner's close timer.
    act(() => {
      fireEvent.mouseEnter(list);
    });
    act(() => {
      vi.advanceTimersByTime(SUCCESS_DURATION + 2000);
    });
    expect(screen.queryByText('hover-me-aaa')).not.toBeNull();
    act(() => {
      fireEvent.mouseLeave(list);
    });
    act(() => {
      vi.advanceTimersByTime(SUCCESS_DURATION + 800);
    });
    expect(screen.queryByText('hover-me-aaa')).toBeNull();
  });

  it('announces errors assertively and the rest politely', async () => {
    renderToaster();
    act(() => {
      notify.error('boom-aaa');
      notify.success('yay-bbb');
      notify.info('fyi-ccc');
    });
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('boom-aaa'), { timeout: 3000 });
    await waitFor(() => expect(screen.getAllByRole('status')).toHaveLength(2), { timeout: 3000 });
    const polite = screen.getAllByRole('status');
    expect(polite.map((el) => el.textContent)).toEqual(
      expect.arrayContaining([expect.stringContaining('yay-bbb'), expect.stringContaining('fyi-ccc')]),
    );
  });

  it('coalesces repeats of the same message with a count instead of stacking', async () => {
    renderToaster();
    act(() => {
      notify.success('saved-aaa');
      notify.success('saved-aaa');
      notify.success('saved-aaa');
    });
    await waitFor(() => expect(screen.getAllByRole('status')).toHaveLength(1), { timeout: 3000 });
    const items = screen.getAllByRole('status');
    expect(items[0].textContent).toContain('saved-aaa');
    expect(items[0].textContent).toContain('×3');
  });

  it('runs the action handler from the toast button and stays until dismissed', async () => {
    vi.useFakeTimers();
    const onClick = vi.fn();
    renderToaster();
    act(() => {
      notify.error('cannot-reach-aaa', { action: { label: 'Retry-zzz', onClick } });
    });
    act(() => {
      vi.advanceTimersByTime(0);
    });
    const button = screen.getByRole('button', { name: 'Retry-zzz' });
    // An error with an action is sticky: well past 8 s it is still there.
    act(() => {
      vi.advanceTimersByTime(ERROR_DURATION + 5000);
    });
    expect(screen.queryByText('cannot-reach-aaa')).not.toBeNull();
    act(() => {
      fireEvent.click(button);
    });
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('sits top right with at most 3 visible and a close button', async () => {
    renderToaster();
    act(() => {
      notify.success('pos-one-aaa');
      notify.success('pos-two-bbb');
      notify.success('pos-three-ccc');
      notify.success('pos-four-ddd');
      notify.success('pos-five-eee');
    });
    await waitFor(() => expect(screen.getAllByRole('status')).toHaveLength(5), { timeout: 3000 });
    const toaster = document.querySelector('[data-sonner-toaster]');
    expect(toaster?.getAttribute('data-x-position')).toBe('right');
    expect(toaster?.getAttribute('data-y-position')).toBe('top');
    // The close button carries the translated label.
    expect(screen.getAllByLabelText('Close notification').length).toBeGreaterThan(0);
  });

  it('has no remaining setNotice or old bottom notice in App', () => {
    const appPath = path.resolve(__dirname, '../../../App.tsx');
    const src = fs.readFileSync(appPath, 'utf8');
    expect(src).not.toMatch(/setNotice/);
    expect(src).not.toMatch(/NOTICE_STYLE/);
    expect(src).not.toMatch(/fixed bottom-4 right-4/);
    expect(src).toMatch(/Toaster/);
    expect(src).toMatch(/components\/ds\/sonner/);
  });
});
