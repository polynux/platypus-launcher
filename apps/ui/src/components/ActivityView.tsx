// i18n: converted
import { useTranslation } from 'react-i18next';
import React from 'react';
import type { ActivityEvent } from '../types';
import { ActivityFeed } from './ActivityFeed';
import { Panel, Section, ViewHeader } from './ui';

export function ActivityView({ events }: { events: ActivityEvent[] }) {
  const { t } = useTranslation(['activity', 'nav']);
  return (
    <div className="p-7">
      <ViewHeader
        title={t('nav:title.activity')}
        subtitle={<p className="mt-0.5 text-[12.5px] text-white/70">{t('activity:view.subtitle')}</p>}
      />
      <Panel as="section" className="bg-white/[0.04]">
        <Section>
          <ActivityFeed events={events} limit={50} />
        </Section>
      </Panel>
    </div>
  );
}
