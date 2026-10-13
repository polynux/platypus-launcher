// i18n: converted
import { useTranslation, Trans } from 'react-i18next';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import { ExternalLink, FolderOpen, Loader2, LogIn, LogOut, RefreshCw, RotateCcw, ShieldAlert, ShieldCheck } from 'lucide-react';
import type { AuthPrompt, AuthStatus, PluginStatus, SourceWarning, StorePlugin } from '../types';
import { LauncherApi, onAuth } from '../effect/runtime';
import { openExternal } from '../lib/external';
import { Modal } from './Modal';
import { EmptyState, MonoLabel, Panel, cx } from './ui';
import { Button } from './ds/button';
import { Badge, type badgeVariants } from './ds/badge';
import { Input } from './ds/input';
import { PluginCapabilities } from './PluginCapabilities';
import { PluginLogo } from './PluginLogo';
import { PluginSettingsPanel } from './PluginSettings';
import { offers } from '../lib/capabilities';
import type { VariantProps } from 'class-variance-authority';
import { ErrorNotice } from './ErrorNotice';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

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
  { key: 'steam', name: 'Steam' },
  { key: 'protondb', name: 'ProtonDB' },
];

const hasAuth = (p: StorePlugin) => offers(p, 'auth');

type AuthState = AuthStatus | { error: string };

// Login states outlive one visit: the core serves its cached state instantly
// and refreshes it in the background, so a revisit shows the last known state
// at once instead of flashing "checking login…" while the refresh runs.
const authCache = new Map<string, AuthState>();

/** Forgets every cached login state (tests). */
export function resetPluginsAuthCache(): void {
  authCache.clear();
}

