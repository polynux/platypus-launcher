// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { LauncherSettings, StorePlugin, ThemeInfo } from '../types';
import { NumberField } from '@/components/ds/number-field';
import { Input } from '@/components/ds/input';
import { Switch } from '@/components/ds/switch';
import { Button } from '@/components/ds/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { PSEUDO_LOCALE, applyLanguage, loadSessionInfo, pseudoAvailable } from '../lib/i18n';
import { CloudSavesDefaults } from './CloudSavesSetting';
import { GameFoldersSettings } from './GameFoldersSettings';
import { BackupsSetting } from './BackupsSetting';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';
import { DEFAULT_THEME_ID, loadTheme } from '../lib/theme';

/** A settings section: a hairline and air, not a card (no card inside the page). */
const SECTION = 'max-w-2xl border-t border-white/[0.07] py-6';

interface LauncherSettingsViewProps {
  onNotify: (msg: string, kind?: 'ok' | 'err') => void;
  /** The plugins list: the cloud-saves defaults come from the stores that offer them. */
  plugins: StorePlugin[];
}

export function LauncherSettingsView({ onNotify, plugins }: LauncherSettingsViewProps) {
  const { t } = useTranslation('settings');
  const [saved, setSaved] = useState<LauncherSettings | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [cleanupDays, setCleanupDays] = useState<number | ''>('');
  const [cleanupError, setCleanupError] = useState<string | null>(null);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => { setSaved(s); setCleanupDays(s.cancel_cleanup_days); })
      .catch((e) => setError(coreErrMsg(e)));
  }, []);

  const saveCleanupDays = (days: number) => {
    setBusy(true);
    setCleanupError(null);
    Effect.runPromise(LauncherApi.setCancelCleanupDays(days))
      .then((s) => {
        setSaved(s);
        setCleanupDays(s.cancel_cleanup_days);
        onNotify(t('cleanup.saved'));
      })
      .catch((e) => setCleanupError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };

  const cleanupDirty = saved != null && typeof cleanupDays === 'number' && cleanupDays !== saved.cancel_cleanup_days;

  return (
    <div className="p-7">
      <header className="mb-2">
        <h1 className="text-[23px] font-extrabold tracking-tight">{t('settings:title')}</h1>
        <p className="mt-0.5 max-w-[75ch] text-[12.5px] text-white/70">{t('subtitle')}</p>
      </header>
      {error && <ErrorNotice className="mt-2" message={error} fallback={t('loadError')} />}

      <LanguageSetting onNotify={onNotify} />

      <ThemeSetting onNotify={onNotify} />

      <GameFoldersSettings onNotify={onNotify} />

      <PrefixesFolderSetting onNotify={onNotify} />

      <section className={SECTION}>
        <h2 className="text-[14px] font-bold">{t('cleanup.heading')}</h2>
        <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
{t('cleanup.description')}
        </p>
        <form
          className="flex flex-wrap items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (cleanupDirty && typeof cleanupDays === 'number') saveCleanupDays(cleanupDays);
          }}
        >
          <label className="flex items-center gap-2">
            <span className="text-[12.5px] text-white/70">{t('cleanup.keepFor')}</span>
            <NumberField
              min={0}
              max={365}
              step={1}
              hideSteppers
              value={cleanupDays === '' ? undefined : cleanupDays}
              onChange={(v) => { setCleanupDays(v ?? ''); setCleanupError(null); }}
              disabled={!saved}
              aria-label={t('cleanup.daysLabel')}
              className="w-24"
            />
            <span className="text-[12.5px] text-white/70">{t('cleanup.days')}</span>
          </label>
          <Button type="submit" variant="primary" disabled={!cleanupDirty || busy}>{t('common:save')}</Button>
        </form>
        <div className="mt-2.5 flex min-h-4 items-center gap-4">
          {cleanupError ? <ErrorNotice message={cleanupError} fallback={t('cleanupError')} /> : <span />}
        </div>
      </section>

      <RatingsSetting onNotify={onNotify} />

      <ArtworkSetting onNotify={onNotify} />

      <GameInfoSetting onNotify={onNotify} />

      <ChatNotificationsSetting onNotify={onNotify} />

      <TraySetting onNotify={onNotify} />

      <BackupsSetting onNotify={onNotify} />

      <CloudSavesDefaults plugins={plugins} />
    </div>
  );
}

/** The UI language (issue #39): System follows the desktop language, or an
 *  explicit language. Saving applies it live, without a restart. */
