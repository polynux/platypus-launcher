// i18n: converted
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import type { FamilyMember, StorePlugin } from '../types';
import { MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import { CapabilitySurface } from '../lib/capabilities';
import { memberInitial, roleLabel } from '../lib/family';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';

/** One member: an avatar (or the initial when there is none or it fails to
 *  load), the name, the role and the state the store reports. */
function MemberRow({ member }: { member: FamilyMember }) {
  const [broken, setBroken] = useState(false);
  const role = roleLabel(member.role);
  return (
    <li className="flex items-center gap-3">
      <span className="grid h-8 w-8 flex-none place-items-center overflow-hidden rounded-full bg-white/10 font-mono text-[12px] text-white/70">
        {member.avatar_url && !broken ? (
          <img src={member.avatar_url} alt="" onError={() => setBroken(true)} className="h-full w-full object-cover" />
        ) : (
          memberInitial(member)
        )}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[12.5px] font-bold text-white/90">{member.name}</span>
        {role && <span className="font-mono text-micro uppercase tracking-wider text-white/55">{role}</span>}
      </span>
      {member.state && <span className={cx('flex-none font-mono text-hint', member.state === 'online' ? 'text-emerald-300' : 'text-white/55')}>{member.state}</span>}
    </li>
  );
}

/** The Families section of a store plugin: its family group, members with their
 *  role and state. It shows what the store reports and enforces nothing; the
 *  store's own rules (exclusions, limits) apply where the game is played. */
export function FamiliesPanel({ plugin }: { plugin: StorePlugin }) {
  return (
    <CapabilitySurface plugin={plugin} capability="family">
      <FamiliesList plugin={plugin} />
    </CapabilitySurface>
  );
}

/** Loads the members once the capability is on: nothing is asked of a plugin without it. */
function FamiliesList({ plugin }: { plugin: StorePlugin }) {
  const { t } = useTranslation('family');
  const [members, setMembers] = useState<FamilyMember[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    setMembers(null);
    setError(null);
    Effect.runPromise(LauncherApi.getFamilyMembers(plugin.id)).then(
      (list) => { if (live) setMembers(list); },
      (e) => { if (live) setError(coreErrMsg(e)); },
    );
    return () => { live = false; };
  }, [plugin.id]);

  return (
    <section aria-label={t('family:panel.label')} className="space-y-2.5">
      <MonoLabel>{t('family:panel.heading')}</MonoLabel>
      {error && <ErrorNotice message={error} fallback={t('family:panel.error')} />}
      {!error && members === null && <p className="font-mono text-hint text-white/55">{t('family:panel.loading')}</p>}
      {members && members.length === 0 && <p className="text-[12px] text-white/70">{t('family:panel.empty')}</p>}
      {members && members.length > 0 && (
        <ul aria-label={t('family:panel.members')} className="space-y-2">
          {members.map((m) => <MemberRow key={m.id} member={m} />)}
        </ul>
      )}
      <p className="text-hint leading-relaxed text-white/55">{t('family:panel.note')}</p>
    </section>
  );
}
