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
    // Issue #64: the toast layer is Sonner (ds/sonner.tsx + style.css), not
    // the old bottom notice in App.tsx. The shape stays spec'd by §6.6
    // (radius/border/mono); Sonner's own z-index (999999999) beats any z-50
    // overlay wherever the toast mounts.
    const sonner = src('components/ds/sonner.tsx');
    expect(sonner).toContain('rounded-[3px]');
    expect(sonner).toContain('bg-[#191410]');
    expect(sonner).toContain('font-mono');
    expect(sonner).toContain('top-right');
    const css = src('style.css');
    expect(css).toContain('platypus-toast');
    const app = src('App.tsx');
    expect(app).toContain('Toaster');
    expect(app.includes('fixed bottom-4 right-4')).toBe(false);
    const modal = src('components/ds/dialog.tsx');
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
