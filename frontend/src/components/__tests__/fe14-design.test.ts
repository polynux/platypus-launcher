// @ts-nocheck — node fs/path guard test, outside the app's DOM tsconfig.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

const src = (p: string) => fs.readFileSync(path.join(__dirname, '../..', p), 'utf8');

describe('FE-14 design drift guards', () => {
  it('uses backdrop-blur only on hero stat chips (Dashboard), not JobProgress', () => {
    const gamePage = src('components/GamePage.tsx');
    const dashboard = src('components/Dashboard.tsx');
    expect(gamePage.includes('backdrop-blur')).toBe(false);
    // Hero stat chips explicitly may keep one per DESIGN §1.1/§6.1.
    expect(dashboard.includes('backdrop-blur')).toBe(true);
  });

  it('toast matches DESIGN §6.6 instrument readout (rounded-[3px]) and sits above an open modal', () => {
    const app = src('App.tsx');
    // FE-14 gave the toast z-50, the same as the modal overlay, so visibility
    // over a modal relied on DOM order. The shape stays spec'd by §6.6
    // (radius/border/mono); the stacking plane must not rely on DOM order:
    // z-[60] beats any z-50 overlay wherever the toast mounts.
    expect(app).toContain('z-[60]');
    expect(app).toContain('rounded-[3px]');
    expect(app.includes('z-[60] flex max-w-md')).toBe(true);
    const modal = src('components/Modal.tsx');
    expect(modal).toContain('z-50');
  });

  it('ach-grid matches DESIGN §6.3 tile sizing', () => {
    const css = src('style.css');
    expect(css).toContain('minmax(190px,220px)');
  });

  it('has no size_on_disk legacy field in frontend sources', () => {
    for (const f of ['types.ts', 'components/GamePage.tsx', 'components/LibraryView.tsx']) {
      expect(src(f).includes('size_on_disk')).toBe(false);
    }
  });

  it('ships no unreferenced nunito font asset', () => {
    const dir = path.join(__dirname, '../../assets/fonts');
    const files = fs.existsSync(dir) ? fs.readdirSync(dir) : [];
    expect(files.some((f) => f.toLowerCase().includes('nunito'))).toBe(false);
  });
});
