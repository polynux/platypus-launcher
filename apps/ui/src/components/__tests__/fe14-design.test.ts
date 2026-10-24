// @ts-nocheck — node fs/path guard test, outside the app's DOM tsconfig.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

const src = (p: string) => fs.readFileSync(path.join(__dirname, '../..', p), 'utf8');

describe('FE-14 design drift guards', () => {
  it('uses backdrop-blur only on hero stat chips (the Hero frame), not JobProgress', () => {
    const gamePage = src('components/GamePage.tsx');
    const hero = src('components/Hero.tsx');
    expect(gamePage.includes('backdrop-blur')).toBe(false);
    // Hero stat chips explicitly may keep one per DESIGN §1.1/§6.1. The frame
    // serves the dashboard's glass chips (issue #101); the game page default
    // is solid, so GamePage stays blur-free.
    expect(hero.includes('backdrop-blur')).toBe(true);
  });

  it('toast uses semantic launcher tokens and sits above an open modal', () => {
    // Issue #64: the toast layer is Sonner (ds/sonner.tsx + style.css), not
    // the old bottom notice in App.tsx. It is styled like the other floating
    // panels (dialog surface, JobProgress), not a terminal box; Sonner's own
    // z-index (999999999) beats any z-50 overlay wherever the toast mounts.
    const sonner = src('components/ds/sonner.tsx');
    expect(sonner).toContain('rounded-2xl');
    expect(sonner).toContain('bg-raised');
    expect(sonner).toContain('text-text');
    expect(sonner).toContain('font-sans');
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

  it('defines the token-set-v1 semantic colors and radii', () => {
    const css = src('style.css');
    for (const token of [
      '--color-surface:', '--color-surface-strong:', '--color-hairline:',
      '--color-text-muted:', '--color-danger:', '--color-focus:',
    ]) expect(css).toContain(token);
    for (const token of ['--radius-chip:', '--radius-card:', '--radius-panel:']) expect(css).not.toContain(token);
  });

  it('does not reintroduce migrated raw palette classes', () => {
    const files = [
      'App.tsx', 'components/ui.tsx', 'components/Sidebar.tsx',
      'components/JobProgress.tsx', 'components/Modal.tsx',
      'components/Dashboard.tsx', 'components/LibraryView.tsx',
      'components/GamePage.tsx', 'components/SettingsModal.tsx',
      'components/PluginsView.tsx',
      ...fs.readdirSync(path.join(__dirname, '../ds'))
        .filter((f) => f.endsWith('.tsx') && !f.endsWith('.test.tsx'))
        .map((f) => `components/ds/${f}`),
    ];
    const rawPalette = /(?:text-white\/(?:90|70|60|55|40)|placeholder-white\/55|bg-white\/\[(?:0\.(?:03|04|045|05|06))\]|ring-white\/\[(?:0\.(?:06|07|08))\]|(?:border|divide)-white\/\[(?:0\.(?:05|06|07))\]|(?:bg|text|ring|border|divide)-amber-(?:300|400)|(?:bg|text|ring|border|divide)-(?:red-(?:400|500)|orange-400|sky-300|violet-400)|(?:bg|text)-\[#(?:191410|130f0a|ece6da|7dd3fc|a78bfa)\])/;
    for (const file of files) {
      expect(src(file).match(rawPalette), `${file} contains a raw palette class`).toBeNull();
    }
  });
});
