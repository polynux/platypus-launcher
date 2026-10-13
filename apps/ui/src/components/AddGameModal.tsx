// i18n: converted
import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { ChevronDown } from 'lucide-react';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';

/** Suggest a title from the executable's file name: the base without its
 *  extension, underscores turned into spaces. */
export function suggestTitle(exe: string): string {
  const base = exe.split('/').pop() ?? exe;
  const noExt = base.includes('.') ? base.slice(0, base.lastIndexOf('.')) : base;
  return noExt.replace(/[_]+/g, ' ').trim();
}

/** Advanced sideload options, stored in the new game's GameSettings (and its release year). */
export interface AddGameOptions {
  workingDir: string;
  winePrefix: string;
  /** How the game runs: "auto" (detected) by default, "native" or "wine" to override detection. */
  runMode?: string;
  /** Release year, when the user set one (validated by core). */
  releaseYear?: number;
}

/** Whether a release-year input holds a usable year: empty clears it (unknown).
 *  The `code` lets callers render the failure from their own catalogs. */
export function parseReleaseYear(raw: string): { year?: number; error?: string; code?: 'format' | 'range' } {
  const s = raw.trim();
  if (!s) return {};
  if (!/^\d{4}$/.test(s)) return { error: 'The release year is four digits, e.g. 2011.', code: 'format' };
  const year = Number(s);
  const max = new Date().getFullYear() + 1;
  if (year < 1970 || year > max) return { error: `The release year is between 1970 and ${max}.`, code: 'range' };
  return { year };
}

/** Adding is all-or-nothing: either it resolves or it throws, and the dialog
 *  stays open until it settles. */
export type AddGameResult = Promise<void> | void;

