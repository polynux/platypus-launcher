// i18n: converted
import { useTranslation } from 'react-i18next';
import type { GameSettings, SettingField, SettingGroup } from '../types';
import { Input } from '@/components/ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Switch } from '@/components/ds/switch';
import { MonoLabel } from './ui';

/** Radix forbids an empty value, so the empty choice ("game default") uses this one. */
const DEFAULT_CHOICE = '__default__';

/** The runner's per-game choices, drawn from the schema the core got from
 *  runner.options: one block per group in `ids`, in the schema's order. Labels
 *  and options come from the schema, already in the user's language. */
export function RunnerChoices({ groups, ids, draft, onChange }: {
  groups: SettingGroup[];
  ids: string[];
  draft: GameSettings;
  onChange: (key: string, value: boolean | string | number) => void;
}) {
  return (
    <>
      {groups.filter((g) => ids.includes(g.id)).map((g) => (
        <div key={g.id} className="space-y-3">
          <MonoLabel>{g.label}</MonoLabel>
          {g.description && <p className="text-[12px] leading-relaxed text-white/70">{g.description}</p>}
          <div className="divide-y divide-white/[0.07]">
            {g.fields.map((f) => (
              <Choice key={f.id} field={f} draft={draft} onChange={onChange} />
            ))}
          </div>
        </div>
      ))}
    </>
  );
}

function Choice({ field, draft, onChange }: { field: SettingField; draft: GameSettings; onChange: (key: string, value: boolean | string | number) => void }) {
  const { t } = useTranslation('gameSettings');
  const raw = (draft as unknown as Record<string, unknown>)[field.id];
  const value = raw ?? field.default ?? '';

  if (field.kind === 'bool') {
    return (
      <div className="flex items-start justify-between gap-4 py-2.5">
        <div className="min-w-0">
          <div className="text-[13px] text-white/90">{field.label}</div>
          {field.description && <div className="mt-0.5 text-[12px] leading-relaxed text-white/60">{field.description}</div>}
        </div>
        <Switch aria-label={field.label} checked={value === true} onCheckedChange={(v) => onChange(field.id, v)} />
      </div>
    );
  }

  if (field.kind === 'select') {
    const current = String(value) === '' ? DEFAULT_CHOICE : String(value);
    const known = (field.options ?? []).some((o) => (o.id === '' ? DEFAULT_CHOICE : o.id) === current);
    // runner_id names a runner that isn't installed any more: say so, as before.
    const missing = field.id === 'runner_id' && !known && current !== 'auto';
    return (
      <div className="space-y-2 py-2.5">
        <div className="text-[13px] text-white/90">{field.label}</div>
        {field.description && <div className="text-[12px] leading-relaxed text-white/60">{field.description}</div>}
        <Select value={current} onValueChange={(v) => onChange(field.id, v === DEFAULT_CHOICE ? '' : v)}>
          <SelectTrigger aria-label={field.label} className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {(field.options ?? []).map((o) => (
              <SelectItem key={o.id || DEFAULT_CHOICE} value={o.id || DEFAULT_CHOICE}>{o.label}</SelectItem>
            ))}
            {/* A saved value the schema doesn't list (a custom size, say) stays visible and kept. */}
            {!known && current !== DEFAULT_CHOICE && <SelectItem value={current}>{current}</SelectItem>}
          </SelectContent>
        </Select>
        {missing && <p role="status" className="font-mono text-hint text-orange-300">{t('runner.savedMissing')}</p>}
      </div>
    );
  }

  return (
    <div className="space-y-2 py-2.5">
      <div className="text-[13px] text-white/90">{field.label}</div>
      {field.description && <div className="text-[12px] leading-relaxed text-white/60">{field.description}</div>}
      <Input
        aria-label={field.label}
        type={field.kind === 'number' ? 'number' : 'text'}
        min={field.min}
        value={String(value)}
        placeholder={field.placeholder}
        onChange={(e) => onChange(field.id, field.kind === 'number' ? Number(e.target.value) || 0 : e.target.value)}
      />
    </div>
  );
}
