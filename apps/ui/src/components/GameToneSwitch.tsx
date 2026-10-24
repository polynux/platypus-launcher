// The game colour switch (issue #123): "Accent" keeps the game's colour on
// buttons and highlights, "Whole app" retints the surfaces, lines and text
// too. The theme's color.game-tone axis says what it allows; a theme that only
// takes the accent has nothing to switch, so the switch hides there (and so
// does it on a theme that leaves the axis off). The shell places it.
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import type { GameTone } from '../lib/gameTone';
import { ToggleGroup, ToggleGroupItem } from './ds/toggle-group';

export interface GameToneSwitchProps {
  /** The app's game tone (App owns the hook, so the tone also applies where
   *  no switch is shown). */
  tone: GameTone;
  className?: string;
}

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
      className={cn('gap-0 rounded-lg bg-surface p-0.5 ring-1 ring-hairline', className)}
    >
      <ToggleGroupItem
        value="accent"
        title={t('gameTone:accentHint')}
        className="px-2.5 py-1 font-sans text-[12px]"
      >
        {t('gameTone:accent')}
      </ToggleGroupItem>
      <ToggleGroupItem
        value="whole"
        title={t('gameTone:wholeHint')}
        className="px-2.5 py-1 font-sans text-[12px]"
      >
        {t('gameTone:whole')}
      </ToggleGroupItem>
    </ToggleGroup>
  );
}
