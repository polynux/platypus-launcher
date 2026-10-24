import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Effect } from 'effect';
import React, { useState } from 'react';
import type { Game, StorePlugin, Tweak } from '../types';

const state = vi.hoisted(() => ({
  variants: {
    'game-settings.presentation': 'dialog',
    'game-settings.tabs': 'vertical',
    'section.chrome': 'hairline',
    'dialog.presentation': 'dialog',
    'dialog.size': 'auto',
  } as Record<string, string>,
}));
const api = vi.hoisted(() => ({
  getGameDetails: vi.fn(),
  getInstallSize: vi.fn(),
  getGameOptions: vi.fn(),
  getCloudInfo: vi.fn(),
  checkCloudSaves: vi.fn(),
  getGameSettingsFields: vi.fn(),
  getGameVersions: vi.fn(),
  getCustomization: vi.fn(),
  previewCommand: vi.fn(),
  getLauncherSettings: vi.fn(),
}));

vi.mock('../lib/theme', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/theme')>()),
  useVariant: (axis: string) => state.variants[axis],
}));
vi.mock('../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
}));
vi.mock('./SettingsModal', () => ({
  GeneralSettings: () => <div>General fixture</div>,
  useGameSettingsDraft: (game: Game) => ({ current: game.settings, applySaved: () => {} }),
}));
vi.mock('./LegalSection', () => ({ LegalSection: () => null }));
vi.mock('./maintenancePrefetch', () => ({
  MaintenancePrefetchProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useMaintenancePrefetchSource: () => ({}),
  useMaintenancePrefetch: () => null,
}));

import { GameSettingsModal } from './GameSettingsModal';

expect.extend(matchers);
afterEach(() => { cleanup(); });
// Radix attaches its outside-pointer listener on the next tick.
const tick = () => new Promise((r) => setTimeout(r, 10));
// Radix defers outside-dismiss to the click that follows the primary pointerdown.
const clickBackdrop = (el: Element) => {
  fireEvent.pointerDown(el, { pointerType: 'mouse', button: 0 });
  fireEvent.click(el);
};
beforeEach(() => {
  state.variants['game-settings.presentation'] = 'dialog';
  state.variants['game-settings.tabs'] = 'vertical';
  state.variants['dialog.size'] = 'auto';
  Object.values(api).forEach((f) => f.mockReset());
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getInstallSize.mockResolvedValue(0);
  api.getCloudInfo.mockResolvedValue({ mode: 'ask', plugin_mode: 'ask', game_mode: '', running: false, status: { state: 'synced', checkedAt: 1, counts: { same: 1, download: 0, upload: 0, conflict: 0 } } });
  api.getGameSettingsFields.mockResolvedValue({ groups: [], values: {} });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
  api.getGameOptions.mockResolvedValue({ groups: [], chosen: {} });
  api.getCustomization.mockResolvedValue({ title: '', sort_title: '', art: {} });
  api.previewCommand.mockResolvedValue('wine game.exe');
  api.getLauncherSettings.mockResolvedValue({ prefixes_folder: '', default_prefixes_folder: '/home/you/.local/share/platypus/prefixes' });
});

const ALL = ['install', 'cloud-saves', 'versions', 'verify', 'install-options', 'settings', 'move'];
const testPlugin = (): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: ALL, declared: ALL, install_folder: '/games' }) as StorePlugin;
const testGame = (): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '/games/echo/tiny', executable: '', is_installed: true, installable: true, features: null,
  settings: { runner_id: 'auto', env_vars: {}, launch_arguments: '' },
}) as Game;
const testTweak = { id: 't1', kind: 'recommended', title: 'Use fsync', reason: 'faster', env: { DXVK_ASYNC: '1' } } as unknown as Tweak;

function modalProps(over: Partial<React.ComponentProps<typeof GameSettingsModal>> = {}) {
  return {
    game: testGame(), plugin: testPlugin(), runners: [], tweaks: [testTweak], jobs: [],
    onClose: () => {}, onSaveSettings: () => {}, onChangeSettings: () => {}, onJob: () => {}, onJobAction: () => {},
    onNotify: () => {}, onChanged: () => {}, onCustomized: () => {}, onCopied: () => {},
    ...over,
  };
}

