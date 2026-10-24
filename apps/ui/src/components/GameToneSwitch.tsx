// The game colour switch (issue #123): "Accent" keeps the game's colour on
// buttons and highlights, "Whole app" retints the surfaces, lines and text
// too. The theme's color.game-tone axis says what it allows; a theme that only
// takes the accent has nothing to switch, so the switch hides there (and so
// does it on a theme that leaves the axis off). The shell places it.
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { useGameTone } from '../lib/gameTone';
import { ToggleGroup, ToggleGroupItem } from './ds/toggle-group';

export interface GameToneSwitchProps {
  /** The selected game's hero art: its colour drives the tone. */
  heroUrl?: string | null;
  className?: string;
}

export function GameToneSwitch({ heroUrl, className }: GameToneSwitchProps) {
  const { t } = useTranslation('gameTone');
  const tone = useGameTone(heroUrl);
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
