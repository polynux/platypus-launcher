import React, { useCallback, useEffect, useState } from 'react';
import { Effect } from 'effect';
import { Check, ExternalLink, KeyRound, Lock, Pin, TriangleAlert } from 'lucide-react';
import type { Game, StorePlugin, VersionBuild, VersionChannel, VersionInfo } from '../types';
import { ModalOrInline } from './Modal';
import { Badge } from '@/components/ds/badge';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import { ensureTerms } from '../lib/termsRequest';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import {
  VERSION_WARNING, buildTitle, channelLocked, choiceIsEmpty, isCurrent, lastInstall, pinLabel, pinOf, samePin, shortDate, sourceLabel, targetOf,
  type VersionChoice,
} from '../lib/versions';
import { ErrorNotice } from './ErrorNotice';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

export interface VersionsDialogProps {
  game: Game;
  plugin: StorePlugin;
  onClose: () => void;
  /** Something changed that the library shows (a job queued, a pin set or cleared). */
  onChanged: () => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** Show the content inline (a tab of the game settings) instead of in a dialog of its own. */
  embedded?: boolean;
}

/** Pick which version of a game to install or switch to, hold it there (pin),
 *  or return to the latest. The plugin supplies channels, builds and the typed
 *  input; Platypus adds the install history and the pin. */
