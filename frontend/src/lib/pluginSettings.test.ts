import { describe, it, expect } from 'vitest';
import type { SettingField } from '../types';
import { effectiveValue, fieldsOf } from './pluginSettings';

const field = (o: Partial<SettingField> & Pick<SettingField, 'id' | 'kind'>): SettingField =>
  ({ label: o.id, scope: 'plugin', default: null, ...o }) as SettingField;

describe('effectiveValue', () => {
  it('prefers the saved value, then the default, then the kind\'s empty value', () => {
    const eos = field({ id: 'eos', kind: 'bool', default: true });
    expect(effectiveValue(eos, {})).toBe(true);
    expect(effectiveValue(eos, { eos: false })).toBe(false);
    expect(effectiveValue(field({ id: 'b', kind: 'bool' }), {})).toBe(false);
    expect(effectiveValue(field({ id: 't', kind: 'text' }), {})).toBe('');
  });

  it('a select without a default shows its first option', () => {
    const region = field({ id: 'region', kind: 'select', options: [{ id: 'eu', label: 'Europe' }, { id: 'us', label: 'US' }] });
    expect(effectiveValue(region, {})).toBe('eu');
    expect(effectiveValue(region, { region: 'us' })).toBe('us');
    expect(effectiveValue(field({ id: 'empty', kind: 'select', options: [] }), {})).toBe('');
  });

  it('a number without a default starts at its minimum, or zero', () => {
    expect(effectiveValue(field({ id: 'v', kind: 'number', min: 2 }), {})).toBe(2);
    expect(effectiveValue(field({ id: 'v', kind: 'number' }), {})).toBe(0);
    expect(effectiveValue(field({ id: 'v', kind: 'number', default: 7 }), {})).toBe(7);
  });
});

describe('fieldsOf', () => {
  const groups = [
    { id: 'a', label: 'A', fields: [field({ id: 'eos', kind: 'bool', default: true }), field({ id: 'note', kind: 'text' })] },
    { id: 'b', label: 'B', fields: [field({ id: 'volume', kind: 'number', default: 5 })] },
  ];

  it('lists the fields in the order the plugin sent them', () => {
    expect(fieldsOf(groups).map((f) => f.id)).toEqual(['eos', 'note', 'volume']);
  });
});
