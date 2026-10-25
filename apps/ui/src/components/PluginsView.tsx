// i18n: converted
import { useTranslation, Trans } from 'react-i18next';
import React, { useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import { ExternalLink, FolderOpen, Loader2, LogIn, LogOut, RefreshCw, RotateCcw, ShieldAlert, ShieldCheck } from 'lucide-react';
import type { AuthPrompt, AuthStatus, PluginStatus, SourceWarning, StorePlugin } from '../types';
import { LauncherApi, onAuth } from '../effect/runtime';
import { openExternal } from '../lib/external';
import { Modal } from './Modal';
import { EmptyState, MonoLabel, Panel, ViewHeader, cx } from './ui';
import { Button } from './ds/button';
import { Badge, type badgeVariants } from './ds/badge';
import { Input } from './ds/input';
import { PluginCapabilities } from './PluginCapabilities';
import { PluginLogo } from './PluginLogo';
import { PluginSettingsPanel } from './PluginSettings';
import { offers } from '../lib/capabilities';
import type { VariantProps } from 'class-variance-authority';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';
import { ThemeLayout } from '../lib/layout';
import { useThemeLayout, useVariant } from '../lib/theme';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';
import { usePluginsData, type PluginsAuthState } from './usePluginsData';

interface PluginsViewProps {
  plugins: StorePlugin[];
  warnings: SourceWarning[];
  onPlugins: (p: StorePlugin[]) => void;
  /** Approval, login and folder changes change what the library lists. */
  onLibraryChanged: () => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}

type BadgeVariant = NonNullable<VariantProps<typeof badgeVariants>['variant']>;
const STATUS: Record<PluginStatus, { key: string; variant: BadgeVariant }> = {
  ready: { key: 'plugins:status.approved', variant: 'ok' },
  unapproved: { key: 'plugins:status.needsApproval', variant: 'gold' },
  changed: { key: 'plugins:status.changed', variant: 'bronze' },
  disabled: { key: 'plugins:status.disabled', variant: 'neutral' },
  'missing-requirements': { key: 'plugins:status.missingRequirements', variant: 'bronze' },
  invalid: { key: 'plugins:status.invalid', variant: 'borked' },
  'needs-newer-platypus': { key: 'plugins:status.needsNewer', variant: 'bronze' },
  'too-old': { key: 'plugins:status.tooOld', variant: 'bronze' },
};

const BUILT_IN = [
  { key: 'protondb', name: 'ProtonDB' },
];

const hasAuth = (p: StorePlugin) => offers(p, 'auth');

/** Forgets every cached login state (tests). Kept here so existing imports
 *  keep working; the cache itself lives in the view-owned data hook. */
export { resetPluginsAuthCache } from './usePluginsData';

export function PluginsView({ plugins, warnings, onPlugins, onLibraryChanged, onNotify }: PluginsViewProps) {
  const { t } = useTranslation(['plugins', 'common']);
  const data = usePluginsData({ plugins, onPlugins, onLibraryChanged, onNotify });
  const { reviewing, setReviewing, loggingIn, setLoggingIn, auth, busy, storeAuth, refresh, act } = data;
  const atlas = useVariant('shell.navigation') === 'sourcebar';
  // The built-in arrangement is the initial layout, so the first paint
  // already shows today's plugins before the core answers over RPC.
  const layout = useThemeLayout('plugins') ?? BUILTIN_LAYOUTS['plugins'];
  if (atlas) {
    return (
      <>
        <AtlasPlugins
          plugins={plugins}
          warnings={warnings}
          auth={auth}
          busy={busy}
          data={data}
          onNotify={onNotify}
        />
        {reviewing && (
          <ApproveModal
            plugin={reviewing}
            onClose={() => setReviewing(null)}
            onApprove={() => {
              const p = reviewing;
              setReviewing(null);
              act(p.id, 'approve', LauncherApi.approvePlugin(p.id), t('plugins:notify.approved', { name: p.name }));
            }}
          />
        )}
        {loggingIn && (
          <AuthModal
            plugin={loggingIn}
            onClose={() => setLoggingIn(null)}
            onDone={(s) => {
              const p = loggingIn;
              setLoggingIn(null);
              storeAuth(p.id, s);
              onNotify(s.user ? t('plugins:notify.loggedInAs', { name: p.name, user: s.user }) : t('plugins:notify.loggedIn', { name: p.name }));
              onLibraryChanged();
            }}
          />
        )}
      </>
    );
  }
  return (
    <div className="p-7">
      <ThemeLayout
        layout={layout}
        blocks={pluginsBlocks({ plugins, warnings, auth, busy, data, onNotify, onLibraryChanged })}
      />

      {reviewing && (
        <ApproveModal
          plugin={reviewing}
          onClose={() => setReviewing(null)}
          onApprove={() => {
            const p = reviewing;
            setReviewing(null);
            act(p.id, 'approve', LauncherApi.approvePlugin(p.id), t('plugins:notify.approved', { name: p.name }));
          }}
        />
      )}
      {loggingIn && (
        <AuthModal
          plugin={loggingIn}
          onClose={() => setLoggingIn(null)}
          onDone={(s) => {
            const p = loggingIn;
            setLoggingIn(null);
            storeAuth(p.id, s);
            onNotify(s.user ? t('plugins:notify.loggedInAs', { name: p.name, user: s.user }) : t('plugins:notify.loggedIn', { name: p.name }));
            onLibraryChanged();
          }}
        />
      )}
    </div>
  );
}

// The plugins' layout blocks (issue #102): each takes its data as props only
// and renders a single root, so a layout orders blocks without conditionals
// of its own. The store block renders each card with its own login row and
// approve/enable actions, so a layout can't hide the required parts on their
// own; the approve and login dialogs stay owned by the view.
function pluginsBlocks({ plugins, warnings, auth, busy, data, onNotify, onLibraryChanged }: {
  plugins: StorePlugin[];
  warnings: SourceWarning[];
  auth: Record<string, PluginsAuthState>;
  busy: string | null;
  data: ReturnType<typeof usePluginsData>;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  onLibraryChanged: () => void;
}): Record<string, React.ReactNode> {
  return {
    header: <PluginsHeader onRescan={() => { void data.refresh(true).then(onLibraryChanged); }} onOpenFolder={() => {
      Effect.runPromise(LauncherApi.openPluginsFolder()).catch((e) => onNotify(coreErrMsg(e), 'err'));
    }} />,
    store: (
      <PluginsStore
        plugins={plugins.filter((p) => p.kind === 'store')}
        warnings={warnings}
        auth={auth}
        busy={busy}
        onLogout={data.logout}
        onChooseFolder={data.chooseFolder}
        onAct={data.act}
        onReview={(p) => data.setReviewing(p)}
        onLogin={(p) => data.setLoggingIn(p)}
        onNotify={onNotify}
      />
    ),
    data: <PluginsData plugins={plugins.filter((p) => p.kind === 'data')} />,
    builtin: <PluginsBuiltIn />,
  };
}

export function PluginsHeader({ onRescan, onOpenFolder }: { onRescan: () => void; onOpenFolder: () => void }) {
  const { t } = useTranslation(['plugins', 'nav']);
  return (
    <ViewHeader
      title={t('nav:title.plugins')}
      subtitle={<p className="mt-0.5 max-w-xl text-[12.5px] text-text-body">
        {t('plugins:header.subtitle')}
      </p>}
      actions={<div className="flex gap-2">
        <Button variant="neutral" onClick={onOpenFolder}>
          <FolderOpen className="h-3.5 w-3.5" /> {t('plugins:header.pluginsFolder')}
        </Button>
        <Button variant="neutral" onClick={onRescan}>
          <RefreshCw className="h-3.5 w-3.5" /> {t('plugins:header.rescan')}
        </Button>
      </div>}
    />
  );
}

export function PluginsStore({ plugins, warnings, auth, busy, onLogout, onChooseFolder, onAct, onReview, onLogin, onNotify }: {
  plugins: StorePlugin[];
  warnings: SourceWarning[];
  auth: Record<string, PluginsAuthState>;
  busy: string | null;
  onLogout: (p: StorePlugin) => void;
  onChooseFolder: (p: StorePlugin) => void;
  onAct: (id: string, what: string, program: Effect.Effect<unknown, Error>, done: string) => void;
  onReview: (p: StorePlugin) => void;
  onLogin: (p: StorePlugin) => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}) {
  const { t } = useTranslation('plugins');
  // One root: the heading and the cards (or the empty state) stay together,
  // so a row counts the store block as one column.
  return (
    <div>
      <MonoLabel as="h2" className="mb-3">{t('plugins:section.store')}</MonoLabel>
      {plugins.length === 0 ? (
        <EmptyState label={t('plugins:empty.label')}>
          {t('plugins:empty.detail')}
        </EmptyState>
      ) : (
        <div className="grid gap-4 xl:grid-cols-2">
          {plugins.map((p) => (
            <StorePluginCard
              key={p.id}
              plugin={p}
              authState={auth[p.id]}
              warning={warnings.find((w) => w.source === p.id)}
              busy={busy}
              onLogout={() => onLogout(p)}
              onChooseFolder={() => onChooseFolder(p)}
              onAct={onAct}
              onReview={() => onReview(p)}
              onLogin={() => onLogin(p)}
              onNotify={onNotify}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/** The /22 Plugins page (issue #138, Atlas only): the plugin list on the left
 *  and the selected plugin's detail on the right, with the same cards,
 *  login rows, approve/enable actions and dialogs. The lab's "Add a store"
 *  button has no backing (store plugins install through the plugins folder),
 *  so the header keeps Rescan and the Plugins folder entry. */
function AtlasPlugins({ plugins, warnings, auth, busy, data, onNotify }: {
  plugins: StorePlugin[];
  warnings: SourceWarning[];
  auth: Record<string, PluginsAuthState>;
  busy: string | null;
  data: ReturnType<typeof usePluginsData>;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}) {
  const { t } = useTranslation(['plugins', 'nav']);
  const store = plugins.filter((p) => p.kind === 'store');
  const rest = plugins.filter((p) => p.kind !== 'store');
  const [selected, setSelected] = useState(() => store.find((p) => p.status !== 'ready')?.id ?? store[0]?.id ?? rest[0]?.id ?? null);
  const current = plugins.find((p) => p.id === selected) ?? store[0] ?? rest[0];
  const statusDot = (p: StorePlugin): string | null => {
    switch (p.status) {
      case 'ready': return null;
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
    return t('plugins:auth.loggedOut');
  };
  return (
    <div className="mx-auto grid w-full max-w-6xl content-start gap-6 px-7 py-7">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[24px] font-extrabold tracking-tight">{t('nav:title.plugins')}</h1>
          <p className="mt-1 text-[13px] text-text-muted">{t('plugins:header.subtitle')}</p>
        </div>
        <div className="flex gap-2">
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
                  <span className="grid h-7 w-7 flex-none place-items-center overflow-hidden rounded-lg bg-surface-strong">
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
              authState={auth[current.id]}
              warning={warnings.find((w) => w.source === current.id)}
              busy={busy}
              onLogout={() => data.logout(current)}
              onChooseFolder={() => data.chooseFolder(current)}
              onAct={data.act}
              onReview={() => data.setReviewing(current)}
              onLogin={() => data.setLoggingIn(current)}
              onRescan={() => { void data.refresh(true); }}
              onNotify={onNotify}
            />
          ) : current ? (
            <DataPluginCard plugin={current} />
          ) : (
            <p className="text-[13px] text-text-muted">{t('plugins:empty.detail')}</p>
          )}
        </div>
      </div>
      {store.length > 0 && rest.length > 0 && <PluginsData plugins={rest} />}
      <PluginsBuiltIn />
    </div>
  );
}

/** The selected store plugin's detail: the same identity, alert, approval,
 *  account, capabilities and folder rows as the card, without the card frame
 *  so the pane reads like the lab's detail column. */
function AtlasPluginDetail({ plugin: p, authState: a, warning, busy, onLogout, onChooseFolder, onAct, onReview, onLogin, onRescan, onNotify }: {
  plugin: StorePlugin;
  authState: PluginsAuthState | undefined;
  warning: SourceWarning | undefined;
  busy: string | null;
  onLogout: () => void;
  onChooseFolder: () => void;
  onAct: (id: string, what: string, program: Effect.Effect<unknown, Error>, done: string) => void;
  onReview: () => void;
  onLogin: () => void;
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
        <span className="grid h-7 w-7 flex-none place-items-center overflow-hidden rounded-lg bg-surface-strong">
          <PluginLogo src={p.icon_url} name={p.name} size="xs" />
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-[19px] font-extrabold tracking-tight">{p.name}</h2>
          <p className="truncate text-[13px] text-text-muted">
            {p.author
              ? t('plugins:atlas.versionBy', { version: `v${p.version}`, author: p.author })
              : `v${p.version}`}
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
      {p.status !== 'invalid' && <PluginCapabilities plugin={p} />}
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
      {p.capabilities.length > 0 && (
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-[18px] border-b border-hairline py-3.5">
          <div className="grid min-w-0 gap-[3px]">
            <span className="text-[13px] font-semibold text-text">{t('plugins:atlas.brings')}</span>
            <span className="text-[0.93em] leading-normal text-text-muted">{p.capabilities.map((c) => c.replace(/-/g, ' ')).join(', ')}</span>
          </div>
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
        </div>
      )}
    </div>
  );
}
/** One store plugin's card: identity, capabilities, settings, requirements,
 *  the login row and the approve/enable actions. The required parts (login
 *  row, approval actions) render here, inside the card, so a layout can't
 *  hide them on their own. */
export function StorePluginCard({ plugin: p, authState: a, warning, busy, onLogout, onChooseFolder, onAct, onReview, onLogin, onNotify }: {
  plugin: StorePlugin;
  authState: PluginsAuthState | undefined;
  warning: SourceWarning | undefined;
  busy: string | null;
  onLogout: () => void;
  onChooseFolder: () => void;
  onAct: (id: string, what: string, program: Effect.Effect<unknown, Error>, done: string) => void;
  onReview: () => void;
  onLogin: () => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}) {
  const { t } = useTranslation('plugins');
  const st = STATUS[p.status] ?? STATUS.invalid;
  return (
    <Panel className="flex flex-col gap-4">
      <div>
        <div className="flex items-start justify-between gap-4">
          <div className="flex min-w-0 items-center gap-3">
            <PluginLogo src={p.icon_url} name={p.name} />
            <div className="min-w-0">
              <h2 className="truncate text-[14px] font-bold">{p.name}</h2>
              <p className="font-mono text-hint text-text-muted">
                v{p.version}{p.author ? ` · ${p.author}` : ''} · {p.store}{p.running ? ` · ${t('plugins:card.running')}` : ''}
              </p>
            </div>
          </div>
          <Badge variant={st.variant} className="flex-none">{t(st.key)}</Badge>
        </div>
        {p.description && <p className="mt-2.5 text-[12.5px] leading-relaxed text-text-muted">{p.description}</p>}
        {p.problem && p.status !== 'missing-requirements' && (
          <p className={cx('mt-2 font-mono text-hint', p.status === 'invalid' ? 'text-danger' : 'text-attention')}>{p.problem}</p>
        )}
      </div>

      {p.status !== 'invalid' && <PluginCapabilities plugin={p} />}

      {p.status === 'ready' && <PluginSettingsPanel plugin={p} onNotify={onNotify} />}

      {p.missing.length > 0 && (
        <div className="rounded-xl bg-attention/[0.07] px-3.5 py-2.5 ring-1 ring-attention/25">
          <MonoLabel className="mb-1.5 text-attention">{t('plugins:card.installFirst')}</MonoLabel>
          {p.missing.map((r) => (
            <div key={r.bin} className="font-mono text-[12px] text-text">
              <span className="text-attention">{r.bin}</span>
              {r.hint && <> · <span className="select-text">{r.hint}</span></>}
            </div>
          ))}
          <p className="mt-1.5 text-[12px] text-text-body">{t('plugins:card.rescanHint')}</p>
        </div>
      )}

      {p.status === 'ready' && hasAuth(p) && (
        <div className="flex items-center justify-between gap-3 rounded-xl bg-surface px-3.5 py-2.5">
          <span className="min-w-0 truncate text-[12px] text-text-body">
            {!a ? (
              <span className="flex items-center gap-2 text-text-muted"><Loader2 className="h-3 w-3 animate-spin" /> {t('plugins:auth.checking')}</span>
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
          {a && !('error' in a) && a.loggedIn ? (
            <Button variant="neutral" size="sm" className="flex-none" onClick={onLogout} disabled={busy === p.id + 'logout'}>
              <LogOut className="h-3 w-3" /> {t('plugins:auth.logOut')}
            </Button>
          ) : (
            <Button variant="primary" size="sm" className="flex-none" onClick={onLogin}>
              <LogIn className="h-3 w-3" /> {t('plugins:auth.logIn')}
            </Button>
          )}
        </div>
      )}

      {offers(p, 'install') && p.status !== 'invalid' && p.status !== 'unapproved' && (
        <div className="space-y-1 font-mono text-hint">
          <div className="flex items-center justify-between gap-3">
            <span className="text-text-muted">{t('plugins:card.installsInto')}</span>
            <span className="flex min-w-0 items-center gap-2">
              <span className="truncate text-text select-text" title={p.install_folder}>{p.install_folder}</span>
              <Button variant="link" className="flex-none" onClick={onChooseFolder}>{t('plugins:card.change')}</Button>
              {p.install_folder !== p.default_install_folder && (
                <Button variant="link" className="flex-none" onClick={() => onAct(p.id, 'folder', LauncherApi.setPluginInstallFolder(p.id, ''), t('plugins:notify.installsIntoGames', { name: p.name }))}>
                  {t('plugins:card.reset')}
                </Button>
              )}
            </span>
          </div>
          {p.extra_folders.map((f) => (
            <div key={f} className="flex justify-between gap-3">
              <span className="text-text-muted">{t('plugins:card.alsoAllowed')}</span>
              <span className="truncate text-text-muted select-text" title={f}>{f}</span>
            </div>
          ))}
        </div>
      )}

      <div className="mt-auto flex gap-2">
        {(p.status === 'unapproved' || p.status === 'changed') && (
          <Button variant="primary" className="flex-1" onClick={onReview}>
            <ShieldCheck className="h-4 w-4" /> {p.status === 'changed' ? t('plugins:card.reapprove') : t('plugins:card.approve')}
          </Button>
        )}
        {(p.status === 'ready' || p.status === 'missing-requirements') && (
          <Button
            variant="neutral"
            className="flex-1"
            onClick={() => onAct(p.id, 'toggle', LauncherApi.setPluginEnabled(p.id, false), t('plugins:notify.disabled', { name: p.name }))}
            disabled={busy === p.id + 'toggle'}
          >
            {t('plugins:card.disable')}
          </Button>
        )}
        {p.status === 'disabled' && (
          <Button
            variant="primary"
            className="flex-1"
            onClick={() => onAct(p.id, 'toggle', LauncherApi.setPluginEnabled(p.id, true), t('plugins:notify.enabled', { name: p.name }))}
            disabled={busy === p.id + 'toggle'}
          >
            {t('plugins:card.enable')}
          </Button>
        )}
        {/* CORE-10: a plugin that gave up after repeated crashes
            fails fast until its crash count is cleared. */}
        {p.gave_up && p.status === 'ready' && (
          <Button
            variant="neutral"
            className="flex-1"
            onClick={() => onAct(p.id, 'retry', LauncherApi.retryPlugin(p.id), t('plugins:notify.willRetry', { name: p.name }))}
            disabled={busy === p.id + 'retry'}
          >
            <RotateCcw className="h-3.5 w-3.5" /> {t('common:retry')}
          </Button>
        )}
      </div>
    </Panel>
  );
}

export function PluginsData({ plugins }: { plugins: StorePlugin[] }) {
  const { t } = useTranslation('plugins');
  if (plugins.length === 0) return null;
  // One root: the heading and the cards stay together, so a row counts the
  // data block as one column.
  return (
    <div>
      <MonoLabel as="h2" className="mb-3 mt-8 normal-case tracking-normal">{t('plugins:section.data')}</MonoLabel>
      <div className="grid gap-4 xl:grid-cols-2">
        {plugins.map((p) => (
          <DataPluginCard key={p.id} plugin={p} />
        ))}
      </div>
    </div>
  );
}

export function DataPluginCard({ plugin: p }: { plugin: StorePlugin }) {
  const { t } = useTranslation('plugins');
  const st = STATUS[p.status] ?? STATUS.invalid;
  return (
    <Panel className="flex flex-col gap-2">
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 items-center gap-3">
          <PluginLogo src={p.icon_url} name={p.name} />
          <div className="min-w-0">
            <h2 className="truncate text-[14px] font-bold">{p.name}</h2>
            <p className="font-mono text-hint text-text-muted">v{p.version}{p.author ? ` · ${p.author}` : ''}</p>
          </div>
        </div>
        <Badge variant={p.status === 'ready' ? 'neutral' : st.variant} className="flex-none">
          {p.status === 'ready' ? t('plugins:status.data') : t(st.key)}
        </Badge>
      </div>
      {p.description && <p className="text-[12.5px] leading-relaxed text-text-muted">{p.description}</p>}
      {p.problem && <p className="font-mono text-hint text-danger">{p.problem}</p>}
      {p.status === 'ready' && (
        <p className="mt-auto text-[12px] text-text-body">
          {t('plugins:card.dataNote')}
        </p>
      )}
    </Panel>
  );
}

export function PluginsBuiltIn() {
  const { t } = useTranslation('plugins');
  // One root: the heading and the list stay together, so a row counts the
  // built-in block as one column.
  return (
    <div>
      <MonoLabel as="h2" className="mb-3 mt-8">{t('plugins:section.builtIn')}</MonoLabel>
      <div className="grid gap-2 xl:grid-cols-2">
        {BUILT_IN.map((b) => (
          <div key={b.name} className="flex items-center justify-between gap-4 rounded-2xl bg-surface px-4 py-3 ring-1 ring-hairline">
            <div className="min-w-0">
              <div className="text-[13px] font-bold">{b.name}</div>
              <div className="text-[12px] text-text-muted">{t('plugins:builtIn.protondb')}</div>
            </div>
            <Badge variant="neutral" className="flex-none">{t('plugins:status.builtIn')}</Badge>
          </div>
        ))}
      </div>
    </div>
  );
}

function ManifestRow({ label, items, empty }: { label: string; items: string[]; empty: string }) {
  return (
    <div className="flex gap-4 border-t border-hairline pt-2">
      <span className="w-24 flex-none text-text-muted">{label}</span>
      <span className="min-w-0 flex-1 break-words text-text select-text">{items.length ? items.join(', ') : <span className="text-text-muted">{empty}</span>}</span>
    </div>
  );
}

/** Shows what the plugin runs and declares before the user trusts it. */
function ApproveModal({ plugin: p, onClose, onApprove }: { plugin: StorePlugin; onClose: () => void; onApprove: () => void }) {
  const { t } = useTranslation(['plugins', 'common']);
  return (
    <Modal label={t('plugins:approve.title', { name: p.name })} onClose={onClose} size="md">
      <div>
        <h2 className="text-[20px] font-extrabold tracking-tight">{p.status === 'changed' ? t('plugins:approve.titleChanged', { name: p.name }) : t('plugins:approve.title', { name: p.name })}</h2>
        <p className="mt-1 font-mono text-hint text-text-muted">v{p.version}{p.author ? ` · ${p.author}` : ''} · {p.store}</p>
        {p.status === 'changed' && (
          <p className="mt-3 rounded-xl bg-attention/[0.08] px-3.5 py-2.5 text-[12px] text-attention ring-1 ring-attention/25">
            {t('plugins:approve.changedWarning')}
          </p>
        )}

        <div className="mt-4 space-y-2 font-mono text-hint">
          <ManifestRow label={t('plugins:approve.row.runs')} items={[`${p.dir}/${p.exec}`]} empty="" />
          <ManifestRow label={t('plugins:approve.row.sha')} items={[p.hash]} empty="" />
          <ManifestRow label={t('plugins:approve.row.can')} items={p.declared} empty={t('plugins:approve.empty.nothingDeclared')} />
          <ManifestRow label={t('plugins:approve.row.needsPlatypus')} items={p.needs} empty={t('plugins:approve.empty.nothingSpecial')} />
          {p.protocol_versions && <ManifestRow label={t('plugins:approve.row.speaks')} items={[`protocol ${p.protocol_versions} over ${p.transport ?? 'jsonrpc-stdio'}`]} empty="" />}
          <ManifestRow label={t('plugins:approve.row.network')} items={p.network} empty={t('plugins:approve.empty.noHosts')} />
          <ManifestRow label={t('plugins:approve.row.reads')} items={p.read} empty={t('plugins:approve.empty.noPaths')} />
          <ManifestRow label={t('plugins:approve.row.writes')} items={p.write} empty={t('plugins:approve.empty.noPaths')} />
          <ManifestRow
            label={t('plugins:approve.row.needs')}
            items={p.requires.map((r) => `${r.bin}${p.missing.some((m) => m.bin === r.bin) ? ` ${t('plugins:approve.missingMark')}` : ''}`)}
            empty={t('plugins:approve.empty.noTools')}
          />
          <ManifestRow label={t('plugins:approve.row.installsInto')} items={[p.install_folder]} empty="" />
        </div>

        <div className="mt-4 flex gap-3 rounded-xl bg-surface px-3.5 py-3 text-hint leading-relaxed text-text-muted">
          <ShieldAlert className="mt-0.5 h-4 w-4 flex-none text-accent-strong/80" />
          <p>
            {t('plugins:approve.warning')}
          </p>
        </div>

        <div className="mt-5 flex gap-2.5">
          <Button variant="neutral" className="flex-1" onClick={onClose}>{t('common:cancel')}</Button>
          <Button variant="primary" className="flex-1" onClick={onApprove}>{t('plugins:approve.approveAndEnable')}</Button>
        </div>
      </div>
    </Modal>
  );
}

/**
 * The plugin's QR login data: block-art text straight from the store tool's
 * stdout ("█" = dark module, two chars per module, one row per module).
 * Drawn as exact square modules so the phone can scan it off the screen.
 */
function QrArt({ art }: { art: string }) {
  const { t } = useTranslation('plugins');
  const rows = art.split('\n').filter((r) => r.includes('█'));
  const lead = Math.min(...rows.map((r) => r.length - r.trimStart().length));
  let width = Math.max(...rows.map((r) => r.trimEnd().length)) - lead;
  if (width % 2) width += 1;
  const modules = rows.map((r) => {
    const line = r.slice(lead).padEnd(width, ' ').slice(0, width);
    const out: boolean[] = [];
    for (let i = 0; i < width; i += 2) out.push(line[i] === '█');
    return out;
  });
  const h = modules.length;
  const w = modules[0]?.length ?? 0;
  const quiet = 4;
  const size = Math.max(w, h) + quiet * 2;
  return (
    <svg viewBox={`0 0 ${size} ${size}`} className="h-48 w-48" role="img" aria-label={t('plugins:auth.qrLabel')} shapeRendering="crispEdges">
      <rect width={size} height={size} fill="#fff" />
      {modules.map((row, y) =>
        row.map((dark, x) =>
          dark ? <rect key={`${x}-${y}`} x={x + quiet} y={y + quiet} width={1} height={1} fill="#000" /> : null,
        ),
      )}
    </svg>
  );
}

// QR poll backoff (FE-8): 1.5s, then escalate on consecutive failures.
export function qrPollDelay(consecutiveFailures: number): number {
  if (consecutiveFailures <= 1) return 1500;
  if (consecutiveFailures === 2) return 3000;
  if (consecutiveFailures === 3) return 5000;
  return 15000;
}
export const QR_ERROR_AFTER = 2;
export const QR_MAX_ATTEMPTS = 40;

/** auth.begin → open the store's page and/or fill in fields → auth.complete. */
function AuthModal({ plugin, onClose, onDone }: { plugin: StorePlugin; onClose: () => void; onDone: (s: AuthStatus) => void }) {
  const { t } = useTranslation(['plugins', 'common']);
  const [prompt, setPrompt] = useState<AuthPrompt | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // StrictMode mounts effects twice in dev; starting login twice spawns two
  // store-tool flows (two QR codes / two legendary attempts racing).
  const begunFor = useRef<string | null>(null);
  useEffect(() => {
    if (begunFor.current === plugin.id) return;
    begunFor.current = plugin.id;
    Effect.runPromise(LauncherApi.authBegin(plugin.id))
      .then(setPrompt)
      .catch((e) => setError(coreErrMsg(e)));
  }, [plugin.id]);

  // QR logins finish outside the form: the store's mobile app approves and
  // the plugin's tool persists the login, so poll the status while the
  // dialog is open and close it the moment the plugin says logged in. The
  // status also carries the current QR code, which Steam refreshes when it
  // expires — redraw whenever it changed.
  const qrPending = !!prompt?.qr;
  const onDoneRef = useRef(onDone);
  onDoneRef.current = onDone;
  useEffect(() => {
    if (!qrPending) return;
    let stop = false;
    const timers: ReturnType<typeof setTimeout>[] = [];
    let failures = 0;
    let attempts = 0;
    // Whether the error on screen is ours (a QR poll failure): only ours is
    // cleared on the next success, so a submit error is never wiped. Matched
    // by flag, not by text, so it works in every language.
    let qrError = false;
    // A background refresh may notice the login first: finish the moment it
    // does, and redraw the QR code when it reports a new one.
    const offAuth = onAuth((e) => {
      if (e.plugin !== plugin.id || stop) return;
      if (e.status.loggedIn) {
        onDoneRef.current(e.status);
        return;
      }
      if (e.status.qr) {
        setPrompt((p) => (p && p.qr !== e.status.qr ? { ...p, qr: e.status.qr } : p));
      }
    });
    const schedule = (ms: number) => {
      if (stop) return;
      // Don't hammer a hidden tab; poll at most every 15s there.
      const delay = typeof document !== 'undefined' && document.hidden ? Math.max(ms, 15000) : ms;
      timers.push(setTimeout(poll, delay));
    };
    const poll = () => {
      if (stop) return;
      attempts += 1;
      Effect.runPromise(LauncherApi.authStatus(plugin.id))
        .then((s) => {
          if (stop) return;
          if (s.loggedIn) {
            onDoneRef.current(s);
            return;
          }
          if (s.qr) {
            setPrompt((p) => (p && p.qr !== s.qr ? { ...p, qr: s.qr } : p));
          }
          failures = 0;
          if (qrError) {
            qrError = false;
            setError(null);
          }
          schedule(1500);
        })
        .catch(() => {
          if (stop) return;
          failures += 1;
          if (failures >= QR_ERROR_AFTER) {
            qrError = true;
            setError(
              attempts >= QR_MAX_ATTEMPTS
                ? t('plugins:auth.checkDead')
                : t('plugins:auth.checkFailing', { count: failures }),
            );
          }
          if (attempts >= QR_MAX_ATTEMPTS) return;
          schedule(qrPollDelay(failures));
        });
    };
    schedule(1500);
    return () => {
      stop = true;
      offAuth();
      timers.forEach(clearTimeout);
    };
  }, [qrPending, plugin.id, t]);

  const submit = () => {
    setSubmitting(true);
    setError(null);
    Effect.runPromise(LauncherApi.authComplete(plugin.id, values))
      .then((s) => (s.loggedIn ? onDone(s) : setError(t('plugins:auth.stillLoggedOut'))))
      .catch((e) => setError(coreErrMsg(e)))
      .finally(() => setSubmitting(false));
  };

  // A QR prompt can be confirmed with an empty form ("I scanned it"); with
  // fields only, every field must be filled.
  const filled = prompt?.fields.every((f) => (values[f.id] ?? '').trim()) ?? false;
  const canSubmit = (prompt?.qr && !filled) || filled;

  return (
    <Modal label={t('plugins:auth.loginTo', { name: plugin.name })} onClose={onClose} size="md">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit && !submitting) submit();
        }}
      >
        <h2 className="text-[20px] font-extrabold tracking-tight">{t('plugins:auth.loginTo', { name: plugin.name })}</h2>
        <p className="mt-1 text-[12px] text-text-body">{t('plugins:auth.loginStays', { name: plugin.name })}</p>
        {!prompt && !error && (
          <div className="mt-5 flex items-center gap-2 font-mono text-[12px] text-text-muted"><Loader2 className="h-3.5 w-3.5 animate-spin" /> {t('plugins:auth.startingLogin')}</div>
        )}
        {prompt && (
          <div className="mt-4 space-y-4">
            {prompt.url && (
              <div>
                <MonoLabel className="mb-1.5 normal-case tracking-normal">{t('plugins:auth.stepStorePage')}</MonoLabel>
                <Button variant="neutral" className="w-full" onClick={() => openExternal(prompt.url)}>
                  <ExternalLink className="h-3.5 w-3.5" /> {t('plugins:auth.openLoginPage')}
                </Button>
                <p className="mt-1.5 break-all font-mono text-[12px] text-text-muted select-text">{prompt.url}</p>
              </div>
            )}
            {prompt.qr && (
              <div>
                <MonoLabel className="mb-1.5 normal-case tracking-normal">{t('plugins:auth.stepScan', { app: plugin.name.replace(/ \(.*\)$/, '') })}</MonoLabel>
                <div className="flex justify-center rounded-xl bg-white p-3">
                  <QrArt art={prompt.qr} />
                </div>
              </div>
            )}
            {prompt.instructions && <p className="text-[12px] leading-relaxed text-text-body">{prompt.instructions}</p>}
            <div className="space-y-3">
              {prompt.url && <MonoLabel className="normal-case tracking-normal">{t('plugins:auth.stepPaste')}</MonoLabel>}
              {prompt.fields.map((f, i) => (
                <label key={f.id} className="block">
                  <span className="mb-1 block text-[12px] font-semibold text-text">{f.label}</span>
                  <Input
                    autoFocus={i === 0 && !prompt.url}
                    type={f.secret ? 'password' : 'text'}
                    value={values[f.id] ?? ''}
                    onChange={(e) => setValues((v) => ({ ...v, [f.id]: e.target.value }))}
                    placeholder={f.placeholder}
                    spellCheck={false}
                    autoComplete="off"
                  />
                </label>
              ))}
            </div>
          </div>
        )}
        {error && <ErrorNotice className="mt-3" message={error} fallback={t('plugins:auth.actionFailed')} />}
        <div className="mt-5 flex gap-2.5">
          <Button type="button" variant="neutral" className="flex-1" onClick={onClose}>{t('common:cancel')}</Button>
          <Button type="submit" variant="primary" className="flex-1" disabled={!canSubmit || submitting}>
            {submitting && <Loader2 className="h-3.5 w-3.5 animate-spin" />} {prompt?.qr && !filled ? t('plugins:auth.scannedIt') : t('plugins:auth.logIn')}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