export function LanguageSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [value, setValue] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [dev, setDev] = useState(false);
  useEffect(() => {
    let live = true;
    try {
      Effect.runPromise(LauncherApi.getLauncherSettings())
        .then((s) => live && setValue(s?.language ?? ''))
        .catch(() => live && setValue(''));
    } catch {
      setValue('');
    }
    void loadSessionInfo().then(() => live && setDev(pseudoAvailable()));
    return () => { live = false; };
  }, []);
  const change = (next: string) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setLanguage(next))
      .then((s) => {
        const saved = s.language ?? next;
        setValue(saved);
        return applyLanguage(saved);
      })
      .then(() => onNotify(t('language.saved')))
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  // Language names stay in their own language (endonyms): translating them
  // would hide what the option means.
  // Radix Select treats "" as no value (an empty trigger), so System is
  // 'system' inside the picker and "" in the setting.
  const SYSTEM = 'system';
  const options = [
    { value: SYSTEM, label: t('language.system') },
    { value: 'en', label: 'English' },
    { value: 'fr', label: 'Français' },
  ];
  if (dev) options.push({ value: PSEUDO_LOCALE, label: `${PSEUDO_LOCALE} (pseudo)` });
  return (
    <section className={SECTION}>
      <h2 className="text-[14px] font-bold">{t('language.heading')}</h2>
      <p className="mb-4 mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">{t('language.description')}</p>
      <Select value={value === null ? undefined : value || SYSTEM} onValueChange={(v) => change(v === SYSTEM ? '' : v)} disabled={value === null || busy}>
        <SelectTrigger aria-label={t('language.label')} className="w-64">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      {dev && <p className="mt-2 font-mono text-hint text-white/55">{t('language.pseudoNote')}</p>}
    </section>
  );
}

/** The launcher's look (issue #93): a built-in theme restyles the whole app
 *  as soon as it is picked. A theme that needs a newer Platypus, or whose file
 *  is broken, stays in the list with the reason, and can't be picked. */
