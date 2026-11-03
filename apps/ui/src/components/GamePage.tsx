// The game page split (issue #119): the page and the data it owns live in
// ./game/GamePage.tsx, the hero blocks in ./game/blocks.tsx and the sections
// in ./game/sections.tsx. This barrel keeps every existing import working.
export { GamePage, invalidateGameDetailsCache } from './game/GamePage';
export {
  GameActivity,
  GameBackLink,
  GameCompat,
  GameHeroActions,
  GameHeroHeading,
  GameInstallNote,
  GameOutsideAlert,
  TabsNoteLink,
} from './game/blocks';
export { AboutSection, AchievementsSection, SafeHtml } from './game/sections';
