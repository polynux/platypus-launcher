import type { SettingField, SettingGroup, SettingValue } from '../types';

/** The value a field shows: the one the user saved, else the plugin's default,
 *  else the empty value of its kind (off, '', 0, or the first option of a select). */
export function effectiveValue(field: SettingField, saved: Record<string, SettingValue>): SettingValue {
  if (saved[field.id] !== undefined) return saved[field.id];
  if (field.default != null) return field.default;
  switch (field.kind) {
    case 'bool':
      return false;
    case 'text':
      return '';
    case 'number':
      return field.min ?? 0;
    case 'select':
      return field.options?.[0]?.id ?? '';
  }
}

/** The fields of a view, in the order the plugin sent them. */
export function fieldsOf(groups: SettingGroup[]): SettingField[] {
  return groups.flatMap((g) => g.fields);
}
