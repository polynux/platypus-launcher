import type { InstallChoices, OptionGroup, OptionValue } from '../types';

// Install options (protocol 1.5): what a choice means for the dialog. Core does
// the checks that matter (owned DLC, known values); these keep the dialog's
// view of the same rules.

/** The value ids a group holds right now: a select's one id, a multi's list. */
export function chosenIds(group: OptionGroup, choices: InstallChoices): string[] {
  const c = choices[group.id];
  if (group.kind === 'select') return typeof c === 'string' ? [c] : [];
  if (group.kind === 'multi') return Array.isArray(c) ? c : [];
  return [];
}

/** Whether the user can pick a value now: unowned DLC can't be installed, but
 *  one already installed stays on the list. */
export const canPick = (group: OptionGroup, value: OptionValue): boolean =>
  !group.dlc || value.owned || !!value.installed;

/** Bytes the chosen values add to the install. */
export function addedBytes(groups: OptionGroup[], choices: InstallChoices): number {
  let total = 0;
  for (const g of groups) {
    const ids = chosenIds(g, choices);
    for (const v of g.values ?? []) if (ids.includes(v.id)) total += v.sizeBytes ?? 0;
  }
  return total;
}

/** The choices with one multi value switched on or off. */
export function toggleValue(group: OptionGroup, choices: InstallChoices, valueId: string, on: boolean): InstallChoices {
  const current = chosenIds(group, choices);
  const next = on ? [...current.filter((id) => id !== valueId), valueId] : current.filter((id) => id !== valueId);
  return { ...choices, [group.id]: next };
}

/** The choices with a select's value replaced. */
export const pickValue = (choices: InstallChoices, group: OptionGroup, valueId: string): InstallChoices =>
  ({ ...choices, [group.id]: valueId });

/** The choices with a bool group set. */
export const setFlag = (choices: InstallChoices, group: OptionGroup, on: boolean): InstallChoices =>
  ({ ...choices, [group.id]: on });
