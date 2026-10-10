import React from 'react';
import { X } from 'lucide-react';
import type { Job } from '../types';
import { Button } from '@/components/ds/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ds/tooltip';
import { cx } from './ui';
import { formatBytes } from '../lib/games';

/** A job's progress: its phase, a bar, bytes and speed, and a cancel button. */
export function JobProgress({ job, onCancel }: { job: Job; onCancel: (id: string) => void }) {
  const pct = job.percent >= 0 ? Math.round(job.percent) : null;
  const what = job.state === 'queued' ? 'queued' : job.phase || job.kind;
  return (
    <div className="w-72 rounded-2xl bg-black/70 px-4 py-3 ring-1 ring-white/10">
      <div className="flex items-center justify-between gap-3 font-mono text-micro uppercase tracking-wider">
        <span className="truncate text-amber-200">{what}{pct != null && job.state === 'running' ? ` · ${pct}%` : ''}</span>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="none" onClick={() => onCancel(job.id)} aria-label={`Cancel ${job.kind}`} className="rounded p-0.5 hover:bg-transparent">
              <X className="h-3.5 w-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Cancel {job.kind}</TooltipContent>
        </Tooltip>
      </div>
      <div className="mt-2 h-1.5 rounded-full bg-white/15" role="progressbar" aria-valuenow={pct ?? undefined} aria-valuemin={0} aria-valuemax={100}>
        <div className={cx('h-full rounded-full bg-amber-300 transition-[width]', pct == null && 'animate-pulse')} style={{ width: `${pct ?? 100}%` }} />
      </div>
      {job.state === 'running' && job.total > 0 && (
        <div className="mt-1.5 font-mono text-hint tabular-nums text-white/55">
          {formatBytes(job.bytes) || '0 B'} of {formatBytes(job.total)}
          {job.speed > 0 && ` · ${formatBytes(job.speed)}/s`}
        </div>
      )}
    </div>
  );
}