export function VersionsDialog({ game, plugin, onClose, onChanged, onNotify, embedded }: VersionsDialogProps) {
  const [info, setInfo] = useState<VersionInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [choice, setChoice] = useState<VersionChoice | null>(null);
  const [typed, setTyped] = useState('');
  const [pinIt, setPinIt] = useState(false);
  const [step, setStep] = useState<'pick' | 'confirm'>('pick');
  const [busy, setBusy] = useState(false);
  const installed = game.is_installed;

  // resetPin: take the checkbox from the saved pin. A reload after an unlock
  // keeps what the user already ticked.
  const load = useCallback((resetPin = true) => {
    setError(null);
    return Effect.runPromise(LauncherApi.getGameVersions(game)).then(
      (v) => { setInfo(v); if (resetPin) setPinIt(v.pinned != null); },
      (e) => setError(errMsg(e)),
    );
  }, [game]);
  useEffect(() => { void load(); }, [load]);

  const select = (c: VersionChoice) => { setChoice(c); setStep('pick'); };
  const type = (text: string) => { setTyped(text); setChoice({ kind: 'manual', text }); setStep('pick'); };

  const current = info != null && isCurrent(info, choice);
  const pin = choice ? pinOf(choice) : null;
  const wantPin = pinIt && pin != null;
  const pinChanged = info != null && (wantPin ? !samePin(info.pinned, pin) : info.pinned != null);
  const ready = info != null && !choiceIsEmpty(choice) && (!current || pinChanged);

  const apply = async () => {
    if (!info || !choice) return;
    setBusy(true);
    const before = info.pinned;
    const pinMoves = wantPin && pin ? !samePin(before, pin) : before != null;
    try {
      // The terms come first: a pin that moves for a job the user won't accept is never set.
      if (!current && !(await ensureTerms(game))) return;
      // The pin goes first: one that can't be set or cleared queues nothing,
      // so a job never runs at a version the pin didn't take.
      if (pinMoves) {
        if (wantPin && pin) await Effect.runPromise(LauncherApi.pinGameVersion(game, pin));
        else await Effect.runPromise(LauncherApi.unpinGameVersion(game.id));
      }
      if (!current) {
        const target = targetOf(choice);
        try {
          await Effect.runPromise(installed ? LauncherApi.updateGameVersion(game, target) : LauncherApi.installGameVersion(game, target));
        } catch (e) {
          if (pinMoves) await putPinBack(before);
          throw e;
        }
        onNotify(`${installed ? 'Switch of' : 'Install of'} ${game.title} queued`, 'busy');
      }
      onChanged();
      onClose();
    } catch (e) {
      onNotify(errMsg(e), 'err');
      setStep('pick');
    } finally {
      setBusy(false);
    }
  };

  /** Undoes a pin change whose job was not queued. */
  const putPinBack = async (previous: VersionInfo['pinned']) => {
    try {
      if (previous) await Effect.runPromise(LauncherApi.pinGameVersion(game, previous));
      else await Effect.runPromise(LauncherApi.unpinGameVersion(game.id));
    } catch (e) {
      onNotify(`The pin couldn't be put back: ${errMsg(e)}`, 'err');
    }
  };

  const returnToLatest = async () => {
    setBusy(true);
    try {
      await Effect.runPromise(LauncherApi.unpinGameVersion(game.id));
      onNotify(`${game.title} follows the latest version again`);
      onChanged();
      await load();
    } catch (e) {
      onNotify(errMsg(e), 'err');
    } finally {
      setBusy(false);
    }
  };

  const updateToPin = async () => {
    setBusy(true);
    try {
      if (!(await ensureTerms(game))) return;
      await Effect.runPromise(LauncherApi.updateGame(game));
      onNotify(`Update of ${game.title} queued`, 'busy');
      onChanged();
      onClose();
    } catch (e) {
      onNotify(errMsg(e), 'err');
    } finally {
      setBusy(false);
    }
  };

  const last = info ? lastInstall(info.ledger) : undefined;

  return (
    <ModalOrInline embedded={embedded} label={`Version of ${game.title}`} onClose={onClose} size="lg" dismissable={!busy}>
      {!embedded && (
        <>
          <h2 className="text-[20px] font-extrabold tracking-tight">{installed ? 'Version' : 'Choose a version'}</h2>
          <p className="mt-1 text-[12.5px] text-white/55">
            {game.title} · {plugin.name}
          </p>
        </>
      )}

      {!info && !error && <p className="mt-6 font-mono text-micro uppercase tracking-widest text-white/55">loading versions…</p>}
      {error && (
        <ErrorNotice className="mt-5 rounded-xl bg-red-500/10 px-4 py-3" message={error} fallback="Platypus couldn't list this game's versions." />
      )}

      {info && (
        <>
          <dl className="mt-4 space-y-2 font-mono text-hint">
            {installed && (
              <div className="flex justify-between gap-4 border-t border-white/[0.05] pt-2">
                <dt className="text-white/55">Installed</dt>
                <dd className="truncate text-right text-white/85" title={game.version}>{game.version || '—'}</dd>
              </div>
            )}
            {last && (
              <div className="flex justify-between gap-4 border-t border-white/[0.05] pt-2">
                <dt className="text-white/55">Last install</dt>
                <dd className="truncate text-right text-white/85">
                  {[last.versionName || last.build, last.channel, shortDate(last.date)].filter(Boolean).join(' · ')}
                </dd>
              </div>
            )}
          </dl>

          {info.pinned && (
            <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-amber-300/[0.07] px-4 py-3 ring-1 ring-amber-300/25">
              <p className="min-w-0 text-[12.5px] leading-relaxed text-white/75">
                <span className="inline-flex items-center gap-1.5 font-bold text-amber-200"><Pin className="h-3.5 w-3.5" /> Pinned to {pinLabel(info.pinned, info)}</span>
                <br />
                Updates install this version and the update prompt stays off.
              </p>
              <div className="flex flex-none gap-2">
                {installed && info.updateAvailable && (
                  <Button variant="primary" size="sm" disabled={busy} onClick={updateToPin}>Update to pinned version</Button>
                )}
                <Button variant="neutral" size="sm" disabled={busy} onClick={returnToLatest}>Return to latest</Button>
              </div>
            </div>
          )}

          {info.channels.length > 0 && (
            <section className="mt-5">
              <MonoLabel className="mb-1.5">channels</MonoLabel>
              <div role="radiogroup" aria-label="Channels" className="space-y-1.5">
                {info.channels.map((c) => (
                  <ChannelRow
                    key={c.id}
                    channel={c}
                    selected={choice?.kind === 'channel' && choice.id === c.id}
                    onSelect={() => select({ kind: 'channel', id: c.id })}
                    game={game}
                    onUnlocked={() => load(false)}
                    onNotify={onNotify}
                  />
                ))}
              </div>
            </section>
          )}

          {info.builds.length > 0 && (
            <section className="mt-5">
              <MonoLabel className="mb-1.5">builds</MonoLabel>
              <div role="radiogroup" aria-label="Builds" className="space-y-1.5">
                {info.builds.map((b) => (
                  <BuildRow
                    key={b.id}
                    build={b}
                    selected={choice?.kind === 'build' && choice.id === b.id}
                    onSelect={() => select({ kind: 'build', id: b.id })}
                  />
                ))}
              </div>
            </section>
          )}

          {info.manual && (
            <section className="mt-5">
              <label className="block">
                <MonoLabel className="mb-1.5">{info.manual.label}</MonoLabel>
                <textarea
                  value={typed}
                  onChange={(e) => type(e.target.value)}
                  rows={2}
                  aria-label={info.manual.label}
                  className="w-full rounded-xl bg-white/[0.06] px-3 py-2 font-mono text-[12px] text-white placeholder-white/55 outline-none ring-amber-300/50 transition focus:ring-1"
                />
              </label>
              {info.manual.hint && <p className="mt-1 text-[12px] text-white/55">{info.manual.hint}</p>}
            </section>
          )}

          {info.links.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
              {info.links.map((l) => (
                <Button key={l.url} variant="link" size="sm" onClick={() => BrowserOpenURL(l.url)} className="gap-1">
                  {l.label} <ExternalLink className="h-3 w-3" />
                </Button>
              ))}
            </div>
          )}

          {info.channels.length === 0 && info.builds.length === 0 && !info.manual && (
            <p className="mt-5 rounded-xl bg-white/[0.04] px-4 py-3 text-[12.5px] text-white/60">
              {plugin.name} lists only one version of this game.
            </p>
          )}

          {step === 'confirm' ? (
            <div role="alert" className="mt-6 rounded-2xl bg-amber-300/[0.07] px-4 py-3.5 ring-1 ring-amber-300/25">
              <p className="flex items-center gap-1.5 text-[12.5px] font-bold text-amber-200"><TriangleAlert className="h-4 w-4" /> Before you {installed ? 'switch' : 'install'}</p>
              <p className="mt-1 text-[12.5px] leading-relaxed text-white/75">{VERSION_WARNING}</p>
              <div className="mt-3 flex justify-end gap-2.5">
                <Button variant="neutral" size="sm" disabled={busy} onClick={() => setStep('pick')}>Back</Button>
                <Button variant="primary" size="sm" disabled={busy} onClick={apply}>{installed ? 'Switch anyway' : 'Install anyway'}</Button>
              </div>
            </div>
          ) : (
            <div className="mt-6">
              <label className={cx('flex items-start gap-2.5 text-[12.5px] leading-snug', pin == null ? 'text-white/55' : 'text-white/70')}>
                <input
                  type="checkbox"
                  checked={pinIt && pin != null}
                  disabled={pin == null}
                  onChange={(e) => setPinIt(e.target.checked)}
                  className="mt-0.5 h-3.5 w-3.5 accent-amber-300"
                />
                <span>
                  Keep this version (pin)
                  <span className="block text-[12px] text-white/55">
                    {pin != null
                      ? 'Updates install it, and the update prompt stays off.'
                      : choice == null
                        ? 'Pick a build or channel to pin it.'
                        : `Typed values can’t be pinned.${info.pinned ? ' Installing this removes the current pin.' : ''}`}
                  </span>
                </span>
              </label>
              <div className="mt-4 flex justify-end gap-2.5">
                <Button variant="neutral" onClick={onClose} disabled={busy}>Close</Button>
                <Button
                  variant="primary"
                  disabled={!ready || busy}
                  onClick={() => (current ? void apply() : setStep('confirm'))}
                >
                  {current && pinChanged ? (wantPin ? 'Pin this version' : 'Remove the pin') : current ? 'This is the installed version' : installed ? 'Switch to this version' : 'Install this version'}
                </Button>
              </div>
            </div>
          )}
        </>
      )}
      {error && (
        <div className="mt-5 flex justify-end">
          <Button variant="neutral" onClick={onClose}>Close</Button>
        </div>
      )}
    </ModalOrInline>
  );
}

