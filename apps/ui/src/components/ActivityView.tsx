// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useMemo, useState } from 'react';
import { ArrowDownToLine, CircleDot, Cloud, Clock, Download, Trophy } from 'lucide-react';
import type { ActivityEvent, ActivityType, Game } from '../types';
import { ActivityFeed } from './ActivityFeed';
import { ATLAS_PAGE, Panel, Section, ViewHeader, EmptyState, clock, cx } from './ui';
import { Button } from './ds/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { currentLocale } from '../lib/format';
import { useVariant } from '../lib/theme';

export function ActivityView({ events, games = [] }: { events: ActivityEvent[]; games?: Game[] }) {
  const { t } = useTranslation(['activity', 'nav']);
  // Atlas renders the /22 Activity page (issue #138); other themes keep the feed panel.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return <AtlasActivity events={events} games={games} />;
  }
  return (
    <div className="p-7">
      <ViewHeader
        title={t('nav:title.activity')}
        subtitle={<p className="mt-0.5 text-[12.5px] text-text-muted">{t('activity:view.subtitle')}</p>}
      />
      <Panel as="section">
        <Section>
          <ActivityFeed events={events} limit={50} />
        </Section>
      </Panel>
    </div>
  );
}

/** The lab's kind chips, mapped onto the app's real event types. Installs
 *  cover downloads and adds; the fix, remove and tweak types show only when
 *  no chip filters them. */
const KIND_TYPES: Record<string, ActivityType[]> = {
  sessions: ['session'],
  installs: ['download', 'add'],
  updates: ['update'],
  cloud: ['cloud'],
  achievements: ['unlock'],
};

const KIND_ICON: Record<ActivityType, React.ComponentType<{ className?: string }>> = {
  session: Clock,
  download: Download,
  add: Download,
  update: ArrowDownToLine,
  cloud: Cloud,
  unlock: Trophy,
  fix: CircleDot,
  tweak: CircleDot,
  remove: CircleDot,
};

/** The /22 Activity page (issue #138, Atlas only): kind chips, a game
 *  filter and the day-grouped feed. Same events; only the markup is /22. */
function AtlasActivity({ events, games }: { events: ActivityEvent[]; games: Game[] }) {
  const { t } = useTranslation(['activity', 'nav', 'common']);
  const [kinds, setKinds] = useState<string[]>([]);
  const [game, setGame] = useState('all');
  const byId = useMemo(() => new Map(games.map((g) => [g.id, g])), [games]);
  const activeTypes = useMemo(() => kinds.flatMap((k) => KIND_TYPES[k] ?? []), [kinds]);
  const items = events.filter(
    (e) => (!activeTypes.length || activeTypes.includes(e.type)) && (game === 'all' || e.game === game),
  );
  const days = useMemo(() => {
    const groups: { key: string; label: string; items: ActivityEvent[] }[] = [];
    for (const e of items) {
      const d = new Date(e.ts);
      const key = `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
      const today = new Date();
      const yesterday = new Date();
      yesterday.setDate(today.getDate() - 1);
      const label = d.toDateString() === today.toDateString()
        ? t('activity:atlas.today')
        : d.toDateString() === yesterday.toDateString()
          ? t('activity:atlas.yesterday')
          : d.toLocaleDateString(currentLocale(), { weekday: 'short', day: 'numeric', month: 'short' });
      const last = groups[groups.length - 1];
      if (last?.key === key) last.items.push(e);
      else groups.push({ key, label, items: [e] });
    }
    return groups;
  }, [items, t]);
  const kindIds = Object.keys(KIND_TYPES);
  return (
    <div className={ATLAS_PAGE}>
      <header>
        <h1 className="text-[24px] font-extrabold tracking-tight">{t('nav:title.activity')}</h1>
        <p className="mt-[3px] text-[13px] text-text-muted">{t('activity:view.subtitle')}</p>
      </header>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap gap-1.5" role="group" aria-label={t('activity:atlas.filterKinds')}>
          {kindIds.map((k) => {
            const on = kinds.includes(k);
            return (
              <button
                key={k}
                type="button"
                aria-pressed={on}
                onClick={() => setKinds((x) => (x.includes(k) ? x.filter((y) => y !== k) : [...x, k]))}
                className={cx(
                  'rounded-full px-3 py-1.5 text-[12.5px] font-semibold ring-1 transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                  on ? 'bg-accent font-bold text-on-accent ring-accent' : 'bg-surface text-text-muted ring-hairline hover:text-text',
                )}
              >
                {t(`activity:atlas.kinds.${k}`)}
              </button>
            );
          })}
        </div>
        {games.length > 0 && (
          <Select value={game} onValueChange={setGame}>
            <SelectTrigger aria-label={t('activity:atlas.filterGame')} className="w-52">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t('activity:atlas.everyGame')}</SelectItem>
              {games.map((g) => (
                <SelectItem key={g.id} value={g.id}>{g.title}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>
      {items.length === 0 ? (
        <EmptyState
          icon={Clock}
          label={t('activity:atlas.noMatch')}
          action={
            (kinds.length > 0 || game !== 'all') ? (
              <Button variant="neutral" size="sm" onClick={() => { setKinds([]); setGame('all'); }}>
                {t('activity:atlas.clearFilters')}
              </Button>
            ) : undefined
          }
        />
      ) : (
        days.map((d) => (
          <section key={d.key} aria-label={d.label}>
            <h2 className="mb-1.5 text-[13px] font-bold text-text-muted">{d.label}</h2>
            <ul>
              {d.items.map((e) => {
                const Icon = KIND_ICON[e.type] ?? CircleDot;
                const title = (e.game && byId.get(e.game)?.title) || e.game || '';
                return (
                  <li key={e.id} className="grid grid-cols-[28px_52px_minmax(140px,220px)_minmax(0,1fr)] items-center gap-2.5 rounded-lg px-2 py-1 transition hover:bg-surface">
                    <span className="grid size-[26px] place-items-center rounded-lg bg-surface-strong text-text-muted">
                      <Icon className="size-3.5" />
                    </span>
                    <time className="font-mono text-hint tabular-nums text-text-faint">{clock(e.ts)}</time>
                    <span className="truncate text-left text-[13px] font-semibold">{title}</span>
                    <span className="truncate text-[13px] text-text-muted">{e.detail}{e.meta ? ` · ${e.meta}` : ''}</span>
                  </li>
                );
              })}
            </ul>
          </section>
        ))
      )}
    </div>
  );
}
