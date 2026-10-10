import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import { ExternalLink, FolderOpen, Loader2, LogIn, LogOut, RefreshCw, RotateCcw, ShieldAlert, ShieldCheck } from 'lucide-react';
import type { AuthPrompt, AuthStatus, PluginStatus, SourceWarning, StorePlugin } from '../types';
import { LauncherApi } from '../effect/runtime';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { Modal } from './Modal';
import { EmptyState, MonoLabel, Panel, cx } from './ui';
import { Button } from './ds/button';
import { Badge, type badgeVariants } from './ds/badge';
import { Input } from './ds/input';
import { PluginCapabilities } from './PluginCapabilities';
import { PluginSettingsPanel } from './PluginSettings';
import { offers } from '../lib/capabilities';
import type { VariantProps } from 'class-variance-authority';

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
const STATUS: Record<PluginStatus, { label: string; variant: BadgeVariant }> = {
  ready: { label: 'approved', variant: 'ok' },
  unapproved: { label: 'needs approval', variant: 'gold' },
  changed: { label: 'changed · re-approve', variant: 'bronze' },
  disabled: { label: 'disabled', variant: 'neutral' },
  'missing-requirements': { label: 'missing a tool', variant: 'bronze' },
  invalid: { label: 'invalid', variant: 'borked' },
  'needs-newer-platypus': { label: 'needs a newer Platypus', variant: 'bronze' },
  'too-old': { label: 'too old for this Platypus', variant: 'bronze' },
};

const BUILT_IN = [
  { name: 'Steam', what: 'Reads your local Steam libraries and launches games through the Steam client.' },
  { name: 'ProtonDB', what: 'Compatibility ratings and community notes for Steam games.' },
];

const hasAuth = (p: StorePlugin) => offers(p, 'auth');