function RowShell({
  selected, disabled, onSelect, label, children,
}: { selected: boolean; disabled?: boolean; onSelect: () => void; label: string; children: React.ReactNode }) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      aria-label={label}
      disabled={disabled}
      onClick={onSelect}
      className={cx(
        'flex w-full items-start gap-3 rounded-xl px-3 py-2.5 text-left ring-1 transition',
        selected ? 'bg-amber-300/[0.07] ring-amber-300/30' : 'ring-white/[0.06] hover:bg-white/[0.04]',
        disabled && 'cursor-not-allowed opacity-50 hover:bg-transparent',
      )}
    >
      <span aria-hidden className={cx('mt-0.5 grid h-4 w-4 flex-none place-items-center rounded-full ring-1', selected ? 'bg-amber-300 text-[#130f0a] ring-amber-300' : 'ring-white/25')}>
        {selected && <Check className="h-3 w-3" strokeWidth={3} />}
      </span>
      <span className="min-w-0 flex-1">{children}</span>
    </button>
  );
}

function ChannelRow({
  channel, selected, onSelect, game, onUnlocked, onNotify,
}: {
  channel: VersionChannel;
  selected: boolean;
  onSelect: () => void;
  game: Game;
  onUnlocked: () => Promise<void> | void;
  onNotify: VersionsDialogProps['onNotify'];
}) {
  const locked = channelLocked(channel);
  return (
    <div>
      <RowShell selected={selected} disabled={locked} onSelect={onSelect} label={channel.name}>
        <span className="flex flex-wrap items-center gap-2">
          <span className="text-[12.5px] font-semibold text-white/90">{channel.name}</span>
          {channel.current && <Badge variant="ok">installed</Badge>}
          {channel.requires === 'password' && (
            <Badge variant={locked ? 'bronze' : 'neutral'} className="gap-1">
              {locked ? <Lock className="h-3 w-3" /> : <KeyRound className="h-3 w-3" />} {locked ? 'password' : 'unlocked'}
            </Badge>
          )}
        </span>
        {channel.description && <span className="block text-[12px] leading-snug text-white/55">{channel.description}</span>}
        {(channel.updatedAt || channel.buildId) && (
          <span className="mt-0.5 block font-mono text-hint text-white/55">
            {[channel.buildId && `build ${channel.buildId}`, channel.updatedAt && `updated ${channel.updatedAt}`].filter(Boolean).join(' · ')}
          </span>
        )}
      </RowShell>
      {locked && <UnlockForm channel={channel} game={game} onUnlocked={onUnlocked} onNotify={onNotify} />}
    </div>
  );
}