export function PluginsView({ plugins, warnings, onPlugins, onLibraryChanged, onNotify }: PluginsViewProps) {
  const { t } = useTranslation(['plugins', 'nav', 'common']);
  const [auth, setAuth] = useState<Record<string, AuthState>>(() => Object.fromEntries(authCache));
  const [reviewing, setReviewing] = useState<StorePlugin | null>(null);
  const [loggingIn, setLoggingIn] = useState<StorePlugin | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const storeAuth = useCallback((id: string, s: AuthState) => {
    authCache.set(id, s);
    setAuth((a) => ({ ...a, [id]: s }));
  }, []);

  const refresh = useCallback(
    (rescan = false) =>
      Effect.runPromise(rescan ? LauncherApi.rescanPlugins() : LauncherApi.listPlugins())
        .then(onPlugins)
        .catch((e) => onNotify(errMsg(e), 'err')),
    [onPlugins, onNotify]
  );

  // Login state of every approved plugin with a login. Asking starts the plugin.
  const readyIds = plugins.filter((p) => p.status === 'ready' && hasAuth(p)).map((p) => p.id).join(',');
  // StrictMode mounts effects twice: only ask once per plugin id (FE-7).
  // The core answers a known state from its cache at once and refreshes it
  // in the background; the answer below only seeds the state, whose update
  // (or the auth event) re-renders.
  const statusFetched = useRef(new Set<string>());
  useEffect(() => {
    for (const id of readyIds ? readyIds.split(',') : []) {
      if (statusFetched.current.has(id)) continue;
      statusFetched.current.add(id);
      Effect.runPromise(LauncherApi.authStatus(id))
        .then((s) => storeAuth(id, s))
        .catch((e) => storeAuth(id, { error: errMsg(e) }));
    }
  }, [readyIds, storeAuth]);

  // A background refresh that finds a changed state arrives as an event.
  useEffect(() => onAuth((e) => storeAuth(e.plugin, e.status)), [storeAuth]);

  const act = (id: string, what: string, program: Effect.Effect<unknown, Error>, done: string) => {
    setBusy(id + what);
    Effect.runPromise(program)
      .then(() => {
        onNotify(done);
        return refresh().then(onLibraryChanged);
      })
      .catch((e) => onNotify(errMsg(e), 'err'))
      .finally(() => setBusy(null));
  };

  const logout = (p: StorePlugin) =>
    act(
      p.id,
      'logout',
      LauncherApi.logout(p.id).pipe(Effect.tap((s: AuthStatus) => Effect.sync(() => storeAuth(p.id, s)))),
      t('plugins:notify.loggedOutOf', { name: p.name })
    );

  const chooseFolder = (p: StorePlugin) =>
    Effect.runPromise(LauncherApi.choosePluginInstallFolder(p.id))
      .then((path) => path && act(p.id, 'folder', LauncherApi.setPluginInstallFolder(p.id, path), t('plugins:notify.installsInto', { name: p.name, path })))
      .catch((e) => onNotify(errMsg(e), 'err'));

  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">{t('nav:title.plugins')}</h1>
          <p className="mt-0.5 max-w-xl text-[12.5px] text-white/70">
            {t('plugins:header.subtitle')}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="neutral" onClick={() => Effect.runPromise(LauncherApi.openPluginsFolder()).catch((e) => onNotify(errMsg(e), 'err'))}>
            <FolderOpen className="h-3.5 w-3.5" /> {t('plugins:header.pluginsFolder')}
          </Button>
          <Button variant="neutral" onClick={() => refresh(true).then(onLibraryChanged)}>
            <RefreshCw className="h-3.5 w-3.5" /> {t('plugins:header.rescan')}
          </Button>
        </div>
      </header>

      <MonoLabel as="h2" className="mb-3">{t('plugins:section.store')}</MonoLabel>
      {plugins.filter((p) => p.kind === 'store').length === 0 ? (
        <EmptyState label={t('plugins:empty.label')}>
          {t('plugins:empty.detail')}
        </EmptyState>
      ) : (
        <div className="grid gap-4 xl:grid-cols-2">
          {plugins.filter((p) => p.kind === 'store').map((p) => {
            const st = STATUS[p.status] ?? STATUS.invalid;
            const a = auth[p.id];
            const warning = warnings.find((w) => w.source === p.id);
            return (
              <Panel key={p.id} className="flex flex-col gap-4">
                <div>
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex min-w-0 items-center gap-3">
                      <PluginLogo src={p.icon_url} name={p.name} />
                      <div className="min-w-0">
                        <h2 className="truncate text-[14px] font-bold">{p.name}</h2>
                        <p className="font-mono text-hint text-white/55">
                          v{p.version}{p.author ? ` · ${p.author}` : ''} · {p.store}{p.running ? ` · ${t('plugins:card.running')}` : ''}
                        </p>
                      </div>
                    </div>
                    <Badge variant={st.variant} className="flex-none">{t(st.key)}</Badge>
                  </div>
                  {p.description && <p className="mt-2.5 text-[12.5px] leading-relaxed text-white/60">{p.description}</p>}
                  {p.problem && p.status !== 'missing-requirements' && (
                    <p className={cx('mt-2 font-mono text-hint', p.status === 'invalid' ? 'text-red-300' : 'text-orange-200')}>{p.problem}</p>
                  )}
                </div>

                {p.status !== 'invalid' && <PluginCapabilities plugin={p} />}

                {p.status === 'ready' && <PluginSettingsPanel plugin={p} onNotify={onNotify} />}

                {p.missing.length > 0 && (
                  <div className="rounded-xl bg-orange-400/[0.07] px-3.5 py-2.5 ring-1 ring-orange-300/20">
                    <MonoLabel className="mb-1.5 text-orange-200/80">{t('plugins:card.installFirst')}</MonoLabel>
                    {p.missing.map((r) => (
                      <div key={r.bin} className="font-mono text-[12px] text-white/80">
                        <span className="text-orange-200">{r.bin}</span>
                        {r.hint && <> · <span className="select-text">{r.hint}</span></>}
                      </div>
                    ))}
                    <p className="mt-1.5 text-[12px] text-white/70">{t('plugins:card.rescanHint')}</p>
                  </div>
                )}

                {p.status === 'ready' && hasAuth(p) && (
                  <div className="flex items-center justify-between gap-3 rounded-xl bg-white/[0.04] px-3.5 py-2.5">
                    <span className="min-w-0 truncate text-[12px] text-white/70">
                      {!a ? (
                        <span className="flex items-center gap-2 text-white/55"><Loader2 className="h-3 w-3 animate-spin" /> {t('plugins:auth.checking')}</span>
                      ) : 'error' in a ? (
                        <span className="font-mono text-hint text-red-300">{a.error}</span>
                      ) : a.loggedIn ? (
                        <>{a.user ? (
                          <Trans i18nKey="plugins:auth.loggedInAs" values={{ user: a.user }} components={{ b: <span className="font-bold text-white/90" /> }} />
                        ) : t('plugins:auth.loggedIn')}</>
                      ) : (
                        <span className="text-amber-200">{warning?.needs_auth ? t('plugins:auth.loggedOutHidden') : t('plugins:auth.loggedOut')}</span>
                      )}
                    </span>
                    {a && !('error' in a) && a.loggedIn ? (
                      <Button variant="neutral" size="sm" className="flex-none" onClick={() => logout(p)} disabled={busy === p.id + 'logout'}>
                        <LogOut className="h-3 w-3" /> {t('plugins:auth.logOut')}
                      </Button>
                    ) : (
                      <Button variant="primary" size="sm" className="flex-none" onClick={() => setLoggingIn(p)}>
                        <LogIn className="h-3 w-3" /> {t('plugins:auth.logIn')}
                      </Button>
                    )}
                  </div>
                )}

                {offers(p, 'install') && p.status !== 'invalid' && p.status !== 'unapproved' && (
                  <div className="space-y-1 font-mono text-hint">
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-white/55">{t('plugins:card.installsInto')}</span>
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="truncate text-white/80 select-text" title={p.install_folder}>{p.install_folder}</span>
                        <Button variant="link" className="flex-none" onClick={() => chooseFolder(p)}>{t('plugins:card.change')}</Button>
                        {p.install_folder !== p.default_install_folder && (
                          <Button variant="link" className="flex-none" onClick={() => act(p.id, 'folder', LauncherApi.setPluginInstallFolder(p.id, ''), t('plugins:notify.installsIntoGames', { name: p.name }))}>
                            {t('plugins:card.reset')}
                          </Button>
                        )}
                      </span>
                    </div>
                    {p.extra_folders.map((f) => (
                      <div key={f} className="flex justify-between gap-3">
                        <span className="text-white/55">{t('plugins:card.alsoAllowed')}</span>
                        <span className="truncate text-white/60 select-text" title={f}>{f}</span>
                      </div>
                    ))}
                  </div>
                )}

                <div className="mt-auto flex gap-2">
                  {(p.status === 'unapproved' || p.status === 'changed') && (
                    <Button variant="primary" className="flex-1" onClick={() => setReviewing(p)}>
                      <ShieldCheck className="h-4 w-4" /> {p.status === 'changed' ? t('plugins:card.reapprove') : t('plugins:card.approve')}
                    </Button>
                  )}
                  {(p.status === 'ready' || p.status === 'missing-requirements') && (
                    <Button
                      variant="neutral"
                      className="flex-1"
                      onClick={() => act(p.id, 'toggle', LauncherApi.setPluginEnabled(p.id, false), t('plugins:notify.disabled', { name: p.name }))}
                      disabled={busy === p.id + 'toggle'}
                    >
                      {t('plugins:card.disable')}
                    </Button>
                  )}
                  {p.status === 'disabled' && (
                    <Button
                      variant="primary"
                      className="flex-1"
                      onClick={() => act(p.id, 'toggle', LauncherApi.setPluginEnabled(p.id, true), t('plugins:notify.enabled', { name: p.name }))}
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
                      onClick={() => act(p.id, 'retry', LauncherApi.retryPlugin(p.id), t('plugins:notify.willRetry', { name: p.name }))}
                      disabled={busy === p.id + 'retry'}
                    >
                      <RotateCcw className="h-3.5 w-3.5" /> {t('common:retry')}
                    </Button>
                  )}
                </div>
              </Panel>
            );
          })}
        </div>
      )}

      {plugins.some((p) => p.kind === 'data') && (
        <>
          <MonoLabel as="h2" className="mb-3 mt-8 normal-case tracking-normal">{t('plugins:section.data')}</MonoLabel>
          <div className="grid gap-4 xl:grid-cols-2">
            {plugins
              .filter((p) => p.kind === 'data')
              .map((p) => {
                const st = STATUS[p.status] ?? STATUS.invalid;
                return (
                  <Panel key={p.id} className="flex flex-col gap-2">
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex min-w-0 items-center gap-3">
                        <PluginLogo src={p.icon_url} name={p.name} />
                        <div className="min-w-0">
                          <h2 className="truncate text-[14px] font-bold">{p.name}</h2>
                          <p className="font-mono text-hint text-white/55">v{p.version}{p.author ? ` · ${p.author}` : ''}</p>
                        </div>
                      </div>
                      <Badge variant={p.status === 'ready' ? 'neutral' : st.variant} className="flex-none">
                        {p.status === 'ready' ? t('plugins:status.data') : t(st.key)}
                      </Badge>
                    </div>
                    {p.description && <p className="text-[12.5px] leading-relaxed text-white/60">{p.description}</p>}
                    {p.problem && <p className="font-mono text-hint text-red-300">{p.problem}</p>}
                    {p.status === 'ready' && (
                      <p className="mt-auto text-[12px] text-white/70">
                        {t('plugins:card.dataNote')}
                      </p>
                    )}
                  </Panel>
                );
              })}
          </div>
        </>
      )}

      <MonoLabel as="h2" className="mb-3 mt-8">{t('plugins:section.builtIn')}</MonoLabel>
      <div className="grid gap-2 xl:grid-cols-2">
        {BUILT_IN.map((b) => (
          <div key={b.name} className="flex items-center justify-between gap-4 rounded-2xl bg-white/[0.03] px-4 py-3 ring-1 ring-white/[0.06]">
            <div className="min-w-0">
              <div className="text-[13px] font-bold">{b.name}</div>
              <div className="text-[12px] text-white/55">{b.key === 'steam' ? t('plugins:builtIn.steam') : t('plugins:builtIn.protondb')}</div>
            </div>
            <Badge variant="neutral" className="flex-none">{t('plugins:status.builtIn')}</Badge>
          </div>
        ))}
      </div>

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

function ManifestRow({ label, items, empty }: { label: string; items: string[]; empty: string }) {
  return (
    <div className="flex gap-4 border-t border-white/[0.05] pt-2">
      <span className="w-24 flex-none text-white/55">{label}</span>
      <span className="min-w-0 flex-1 break-words text-white/85 select-text">{items.length ? items.join(', ') : <span className="text-white/55">{empty}</span>}</span>
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
        <p className="mt-1 font-mono text-hint text-white/55">v{p.version}{p.author ? ` · ${p.author}` : ''} · {p.store}</p>
        {p.status === 'changed' && (
          <p className="mt-3 rounded-xl bg-orange-400/[0.08] px-3.5 py-2.5 text-[12px] text-orange-100 ring-1 ring-orange-300/25">
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

        <div className="mt-4 flex gap-3 rounded-xl bg-white/[0.04] px-3.5 py-3 text-hint leading-relaxed text-white/60">
          <ShieldAlert className="mt-0.5 h-4 w-4 flex-none text-amber-300/80" />
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
 * stdout ("\u2588" = dark module, two chars per module, one row per module).
 * Drawn as exact square modules so the phone can scan it off the screen.
 */
function QrArt({ art }: { art: string }) {
  const { t } = useTranslation('plugins');
  const rows = art.split('\n').filter((r) => r.includes('\u2588'));
  const lead = Math.min(...rows.map((r) => r.length - r.trimStart().length));
  let width = Math.max(...rows.map((r) => r.trimEnd().length)) - lead;
  if (width % 2) width += 1;
  const modules = rows.map((r) => {
    const line = r.slice(lead).padEnd(width, ' ').slice(0, width);
    const out: boolean[] = [];
    for (let i = 0; i < width; i += 2) out.push(line[i] === '\u2588');
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
      .catch((e) => setError(errMsg(e)));
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
      .catch((e) => setError(errMsg(e)))
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
        <p className="mt-1 text-[12px] text-white/70">{t('plugins:auth.loginStays', { name: plugin.name })}</p>
        {!prompt && !error && (
          <div className="mt-5 flex items-center gap-2 font-mono text-[12px] text-white/55"><Loader2 className="h-3.5 w-3.5 animate-spin" /> {t('plugins:auth.startingLogin')}</div>
        )}
        {prompt && (
          <div className="mt-4 space-y-4">
            {prompt.url && (
              <div>
                <MonoLabel className="mb-1.5 normal-case tracking-normal">{t('plugins:auth.stepStorePage')}</MonoLabel>
                <Button variant="neutral" className="w-full" onClick={() => openExternal(prompt.url)}>
                  <ExternalLink className="h-3.5 w-3.5" /> {t('plugins:auth.openLoginPage')}
                </Button>
                <p className="mt-1.5 break-all font-mono text-[12px] text-white/55 select-text">{prompt.url}</p>
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
            {prompt.instructions && <p className="text-[12px] leading-relaxed text-white/65">{prompt.instructions}</p>}
            <div className="space-y-3">
              {prompt.url && <MonoLabel className="normal-case tracking-normal">{t('plugins:auth.stepPaste')}</MonoLabel>}
              {prompt.fields.map((f, i) => (
                <label key={f.id} className="block">
                  <span className="mb-1 block text-[12px] font-semibold text-white/75">{f.label}</span>
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