export function AddGameModal({ onClose, onAdd }: { onClose: () => void; onAdd: (title: string, exe: string, opts: AddGameOptions) => AddGameResult }) {
  const { t } = useTranslation(['library', 'common']);
  const [title, setTitle] = useState('');
  const [exe, setExe] = useState('');
  const [workingDir, setWorkingDir] = useState('');
  const [winePrefix, setWinePrefix] = useState('');
  const [runMode, setRunMode] = useState('auto');
  const [releaseYear, setReleaseYear] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [workError, setWorkError] = useState<string | null>(null);
  const [prefixError, setPrefixError] = useState<string | null>(null);
  const [yearError, setYearError] = useState<string | null>(null);
  // Adding is all-or-nothing: the dialog stays open until onAdd resolves, and
  // a rejection shows where it belongs instead of stranding a half-added game.
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const valid = title.trim() && exe.trim();
  const yearParsed = parseReleaseYear(releaseYear);
  // What the executable looks like by content (issue #47): the default of the
  // "Run with" choice. Null until it answers (or when there is no backend).
  const [detected, setDetected] = useState<{ native: boolean; kind: string } | null>(null);
  React.useEffect(() => {
    const path = exe.trim();
    if (!path) {
      setDetected(null);
      return;
    }
    let live = true;
    Effect.runPromise(LauncherApi.detectGameExecutable(path)).then(
      (d) => { if (live) setDetected(d); },
      () => { if (live) setDetected(null); },
    );
    return () => { live = false; };
  }, [exe]);
  const effectiveNative = runMode === 'native' ? true : runMode === 'wine' ? false : (detected?.native ?? false);
  const advancedSet = (workingDir.trim() ? 1 : 0) + (!effectiveNative && winePrefix.trim() ? 1 : 0) + (runMode !== 'auto' ? 1 : 0) + (yearParsed.year ? 1 : 0);

  const yearErrorText = (code: 'format' | 'range'): string =>
    code === 'format'
      ? t('library:addDialog.yearFormatError')
      : t('library:addDialog.yearRangeError', { max: new Date().getFullYear() + 1 });

  const browseExe = () => {
    Effect.runPromise(LauncherApi.pickGameExecutable(exe.trim()))
      .then((picked) => {
        if (!picked) return; // cancelled: leave the field alone
        setExe(picked);
        if (!title.trim()) {
          const suggestion = suggestTitle(picked);
          if (suggestion) setTitle(suggestion);
        }
      })
      .catch(() => {}); // no picker here: the text field still works
  };

  const browseWork = () => {
    Effect.runPromise(LauncherApi.pickFolder(t('library:addDialog.pickWorkDir'), workingDir.trim() || exe.trim()))
      .then((picked) => {
        if (!picked) return;
        setWorkingDir(picked);
        setWorkError(null);
      })
      .catch(() => {});
  };

  const browsePrefix = () => {
    Effect.runPromise(LauncherApi.pickFolder(t('library:addDialog.pickPrefix'), winePrefix.trim() || workingDir.trim() || exe.trim()))
      .then((picked) => {
        if (!picked) return;
        setWinePrefix(picked);
        setPrefixError(null);
      })
      .catch(() => {});
  };

  const submit = () => {
    const work = workingDir.trim();
    if (work && !work.startsWith('/')) {
      setWorkError(t('library:addDialog.workError'));
      return;
    }
    const prefix = winePrefix.trim();
    if (prefix && !prefix.startsWith('/') && !prefix.startsWith('~/')) {
      setPrefixError(t('library:addDialog.prefixError'));
      return;
    }
    const parsed = parseReleaseYear(releaseYear);
    if (parsed.error) {
      setYearError(yearErrorText(parsed.code ?? 'format'));
      setAdvancedOpen(true); // a field error must be visible, not folded shut
      return;
    }
    if (!valid || saving) return;
    let result: Promise<void> | void;
    try {
      result = onAdd(title.trim(), exe.trim(), { workingDir: work, winePrefix: effectiveNative ? '' : prefix, ...(runMode !== 'auto' ? { runMode } : {}), ...(parsed.year ? { releaseYear: parsed.year } : {}) });
    } catch (e) {
      fail(e);
      return;
    }
    setSaving(true);
    setSaveError(null);
    Promise.resolve(result).then(() => onClose(), fail);
  };

  const fail = (e: unknown) => {
    const msg = e instanceof Error ? e.message : String(e);
    if (/working dir/i.test(msg)) {
      setWorkError(msg);
      setAdvancedOpen(true); // a field error must be visible, not folded shut
    } else if (/wine prefix/i.test(msg)) {
      setPrefixError(msg);
      setAdvancedOpen(true);
    } else if (/release year/i.test(msg)) {
      setYearError(msg);
      setAdvancedOpen(true);
    } else {
      setSaveError(msg);
    }
    setSaving(false);
  };

  return (
    <Modal label={t('library:addDialog.title')} onClose={onClose} size="md">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        <h2 className="text-[20px] font-extrabold tracking-tight">{t('library:addDialog.title')}</h2>
        <p className="mt-1 text-[12.5px] text-white/70">{t('library:addDialog.description')}</p>
        <label className="mt-5 block">
          <MonoLabel className="mb-1.5">{t('library:addDialog.titleSection')}</MonoLabel>
          <Input autoFocus value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t('library:addDialog.titlePlaceholder')} />
        </label>
        <div className="mt-4">
          <MonoLabel className="mb-1.5">{t('library:addDialog.exeSection')}</MonoLabel>
          <div className="flex items-center gap-2">
            <Input
              value={exe}
              onChange={(e) => setExe(e.target.value)}
              placeholder={t('library:addDialog.exePlaceholder')}
              aria-label={t('library:addDialog.exeAria')}
              className="min-w-0 flex-1"
            />
            <Button type="button" variant="neutral" onClick={browseExe} className="flex-none">
              {t('library:addDialog.browse')}
            </Button>
          </div>
        </div>
        <div className="mt-4 border-t border-white/[0.07] pt-3">
          <button
            type="button"
            aria-expanded={advancedOpen}
            onClick={() => setAdvancedOpen((o) => !o)}
            className="flex items-center gap-1.5 font-mono text-[12px] text-white/60 transition hover:text-white/85"
          >
            <ChevronDown className={cx('size-3.5 transition-transform', advancedOpen ? '' : '-rotate-90')} aria-hidden />
            {t('library:addDialog.advanced')}
            {advancedSet > 0 && <span className="tabular-nums text-amber-200/90">{t('library:addDialog.advancedSet', { count: advancedSet })}</span>}
          </button>
          <div hidden={!advancedOpen} className="mt-4 space-y-5">
            <div>
              <MonoLabel className="mb-1.5">{t('library:addDialog.runWith')}</MonoLabel>
              <select
                aria-label={t('library:addDialog.runWith')}
                value={runMode}
                onChange={(e) => setRunMode(e.target.value)}
                className="w-full rounded-lg bg-white/[0.06] px-3 py-2 text-[13px] text-white/90 outline-none ring-1 ring-white/10 focus:ring-amber-300/40"
              >
                <option value="auto">{detected == null ? t('library:addDialog.runAutoUnknown') : detected.native ? t('library:addDialog.runAutoNative') : t('library:addDialog.runAutoWine')}</option>
                <option value="native">{t('library:addDialog.runNative')}</option>
                <option value="wine">{t('library:addDialog.runWine')}</option>
              </select>
              <p className="mt-1.5 text-[12px] leading-relaxed text-white/70">{t('library:addDialog.runHelp')}</p>
            </div>
            <div>
              <MonoLabel className="mb-1.5">{t('library:addDialog.workDir')} <span className="normal-case tracking-normal text-white/55">· {t('library:addDialog.optional')}</span></MonoLabel>
              <div className="flex items-center gap-2">
                <Input
                  value={workingDir}
                  onChange={(e) => { setWorkingDir(e.target.value); setWorkError(null); }}
                  placeholder={t('library:addDialog.workPlaceholder')}
                  aria-label={t('library:addDialog.workAria')}
                  className="min-w-0 flex-1"
                />
                <Button type="button" variant="neutral" onClick={browseWork} className="flex-none">
                  {t('library:addDialog.browse')}
                </Button>
              </div>
              <p className="mt-1.5 text-[12px] leading-relaxed text-white/70">{t('library:addDialog.workHelp')}</p>
              {workError && <p role="alert" className="mt-1 font-mono text-hint text-orange-300">{workError}</p>}
            </div>
            {!effectiveNative && (
            <div>
              <MonoLabel className="mb-1.5">{t('library:addDialog.prefix')} <span className="normal-case tracking-normal text-white/55">· {t('library:addDialog.optional')}</span></MonoLabel>
              <div className="flex items-center gap-2">
                <Input
                  value={winePrefix}
                  onChange={(e) => { setWinePrefix(e.target.value); setPrefixError(null); }}
                  placeholder={t('library:addDialog.prefixPlaceholder')}
                  aria-label={t('library:addDialog.prefixAria')}
                  className="min-w-0 flex-1"
                />
                <Button type="button" variant="neutral" onClick={browsePrefix} className="flex-none">
                  {t('library:addDialog.browse')}
                </Button>
              </div>
              <p className="mt-1.5 text-[12px] leading-relaxed text-white/70">{t('library:addDialog.prefixHelp')}</p>
              {prefixError && <p role="alert" className="mt-1 font-mono text-hint text-orange-300">{prefixError}</p>}
            </div>
            )}
            <div>
              <MonoLabel className="mb-1.5">{t('library:addDialog.year')} <span className="normal-case tracking-normal text-white/55">· {t('library:addDialog.optional')}</span></MonoLabel>
              <Input
                value={releaseYear}
                onChange={(e) => { setReleaseYear(e.target.value); setYearError(null); }}
                placeholder={t('library:addDialog.yearPlaceholder')}
                aria-label={t('library:addDialog.yearAria')}
                inputMode="numeric"
                className="w-32"
              />
              <p className="mt-1.5 text-[12px] leading-relaxed text-white/70">{t('library:addDialog.yearHelp')}</p>
              {yearError && <p role="alert" className="mt-1 font-mono text-hint text-orange-300">{yearError}</p>}
            </div>
          </div>
        </div>
        <div className="mt-6 flex justify-end gap-2.5">
          <Button variant="neutral" onClick={onClose}>{t('common:cancel')}</Button>
          <Button type="submit" variant="primary" disabled={!valid || saving}>{t('library:addDialog.submit')}</Button>
        </div>
        {saveError && <p role="alert" className="mt-3 font-mono text-hint text-orange-300">{saveError}</p>}
      </form>
    </Modal>
  );
}
