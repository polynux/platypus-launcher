// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useEffect, useRef, useState } from 'react';
import { Download, Loader2, RefreshCw, Trash2 } from 'lucide-react';
import { Effect } from 'effect';
import type { Game, Job, ProtonVersion } from '../types';
import type { types as gen } from '../gen/models';
import { LauncherApi } from '../effect/runtime';
import { coreErrMsg } from '../lib/coreError';
import { Cover, EmptyState, ListRow, Section, ViewHeader } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { formatBytes } from '../lib/games';
import { useVariant } from '../lib/theme';
import { AtlasRunners } from './AtlasRunners';

export interface SystemViewProps {
  runners: ProtonVersion[];
  onRefresh: () => void;
  scanning: boolean;
  /** Installed games, for each runner's usage strip (Atlas only). */
  games?: Game[];
  /** Makes a runner the launcher's default (issue #146): what "auto" runs. */
  onDefaultRunner?: (id: string) => void;
  /** The download queue, so the page refreshes when a runner install lands. */
  jobs?: Job[];
  /** Reports install/remove failures (issue #142, M7). */
  onNotify?: (msg: string, kind: 'err') => void;
}

/** The picker's value for "no default": Radix needs a non-empty item value. */
const AUTOMATIC = 'automatic';

export function SystemView({ runners, onRefresh, scanning, games, onDefaultRunner, jobs, onNotify }: SystemViewProps) {
  const { t } = useTranslation(['nav', 'runners', 'errors']);
  // The stored pick (issue #146): "" is the automatic one, null is not read yet.
  // The runner list names the runner "auto" runs, not whether the user picked
  // it, so the page asks the core for the setting and can offer a way back.
  const [picked, setPicked] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => live && setPicked(s.default_runner_id ?? ''))
      .catch(() => {});
    return () => { live = false; };
  }, []);
  const choose = (id: string) => {
    setPicked(id);
    onDefaultRunner?.(id);
  };
  // Installable builds (issue #142): listed through the core, installed as a
  // job with progress (the job shows in Downloads), removed here.
  const [available, setAvailable] = useState<gen.AvailableRunner[]>([]);
  const [installing, setInstalling] = useState<string | null>(null);
  const [removing, setRemoving] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    const api = LauncherApi as unknown as { listAvailableRunners?: () => Effect.Effect<gen.AvailableRunner[], Error> };
    if (typeof api.listAvailableRunners !== 'function') return;
    Effect.runPromise(api.listAvailableRunners())
      .then((items) => live && setAvailable(items ?? []))
      .catch((err: unknown) => {
        if (!live) return;
        const failure = err instanceof Error ? err : new Error(String(err));
        const net = (LauncherApi as unknown as { getNetState?: () => Effect.Effect<{ online: boolean }, Error> }).getNetState;
        if (typeof net !== 'function') {
          onNotify?.(coreErrMsg(failure), 'err');
          return;
        }
        Effect.runPromise(net()).then(
          (ns) => live && onNotify?.(ns && !ns.online ? t('runners:offline') : coreErrMsg(failure), 'err'),
          () => live && onNotify?.(coreErrMsg(failure), 'err'),
        );
      });
    return () => { live = false; };
  }, [onNotify, t]);
  const installedTags = new Set(
    runners.map((r) => (r.id.startsWith('compat:') ? r.id.slice('compat:'.length) : '')),
  );
  const install = (tag: string) => {
    setInstalling(tag);
    Effect.runPromise(LauncherApi.installRunner(tag))
      .then(() => onRefresh())
      .catch((err: unknown) => onNotify?.(coreErrMsg(err instanceof Error ? err : new Error(String(err))), 'err'))
      .finally(() => setInstalling(null));
  };
  const remove = (id: string, name: string, size: number) => {
    setRemoving(id);
    Effect.runPromise(LauncherApi.checkRemoveRunner(id))
      .then((check) => {
        const used = check?.used_by ?? [];
        const sizeLabel = size > 0 ? formatBytes(size) : t('runners:sizeUnknown');
        if (used.length > 0 && !window.confirm(t('runners:removeInUse', { size: sizeLabel, count: used.length }))) {
          return;
        }
        if (used.length === 0 && !window.confirm(t('runners:removeConfirmTitle', { runner: name }))) {
          return;
        }
        return Effect.runPromise(LauncherApi.removeRunner(id, used.length > 0)).then(() => onRefresh());
      })
      .catch((err: unknown) => onNotify?.(coreErrMsg(err instanceof Error ? err : new Error(String(err))), 'err'))
      .finally(() => setRemoving(null));
  };
  // A finished runner install lands outside this page's own actions (issue
  // #142, M7): refresh the list when one completes, and surface failures.
  const seenJobs = useRef<Set<string>>(new Set());
  useEffect(() => {
    if (!jobs) return;
    for (const j of jobs) {
      if (j.kind !== 'runner-install') continue;
      if (j.state !== 'done' && j.state !== 'failed') continue;
      if (seenJobs.current.has(j.id)) continue;
      seenJobs.current.add(j.id);
      if (j.state === 'done') onRefresh();
      else onNotify?.(j.error || t('errors:fallback'), 'err');
    }
  }, [jobs, onRefresh, onNotify, t]);
  const sizeLabel = (n?: number) => (n && n > 0 ? formatBytes(n) : t('runners:sizeUnknown'));
  // One branch per view root (issue #156): the runners presentation, not the shell.
  const presentation = useVariant('runners.presentation');
  if (presentation === 'table') {
    return <AtlasRunners runners={runners} games={games ?? []} onRefresh={onRefresh} scanning={scanning} picked={picked} onDefaultRunner={onDefaultRunner ? choose : undefined} available={available} installing={installing} removing={removing} onInstall={install} onRemove={remove} sizeLabel={sizeLabel} />;
  }
  return (
    <div className="p-7">
      <ViewHeader
        title={t('nav:title.runners')}
        subtitle={<><p className="mt-0.5 text-[12.5px] text-text-muted">{t('runners:intro')}</p>
        <p className="mt-1.5 max-w-xl text-[12px] leading-relaxed text-text-muted">
          {t('runners:about')}
        </p></>}
        actions={<>
          {onDefaultRunner && picked ? (
            <Button variant="ghost" onClick={() => choose('')}>{t('runners:useAutomatic')}</Button>
          ) : null}
          <Button variant="neutral" onClick={onRefresh} disabled={scanning}>
            {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
            {scanning ? t('runners:scanning') : t('runners:rescan')}
          </Button>
        </>}
      />

      {runners.length === 0 ? (
        <EmptyState label={t('runners:emptyTitle')}>
          {t('runners:emptyBody')}
        </EmptyState>
      ) : (
        <Section className="space-y-2">
          <h2 className="text-[13px] font-bold text-text-muted">{t('runners:installedTitle')}</h2>
          {runners.map((r) => (
            <ListRow key={r.id} kind="runner">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-[13px] font-bold">{r.name}</span>
                  {r.is_custom && <Badge variant="gold">{t('runners:badge.custom')}</Badge>}
                  {r.is_system && <Badge variant="neutral">{t('runners:badge.system')}</Badge>}
                  {r.is_default && <Badge variant="neutral">{t('runners:badge.default')}</Badge>}
                </div>
                <div className="truncate font-mono text-hint text-text-muted select-text">{r.path}</div>
                <div className="text-[12px] text-text-muted">{sizeLabel(r.size_bytes)}</div>
              </div>
              <div className="flex flex-none items-center gap-2">
                {onDefaultRunner && !r.is_default && (
                  <Button variant="ghost" size="sm" onClick={() => onDefaultRunner(r.id)}>{t('runners:makeDefault')}</Button>
                )}
                {!r.is_system && r.id.startsWith('compat:') && (
                  <Button variant="ghost" size="sm" onClick={() => remove(r.id, r.name, r.size_bytes ?? 0)} disabled={removing === r.id}>
                    {removing === r.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />}
                    {removing === r.id ? t('runners:removing') : t('runners:remove')}
                  </Button>
                )}
                <Badge variant="ok" className="flex-none gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full bg-ok" /> {t('runners:badge.found')}
                </Badge>
              </div>
            </ListRow>
          ))}
        </Section>
      )}
      {available.filter((a) => !installedTags.has(a.tag)).length > 0 && (
        <Section className="mt-6 space-y-2">
          <h2 className="text-[13px] font-bold text-text-muted">{t('runners:availableTitle')}</h2>
          {available.filter((a) => !installedTags.has(a.tag)).map((a) => (
            <ListRow key={a.tag} kind="runner">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-[13px] font-bold">{a.name}</span>
                </div>
                <div className="text-[12px] text-text-muted">{sizeLabel(a.size_bytes)}</div>
              </div>
              <div className="flex flex-none items-center gap-2">
                <Button variant="neutral" size="sm" onClick={() => install(a.tag)} disabled={installing === a.tag}>
                  {installing === a.tag ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
                  {installing === a.tag ? t('runners:installing') : t('runners:install')}
                </Button>
              </div>
            </ListRow>
          ))}
        </Section>
      )}
    </div>
  );
}
