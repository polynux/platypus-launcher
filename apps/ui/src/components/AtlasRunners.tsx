// i18n: converted
// The /22 Runners page (issue #138): default picker, usage, install/remove.
// Moved out of SystemView.tsx (issue #156) so each file holds one presentation.
import { useTranslation } from 'react-i18next';
import { Download, Loader2, RefreshCw, Trash2 } from 'lucide-react';
import type { Game, Job } from '../types';
import type { types as gen } from '../gen/models';
import { ATLAS_PAGE, Cover, EmptyState } from './ui';
import { Button } from './ds/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { heroImages } from '../lib/games';
import { formatBytes } from '../lib/games';
import type { SystemViewProps } from './SystemView';

const AUTOMATIC = 'automatic';

/** The /22 Runners page (issue #138, Atlas only): the default picker row is
 *  the current default, and each row names the build with its path, size and
 *  per-game usage, with Install/Remove backed by the core (issue #142). */
export function AtlasRunners({ runners, games, onRefresh, scanning, picked, onDefaultRunner, available, installing, removing, onInstall, onRemove, sizeLabel }: SystemViewProps & { games: Game[]; picked: string | null; available: gen.AvailableRunner[]; installing: string | null; removing: string | null; onInstall: (tag: string) => void; onRemove: (id: string, name: string, size: number) => void; sizeLabel: (n?: number) => string }) {
  const { t } = useTranslation(['nav', 'runners']);
  const fallback = runners.find((r) => r.is_default) ?? runners[0];
  // A default that is no longer installed runs the automatic pick, so the
  // picker says so rather than showing a runner the list doesn't have.
  const stored = picked && runners.some((r) => r.id === picked) ? picked : '';
  const shown = picked === null ? (fallback?.id ?? '') : stored === '' ? AUTOMATIC : stored;
  const uses = (id: string, name: string): Game[] =>
    games.filter((g) => !g.native && (g.settings.runner_id === id || g.settings.runner_id === name || ((!g.settings.runner_id || g.settings.runner_id === 'auto') && fallback?.id === id)));
  const installedTags = new Set(runners.map((r) => (r.id.startsWith('compat:') ? r.id.slice('compat:'.length) : '')));
  const pending = available.filter((a) => !installedTags.has(a.tag));
  return (
    <div className={ATLAS_PAGE}>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[24px] font-extrabold tracking-tight">{t('nav:title.runners')}</h1>
          <p className="mt-[3px] text-[13px] text-text-muted">{t('runners:atlas.sub')}</p>
        </div>
        <Button variant="neutral" onClick={onRefresh} disabled={scanning}>
          {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {scanning ? t('runners:scanning') : t('runners:rescan')}
        </Button>
      </header>
      {runners.length === 0 ? (
        <EmptyState label={t('runners:emptyTitle')}>{t('runners:emptyBody')}</EmptyState>
      ) : (
        <>
          <section aria-label={t('runners:atlas.defaultLabel')} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-[18px] border-b border-hairline py-3.5">
            <div className="grid min-w-0 gap-[3px]">
              <h2 className="text-[13px] font-semibold text-text">{t('runners:atlas.defaultLabel')}</h2>
              <p className="text-[0.93em] leading-normal text-text-muted">{t('runners:atlas.defaultDesc')}</p>
            </div>
            {/* The core names the default and takes the pick (issue #146). */}
            <Select
              value={shown}
              onValueChange={(id) => onDefaultRunner?.(id === AUTOMATIC ? '' : id)}
              disabled={!onDefaultRunner}
              aria-label={t('runners:atlas.defaultLabel')}
            >
              <SelectTrigger className="h-8 w-[210px] flex-none bg-surface font-sans text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={AUTOMATIC} className="font-sans text-[13px]">{t('runners:atlas.automatic')}</SelectItem>
                {runners.map((r) => (
                  <SelectItem key={r.id} value={r.id} className="font-sans text-[13px]">{r.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </section>
          <section aria-label={t('runners:atlas.installed')}>
            <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('runners:atlas.installed')}</h2>
            <ul className="divide-y divide-hairline">
              {runners.map((r) => {
                const used = uses(r.id, r.name);
                const removable = !r.is_system && r.id.startsWith('compat:');
                return (
                  <li key={r.id} className="grid min-h-14 grid-cols-[minmax(200px,1.3fr)_minmax(160px,1fr)_70px_130px] items-center gap-3.5 border-b border-hairline">
                    <div className="min-w-0 flex flex-wrap items-center gap-2">
                      <div className="flex basis-full items-center gap-2"><span className="basis-auto text-[13px] font-bold text-text">{r.name}</span>
                        {r.is_default && (
                          <span className="rounded-md bg-surface-strong px-1.5 py-0.5 text-[11.5px] font-semibold text-text-muted">
                            {t('runners:atlas.defaultBadge')}
                          </span>
                        )}
                      </div>
                      <div className="basis-full truncate text-[12.5px] text-text-faint select-text">{r.path}</div>
                    </div>
                    <div className="flex min-w-0 items-center gap-1">
                      {used.length > 0 ? (
                        <>
                          <span className="flex items-center gap-1" aria-hidden>
                            {used.slice(0, 5).map((g) => (
                              <Cover key={g.id} src={heroImages(g)} title="" className="h-5 w-[42px] rounded" />
                            ))}
                          </span>
                          <span className="ml-1.5 text-[12.5px] text-text-faint">{t('runners:atlas.gamesCount', { count: used.length })}</span>
                        </>
                      ) : (
                        <span className="text-[12.5px] text-text-faint">{t('runners:atlas.noGames')}</span>
                      )}
                    </div>
                    <span className="text-right tabular-nums text-text-faint">{sizeLabel(r.size_bytes)}</span>
                    {r.is_system ? (
                      <span className="text-[12.5px] text-text-faint">{t('runners:atlas.fromSystem')}</span>
                    ) : removable ? (
                      <Button variant="ghost" size="sm" onClick={() => onRemove(r.id, r.name, r.size_bytes ?? 0)} disabled={removing === r.id}>
                        {removing === r.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />}
                        {removing === r.id ? t('runners:removing') : t('runners:remove')}
                      </Button>
                    ) : (
                      <span />
                    )}
                  </li>
                );
              })}
            </ul>
          </section>
          {pending.length > 0 && (
            <section aria-label={t('runners:atlas.available')}>
              <h2 className="mb-2.5 mt-6 text-[13px] font-bold text-text-muted">{t('runners:atlas.available')}</h2>
              <ul className="divide-y divide-hairline">
                {pending.map((a) => (
                  <li key={a.tag} className="grid min-h-14 grid-cols-[minmax(200px,1.3fr)_minmax(160px,1fr)_70px_130px] items-center gap-3.5 border-b border-hairline">
                    <div className="min-w-0 flex flex-wrap items-center gap-2">
                      <div className="flex basis-full items-center gap-2"><span className="basis-auto text-[13px] font-bold text-text">{a.name}</span></div>
                      <div className="basis-full truncate text-[12.5px] text-text-faint select-text">{a.tag}</div>
                    </div>
                    <div className="flex min-w-0 items-center gap-1">
                      <span className="text-[12.5px] text-text-faint">{t('runners:atlas.noGames')}</span>
                    </div>
                    <span className="text-right tabular-nums text-text-faint">{sizeLabel(a.size_bytes)}</span>
                    <Button variant="neutral" size="sm" onClick={() => onInstall(a.tag)} disabled={installing === a.tag}>
                      {installing === a.tag ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
                      {installing === a.tag ? t('runners:installing') : t('runners:install')}
                    </Button>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </div>
  );
}