export function PluginsView({ plugins, warnings, onPlugins, onLibraryChanged, onNotify }: PluginsViewProps) {
  const [auth, setAuth] = useState<Record<string, AuthStatus | { error: string }>>({});
  const [reviewing, setReviewing] = useState<StorePlugin | null>(null);
  const [loggingIn, setLoggingIn] = useState<StorePlugin | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

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
  const statusFetched = useRef(new Set<string>());
  useEffect(() => {
    for (const id of readyIds ? readyIds.split(',') : []) {
      if (statusFetched.current.has(id)) continue;
      statusFetched.current.add(id);
      Effect.runPromise(LauncherApi.authStatus(id))
        .then((s) => setAuth((a) => ({ ...a, [id]: s })))
        .catch((e) => setAuth((a) => ({ ...a, [id]: { error: errMsg(e) } })));
    }
  }, [readyIds]);

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
      LauncherApi.logout(p.id).pipe(Effect.tap((s: AuthStatus) => Effect.sync(() => setAuth((a) => ({ ...a, [p.id]: s }))))),
      `Logged out of ${p.name}`
    );

  const chooseFolder = (p: StorePlugin) =>
    Effect.runPromise(LauncherApi.choosePluginInstallFolder(p.id))
      .then((path) => path && act(p.id, 'folder', LauncherApi.setPluginInstallFolder(p.id, path), `${p.name} installs into ${path}`))
      .catch((e) => onNotify(errMsg(e), 'err'));

  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">Plugins</h1>
          <p className="mt-0.5 max-w-xl text-[12.5px] text-white/50">
            Store plugins bring in libraries from other stores and install games you own. Each one is a separate program; Platypus only runs the ones you approve.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="neutral" onClick={() => Effect.runPromise(LauncherApi.openPluginsFolder()).catch((e) => onNotify(errMsg(e), 'err'))}>
            <FolderOpen className="h-3.5 w-3.5" /> Plugins folder
          </Button>
          <Button variant="neutral" onClick={() => refresh(true).then(onLibraryChanged)}>
            <RefreshCw className="h-3.5 w-3.5" /> Rescan
          </Button>
        </div>
      </header>

      <MonoLabel className="mb-3">store plugins</MonoLabel>
      {plugins.filter((p) => p.kind === 'store').length === 0 ? (
        <EmptyState label="no store plugins">
          Put a plugin folder in ~/.local/share/platypus/plugins and rescan. From Platypus's source: plugins/install.sh epic-legendary.
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
                    <div className="min-w-0">
                      <h3 className="truncate text-[14px] font-bold">{p.name}</h3>
                      <p className="font-mono text-[11px] text-white/45">
                        v{p.version}{p.author ? ` · ${p.author}` : ''} · {p.store}{p.running ? ' · running' : ''}
                      </p>
                    </div>
                    <Badge variant={st.variant} className="flex-none">{st.label}</Badge>
                  </div>
                  {p.description && <p className="mt-2.5 text-[12.5px] leading-relaxed text-white/60">{p.description}</p>}
                  {p.problem && p.status !== 'missing-requirements' && (
                    <p className={cx('mt-2 font-mono text-[11.5px]', p.status === 'invalid' ? 'text-red-300' : 'text-orange-200')}>{p.problem}</p>
                  )}
                </div>

                {p.status !== 'invalid' && <PluginCapabilities plugin={p} />}

                {p.status === 'ready' && <PluginSettingsPanel plugin={p} onNotify={onNotify} />}

                {p.missing.length > 0 && (
                  <div className="rounded-xl bg-orange-400/[0.07] px-3.5 py-2.5 ring-1 ring-orange-300/20">
                    <MonoLabel className="mb-1.5 text-orange-200/80">install first</MonoLabel>
                    {p.missing.map((r) => (
                      <div key={r.bin} className="font-mono text-[12px] text-white/80">
                        <span className="text-orange-200">{r.bin}</span>
                        {r.hint && <> · <span className="select-text">{r.hint}</span></>}
                      </div>
                    ))}
                    <p className="mt-1.5 text-[12px] text-white/55">Then rescan.</p>
                  </div>
                )}

                {p.status === 'ready' && hasAuth(p) && (
                  <div className="flex items-center justify-between gap-3 rounded-xl bg-white/[0.04] px-3.5 py-2.5">
                    <span className="min-w-0 truncate text-[12px] text-white/70">
                      {!a ? (
                        <span className="flex items-center gap-2 text-white/45"><Loader2 className="h-3 w-3 animate-spin" /> checking login…</span>
                      ) : 'error' in a ? (
                        <span className="font-mono text-[11.5px] text-red-300">{a.error}</span>
                      ) : a.loggedIn ? (
                        <>Logged in{a.user ? <> as <span className="font-bold text-white/90">{a.user}</span></> : ''}</>
                      ) : (
                        <span className="text-amber-200">Logged out{warning?.needs_auth ? ': its games are hidden' : ''}</span>
                      )}
                    </span>
                    {a && !('error' in a) && a.loggedIn ? (
                      <Button variant="neutral" size="sm" className="flex-none" onClick={() => logout(p)} disabled={busy === p.id + 'logout'}>
                        <LogOut className="h-3 w-3" /> Log out
                      </Button>
                    ) : (
                      <Button variant="primary" size="sm" className="flex-none" onClick={() => setLoggingIn(p)}>
                        <LogIn className="h-3 w-3" /> Log in
                      </Button>
                    )}
                  </div>
                )}

                {offers(p, 'install') && p.status !== 'invalid' && p.status !== 'unapproved' && (
                  <div className="space-y-1 font-mono text-[11.5px]">
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-white/45">installs into</span>
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="truncate text-white/80 select-text" title={p.install_folder}>{p.install_folder}</span>
                        <Button variant="link" className="flex-none" onClick={() => chooseFolder(p)}>change</Button>
                        {p.install_folder !== p.default_install_folder && (
                          <Button variant="link" className="flex-none" onClick={() => act(p.id, 'folder', LauncherApi.setPluginInstallFolder(p.id, ''), `${p.name} installs into the Games folder again`)}>
                            reset
                          </Button>
                        )}
                      </span>
                    </div>
                    {p.extra_folders.map((f) => (
                      <div key={f} className="flex justify-between gap-3">
                        <span className="text-white/45">also allowed</span>
                        <span className="truncate text-white/60 select-text" title={f}>{f}</span>
                      </div>
                    ))}
                  </div>
                )}

                <div className="mt-auto flex gap-2">
                  {(p.status === 'unapproved' || p.status === 'changed') && (
                    <Button variant="primary" className="flex-1" onClick={() => setReviewing(p)}>
                      <ShieldCheck className="h-4 w-4" /> {p.status === 'changed' ? 'Review and re-approve' : 'Review and approve'}
                    </Button>
                  )}
                  {(p.status === 'ready' || p.status === 'missing-requirements') && (
                    <Button
                      variant="neutral"
                      className="flex-1"
                      onClick={() => act(p.id, 'toggle', LauncherApi.setPluginEnabled(p.id, false), `${p.name} disabled`)}
                      disabled={busy === p.id + 'toggle'}
                    >
                      Disable
                    </Button>
                  )}
                  {p.status === 'disabled' && (
                    <Button
                      variant="primary"
                      className="flex-1"
                      onClick={() => act(p.id, 'toggle', LauncherApi.setPluginEnabled(p.id, true), `${p.name} enabled`)}
                      disabled={busy === p.id + 'toggle'}
                    >
                      Enable
                    </Button>
                  )}
                  {/* CORE-10: a plugin that gave up after repeated crashes
                      fails fast until its crash count is cleared. */}
                  {p.gave_up && p.status === 'ready' && (
                    <Button
                      variant="neutral"
                      className="flex-1"
                      onClick={() => act(p.id, 'retry', LauncherApi.retryPlugin(p.id), `${p.name} will start again on the next call`)}
                      disabled={busy === p.id + 'retry'}
                    >
                      <RotateCcw className="h-3.5 w-3.5" /> Retry
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
          <MonoLabel className="mb-3 mt-8">data plugins · no code, always on</MonoLabel>
          <div className="grid gap-4 xl:grid-cols-2">
            {plugins
              .filter((p) => p.kind === 'data')
              .map((p) => {
                const st = STATUS[p.status] ?? STATUS.invalid;
                return (
                  <Panel key={p.id} className="flex flex-col gap-2">
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0">
                        <h3 className="truncate text-[14px] font-bold">{p.name}</h3>
                        <p className="font-mono text-[11px] text-white/45">v{p.version}{p.author ? ` · ${p.author}` : ''}</p>
                      </div>
                      <Badge variant={p.status === 'ready' ? 'neutral' : st.variant} className="flex-none">
                        {p.status === 'ready' ? 'data' : st.label}
                      </Badge>
                    </div>
                    {p.description && <p className="text-[12.5px] leading-relaxed text-white/60">{p.description}</p>}
                    {p.problem && <p className="font-mono text-[11.5px] text-red-300">{p.problem}</p>}
                    {p.status === 'ready' && (
                      <p className="mt-auto text-[12px] text-white/55">
                        JSON only — fix recipes and tweaks. It can't run anything, so Platypus applies it without asking.
                      </p>
                    )}
                  </Panel>
                );
              })}
          </div>
        </>
      )}

      <MonoLabel className="mb-3 mt-8">built in</MonoLabel>
      <div className="grid gap-2 xl:grid-cols-2">
        {BUILT_IN.map((b) => (
          <div key={b.name} className="flex items-center justify-between gap-4 rounded-2xl bg-white/[0.03] px-4 py-3 ring-1 ring-white/[0.06]">
            <div className="min-w-0">
              <div className="text-[13px] font-bold">{b.name}</div>
              <div className="text-[12px] text-white/50">{b.what}</div>
            </div>
            <Badge variant="neutral" className="flex-none">built in</Badge>
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
            act(p.id, 'approve', LauncherApi.approvePlugin(p.id), `${p.name} approved`);
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
            setAuth((a) => ({ ...a, [p.id]: s }));
            onNotify(`Logged in to ${p.name}${s.user ? ` as ${s.user}` : ''}`);
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
      <span className="w-24 flex-none text-white/45">{label}</span>
      <span className="min-w-0 flex-1 break-words text-white/85 select-text">{items.length ? items.join(', ') : <span className="text-white/40">{empty}</span>}</span>
    </div>
  );
}

/** Shows what the plugin runs and declares before the user trusts it. */
function ApproveModal({ plugin: p, onClose, onApprove }: { plugin: StorePlugin; onClose: () => void; onApprove: () => void }) {
  return (
    <Modal label={`Approve ${p.name}`} onClose={onClose} size="md">
      <div>
        <h2 className="text-[20px] font-extrabold tracking-tight">{p.status === 'changed' ? `Re-approve ${p.name}` : `Approve ${p.name}`}</h2>
        <p className="mt-1 font-mono text-[11.5px] text-white/45">v{p.version}{p.author ? ` · ${p.author}` : ''} · {p.store}</p>
        {p.status === 'changed' && (
          <p className="mt-3 rounded-xl bg-orange-400/[0.08] px-3.5 py-2.5 text-[12px] text-orange-100 ring-1 ring-orange-300/25">
            Its program changed since you approved it. If you didn't update it yourself, don't approve it.
          </p>
        )}

        <div className="mt-4 space-y-2 font-mono text-[11.5px]">
          <ManifestRow label="runs" items={[`${p.dir}/${p.exec}`]} empty="" />
          <ManifestRow label="sha-256" items={[p.hash]} empty="" />
          <ManifestRow label="can" items={p.declared} empty="nothing declared" />
          <ManifestRow label="needs platypus" items={p.needs} empty="nothing in particular" />
          {p.protocol_versions && <ManifestRow label="speaks" items={[`protocol ${p.protocol_versions} over ${p.transport ?? 'jsonrpc-stdio'}`]} empty="" />}
          <ManifestRow label="network" items={p.network} empty="no hosts declared" />
          <ManifestRow label="reads" items={p.read} empty="no paths declared" />
          <ManifestRow label="writes" items={p.write} empty="no paths declared" />
          <ManifestRow
            label="needs"
            items={p.requires.map((r) => `${r.bin}${p.missing.some((m) => m.bin === r.bin) ? ' (missing)' : ''}`)}
            empty="no other tools"
          />
          <ManifestRow label="installs into" items={[p.install_folder]} empty="" />
        </div>

        <div className="mt-4 flex gap-3 rounded-xl bg-white/[0.04] px-3.5 py-3 text-[11.5px] leading-relaxed text-white/60">
          <ShieldAlert className="mt-0.5 h-4 w-4 flex-none text-amber-300/80" />
          <p>
            A store plugin runs with your user's permissions. The hosts and paths above are what it declares; Platypus doesn't enforce them yet. Platypus does check
            every game it launches is inside the plugin's install folder. Approve plugins you trust.
          </p>
        </div>

        <div className="mt-5 flex gap-2.5">
          <Button variant="neutral" className="flex-1" onClick={onClose}>Cancel</Button>
          <Button variant="primary" className="flex-1" onClick={onApprove}>Approve and enable</Button>
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
    <svg viewBox={`0 0 ${size} ${size}`} className="h-48 w-48" role="img" aria-label="Login QR code" shapeRendering="crispEdges">
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
          setError((prev) => (prev && prev.startsWith('Login check') ? null : prev));
          schedule(1500);
        })
        .catch(() => {
          if (stop) return;
          failures += 1;
          if (failures >= QR_ERROR_AFTER) {
            setError(
              attempts >= QR_MAX_ATTEMPTS
                ? 'Login check keeps failing. Close and try again.'
                : `Login check is failing (${failures} attempts). Retrying…`,
            );
          }
          if (attempts >= QR_MAX_ATTEMPTS) return;
          schedule(qrPollDelay(failures));
        });
    };
    schedule(1500);
    return () => {
      stop = true;
      timers.forEach(clearTimeout);
    };
  }, [qrPending, plugin.id]);

  const submit = () => {
    setSubmitting(true);
    setError(null);
    Effect.runPromise(LauncherApi.authComplete(plugin.id, values))
      .then((s) => (s.loggedIn ? onDone(s) : setError('The store says you are still logged out.')))
      .catch((e) => setError(errMsg(e)))
      .finally(() => setSubmitting(false));
  };

  // A QR prompt can be confirmed with an empty form ("I scanned it"); with
  // fields only, every field must be filled.
  const filled = prompt?.fields.every((f) => (values[f.id] ?? '').trim()) ?? false;
  const canSubmit = (prompt?.qr && !filled) || filled;

  return (
    <Modal label={`Log in to ${plugin.name}`} onClose={onClose} size="md">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit && !submitting) submit();
        }}
      >
        <h2 className="text-[20px] font-extrabold tracking-tight">Log in to {plugin.name}</h2>
        <p className="mt-1 text-[12px] text-white/55">Your login stays with {plugin.name}'s own tool; Platypus doesn't store it.</p>
        {!prompt && !error && (
          <div className="mt-5 flex items-center gap-2 font-mono text-[12px] text-white/45"><Loader2 className="h-3.5 w-3.5 animate-spin" /> starting login…</div>
        )}
        {prompt && (
          <div className="mt-4 space-y-4">
            {prompt.url && (
              <div>
                <MonoLabel className="mb-1.5">1 · log in on the store's page</MonoLabel>
                <Button variant="neutral" className="w-full" onClick={() => BrowserOpenURL(prompt.url!)}>
                  <ExternalLink className="h-3.5 w-3.5" /> Open login page
                </Button>
                <p className="mt-1.5 break-all font-mono text-[12px] text-white/55 select-text">{prompt.url}</p>
              </div>
            )}
            {prompt.qr && (
              <div>
                <MonoLabel className="mb-1.5">1 · scan with the {plugin.name.replace(/ \(.*\)$/, '')} mobile app</MonoLabel>
                <div className="flex justify-center rounded-xl bg-white p-3">
                  <QrArt art={prompt.qr} />
                </div>
              </div>
            )}
            {prompt.instructions && <p className="text-[12px] leading-relaxed text-white/65">{prompt.instructions}</p>}
            <div className="space-y-3">
              {prompt.url && <MonoLabel>2 · paste what the page shows</MonoLabel>}
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
        {error && <p role="alert" className="mt-3 font-mono text-[11.5px] text-red-300">{error}</p>}
        <div className="mt-5 flex gap-2.5">
          <Button type="button" variant="neutral" className="flex-1" onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" className="flex-1" disabled={!canSubmit || submitting}>
            {submitting && <Loader2 className="h-3.5 w-3.5 animate-spin" />} {prompt?.qr && !filled ? 'I scanned it' : 'Log in'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
