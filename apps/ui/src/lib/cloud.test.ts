import { describe, it, expect } from 'vitest';
import { agoLabel, cloudNotice, describeCounts, fileSize, modeLabel } from './cloud';
import type { CloudEvent } from '../types';

describe('cloud helpers', () => {
  it('describes counts and leaves zeros out', () => {
    expect(describeCounts({ same: 3, download: 0, upload: 1, conflict: 0, unmapped: 0 })).toBe('3 in sync · 1 to upload');
    expect(describeCounts({ same: 0, download: 0, upload: 0, conflict: 0, unmapped: 0 })).toBe('');
    expect(describeCounts({ same: 0, download: 2, upload: 0, conflict: 1, unmapped: 4 })).toBe('2 to download · 1 in conflict · 4 not placed on this device');
  });

  it('prints an empty file as 0 B, small files by exact size, big ones rounded', () => {
    expect(fileSize(0)).toBe('0 B');
    expect(fileSize(3705)).toMatch(/^3\D?705 B$/);
    expect(fileSize(3_700_000)).toBe('3.7 MB');
  });

  it('says how long ago something was checked', () => {
    const now = 1_800_000_000_000;
    expect(agoLabel(now / 1000 - 5, now)).toBe('just now');
    expect(agoLabel(now / 1000 - 300, now)).toBe('5 min ago');
    expect(agoLabel(now / 1000 - 7200, now)).toBe('2 h ago');
    expect(agoLabel(0, now)).toBe('');
  });

  it('shows the plugin default when the game has no mode of its own', () => {
    expect(modeLabel({ game_mode: '', plugin_mode: 'ask' })).toBe('follows plugin · ask');
    expect(modeLabel({ game_mode: 'off', plugin_mode: 'ask' })).toBe('off');
  });
});

const ev = (o: Partial<CloudEvent>): CloudEvent => ({ game_id: 'p.echo.tiny', kind: 'status', ...o });

describe('cloudNotice', () => {
  it('toasts and logs a finished push', () => {
    const n = cloudNotice(ev({ kind: 'pushed', message: 'Saved 2 files to the cloud' }), 'Echo Tiny');
    expect(n?.toast).toEqual({ msg: 'Saved 2 files to the cloud · Echo Tiny', kind: 'ok' });
    expect(n?.activity?.detail).toContain('Saved 2 files');
  });

  it('shows a failure as an error and keeps the plugin\'s words', () => {
    const n = cloudNotice(ev({ kind: 'failed', message: "Echo Tiny's saves weren't uploaded: offline" }), 'Echo Tiny');
    expect(n?.toast?.kind).toBe('err');
    expect(n?.toast?.msg).toContain('offline');
  });

  it('is silent for progress and for a status with nothing to say', () => {
    expect(cloudNotice(ev({ kind: 'progress', phase: 'download', done: 1, total: 4 }), 't')).toBeNull();
    expect(cloudNotice(ev({ kind: 'status' }), 't')).toBeNull();
  });

  it('reports what a pre-launch pull downloaded', () => {
    const n = cloudNotice(ev({ kind: 'status', message: 'Downloaded 2 files from the cloud' }), 'Echo Tiny');
    expect(n?.toast?.msg).toBe('Downloaded 2 files from the cloud · Echo Tiny');
  });

  it('logs a prompt without a toast: the dialog is the notice', () => {
    const n = cloudNotice(ev({ kind: 'prompt', prompt: 'conflict' }), 'Echo Tiny');
    expect(n?.toast).toBeUndefined();
    expect(n?.activity?.meta).toBe('conflict');
  });

  it('shows a skipped sync as a note, not an error', () => {
    expect(cloudNotice(ev({ kind: 'skipped', message: 'Not synced: Steam is running' }), 't')?.toast?.kind).toBe('busy');
  });
});
