// @ts-nocheck — node fs/path guard test, outside the app's DOM tsconfig.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

const src = (p: string) => fs.readFileSync(path.join(__dirname, '../..', p), 'utf8');

// Item 4: the toast must sit above an open modal. FE-14 set both the modal
// overlay (Modal.tsx) and the toast (App.tsx) to z-50, so which one won
// depended on DOM order. The toast's z-index must now be strictly higher than
// the modal overlay's, and the toast's DESIGN §6.6 shape must be intact.
const zValue = (cls) => {
  const m = cls.match(/z-\[(\d+)\]|z-(\d+)/);
  return m ? Number(m[1] ?? m[2]) : NaN;
};

describe('toast renders above an open modal', () => {
  it('toast z-index is strictly above the modal overlay z-index', () => {
    const modalOverlay = src('components/Modal.tsx').match(/fixed inset-0 (z-\S+) grid place-items-center/)?.[1];
    const toast = src('App.tsx').match(/fixed bottom-4 right-4 (z-\S+)/)?.[1];
    expect(modalOverlay).toBeTruthy();
    expect(toast).toBeTruthy();
    expect(zValue(toast)).toBeGreaterThan(zValue(modalOverlay));
  });

  it('keeps the DESIGN §6.6 toast shape (rounded-[3px], mono, max-w-md, auto roles)', () => {
    const app = src('App.tsx');
    expect(app).toContain("rounded-[3px] border bg-[#191410] px-3 py-2 font-mono text-[12px]");
    expect(app).toContain('max-w-md');
    expect(app).toContain('role="status"');
  });
});