export function ThemeSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [themes, setThemes] = useState<ThemeInfo[] | null>(null);
  const [value, setValue] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.listThemes())
      .then((list) => live && setThemes(list ?? []))
      .catch(() => live && setThemes([]));
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => live && setValue(s.theme || DEFAULT_THEME_ID))
      .catch(() => live && setValue(DEFAULT_THEME_ID));
    return () => { live = false; };
  }, []);
  const change = (id: string) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setTheme(id))
      .then((s) => {
        setValue(s.theme || DEFAULT_THEME_ID);
        return loadTheme();
      })
      .then(() => onNotify(t('theme.saved')))
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  // A pick the list no longer offers shows as the default, not as an empty trigger.
  const shown = themes?.some((th) => th.id === value) ? value ?? undefined : DEFAULT_THEME_ID;
  const warnings = (themes ?? []).flatMap((th) => th.warnings.map((w) => ({ key: `${th.id}:${w}`, text: `${th.name}: ${w}` })));
  return (
    <section className={SECTION}>
      <h2 className="text-[14px] font-bold">{t('theme.heading')}</h2>
      <p className="mb-4 mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">{t('theme.description')}</p>
      <Select value={shown} onValueChange={change} disabled={themes === null || value === null || busy}>
        <SelectTrigger aria-label={t('theme.label')} className="w-64">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {(themes ?? []).map((th) => (
            <SelectItem key={th.id} value={th.id} disabled={th.status !== 'ok'}>
              {th.status === 'ok' ? th.name : `${th.name} (${t(`theme.status.${th.status}`, { defaultValue: th.status })})`}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {warnings.length > 0 && (
        <ul aria-label={t('theme.warningsLabel')} className="mt-3 space-y-1 font-mono text-hint leading-relaxed text-white/55">
          {warnings.map((w) => <li key={w.key}>{w.text}</li>)}
        </ul>
      )}
    </section>
  );
}

/** "Fetch game info online" and the credits for the data Platypus shows. Off, Platypus
 *  keeps what it fetched before and asks the network for nothing new. */
export function RatingsSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.ratings_online ?? true))
      .catch(() => setOn(true));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setRatingsOnline(next))
      .then((s) => {
        setOn(s.ratings_online ?? next);
        onNotify(next ? t('ratings.on') : t('ratings.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('ratings.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
{t('ratings.description')}
          </p>
        </div>
        <Switch aria-label={t('ratings.label')} checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
      <div className="mt-4 space-y-1.5 border-t border-white/[0.06] pt-3 font-mono text-hint leading-relaxed text-white/55">
        <p>{t('ratings.creditCompat')}</p>
        <p>{t('ratings.creditWiki')}</p>
        <p>{t('ratings.creditAnticheat')}</p>
      </div>
    </section>
  );
}

/** "Find artwork online" for sideloaded games (issue #36): Steam's public
 *  store search after an add, and the on-demand finder in the art panel. Off,
 *  nothing goes out: the placeholder art stays and no search runs. */
export function ArtworkSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.artwork_online ?? true))
      .catch(() => setOn(true));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setArtworkOnline(next))
      .then((s) => {
        setOn(s.artwork_online ?? next);
        onNotify(next ? t('artwork.on') : t('artwork.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('artwork.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
{t('artwork.description')}
          </p>
        </div>
        <Switch aria-label={t('artwork.label')} checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
      <div className="mt-4 space-y-1.5 border-t border-white/[0.06] pt-3 font-mono text-hint leading-relaxed text-white/55">
        <p>{t('artwork.creditArt')}</p>
      </div>
    </section>
  );
}
/** "Find game info online" for sideloaded games (issue #38): the on-demand
 *  description and details lookup. Off, nothing goes out: the game page keeps
 *  what it cached before and no new fetch runs. */
export function GameInfoSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.game_info_online ?? true))
      .catch(() => setOn(true));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setGameInfoOnline(next))
      .then((s) => {
        setOn(s.game_info_online ?? next);
        onNotify(next ? t('gameInfo.on') : t('gameInfo.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('gameInfo.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
{t('gameInfo.description')}
          </p>
        </div>
        <Switch aria-label={t('gameInfo.label')} checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
      <div className="mt-4 space-y-1.5 border-t border-white/[0.06] pt-3 font-mono text-hint leading-relaxed text-white/55">
        <p>{t('gameInfo.creditInfo')}</p>
      </div>
    </section>
  );
}
/** "Desktop notifications for chat" (issue #12): a system notification for a
 *  new chat message when the window isn't focused. Off, only the in-app toasts
 *  stay. On by default, like the other switches. */
export function ChatNotificationsSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.chat_desktop_notify ?? true))
      .catch(() => setOn(true));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setChatDesktopNotify(next))
      .then((s) => {
        setOn(s.chat_desktop_notify ?? next);
        onNotify(next ? t('chat.on') : t('chat.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('chat.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
{t('chat.description')}
          </p>
        </div>
        <Switch aria-label={t('chat.label')} checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
    </section>
  );
}
/** "Minimize to tray while a game runs" (issue #49): while a game runs the
 *  window hides to a tray icon instead of staying open, and comes back when
 *  the game exits. Off by default, unlike the online switches. */
export function TraySetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.minimize_to_tray ?? false))
      .catch(() => setOn(false));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setMinimizeToTray(next))
      .then((s) => {
        setOn(s.minimize_to_tray ?? next);
        onNotify(next ? t('tray.on') : t('tray.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('tray.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
{t('tray.description')}
          </p>
        </div>
        <Switch aria-label={t('tray.label')} checked={on ?? false} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
    </section>
  );
}
/** Where new Wine prefixes are created (issue #37): one folder per game in
 *  this folder, unless the game has its own prefix. Changing it never moves
 *  or deletes existing prefixes; empty resets it to the default. */
export function PrefixesFolderSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [saved, setSaved] = useState<LauncherSettings | null>(null);
  const [folder, setFolder] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => { setSaved(s); setFolder(s.prefixes_folder ?? ''); })
      .catch((e) => setError(coreErrMsg(e)));
  }, []);

  const dirty = saved != null && folder.trim() !== (saved.prefixes_folder ?? '');
  const custom = (saved?.prefixes_folder ?? '') !== '';

  const save = (path: string) => {
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setPrefixesFolder(path))
      .then((s) => {
        setSaved(s);
        setFolder(s.prefixes_folder ?? '');
        onNotify(path ? t('prefixes.saved') : t('prefixes.resetDone'));
      })
      .catch((e) => setError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };

  const browse = () => {
    Effect.runPromise(LauncherApi.pickFolder(t('prefixes.browseTitle'), folder.trim() || saved?.prefixes_folder || saved?.default_prefixes_folder || ''))
      .then((picked) => { if (picked) { setFolder(picked); setError(null); } })
      .catch((e) => setError(coreErrMsg(e)));
  };

  return (
    <section className={SECTION}>
      <h2 className="text-[14px] font-bold">{t('prefixes.heading')}</h2>
      <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
{t('prefixes.description')}
      </p>
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (dirty) save(folder.trim());
        }}
      >
        <Input
          aria-label={t('prefixes.label')}
          value={folder}
          onChange={(e) => { setFolder(e.target.value); setError(null); }}
          placeholder={saved?.default_prefixes_folder || t('prefixes.placeholder')}
          spellCheck={false}
          disabled={!saved}
          className="min-w-0 max-w-xl flex-1"
        />
        <Button type="button" variant="neutral" onClick={browse} disabled={!saved || busy}>{t('prefixes.browse')}</Button>
        <Button type="submit" variant="primary" disabled={!dirty || busy}>{t('prefixes.save')}</Button>
        {custom && (
          <Button type="button" variant="ghost" onClick={() => save('')} disabled={busy}>{t('prefixes.reset')}</Button>
        )}
      </form>
      <div className="mt-2.5 flex min-h-4 flex-wrap items-center gap-4">
        {error ? <ErrorNotice message={error} fallback={t('prefixError')} /> : <span />}
        {saved && (
          <span className="font-mono text-hint text-white/55">
{t('prefixes.effective', { folder: folder.trim() || saved.prefixes_folder || saved.default_prefixes_folder || t('prefixes.placeholder') })}
          </span>
        )}
      </div>
    </section>
  );
}
