// i18n: converted
import { useTranslation } from 'react-i18next';
import React from 'react';
import { Loader2, RefreshCw } from 'lucide-react';
import type { ProtonVersion } from '../types';
import { EmptyState, ListRow, Section, ViewHeader } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';

interface SystemViewProps {
  runners: ProtonVersion[];
  onRefresh: () => void;
  scanning: boolean;
}

export function SystemView({ runners, onRefresh, scanning }: SystemViewProps) {
  const { t } = useTranslation(['nav', 'runners']);
  return (
    <div className="p-7">
      <ViewHeader
        title={t('nav:title.runners')}
        subtitle={<><p className="mt-0.5 text-[12.5px] text-white/70">{t('runners:intro')}</p>
        <p className="mt-1.5 max-w-xl text-[12px] leading-relaxed text-white/70">
          {t('runners:about')}
        </p></>}
        actions={<Button variant="neutral" onClick={onRefresh} disabled={scanning}>
          {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {scanning ? t('runners:scanning') : t('runners:rescan')}
        </Button>}
      />

      {runners.length === 0 ? (
        <EmptyState label={t('runners:emptyTitle')}>
          {t('runners:emptyBody')}
        </EmptyState>
      ) : (
        <Section className="space-y-2">
          {runners.map((r) => (
            <ListRow key={r.id} kind="runner">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-[13px] font-bold">{r.name}</span>
                  {r.is_custom && <Badge variant="gold">{t('runners:badge.custom')}</Badge>}
                  {r.is_system && <Badge variant="neutral">{t('runners:badge.system')}</Badge>}
                </div>
                <div className="truncate font-mono text-hint text-white/55 select-text">{r.path}</div>
              </div>
              <Badge variant="ok" className="flex-none gap-1.5">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" /> {t('runners:badge.found')}
              </Badge>
            </ListRow>
          ))}
        </Section>
      )}
    </div>
  );
}
