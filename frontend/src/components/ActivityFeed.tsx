import React from 'react';
import type { ActivityEvent } from '../types';
import { Panel, LinkButton, EVENT_DOT, EVENT_TEXT, clock, cx } from './ui';

export function ActivityFeed({ events, limit }: { events: ActivityEvent[]; limit?: number }) {
  const rows = limit ? events.slice(0, limit) : events;
  if (rows.length === 0) {
    return <p className="py-3 text-[12.5px] text-white/45">Nothing yet this session. Launches, scans and setting changes show up here.</p>;
  }
  return (
    <ul className="divide-y divide-white/[0.05]">
      {rows.map((e) => (
        <li key={e.id} className="grid grid-cols-[3rem_8px_minmax(0,1fr)_auto] items-center gap-3 py-2">
          <time className="font-mono text-[11.5px] tabular-nums text-white/40">{clock(e.ts)}</time>
          <span className={cx('h-1.5 w-1.5 rounded-full', EVENT_DOT[e.type])} />
          <span className="truncate text-[13px]">
            <span className={cx('mr-2 font-mono text-[11.5px] uppercase tracking-wider', EVENT_TEXT[e.type])}>{e.type}</span>
            <span className="text-white/85">{e.detail}</span>
          </span>
          <span className="font-mono text-[11.5px] text-white/40">{e.meta}</span>
        </li>
      ))}
    </ul>
  );
}

export function ActivityPanel({ events, limit = 8, onFullLog }: { events: ActivityEvent[]; limit?: number; onFullLog: () => void }) {
  return (
    <Panel as="section" className="bg-white/[0.04]">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-[15px] font-bold">Activity</h2>
        <LinkButton onClick={onFullLog}>full log →</LinkButton>
      </div>
      <ActivityFeed events={events} limit={limit} />
    </Panel>
  );
}
