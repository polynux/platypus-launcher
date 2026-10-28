// Persisted feed rows (issues #145, #151; review A4, A8): the real converter
// maps core rows to UI rows by game id, yesterday's session renders on the
// Activity page and on its game page, and other games' rows stay out.
import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render as rtlRender, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { ActivityEvent, Game } from '../../types';
import type { types as gen } from '../../gen/models';

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return { ...mod, useVariant: () => 'cards', useThemeLayout: () => null };
});

const details = {
  summary: '', screenshots: [], genres: [], developers: [], publishers: [],
  release_date: '', hero_url: '', achievements: null,
};

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameDetails: () => Effect.succeed(details as never),
    getCrashHints: () => Effect.succeed(null),
    getInstallSize: () => Effect.succeed({ state: 'known', total: 1500000000, added: 0 } as never),
    getGameOptions: () => Effect.succeed({ groups: [], chosen: {} } as never),
    getGameAchievements: () => Effect.fail(new Error('no achievements')),
    getCloudInfo: () => Effect.succeed({ mode: 'auto', plugin_mode: 'auto', game_mode: '', status: null, running: false } as never),
    checkCloudSaves: () => Effect.succeed({ mode: 'auto', plugin_mode: 'auto', game_mode: '', status: null, running: false } as never),
    listSaveBackups: () => Effect.succeed([] as never),
    setGameStatus: () => Effect.succeed({ status: 'backlog', user_notes: '' } as never),
    setGameNotes: () => Effect.succeed(undefined as never),
    setPreferredCopy: () => Effect.succeed(undefined as never),
    listActivity: () => Effect.succeed([] as never),
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onChat: () => () => {},
  onPlaytime: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
}));

vi.mock('../../lib/compat', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/compat')>();
  return { ...mod, gameCompat: () => Promise.resolve(null), setGameCompat: () => {} };
});

import { toActivityRow } from '../../App';
import { ActivityView } from '../ActivityView';
import { GamePage, invalidateGameDetailsCache } from '../GamePage';

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'echo', store: 'echo', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '/g', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const t = (key: string, opts?: Record<string, unknown>): string => i18n.t(key, opts) as unknown as string;
const byId = (games: Game[]) => new Map(games.map((g) => [g.id, g]));

const yesterday = Date.now() - 26 * 60 * 60 * 1000;
const coreSession = {
  id: 7, ts: yesterday, kind: 'session', game_id: 'g1',
  text_key: 'activity:log.sessionStopped', args: { seconds: '1800' },
} as unknown as gen.ActivityEvent;

beforeEach(() => {
  invalidateGameDetailsCache();
  return i18n.changeLanguage('en');
});

describe('toActivityRow (the real converter)', () => {
  it('maps a yesterday session row by game id', () => {
    const row = toActivityRow(coreSession, byId([mk('g1', { title: 'Hades' })]), t);
    expect(row.game).toBe('g1');
    expect(row.ts).toBe(yesterday);
    expect(row.type).toBe('session');
    expect(row.detail).toContain('Hades');
    expect(row.meta).toContain('30');
  });

  it('keeps stable ids when the library refreshes', () => {
    const a = toActivityRow(coreSession, byId([mk('g1', { title: 'Hades' })]), t);
    const b = toActivityRow(coreSession, byId([mk('g1', { title: 'Hades II' }), mk('g2', { title: 'Other' })]), t);
    expect(a.id).toBe(b.id);
  });

  it('renders the cloud file count, not the English message', () => {
    const e = {
      id: 9, ts: Date.now(), kind: 'cloud', game_id: 'g1',
      text_key: 'activity:log.cloudDownloaded', args: { files: '3' },
    } as unknown as gen.ActivityEvent;
    const row = toActivityRow(e, byId([mk('g1', { title: 'Hades' })]), t);
    expect(row.detail).toContain('3');
    expect(row.detail).toContain('Hades');
  });

  it('uses the translated verb, not the stored action', () => {
    const e = {
      id: 10, ts: Date.now(), kind: 'verify', game_id: 'g1',
      text_key: 'activity:log.jobDone', args: { store: 'echo' },
    } as unknown as gen.ActivityEvent;
    const en = toActivityRow(e, byId([mk('g1', { title: 'Hades' })]), t);
    expect(en.detail).toContain('verified');
    expect(en.detail).not.toContain('undefined');
  });
});

describe('persisted rows on the pages (A4)', () => {
  it('activity page lists yesterday’s session', () => {
    const row = toActivityRow(coreSession, byId([mk('g1', { title: 'Hades' })]), t);
    render(<ActivityView events={[row]} games={[mk('g1', { title: 'Hades' })]} />);
    expect(screen.getByText(/Hades/)).toBeTruthy();
  });

  it('game page shows its own persisted session, not another game’s', async () => {
    const g1 = mk('g1', { title: 'Hades' });
    const other: ActivityEvent = {
      id: 2, ts: yesterday, type: 'session', game: 'g2',
      detail: 'Stopped Other', meta: '30 min session',
    };
    const row = toActivityRow(coreSession, byId([g1, mk('g2', { title: 'Other' })]), t);
    const base = {
      runners: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {},
      launching: false, tweaks: [], onChangeSettings() {}, onSaveSettings: () => Promise.resolve(),
      onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {}, jobs: [], onNotify() {},
      collections: [], onManageCollections: () => {},
    };
    render(
      <GamePage
        game={g1} runners={base.runners as never} activity={[row, other]}
        {...base} onCustomized={() => {}} sourceLogos={{}}
      />,
    );
    await waitFor(() => expect(screen.getByText(row.detail)).toBeTruthy(), { timeout: 5000 });
    expect(screen.queryByText('Stopped Other')).toBeNull();
  });
});
