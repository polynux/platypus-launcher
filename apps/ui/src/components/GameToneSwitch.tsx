// The game colour switch (issue #123): "Accent" keeps the game's colour on
// buttons and highlights, "Whole app" retints the surfaces, lines and text
// too. The theme's color.game-tone axis says what it allows; a theme that only
// takes the accent has nothing to switch, so the switch hides there (and so
// does it on a theme that leaves the axis off). The shell places it.
//
// Only atlas sets color.game-tone = "whole", so this renders there alone: the
// /21 `.tint` reading (2px-padded bg-2 container, 26px buttons, active tab
// with an inset accent underline, never a filled pill).
import { useTranslation } from 'react-i18next';
import { Droplet, PaintBucket } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { GameTone } from '../lib/gameTone';
import { ToggleGroup, ToggleGroupItem } from './ds/toggle-group';

export interface GameToneSwitchProps {
  /** The app's game tone (App owns the hook, so the tone also applies where
   *  no switch is shown). */
  tone: GameTone;
  className?: string;
}

const tintItem =
  'gap-1.5 rounded-md bg-transparent px-2.5 py-0 font-sans text-[12.5px] font-semibold text-text-muted transition hover:bg-transparent hover:text-text ' +
  'data-[state=on]:bg-surface-strong data-[state=on]:font-semibold data-[state=on]:text-text ' +
  'data-[state=on]:shadow-[inset_0_-2px_0_var(--color-accent)]';

export function GameToneSwitch({ tone, className }: GameToneSwitchProps) {
  const { t } = useTranslation('gameTone');
  if (tone.supported !== 'whole') return null;
  return (
    <ToggleGroup
      type="single"
      value={tone.choice}
      onValueChange={(next) => {
        // Radix reports "" when the active item is clicked again: the colour
        // stays, there is no third state.
        if (next === 'accent' || next === 'whole') tone.setChoice(next);
      }}
      aria-label={t('gameTone:label')}
      className={cn('gap-0.5 rounded-lg bg-surface p-0.5', className)}
    >
      <ToggleGroupItem
        value="accent"
        title={t('gameTone:accentHint')}
        className={cn('h-[26px]', tintItem)}
      >
        <Droplet className="size-[13px]" aria-hidden />
        {t('gameTone:accent')}
      </ToggleGroupItem>
      <ToggleGroupItem
        value="whole"
        title={t('gameTone:wholeHint')}
        className={cn('h-[26px]', tintItem)}
      >
        <PaintBucket className="size-[13px]" aria-hidden />
        {t('gameTone:whole')}
      </ToggleGroupItem>
    </ToggleGroup>
  );
}
