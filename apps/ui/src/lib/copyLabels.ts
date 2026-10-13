// i18n: converted
import type { TFunction } from 'i18next';
import type { CopyDiff, GameCopy } from '../types';
import { formatList } from './format';

// User-facing labels for the family-copy picker and comparison (gameOps).
// The data helpers (which copies are pickable, the comparison rows) stay in
// lib/family, owned by another area; only the displayed words live here, so
// the copy dialogs render fully in the UI language.

/** What keeps a copy from being picked or played, for its label: '' when it is ready. */
export function copyStatusLabel(t: TFunction, copy: Pick<GameCopy, 'available' | 'in_use'>): string {
  if (!copy.available) return t('gameOps:copies.unavailable');
  return copy.in_use ? t('gameOps:copies.inUse') : '';
}

/** The add-ons a copy has, as the picker names them. Names come from the store. */
export function copyDlcLabel(t: TFunction, copy: Pick<GameCopy, 'dlc'>): string {
  const names = (copy.dlc ?? []).map((d) => d.name);
  // Joined with commas as before (not formatList): the picker line keeps its
  // compact shape in every language.
  return names.length > 0 ? names.join(', ') : t('gameOps:copies.noAddons');
}

/** One line about a copy against the baseline copy, for the picker: what it
 *  has more or fewer of, soundtracks apart, or that it matches the baseline. */
export function copyDiffSummaryText(t: TFunction, copy: string, baseline: string, diff: CopyDiff): string {
  const parts = [
    dlcPhrase(t, diff.only_here, diff.only_baseline),
    soundtrackPhrase(t, diff.soundtracks_only_here, diff.soundtracks_only_baseline),
  ].filter(Boolean);
  if (parts.length === 0) return t('gameOps:copies.sameAddons', { copy, baseline });
  // At most two parts; joined the locale's way.
  return t('gameOps:copies.diffSummary', { copy, baseline, parts: formatList(parts) });
}

/** "3 more", "2 fewer", "1 more and 2 fewer", or '' when neither side has any. */
function dlcPhrase(t: TFunction, more: number, fewer: number): string {
  if (more > 0 && fewer > 0) return t('gameOps:copies.dlcBoth', { more, fewer });
  if (more > 0) return t('gameOps:copies.dlcMore', { more });
  if (fewer > 0) return t('gameOps:copies.dlcFewer', { fewer });
  return '';
}

function soundtrackPhrase(t: TFunction, more: number, fewer: number): string {
  const total = more + fewer;
  if (total === 0) return '';
  if (more > 0 && fewer > 0) {
    const noun = total === 1 ? t('gameOps:copies.soundtrack_one') : t('gameOps:copies.soundtrack_other');
    return t('gameOps:copies.soundtrackBoth', { more, fewer, noun });
  }
  if (more > 0) {
    if (more === 1) return t('gameOps:copies.soundtrackMore_one', { more });
    return t('gameOps:copies.soundtrackMore_other', { more });
  }
  if (fewer === 1) return t('gameOps:copies.soundtrackFewer_one', { fewer });
  return t('gameOps:copies.soundtrackFewer_other', { fewer });
}