describe('Game settings catalog variants', () => {
  for (const presentation of ['dialog', 'sheet', 'page']) {
    for (const tabs of ['vertical', 'horizontal']) {
      it(`renders game settings as ${presentation} with ${tabs} tabs without axe violations`, async () => {
        state.variants['game-settings.presentation'] = presentation;
        state.variants['game-settings.tabs'] = tabs;
        // A page lives in the app's main landmark (GamePage renders inside
        // App's <main>); a dialog brings its own dialog role.
        const ui = <GameSettingsModal {...modalProps()} />;
        render(presentation === 'page' ? <main>{ui}</main> : ui);
        if (presentation === 'page') {
          expect(screen.queryByRole('dialog')).toBeNull();
          expect(screen.getByRole('button', { name: 'Back' })).toBeInTheDocument();
          expect(screen.getByRole('heading', { name: 'Game settings' })).toBeInTheDocument();
        } else {
          expect(screen.getByRole('dialog', { name: 'Settings for Echo Tiny' })).toBeInTheDocument();
        }
        expect(screen.getByRole('tablist', { name: 'Game settings sections' }))
          .toHaveAttribute('aria-orientation', tabs);
        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(
          ['General', 'Cloud saves', 'Versions', 'Add-ons & options', 'Plugin settings', 'Suggested tweaks', 'Customization', 'Maintenance'],
        );
        // The dialog portals outside the render container, so axe runs on the
        // body: the dialog content is what must be violation-free.
        (expect(await (await import('vitest-axe')).axe(document.body)) as any).toHaveNoViolations();
      });
    }
  }

  it('traps focus, closes with Escape and restores focus to the settings gear', async () => {
    const user = userEvent.setup();
    const closed = vi.fn();
    function Host() {
      const [open, setOpen] = useState(false);
      return <><button onClick={() => setOpen(true)}>Game settings</button>{open && <GameSettingsModal {...modalProps({ onClose: () => { closed(); setOpen(false); } })} />}</>;
    }
    render(<Host />);
    const trigger = screen.getByRole('button', { name: 'Game settings' });
    await user.click(trigger);
    expect(screen.getByRole('dialog', { name: 'Settings for Echo Tiny' })).toBeInTheDocument();
    await user.tab();
    expect(trigger).not.toHaveFocus();
    await user.keyboard('{Escape}');
    expect(closed).toHaveBeenCalledOnce();
    expect(trigger).toHaveFocus();
  });

  it.each(['vertical', 'horizontal'] as const)('moves focus between tabs with arrow keys (%s)', (tabs) => {
    state.variants['game-settings.tabs'] = tabs;
    render(<GameSettingsModal {...modalProps()} />);
    const general = screen.getByRole('tab', { name: 'General' });
    fireEvent.keyDown(general, { key: tabs === 'vertical' ? 'ArrowDown' : 'ArrowRight' });
    expect(screen.getByRole('tab', { name: 'Cloud saves', selected: true })).toBeTruthy();
    expect(document.getElementById('gs-tab-cloud')).toHaveFocus();
  });

  it('keeps horizontal tabs on one scrolling row with an edge fade', () => {
    state.variants['game-settings.tabs'] = 'horizontal';
    render(<GameSettingsModal {...modalProps()} />);
    const tablist = screen.getByRole('tablist', { name: 'Game settings sections' });
    expect(tablist.className).toMatch(/\bflex-nowrap\b/);
    expect(tablist.className).toMatch(/(^|\s)overflow-x-auto(\s|$)/);
    expect(tablist.className).not.toMatch(/flex-wrap/);
    expect(tablist.className).toMatch(/mask-image/);
    for (const tab of screen.getAllByRole('tab')) expect(tab).toHaveClass('shrink-0');
  });

  it('scrolls the active horizontal tab into view', () => {
    state.variants['game-settings.tabs'] = 'horizontal';
    const spy = vi.spyOn(Element.prototype, 'scrollIntoView').mockImplementation(() => {});
    try {
      render(<GameSettingsModal {...modalProps()} />);
      fireEvent.click(screen.getByRole('tab', { name: 'Maintenance' }));
      expect(spy).toHaveBeenCalledWith(expect.objectContaining({ block: 'nearest', inline: 'nearest' }));
      expect(spy.mock.contexts).toContain(document.getElementById('gs-tab-maintenance'));
    } finally {
      spy.mockRestore();
    }
  });

  it('honours an explicit dialog size and keeps xl for auto', () => {
    state.variants['dialog.size'] = 'lg';
    const { unmount } = render(<GameSettingsModal {...modalProps()} />);
    expect(screen.getByRole('dialog')).toHaveClass('max-w-[720px]');
    unmount();
    state.variants['dialog.size'] = 'auto';
    render(<GameSettingsModal {...modalProps()} />);
    expect(screen.getByRole('dialog')).toHaveClass('max-w-[920px]');
  });

  it('goes back to the game from the page presentation', async () => {
    state.variants['game-settings.presentation'] = 'page';
    const user = userEvent.setup();
    const closed = vi.fn();
    render(<GameSettingsModal {...modalProps({ onClose: closed })} />);
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(closed).toHaveBeenCalledOnce();
  });

  // The accordion variant (issue #129): a 520px drawer from the right, with
  // the categories grouped Everyday / Advanced and one open at a time.
  describe('accordion', () => {
    const open = () => {
      state.variants['game-settings.presentation'] = 'sheet';
      state.variants['game-settings.tabs'] = 'accordion';
    };

    it('renders the categories in Everyday and Advanced groups in the drawer, without axe violations', async () => {
      open();
      render(<GameSettingsModal {...modalProps()} />);
      const dialog = screen.getByRole('dialog', { name: 'Settings for Echo Tiny' });
      expect(dialog).toHaveStyle({ width: '520px' });
      expect(screen.getByRole('heading', { name: 'All settings' })).toBeInTheDocument();
      expect(screen.queryByRole('tablist')).toBeNull();
      expect(screen.getByText('Everyday')).toBeInTheDocument();
      expect(screen.getByText('Advanced')).toBeInTheDocument();
      const headers = screen.getAllByRole('button').filter((b) => b.hasAttribute('data-gs-accordion-header'));
      expect(headers.map((h) => h.id)).toEqual([
        'gs-acc-general', 'gs-acc-cloud', 'gs-acc-customization',
        'gs-acc-versions', 'gs-acc-options', 'gs-acc-plugin', 'gs-acc-tweaks', 'gs-acc-maintenance',
      ]);
      // Each header carries its one-line summary of the current values.
      const summaries = headers.map((h) => h.textContent ?? '');
      expect(summaries[0]).toContain('Automatic runner');
      expect(summaries[1]).toContain('Synced by Platypus');
      expect(summaries[2]).toContain('Art, title and sort');
      expect(summaries[6]).toContain('1 suggested');
      expect(summaries[7]).toContain('Verify, move, backups');
      // One section open at a time: General is the first, the rest are closed.
      expect(document.getElementById('gs-acc-general')).toHaveAttribute('aria-expanded', 'true');
      for (const id of ['cloud', 'versions', 'options', 'plugin', 'tweaks', 'customization', 'maintenance']) {
        expect(document.getElementById(`gs-acc-${id}`)).toHaveAttribute('aria-expanded', 'false');
      }
      (expect(await (await import('vitest-axe')).axe(document.body)) as any).toHaveNoViolations();
    });

    it('opens one section at a time and closes it again', async () => {
      open();
      const user = userEvent.setup();
      render(<GameSettingsModal {...modalProps()} />);
      const versions = screen.getByRole('button', { name: /^Versions/ });
      await user.click(versions);
      expect(versions).toHaveAttribute('aria-expanded', 'true');
      expect(document.getElementById('gs-acc-general')).toHaveAttribute('aria-expanded', 'false');
      expect(document.getElementById('gs-acc-panel-versions')).not.toBeNull();
      await user.click(versions);
      expect(versions).toHaveAttribute('aria-expanded', 'false');
      expect(document.getElementById('gs-acc-panel-versions')).toBeNull();
    });

    it('starts focus on the first header, then Escape and a scrim click close it', async () => {
      open();
      const closed = vi.fn();
      render(<GameSettingsModal {...modalProps({ onClose: closed })} />);
      await tick();
      expect(document.getElementById('gs-acc-general')).toHaveFocus();
      fireEvent.keyDown(document, { key: 'Escape' });
      expect(closed).toHaveBeenCalledTimes(1);
      clickBackdrop(screen.getByRole('dialog').parentElement!);
      expect(closed).toHaveBeenCalledTimes(2);
    });

    it('keeps the standard sheet width for the tab variants', () => {
      state.variants['game-settings.presentation'] = 'sheet';
      render(<GameSettingsModal {...modalProps()} />);
      expect(screen.getByRole('dialog').style.width).toBe('');
    });
  });
});
