// i18n: converted
import { useTranslation } from 'react-i18next';
import type { ActivityEvent, Game } from '../types';
import { ActivityFeed } from './ActivityFeed';
import { Panel, Section, ViewHeader } from './ui';
import { useVariant } from '../lib/theme';
import { AtlasActivity } from './AtlasActivity';

export function ActivityView({ events, games = [], onOpenGame }: { events: ActivityEvent[]; games?: Game[]; onOpenGame?: (id: string) => void }) {
  const { t } = useTranslation(['activity', 'nav']);
  // One branch per view root (issue #156): the activity presentation, not the shell.
  const presentation = useVariant('activity.presentation');
  if (presentation === 'days') {
    return <AtlasActivity events={events} games={games} onOpenGame={onOpenGame} />;
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
