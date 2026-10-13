// i18n: converted
import React from 'react';
import { useTranslation } from 'react-i18next';
import { cx } from './ui';
import { describeSummary, initials, stateDot, statusLabel } from '../lib/social';
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
  const { t } = useTranslation('chat');
  if (views.length === 0) return null;
  const summary = describeSummary(views);
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label={t('chat:summary.label', { text: summary.text || t('chat:summary.off') })}
      className="mb-1 flex w-full flex-col gap-2 rounded-xl bg-white/[0.03] px-3 py-2.5 text-left ring-1 ring-white/[0.06] transition hover:bg-white/[0.06]"
    >
      {views.map((v) => {
        const name = labels[v.plugin] ?? v.plugin;
        const self = v.self;
        return (
          <span key={v.plugin} className="flex min-w-0 items-center gap-2">
            {/* The dot sits outside the clipped picture: inside it, overflow-hidden would cut it. */}
            <span className="relative h-6 w-6 flex-none">
              <span className="grid h-full w-full place-items-center overflow-hidden rounded-full bg-white/10 font-mono text-hint font-bold text-white/70">
                {self?.avatarUrl ? <img src={self.avatarUrl} alt="" className="h-full w-full object-cover" /> : initials(self?.name ?? name)}
              </span>
              <span className={cx('absolute bottom-0 right-0 h-2 w-2 rounded-full ring-2 ring-[#15110d]', stateDot(self?.state ?? 'offline'))} />
            </span>
            {/* The name gets the full width; the store and status sit on their own line below it. */}
            <span className="flex min-w-0 flex-1 flex-col">
              <span title={self?.name ?? name} className="min-w-0 truncate text-[12px] font-semibold text-white/85">{self?.name ?? name}</span>
              <span
                title={!v.enabled ? t('chat:summary.disabled') : self ? `${name} · ${statusLabel(self.state)}` : name}
                className="min-w-0 truncate font-mono text-micro uppercase tracking-wider text-white/55"
              >
                {!v.enabled ? t('chat:summary.disabled') : self ? `${name} · ${statusLabel(self.state)}` : name}
              </span>
            </span>
          </span>
        );
      })}
      <span className="flex items-center justify-between gap-2 font-mono text-hint tabular-nums text-white/55">
        <span>{!summary.text ? (views.some((v) => v.enabled) ? t('chat:summary.none') : t('chat:summary.disabled')) : summary.text}</span>
        {unread > 0 && (
          <span aria-label={t('chat:row.unread', { count: unread })} className="rounded-full bg-amber-300 px-1.5 font-bold text-[#130f0a]">
            {t('chat:summary.new', { count: unread })}
          </span>
        )}
      </span>
    </button>
  );
}
