// i18n: converted
import React from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from './ds/button';
import type { ActivityEvent, ActivityType } from '../types';
import { Panel, EVENT_DOT, EVENT_TEXT, clock, cx } from './ui';

const TYPE_KEY: Record<ActivityType, string> = {
  session: 'activity:type.session',
  unlock: 'activity:type.unlock',
  fix: 'activity:type.fix',
  update: 'activity:type.update',
  add: 'activity:type.add',
  remove: 'activity:type.remove',
  download: 'activity:type.download',
  tweak: 'activity:type.tweak',
  cloud: 'activity:type.cloud',
  scan: 'activity:type.update',
};

export function ActivityFeed({ events, limit }: { events: ActivityEvent[]; limit?: number }) {
  const { t } = useTranslation('activity');
  const rows = limit ? events.slice(0, limit) : events;
  if (rows.length === 0) {
    return <p className="py-3 text-[12.5px] text-white/70">{t('activity:feed.empty')}</p>;
  }
  return (
    <ul className="divide-y divide-white/[0.05]">
      {rows.map((e) => (
        <li key={e.id} className="grid grid-cols-[3rem_8px_minmax(0,1fr)_auto] items-center gap-3 py-2">
          <time className="font-mono text-hint tabular-nums text-white/55">{clock(e.ts)}</time>
          <span className={cx('h-1.5 w-1.5 rounded-full', EVENT_DOT[e.type])} />
          <span className="truncate text-[13px]">
            <span className={cx('mr-2 font-mono text-micro uppercase tracking-wider', EVENT_TEXT[e.type])}>{t(TYPE_KEY[e.type])}</span>
            <span className="text-white/85">{e.detail}</span>
          </span>
          <span className="font-mono text-hint text-white/55">{e.meta}</span>
        </li>
      ))}
    </ul>
  );
}

export function ActivityPanel({ events, limit = 8, onFullLog }: { events: ActivityEvent[]; limit?: number; onFullLog: () => void }) {
  const { t } = useTranslation('activity');
  return (
    <Panel as="section" className="bg-white/[0.04]">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-[15px] font-bold">{t('activity:panel.title')}</h2>
        <Button variant="link" onClick={onFullLog}>{t('activity:panel.fullLog')}</Button>
      </div>
      <ActivityFeed events={events} limit={limit} />
    </Panel>
  );
}
