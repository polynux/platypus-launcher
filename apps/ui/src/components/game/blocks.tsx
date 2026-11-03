// The game page's hero blocks (issue #119): the back link, the
// outside-folder alert, the install note, the hero heading and actions, the
// compat facts and the activity. Props only: the page owns the data.
import React, { useState } from 'react';
import { useTranslation, Trans } from 'react-i18next';
import { AlertTriangle, ArrowLeft, Download, FolderCheck, FolderInput, RefreshCw, Settings2, Store as StoreIcon } from 'lucide-react';
import type { ActivityEvent, Game, GameJobKind, Job, SessionEvent, StorePlugin } from '../../types';
import { ActivityFeed } from '../ActivityFeed';
import { PlayButton } from '../PlayButton';
import { JobProgress } from '../JobProgress';
import { CompatFacts, tierMeaning } from '../CompatBadges';
import { useCompat } from '../../lib/useCompat';
import { Badge } from '@/components/ds/badge';
import { Button } from '@/components/ds/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ds/tooltip';
import { MonoLabel, Panel, TierBadge, cx } from '../ui';
import { heroChipClass } from '../Hero';
import { PluginLogo } from '../PluginLogo';
import { formatBytes } from '../../lib/games';
import { formatDecimal1 } from '../../lib/gameLocale';
import { useVariant } from '../../lib/theme';
import type { InstallSize } from '../../lib/installSize';
import { ErrorNotice } from '../ErrorNotice';

/** The back link (catalog game.back): where the game was opened from. Required. */
export function GameBackLink({ backLabel, onBack }: { backLabel: string; onBack: () => void }) {
  return (
    <Button variant="ghost" size="none" onClick={onBack} className="gap-1.5 rounded px-0 text-[12px] font-semibold text-text-muted hover:bg-transparent hover:text-white">
      <ArrowLeft className="h-3.5 w-3.5" /> {backLabel}
    </Button>
  );
}

/** The outside-folder alert (catalog game.outside): a plugin game installed
 *  outside its folder. Required: it tells the truth about the game's state.
 *  In the tabs structure (atlas) it is a slim full-width warning row under
 *  the strip: danger tint with a 2px left inset bar, never a card. */
export function GameOutsideAlert({ game, plugin, onAllowFolder }: {
  game: Game;
  plugin: StorePlugin;
  onAllowFolder: (folder: string) => void;
}) {
  const { t } = useTranslation('game');
  const tabs = useVariant('game.structure') === 'tabs';
  if (!game.is_installed || !game.outside_folder) return null;
  if (tabs) {
    return (
      <div role="alert" className="flex items-center gap-3 bg-danger/10 px-6 py-2.5 text-[13px] shadow-[inset_2px_0_0_var(--color-danger)]">
        <AlertTriangle className="h-4 w-4 flex-none text-danger" aria-hidden />
        <p className="min-w-0 flex-1 leading-relaxed text-text">
          <span className="font-bold">{t('game:outside.title', { name: plugin.name })} </span>
          <Trans
            i18nKey="game:outside.body"
            values={{ name: plugin.name, folder: plugin.install_folder, dir: game.install_dir }}
            components={{
              folder: <span className="font-mono text-[12px]" />,
              dir: <span className="font-mono text-[12px] text-text" />,
            }}
          />
        </p>
        <Button onClick={() => onAllowFolder(game.install_dir)} className="flex-none">
          <FolderCheck className="h-4 w-4" /> {t('game:outside.allow')}
        </Button>
      </div>
    );
  }
  return (
    <div role="alert" className="mb-6 flex flex-wrap items-center justify-between gap-4 rounded-2xl bg-accent-strong/[0.07] px-5 py-4 ring-1 ring-accent-strong/25">
      <div className="min-w-0 text-[12.5px] leading-relaxed text-white/75">
        <p className="font-bold text-amber-200">{t('game:outside.title', { name: plugin.name })}</p>
        <p>
          <Trans
            i18nKey="game:outside.body"
            values={{ name: plugin.name, folder: plugin.install_folder, dir: game.install_dir }}
            components={{
              folder: <span className="font-mono text-[12px]" />,
              dir: <span className="font-mono text-[12px] text-text" />,
            }}
          />
        </p>
      </div>
      <Button onClick={() => onAllowFolder(game.install_dir)} className="flex-none">
        <FolderCheck className="h-4 w-4" /> {t('game:outside.allow')}
      </Button>
    </div>
  );
}

/** The install note (catalog game.note): the sideload note, the fresh-install
 *  size and folder, the failed attempt, and the refreshing line. All small
 *  full-width notices live here, so a layout moves them as one. */
