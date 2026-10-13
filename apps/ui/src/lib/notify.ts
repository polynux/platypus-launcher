import React from 'react';
import { toast } from 'sonner';
import i18n from './i18n';

export const SUCCESS_DURATION = 3500;
export const INFO_DURATION = 5000;
export const ERROR_DURATION = 8000;

/** How long a repeat counts as the same toast (a count badge, not a new row). */
export const COALESCE_WINDOW_MS = 4000;

export type NotifyType = 'success' | 'info' | 'error';
/** The old single-notice kinds, kept for the onNotify props (least churn). */
export type LegacyKind = 'ok' | 'err' | 'busy';

export interface NotifyAction {
  label: string;
  onClick: (event: React.MouseEvent<HTMLButtonElement>) => void;
}

export interface NotifyOptions {
  description?: React.ReactNode;
  action?: NotifyAction;
  duration?: number;
  id?: string | number;
}

interface Coalesced {
  toastId: string | number;
  count: number;
  timer: ReturnType<typeof setTimeout>;
  type: NotifyType;
  message: string;
  options: NotifyOptions;
}

const coalesced = new Map<string, Coalesced>();

function countLabel(count: number): string {
  try {
    return i18n.t('common:toast.count', { count });
  } catch {
    return `×${count}`;
  }
}

/** The toast title: a live region per type (errors assertive, the rest polite)
 *  with an optional repeat-count badge. Sonner's own container is a single
 *  <section aria-live="polite">; the per-toast role is what makes errors
 *  assertive. */
function titleNode(message: string, type: NotifyType, count: number): React.ReactNode {
  const role = type === 'error' ? 'alert' : 'status';
  return React.createElement(
    'div',
    { role, className: 'platypus-toast-title' },
    React.createElement('span', { className: 'platypus-toast-message' }, message),
    count > 1
      ? React.createElement('span', { className: 'platypus-toast-count', 'aria-hidden': true }, countLabel(count))
      : null,
  );
}

function durationFor(type: NotifyType, options: NotifyOptions): number {
  if (options.duration !== undefined) return options.duration;
  // Errors that carry an action stay until dismissed; the rest auto-dismiss
  // by type (success short, info medium, error long).
  if (options.action && type === 'error') return Infinity;
  switch (type) {
    case 'success':
      return SUCCESS_DURATION;
    case 'info':
      return INFO_DURATION;
    case 'error':
      return ERROR_DURATION;
  }
}

function show(type: NotifyType, message: string, options: NotifyOptions = {}): string | number {
  const key = String(options.id ?? `${type}:${message}`);
  const existing = coalesced.get(key);
  if (existing && existing.type === type && existing.message === message) {
    existing.count += 1;
    const duration = durationFor(type, { ...existing.options, ...options });
    const id = toast[type](titleNode(message, type, existing.count), {
      id: existing.toastId,
      description: options.description ?? existing.options.description,
      action: options.action ?? existing.options.action,
      duration,
    });
    existing.toastId = id;
    existing.options = { ...existing.options, ...options };
    clearTimeout(existing.timer);
    existing.timer = setTimeout(() => {
      if (coalesced.get(key) === existing) coalesced.delete(key);
    }, COALESCE_WINDOW_MS);
    return id;
  }
  const duration = durationFor(type, options);
  const fn = type === 'success' ? toast.success : type === 'info' ? toast.info : toast.error;
  const id = fn(titleNode(message, type, 1), {
    id: options.id,
    description: options.description,
    action: options.action,
    duration,
  });
  const entry: Coalesced = {
    toastId: id,
    count: 1,
    timer: setTimeout(() => {
      if (coalesced.get(key) === entry) coalesced.delete(key);
    }, COALESCE_WINDOW_MS),
    type,
    message,
    options,
  };
  coalesced.set(key, entry);
  return id;
}

/** One API for every in-app notice: success (short), info (medium) and error
 *  (long, or sticky with an action). Repeats of the same message within a
 *  short window update the existing toast with a count instead of stacking. */
export const notify = {
  success(message: string, options?: NotifyOptions): string | number {
    return show('success', message, options);
  },
  info(message: string, options?: NotifyOptions): string | number {
    return show('info', message, options);
  },
  error(message: string, options?: NotifyOptions): string | number {
    return show('error', message, options);
  },
  dismiss(id?: string | number): void {
    if (id === undefined) toast.dismiss();
    else toast.dismiss(id);
  },
};

/** The old (msg, kind) signature as a thin wrapper over notify (least churn
 *  for the onNotify props): ok → success, err → error, busy → info. */
export function legacyNotify(msg: string, kind: LegacyKind = 'ok'): string | number {
  switch (kind) {
    case 'err':
      return notify.error(msg);
    case 'busy':
      return notify.info(msg);
    case 'ok':
    default:
      return notify.success(msg);
  }
}

/** Tests only: drop the coalescing window and dismiss every toast. */
export function __resetNotifyForTests(): void {
  for (const entry of coalesced.values()) clearTimeout(entry.timer);
  coalesced.clear();
  toast.dismiss();
}
