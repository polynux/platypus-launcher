import { describe, it, expect } from 'vitest';
import type { Game, ProgressAchievement } from '../types';
import { achievementMeta, hoursOf, playtimeTooltip } from './stats';

const base = { id: 'p.echo.tiny', title: 'Echo Tiny' } as Game;
const ach = (o: Partial<ProgressAchievement> = {}): ProgressAchievement => ({
  id: 'a', name: 'A', description: '', icon: '', hidden: false, unlocked: false, unlocked_at: 0, ...o,
});

describe('playtime tooltip', () => {
  it('shows the local record next to the store report', () => {
    expect(playtimeTooltip({ ...base, playtime_local_minutes: 30, playtime_store_minutes: 150, playtime_minutes: 150 })).toBe(
      'local 0.5 h · store 2.5 h',
    );
  });

  it('is absent without a store report', () => {
    expect(playtimeTooltip({ ...base, playtime_minutes: 600 })).toBeUndefined();
  });

  it('rounds to one decimal of an hour', () => {
    expect(hoursOf(100)).toBe(1.7);
    expect(hoursOf(6)).toBe(0.1);
  });
});

describe('achievement meta line', () => {
  const date = (unix: number) => `day ${unix}`;

  it('gives the unlock date for an unlocked achievement', () => {
    expect(achievementMeta(ach({ unlocked: true, unlocked_at: 1700 }), date)).toBe('unlocked day 1700');
  });

  it('gives the global share for a locked one that has it', () => {
    expect(achievementMeta(ach({ global_percent: 84.14 }), date)).toBe('84.1% of players');
  });

  it('is empty when the store gave neither', () => {
    expect(achievementMeta(ach(), date)).toBe('');
  });
});
