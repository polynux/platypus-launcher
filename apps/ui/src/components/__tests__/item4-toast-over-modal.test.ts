// @ts-nocheck — node fs/path guard test, outside the app's DOM tsconfig.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

const src = (p: string) => fs.readFileSync(path.join(__dirname, '../..', p), 'utf8');

// Item 4: the toast must sit above an open modal. Issue #64 moved the toast
// to Sonner (top-right, ds/sonner.tsx): Sonner's own z-index (999999999)
// beats the modal overlay's z-50 wherever the toast mounts, and the toast's
// DESIGN §6.6 shape lives in the Sonner wrapper + style.css.
const zValue = (cls) => {
  const m = cls.match(/z-\[(\d+)\]|z-(\d+)/);
  return m ? Number(m[1] ?? m[2]) : NaN;
};

describe('toast renders above an open modal', () => {
  it('toast z-index is strictly above the modal overlay z-index', () => {
    const modalOverlay = src('components/ds/dialog.tsx').match(/fixed inset-0 (z-\S+) grid place-items-center/)?.[1];
    expect(modalOverlay).toBeTruthy();
    // Sonner's stylesheet pins the toaster above everything (modal is z-50).
    expect(zValue(modalOverlay)).toBeLessThan(999999999);
    const sonner = src('components/ds/sonner.tsx');
    expect(sonner).toContain('top-right');
  });

  it('keeps the launcher panel shape (rounded-2xl, sans, raised ground, auto roles)', () => {
    const sonner = src('components/ds/sonner.tsx');
    expect(sonner).toContain('rounded-2xl');
    expect(sonner).toContain('font-sans');
    expect(sonner).toContain('bg-raised');
    expect(sonner).not.toContain('rounded-[3px]');
    expect(sonner).not.toContain('font-mono');
    const notify = src('lib/notify.ts');
    expect(notify).toContain("role");
    expect(notify).toContain("alert");
    expect(notify).toContain("status");
  });
});
