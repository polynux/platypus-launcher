// i18n: converted
import React from 'react';
import { useTranslation, Trans } from 'react-i18next';
import { Button } from './ds/button';
import { formatList } from '../lib/format';
import type { MigrationNotice as Notice } from '../types';

/** The one-time Hydra→Platypus migration notice (issue #14): a calm banner in
 *  the app shell, shaped like the library's source warnings (rounded-2xl,
 *  amber ring) — never a modal. Plain English: what happened (the data folder
 *  moved from the old Hydra name) and what to do (reinstall and re-approve
 *  the plugin builds that were set aside, listed by name). */
export function MigrationNotice({
  notice,
  onOpenPlugins,
  onDismiss,
}: {
  notice: Notice;
  onOpenPlugins: () => void;
  onDismiss: () => void;
}) {
  const { t } = useTranslation(['activity', 'nav', 'common']);
  return (
    <div
      role="status"
      aria-label={t('activity:migration.label')}
      className="mb-6 flex items-start justify-between gap-4 rounded-2xl bg-amber-300/[0.07] px-4 py-3 ring-1 ring-amber-300/25"
    >
      <div className="min-w-0 space-y-1.5 text-[12.5px] text-white/75">
        <p>
          <span className="font-bold text-amber-200">{t('activity:migration.welcome')}</span>{' '}
          {notice.copied ? t('activity:migration.copied') : t('activity:migration.moved')}
        </p>
        {notice.set_aside.length > 0 && (
          <p>
            <Trans
              i18nKey="activity:migration.setAside"
              values={{ names: formatList(notice.set_aside) }}
              components={{ mono: <span className="font-mono text-[12px] text-white/90" /> }}
            />
          </p>
        )}
        {notice.warnings.map((w) => (
          <p key={w} className="font-mono text-[12px] text-white/55">{w}</p>
        ))}
      </div>
      <div className="flex flex-none items-center gap-2">
        <Button variant="primary" size="sm" onClick={onOpenPlugins}>
          {t('nav:plugins')}
        </Button>
        <Button variant="ghost" size="sm" onClick={onDismiss}>
          {t('common:dismiss')}
        </Button>
      </div>
    </div>
  );
}