export function GameInstallNote({ game, plugin, installing, installSize, job, canChooseVersion, refreshing, onOpenVersions, onOpenPlugins }: {
  game: Game;
  plugin?: StorePlugin;
  installing: boolean;
  installSize: InstallSize;
  job?: Job;
  canChooseVersion: boolean;
  refreshing?: boolean;
  onOpenVersions: () => void;
  onOpenPlugins: () => void;
}) {
  const { t } = useTranslation('game');
  const active = job && (job.state === 'queued' || job.state === 'running');
  const showNote = !active && (game.note || installing);
  const tabs = useVariant('game.structure') === 'tabs';
  if (tabs) {
    // Atlas (/21 has no row here): the note lives inline at the end of the
    // action strip (muted 12.5px, folder links in the accent), never as a
    // 41px row between the strip and the tabs. The strip owns it.
    return null;
  }
  if (!showNote && !refreshing) return null;
  if (showNote && !refreshing) {
    return (
      <div className="mb-6 rounded-2xl bg-surface px-5 py-3.5 text-[12.5px] leading-relaxed text-white/65 ring-1 ring-hairline">
        {game.note && <p>{game.note}</p>}
        {installing && plugin && (
          <>
            <InstallSizeLine size={installSize} />
            <p className="flex flex-wrap items-center gap-x-1.5 font-mono text-[12px] text-text-muted">
              {canChooseVersion ? (
                <Trans
                  i18nKey="game:install.noteWithVersion"
                  values={{ folder: plugin.install_folder }}
                  components={{
                    v: <NoteLink onClick={onOpenVersions} />,
                    d: <NoteLink onClick={onOpenPlugins} />,
                  }}
                />
              ) : (
                <Trans
                  i18nKey="game:install.notePlain"
                  values={{ folder: plugin.install_folder }}
                  components={{ d: <NoteLink onClick={onOpenPlugins} /> }}
                />
              )}
            </p>
          </>
        )}
        {!game.is_installed && job?.state === 'failed' && <ErrorNotice className="mt-1" message={job.error ?? ''} fallback={t('game:install.lastAttempt')} />}
      </div>
    );
  }
  // Both present (a rescan landing while the note shows): one root for the
  // row count, the same siblings as today inside it.
  return (
    <div>
      {showNote && (
        <div className="mb-6 rounded-2xl bg-surface px-5 py-3.5 text-[12.5px] leading-relaxed text-white/65 ring-1 ring-hairline">
          {game.note && <p>{game.note}</p>}
          {installing && plugin && (
            <>
              <InstallSizeLine size={installSize} />
              <p className="flex flex-wrap items-center gap-x-1.5 font-mono text-[12px] text-text-muted">
                {canChooseVersion ? (
                  <Trans
                    i18nKey="game:install.noteWithVersion"
                    values={{ folder: plugin.install_folder }}
                    components={{
                      v: <NoteLink onClick={onOpenVersions} />,
                      d: <NoteLink onClick={onOpenPlugins} />,
                    }}
                  />
                ) : (
                  <Trans
                    i18nKey="game:install.notePlain"
                    values={{ folder: plugin.install_folder }}
                    components={{ d: <NoteLink onClick={onOpenPlugins} /> }}
                  />
                )}
              </p>
            </>
          )}
          {!game.is_installed && job?.state === 'failed' && <ErrorNotice className="mt-1" message={job.error ?? ''} fallback={t('game:install.lastAttempt')} />}
        </div>
      )}
      {refreshing && (
        <p className="mb-4 font-mono text-micro uppercase tracking-widest text-text-muted">{t('game:refreshing')}</p>
      )}
    </div>
  );
}

/** Compatibility facts (catalog game.compat): the curated recipe or rating,
 *  the runner facts, and the compat badges. Props only (facts come from the
 *  page's useCompat hook). */
export function GameCompat({ compat, fix, facts, info }: {
  compat: string;
  fix: Game['fix_info'];
  facts: [string, string][];
  info: ReturnType<typeof useCompat>['compat'];
}) {
  const { t } = useTranslation('game');
  return (
    <Panel className="bg-surface-strong">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:compat.heading')}</h2>
      <p className="text-[13px] leading-relaxed text-white/65">{compat}</p>
      {fix?.winetricks && fix.winetricks.length > 0 && (
        <p className="mt-2 font-mono text-hint text-sky-200/80">{t('game:compat.recipe', { mods: fix.winetricks.join(' ') })}</p>
      )}
      <div className="mt-4 space-y-2 font-mono text-hint">
        {facts.map(([k, v]) => (
          <div key={k} className="flex justify-between gap-4 border-t border-hairline pt-2">
            <span className="text-text-muted">{k}</span>
            <span className="truncate text-right text-white/85">{v}</span>
          </div>
        ))}
      </div>
      {info && <CompatFacts compat={info} />}
    </Panel>
  );
}

/** Session activity for one game (catalog game.activity): the last events
 *  with this game's title. Props only. */
