import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import type { FamilyMember, StorePlugin } from '../types';
import { MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import { CapabilitySurface } from '../lib/capabilities';
import { memberInitial, roleLabel } from '../lib/family';
import { ErrorNotice } from './ErrorNotice';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

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
        {role && <span className="font-mono text-[10.5px] uppercase tracking-wider text-white/40">{role}</span>}
      </span>
      {member.state && <span className={cx('flex-none font-mono text-[11px]', member.state === 'online' ? 'text-emerald-300' : 'text-white/45')}>{member.state}</span>}
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
  const [members, setMembers] = useState<FamilyMember[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    setMembers(null);
    setError(null);
    Effect.runPromise(LauncherApi.getFamilyMembers(plugin.id)).then(
      (list) => { if (live) setMembers(list); },
      (e) => { if (live) setError(errMsg(e)); },
    );
    return () => { live = false; };
  }, [plugin.id]);

  return (
    <section aria-label="Families" className="space-y-2.5">
      <MonoLabel>families</MonoLabel>
      {error && <ErrorNotice message={error} fallback="Platypus couldn't read the family members." />}
      {!error && members === null && <p className="font-mono text-[11.5px] text-white/45">loading…</p>}
      {members && members.length === 0 && <p className="text-[12px] text-white/50">No family members on this account.</p>}
      {members && members.length > 0 && (
        <ul aria-label="Family members" className="space-y-2">
          {members.map((m) => <MemberRow key={m.id} member={m} />)}
        </ul>
      )}
      <p className="text-[11.5px] leading-relaxed text-white/40">Shown as the store reports them. Platypus doesn't enforce the family's rules.</p>
    </section>
  );
}
