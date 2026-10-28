// i18n: converted
// The /22 Plugins page (issue #138): list + detail.
// Moved out of PluginsView.tsx (issue #156) so each file holds one presentation.
import { useTranslation, Trans } from 'react-i18next';
import { useState } from 'react';
import { Effect } from 'effect';
import { FolderOpen, Loader2, LogIn, LogOut, RefreshCw, RotateCcw, ShieldCheck } from 'lucide-react';
import type { Game, SourceWarning, StorePlugin } from '../types';
import { LauncherApi } from '../effect/runtime';
import { ATLAS_PAGE, cx } from './ui';
import { Button } from './ds/button';
import { PluginLogo } from './PluginLogo';
import { PluginSettingsPanel } from './PluginSettings';
import { offers, offerLabel } from '../lib/capabilities';
import { coreErrMsg } from '../lib/coreError';
import { DataPluginCard, PluginsBuiltIn, PluginsData, STATUS, hasAuth } from './PluginsView';
import type { PluginsAuthState, usePluginsData } from './usePluginsData';

/** The /22 Plugins page (issue #138, Atlas only): the plugin list on the left
 *  and the selected plugin's detail on the right, with the same cards,
 *  login rows, approve/enable actions and dialogs. The lab's "Add a store"
 *  button has no backing (store plugins install through the plugins folder),
 *  so the header keeps Rescan and the Plugins folder entry. */
export function AtlasPlugins({ plugins, games, warnings, auth, busy, data, onNotify, onAddStore }: {
  plugins: StorePlugin[];
  games: Game[];
  warnings: SourceWarning[];
  auth: Record<string, PluginsAuthState>;
  busy: string | null;
  data: ReturnType<typeof usePluginsData>;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  onAddStore: () => void;
}) {
  const { t } = useTranslation(['plugins', 'nav']);
  const store = plugins.filter((p) => p.kind === 'store');
  const rest = plugins.filter((p) => p.kind !== 'store');
  const [selected, setSelected] = useState(() => store.find((p) => p.status !== 'ready')?.id ?? store[0]?.id ?? rest[0]?.id ?? null);
  const current = plugins.find((p) => p.id === selected) ?? store[0] ?? rest[0];
  const statusDot = (p: StorePlugin): string | null => {
    switch (p.status) {
      case 'ready': { const a = auth[p.id]; return hasAuth(p) && a && !('error' in a) && !a.loggedIn ? 'bg-attention' : null; }
      case 'disabled': return 'bg-text-faint';
      case 'invalid': return 'bg-danger';
      case 'unapproved':
      case 'changed': return 'bg-attention';
      default: return 'bg-attention';
    }
  };
  const accountOf = (p: StorePlugin): string => {
    const a = auth[p.id];
    if (a && !('error' in a) && a.loggedIn) return a.user || t('plugins:auth.loggedIn');
    if (!hasAuth(p)) return p.capabilities.map(offerLabel).find((s): s is string => Boolean(s)) ?? '';
    return t('plugins:auth.loggedOut');
  };
  return (
    <div className={ATLAS_PAGE}>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[24px] font-extrabold tracking-tight">{t('nav:title.plugins')}</h1>
          <p className="mt-[3px] text-[13px] text-text-muted">{t('plugins:header.atlasSubtitle')}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="primary" onClick={onAddStore}>
            {t('plugins:header.addStore')}
          </Button>
          <Button variant="neutral" onClick={() => {
            Effect.runPromise(LauncherApi.openPluginsFolder()).catch((e) => onNotify(coreErrMsg(e), 'err'));
          }}>
            <FolderOpen className="h-3.5 w-3.5" /> {t('plugins:header.pluginsFolder')}
          </Button>
          <Button variant="neutral" onClick={() => { void data.refresh(true); }}>
            <RefreshCw className="h-3.5 w-3.5" /> {t('plugins:header.rescan')}
          </Button>
        </div>
      </header>
      <div className="grid items-start gap-5 lg:grid-cols-[300px_minmax(0,1fr)]">
        <ul className="grid gap-0.5" role="listbox" aria-label={t('nav:title.plugins')}>
          {store.map((p) => {
            const dot = statusDot(p);
            const on = current?.id === p.id;
            return (
              <li key={p.id}>
                <button
                  type="button"
                  role="option"
                  aria-selected={on}
                  onClick={() => setSelected(p.id)}
                  className={cx(
                    'flex w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                    on ? 'bg-[color-mix(in_srgb,var(--color-accent)_15%,var(--color-surface))]' : 'hover:bg-surface',
                  )}
                >
                  <span className="grid size-8 flex-none place-items-center overflow-hidden rounded-[9px] bg-surface-strong">
                    <PluginLogo src={p.icon_url} name={p.name} size="xs" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[13px] font-semibold">{p.name}</span>
                    <span className="block truncate text-[12px] text-text-faint">{accountOf(p)}</span>
                  </span>
                  {dot && <span aria-hidden className={cx('size-1.5 flex-none rounded-full', dot)} />}
                </button>
              </li>
            );
          })}
        </ul>
        <div className="min-w-0 rounded-2xl bg-surface p-5 ring-1 ring-hairline">
              {current && current.kind === 'store' ? (
            <AtlasPluginDetail
              plugin={current}
              gameCount={games.filter((g) => g.source === current.id).length}
              authState={auth[current.id]}
              warning={warnings.find((w) => w.source === current.id)}
              busy={busy}
              onLogout={() => data.logout(current)}
              onChooseFolder={() => data.chooseFolder(current)}
              onAct={data.act}
              onReview={() => data.setReviewing(current)}
              onLogin={() => data.setLoggingIn(current)}
              onUninstall={() => data.setUninstalling(current)}
              onRescan={() => { void data.refresh(true); }}
              onNotify={onNotify}
            />
          ) : current ? (
            <DataPluginCard plugin={current} onUninstall={() => data.setUninstalling(current)} onReview={() => data.setReviewing(current)} busy={busy} />
          ) : (
            <p className="text-[13px] text-text-muted">{t('plugins:empty.detail')}</p>
          )}
        </div>
      </div>
      {store.length > 0 && rest.length > 0 && <PluginsData plugins={rest} onUninstall={(p) => data.setUninstalling(p)} onReview={(p) => data.setReviewing(p)} busy={busy} />}
      <PluginsBuiltIn />
    </div>
  );
}