/** The password goes to the plugin and is dropped from state as soon as it is
 *  sent: Platypus keeps it nowhere. */
function UnlockForm({
  channel, game, onUnlocked, onNotify,
}: { channel: VersionChannel; game: Game; onUnlocked: () => Promise<void> | void; onNotify: VersionsDialogProps['onNotify'] }) {
  const [password, setPassword] = useState('');
  const [problem, setProblem] = useState('');
  const [busy, setBusy] = useState(false);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!password) return;
    const sent = password;
    setPassword('');
    setProblem('');
    setBusy(true);
    try {
      const ok = await Effect.runPromise(LauncherApi.unlockVersionChannel(game, channel.id, sent));
      if (ok) {
        onNotify(`${channel.name} unlocked`);
        await onUnlocked();
      } else {
        setProblem('That password wasn’t accepted.');
      }
    } catch (err) {
      setProblem(errMsg(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <form onSubmit={submit} className="mt-1.5 flex items-start gap-2 pl-7">
      <div className="min-w-0 flex-1">
        <Input
          type="password"
          autoComplete="off"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={`Password for ${channel.name}`}
          aria-label={`Password for ${channel.name}`}
          aria-invalid={problem ? true : undefined}
        />
        {problem && <ErrorNotice className="mt-1 text-[12px]" message={problem} fallback="Platypus couldn't check that password." />}
      </div>
      <Button type="submit" size="md" disabled={!password || busy}>Unlock</Button>
    </form>
  );
}

function BuildRow({ build, selected, onSelect }: { build: VersionBuild; selected: boolean; onSelect: () => void }) {
  const title = buildTitle(build);
  return (
    <RowShell selected={selected} disabled={!build.available} onSelect={onSelect} label={title}>
      <span className="flex flex-wrap items-center gap-2">
        <span className="text-[12.5px] font-semibold text-white/90">{title}</span>
        {build.current && <Badge variant="ok">installed</Badge>}
        <Badge variant={build.source === 'store' ? 'neutral' : build.source === 'archive' ? 'info' : 'accent'}>{sourceLabel(build.source)}</Badge>
        {build.channel && <Badge variant="neutral">{build.channel}</Badge>}
      </span>
      <span className="mt-0.5 block font-mono text-hint text-white/55">
        {[build.name && build.name !== build.id ? build.id : '', build.publishedAt, !build.available && 'can’t be fetched right now'].filter(Boolean).join(' · ')}
      </span>
    </RowShell>
  );
}
