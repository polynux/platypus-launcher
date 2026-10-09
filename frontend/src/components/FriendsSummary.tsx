import React from 'react';
import { cx } from './ui';
import { describeSummary, initials, stateDot, STATUS_LABEL } from '../lib/social';
import type { SocialView } from '../types';

interface FriendsSummaryProps {
  views: SocialView[];
  /** Store names by plugin id. */
  labels: Record<string, string>;
  onOpen: () => void;
  /** Unread messages from friends across every store: a badge when above zero. */
  unread?: number;
}

/** The sidebar's friends block: the user's own avatar, name and status per
 *  store, and how many friends are online and in game. Click opens the modal. */
export function FriendsSummary({ views, labels, onOpen, unread = 0 }: FriendsSummaryProps) {
  if (views.length === 0) return null;
  const summary = describeSummary(views);
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label={`Friends: ${summary.text || 'off'}`}
      className="mb-1 flex w-full flex-col gap-2 rounded-xl bg-white/[0.03] px-3 py-2.5 text-left ring-1 ring-white/[0.06] transition hover:bg-white/[0.06]"
    >
      {views.map((v) => {
        const name = labels[v.plugin] ?? v.plugin;
        const self = v.self;
        return (
          <span key={v.plugin} className="flex items-center gap-2">
            <span className="relative grid h-7 w-7 flex-none place-items-center overflow-hidden rounded-full bg-white/10 font-mono text-[10px] font-bold text-white/70">
              {self?.avatarUrl ? <img src={self.avatarUrl} alt="" className="h-full w-full object-cover" /> : initials(self?.name ?? name)}
              <span className={cx('absolute bottom-0 right-0 h-2 w-2 rounded-full ring-2 ring-[#15110d]', stateDot(self?.state ?? 'offline'))} />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-[12px] font-semibold text-white/85">{self?.name ?? name}</span>
              <span className="block font-mono text-[10px] uppercase tracking-wider text-white/40">
                {!v.enabled ? 'friends off' : self ? `${name} · ${STATUS_LABEL[self.state] ?? self.state}` : name}
              </span>
            </span>
          </span>
        );
      })}
      <span className="flex items-center justify-between gap-2 font-mono text-[11px] tabular-nums text-white/55">
        <span>{!summary.text ? (views.some((v) => v.enabled) ? 'no friends online' : 'friends off') : summary.text}</span>
        {unread > 0 && (
          <span aria-label={`${unread} unread messages`} className="rounded-full bg-amber-300 px-1.5 font-bold text-[#130f0a]">
            {unread} new
          </span>
        )}
      </span>
    </button>
  );
}
