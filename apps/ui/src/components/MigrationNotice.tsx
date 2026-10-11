import React from 'react';
import { Button } from './ds/button';
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
  const moved = notice.copied
    ? 'Your data was copied from the old Hydra folder to Platypus. The old folder was kept where it was.'
    : 'Your data was moved from the old Hydra folder to Platypus.';
  return (
    <div
      role="status"
      aria-label="Data moved from Hydra to Platypus"
      className="mb-6 flex items-start justify-between gap-4 rounded-2xl bg-amber-300/[0.07] px-4 py-3 ring-1 ring-amber-300/25"
    >
      <div className="min-w-0 space-y-1.5 text-[12.5px] text-white/75">
        <p>
          <span className="font-bold text-amber-200">Welcome to Platypus. </span>
          {moved}
        </p>
        {notice.set_aside.length > 0 && (
          <p>
            These plugins were built for Hydra and were set aside — reinstall each one and approve it again in
            Plugins: <span className="font-mono text-[12px] text-white/90">{notice.set_aside.join(', ')}</span>.
            Their logins and settings were kept.
          </p>
        )}
        {notice.warnings.map((w) => (
          <p key={w} className="font-mono text-[12px] text-white/55">{w}</p>
        ))}
      </div>
      <div className="flex flex-none items-center gap-2">
        <Button variant="primary" size="sm" onClick={onOpenPlugins}>
          Plugins
        </Button>
        <Button variant="ghost" size="sm" onClick={onDismiss}>
          Dismiss
        </Button>
      </div>
    </div>
  );
}