export function GameActivity({ events }: { events: ActivityEvent[] }) {
  const { t } = useTranslation('game');
  if (events.length === 0) return null;
  return (
    <section className="pb-10">
      <MonoLabel className="mb-2">{t('game:session')}</MonoLabel>
      <ActivityFeed events={events} />
    </section>
  );
}

function InstallButton({
  game, job, onInstall, onInstallTo, onCancel,
}: { game: Game; job?: Job; onInstall: () => void; onInstallTo?: () => void; onCancel: (id: string) => void }) {
  const { t } = useTranslation('game');
  if (job && (job.state === 'queued' || job.state === 'running')) return <JobProgress job={job} onCancel={onCancel} />;
  if (!game.installable) {
    return (
      <span className="rounded-2xl bg-white/[0.08] px-6 py-3.5 text-[13px] font-bold text-text-muted" title={game.note}>
        {t('game:install.cantInstall')}
      </span>
    );
  }
  return (
    <>
      {onInstallTo && (
        <Button variant="neutral" size="none" onClick={onInstallTo} className="gap-2 rounded-2xl bg-black/50 px-4 py-3.5 text-[13px] text-white/80 hover:bg-black/70">
          <FolderInput className="h-4 w-4" /> {t('game:install.installTo')}
        </Button>
      )}
      <Button
        variant="primary"
        size="none"
        onClick={onInstall}
        className="gap-2 rounded-2xl px-8 py-3.5 text-[15px] shadow-[0_12px_30px_rgba(252,211,77,.25)]"
      >
        <Download className="h-4 w-4" strokeWidth={3} />
        {job?.state === 'failed' ? t('game:install.retryInstall') : t('game:install.install')}
      </Button>
    </>
  );
}

/** Today's game page hero heading (catalog game.heading): the badges and the
 *  title. The game page shows it inside the Hero frame; #103 will place it in
 *  a layout. Positioning is structural (as today): the frame lays the heading
 *  and the actions out. */
export function GameHeroHeading({ game, fix, storeLogo, storeName, needsConnection }: {
  game: Game;
  fix: Game['fix_info'];
  storeLogo?: string;
  storeName: string;
  needsConnection: boolean;
}) {
  const { t } = useTranslation('game');
  const chips = useVariant('game-hero.chips');
  return (
    <div className="min-w-0 flex-1 basis-72">
      <div className="mb-2.5 flex flex-wrap gap-2">
        {fix?.tier && <TierBadge tier={fix.tier} suffix={t('game:tierBadge.curated')} title={tierMeaning(t, fix.tier)} />}
        <Badge className={cx(heroChipClass(chips), 'text-white/75')} data-testid="store-badge">
          <span className="inline-flex items-center gap-1.5">
            <PluginLogo src={storeLogo} name={storeName} size="xs" />
            <span>{storeName}</span>
          </span>
        </Badge>
        {formatBytes(game.size_bytes) && <Badge className={cx(heroChipClass(chips), 'text-white/75')}>{formatBytes(game.size_bytes)}</Badge>}
        {!game.is_installed && <Badge className={cx(heroChipClass(chips), 'text-text-muted')}>{t('game:hero.notInstalled')}</Badge>}
        {game.update_available && <Badge variant="info" className="bg-sky-400/15 ring-1 ring-info/30">{t('game:hero.updateAvailable')}</Badge>}
        {game.store_missing && <Badge className={cx(heroChipClass(chips), 'text-text-muted')}>{t('game:hero.storeMissing')}</Badge>}
        {game.pinned && <Badge variant="accent" className="bg-accent-strong/15 ring-1 ring-accent-strong/30">{t('game:hero.pinned')}</Badge>}
        {needsConnection && (
          <Badge className="bg-accent-strong/15 text-amber-100 ring-1 ring-accent-strong/30" data-testid="needs-connection">
            {t('game:hero.needsConnection', { store: storeName })}
          </Badge>
        )}
      </div>
      <GameHeading game={game} />
    </div>
  );
}

/** Today's game page hero actions (catalog game.actions): playtime, Install /
 *  Play / Update, game settings and the store link. Required (it holds
 *  Install, Play and Update); the game page shows it inside the Hero frame
 *  until #103 places it in a layout. */
