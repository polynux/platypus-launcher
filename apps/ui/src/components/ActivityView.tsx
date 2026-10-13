// i18n: converted
import { useTranslation } from 'react-i18next';
import React from 'react';
import type { ActivityEvent } from '../types';
import { ActivityFeed } from './ActivityFeed';
import { Panel } from './ui';

export function ActivityView({ events }: { events: ActivityEvent[] }) {
  const { t } = useTranslation(['activity', 'nav']);
  return (
    <div className="p-7">
      <header className="mb-6">
        <h1 className="text-[23px] font-extrabold tracking-tight">{t('nav:title.activity')}</h1>
        <p className="mt-0.5 text-[12.5px] text-white/70">
          {t('activity:view.subtitle')}
        </p>
      </header>
      <Panel as="section" className="bg-white/[0.04]">
        <ActivityFeed events={events} limit={50} />
      </Panel>
    </div>
  );
}