/** The selected store plugin's detail: the same identity, alert, approval,
 *  account, capabilities and folder rows as the card, without the card frame
 *  so the pane reads like the lab's detail column. */
export function AtlasPluginDetail({ plugin: p, gameCount, authState: a, warning, busy, onLogout, onChooseFolder, onAct, onReview, onLogin, onUninstall, onRescan, onNotify }: {
  plugin: StorePlugin;
  gameCount: number;
  authState: PluginsAuthState | undefined;
  warning: SourceWarning | undefined;
  busy: string | null;
  onLogout: () => void;
  onChooseFolder: () => void;
  onAct: (id: string, what: string, program: Effect.Effect<unknown, Error>, done: string) => void;
  onReview: () => void;
  onLogin: () => void;
  onUninstall: () => void;
  onRescan: () => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}) {
  const { t } = useTranslation('plugins');
  const st = STATUS[p.status] ?? STATUS.invalid;
  const dot: Record<string, string> = {
    ready: 'bg-ok',
    unapproved: 'bg-attention',
    changed: 'bg-attention',
    disabled: 'bg-text-faint',
    'missing-requirements': 'bg-attention',
    invalid: 'bg-danger',
    'needs-newer-platypus': 'bg-attention',
    'too-old': 'bg-attention',
  };
  const needsLogin = p.status === 'ready' && hasAuth(p) && a !== undefined && !('error' in a) && !a.loggedIn;
  const pill = p.status !== 'ready' || needsLogin;
  return (
    <div className="grid content-start gap-4">
      <header className="flex items-center gap-3.5">
        <span className="grid size-[46px] flex-none place-items-center overflow-hidden rounded-xl bg-surface-strong">
          <PluginLogo src={p.icon_url} name={p.name} size="md" />
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-[19px] font-extrabold tracking-tight">{p.name}</h2>
          <p className="truncate text-[13px] text-text-muted">
            {p.author
              ? `${t('plugins:atlas.versionBy', { version: p.version, author: p.author })}${p.kind === 'store' && gameCount > 0 ? `, ${t('plugins:atlas.gameCount', { count: gameCount })}` : ''}`
              : `Version ${p.version}${p.kind === 'store' && gameCount > 0 ? `, ${t('plugins:atlas.gameCount', { count: gameCount })}` : ''}`}
          </p>
        </div>
        {pill && (
          <span className="flex flex-none items-center gap-1.5 text-[12.5px] font-semibold text-attention">
            <span aria-hidden className={cx('size-1.5 rounded-full', dot[p.status] ?? 'bg-attention')} />
            {needsLogin ? t('plugins:atlas.loginNeeded') : t(st.key)}
          </span>
        )}
      </header>
      {p.problem && (
        <p className={cx('rounded-xl px-3.5 py-2.5 text-[12.5px] ring-1', p.status === 'invalid' ? 'bg-danger/10 text-danger ring-danger/25' : 'bg-attention/10 text-attention ring-attention/25')}>
          {p.problem}
        </p>
      )}
      {/* The capabilities count line stays in the classic card; the Atlas
          detail shows only the user-facing list below. */}
      {p.capabilities.length > 0 && (
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-[18px] border-b border-hairline py-3.5">
          <div className="grid min-w-0 gap-[3px]">
            <span className="text-[13px] font-semibold text-text">{t('plugins:atlas.brings')}</span>
            <span className="text-[0.93em] leading-normal text-text-muted">{p.capabilities.map((c) => offerLabel(c)).filter((s): s is string => s != null).join(', ')}</span>
          </div>
        </div>
      )}
      {(p.status === 'unapproved' || p.status === 'changed') && (
        <div className="flex flex-wrap gap-2">
          <Button variant="primary" onClick={onReview}>
            <ShieldCheck className="h-4 w-4" /> {p.status === 'changed' ? t('plugins:card.reapprove') : t('plugins:card.approve')}
          </Button>
        </div>
      )}
      {p.status === 'ready' && hasAuth(p) && (
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-[18px] border-b border-hairline py-3.5">
          <div className="grid min-w-0 gap-[3px]">
            <span className="text-[13px] font-semibold text-text">{t('plugins:atlas.account')}</span>
            <span className="truncate text-[0.93em] text-text-muted">
              {!a ? (
                <span className="flex items-center gap-2"><Loader2 className="h-3 w-3 animate-spin" /> {t('plugins:auth.checking')}</span>
              ) : 'error' in a ? (
                <span className="font-mono text-hint text-danger">{a.error}</span>
              ) : a.loggedIn ? (
                <>{a.user ? (
                  <Trans i18nKey="plugins:auth.loggedInAs" values={{ user: a.user }} components={{ b: <span className="font-bold text-text" /> }} />
                ) : t('plugins:auth.loggedIn')}</>
              ) : (
                <span className="text-attention">{warning?.needs_auth ? t('plugins:auth.loggedOutHidden') : t('plugins:auth.loggedOut')}</span>
              )}
            </span>
          </div>
          <span className="flex min-w-0 items-center gap-2">
            {a && !('error' in a) && a.loggedIn ? (
              <Button variant="neutral" size="sm" className="flex-none" onClick={onLogout} disabled={busy === p.id + 'logout'}>
                <LogOut className="h-3 w-3" /> {t('plugins:auth.logOut')}
              </Button>
            ) : (
              <Button variant="primary" size="sm" className="flex-none" onClick={onLogin}>
                <LogIn className="h-3 w-3" /> {t('plugins:auth.logIn')}
              </Button>
            )}
          </span>
        </div>
      )}
      {p.status === 'ready' && <PluginSettingsPanel plugin={p} onNotify={onNotify} />}
      {offers(p, 'install') && p.status !== 'invalid' && p.status !== 'unapproved' && (
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-[18px] border-b border-hairline py-3.5">
          <div className="grid min-w-0 gap-[3px]">
            <span className="text-[13px] font-semibold text-text">{t('plugins:atlas.libraryList')}</span>
            <span className="truncate text-[0.93em] text-text-faint select-text" title={p.install_folder}>{p.install_folder}</span>
          </div>
          <span className="flex flex-none gap-2">
            <Button variant="neutral" size="sm" onClick={onChooseFolder}>{t('plugins:card.change')}</Button>
            <Button variant="neutral" size="sm" onClick={onRescan}>
              <RefreshCw className="h-3 w-3" /> {t('plugins:header.rescan')}
            </Button>
          </span>
        </div>
      )}
      {(p.status === 'ready' || p.status === 'missing-requirements' || p.status === 'disabled' || p.gave_up) && (
        <div className="flex gap-2">
          {(p.status === 'unapproved' || p.status === 'changed') ? null : (p.status === 'ready' || p.status === 'missing-requirements') ? (
            <Button variant="neutral" onClick={() => onAct(p.id, 'toggle', LauncherApi.setPluginEnabled(p.id, false), t('plugins:notify.disabled', { name: p.name }))} disabled={busy === p.id + 'toggle'}>
              {t('plugins:card.disable')}
            </Button>
          ) : p.status === 'disabled' ? (
            <Button variant="primary" onClick={() => onAct(p.id, 'toggle', LauncherApi.setPluginEnabled(p.id, true), t('plugins:notify.enabled', { name: p.name }))} disabled={busy === p.id + 'toggle'}>
              {t('plugins:card.enable')}
            </Button>
          ) : null}
          {p.gave_up && p.status === 'ready' && (
            <Button variant="neutral" onClick={() => onAct(p.id, 'retry', LauncherApi.retryPlugin(p.id), t('plugins:notify.willRetry', { name: p.name }))} disabled={busy === p.id + 'retry'}>
              <RotateCcw className="h-3.5 w-3.5" /> {t('common:retry')}
            </Button>
          )}
          {!p.bundled && (
            <Button variant="neutral" onClick={onUninstall} disabled={busy === p.id + 'uninstall'}>
              {t('plugins:card.uninstall')}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