export function GameHeroActions({ game, plugin, job, active, session, launching, hours, last, playtimeTitle, canChooseOptions, onJob, onJobAction, onPlay, onStop, onForceStop, onInstall, onInstallTo, onSettings, onOpenStore }: {
  game: Game;
  plugin?: StorePlugin;
  job?: Job;
  active: boolean;
  session?: SessionEvent;
  launching: boolean;
  hours: number | null;
  last?: string;
  playtimeTitle?: string;
  canChooseOptions: boolean;
  onJob: (kind: GameJobKind) => void;
  onJobAction: (action: 'cancel' | 'retry' | 'pause' | 'resume', id: string) => void;
  onPlay: () => void;
  onStop: () => void;
  onForceStop: () => void;
  onInstall: () => void;
  onInstallTo?: () => void;
  onSettings: () => void;
  onOpenStore?: () => void;
}) {
  const { t } = useTranslation('game');
  const chips = useVariant('game-hero.chips');
  return (
    <div className="flex flex-wrap items-center justify-end gap-x-4 gap-y-3">
      {(hours != null || last) && (
        <div className="text-right font-mono text-hint leading-relaxed text-text-muted">
          {hours != null && <div title={playtimeTitle}>{t('game:hero.hoursLogged', { hours: formatDecimal1(hours) })}</div>}
          {last && <div>{t('game:hero.lastPlayed', { when: last })}</div>}
        </div>
      )}
      <div className="flex flex-wrap items-center justify-end gap-2.5">
        {game.is_installed && plugin && game.update_available && !active && (
          <Button variant="neutral" size="none" onClick={() => onJob('update')} className={cx(heroChipClass(chips), 'gap-2 rounded-2xl px-4 py-3.5 text-[13px] text-sky-100 hover:bg-black/70')}>
            <RefreshCw className="h-4 w-4" /> {t('game:hero.update')}
          </Button>
        )}
        {!game.is_installed ? (
          <InstallButton
            game={game}
            job={job}
            onInstall={onInstall}
            onInstallTo={onInstallTo}
            onCancel={(id) => onJobAction('cancel', id)}
          />
        ) : (
          <PlayButton session={session} launching={launching} disabled={game.outside_folder} onPlay={onPlay} onStop={onStop} onForceStop={onForceStop} />
        )}
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="neutral" size="none" onClick={onSettings} aria-label={t('game:hero.gameSettings')} className={cx(heroChipClass(chips), 'size-[50px] rounded-2xl text-white/80 hover:bg-black/70')}>
              <Settings2 className="h-[18px] w-[18px]" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t('game:hero.gameSettings')}</TooltipContent>
        </Tooltip>
        {plugin?.storefront_url && onOpenStore && (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="neutral"
                size="none"
                onClick={onOpenStore}
                aria-label={t('game:hero.openInStore', { title: game.title, store: plugin.name })}
                className={cx(heroChipClass(chips), 'size-[50px] rounded-2xl text-white/80 hover:bg-black/70')}
              >
                <StoreIcon className="h-[18px] w-[18px]" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t('game:hero.openInStoreTip')}</TooltipContent>
          </Tooltip>
        )}
      </div>
    </div>
  );
}

/** The game's name over the banner: the user's logo when they set one (the
 *  title stays as its accessible name), else the title in type. */
function GameHeading({ game }: { game: Game }) {
  const [broken, setBroken] = useState<string | null>(null);
  const logo = game.logo_url && game.logo_url !== broken ? game.logo_url : null;
  return (
    <h1 className={cx(logo ? 'flex' : 'truncate text-[36px] font-extrabold leading-none tracking-tight')}>
      {logo ? (
        <img src={logo} alt={game.title} onError={() => setBroken(logo)} className="max-h-[96px] max-w-full object-contain object-left" />
      ) : (
        game.title
      )}
    </h1>
  );
}

/** The size of a fresh install, said before it is queued. */
function InstallSizeLine({ size }: { size: InstallSize }) {
  const { t } = useTranslation('game');
  return (
    <p className="mb-1 font-mono text-[12px] text-white/75" data-testid="install-size-line">
      {size.state === 'loading' && <span className="text-text-muted">{t('game:installSize.checking')}</span>}
      {size.state === 'unknown' && <span title={t('game:installSize.unknownTitle')}>{t('game:installSize.unknown')}</span>}
      {size.state === 'known' && (
        <>
          {t('game:installSize.known', { size: formatBytes(size.total) })}
          {size.added > 0 && <span className="text-text-muted">{t('game:installSize.addons', { size: formatBytes(size.added) })}</span>}
        </>
      )}
    </p>
  );
}

function NoteLink({ onClick, children }: { onClick: () => void; children?: React.ReactNode }) {
  return (
    <Button variant="ghost" size="none" onClick={onClick} className="inline-flex rounded-none px-0 font-mono text-[12px] font-normal underline decoration-white/25 underline-offset-2 hover:bg-transparent hover:text-white/80">
      {children}
    </Button>
  );
}

/** The install note's folder links in the tabs row: plain sans text in the
 *  accent, underlined on hover (the lab's link style, not the mono links the
 *  sections page keeps). Only the tabs row uses it. */
export function TabsNoteLink({ onClick, children }: { onClick: () => void; children?: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} className="inline font-sans text-accent hover:underline">
      {children}
    </button>
  );
}
