import React from 'react';
import { CinematicDashboard, CinematicGame } from './pages/p6.jsx';
import { GalaxyDashboard, GalaxyGame } from './pages/p7.jsx';
import { ProDashboard, ProGame } from './pages/p8.jsx';
import { EditorialDashboard, EditorialGame } from './pages/p9.jsx';
import { UtilityDashboard, UtilityGame } from './pages/p10.jsx';
import { GlassDashboard, GlassGame } from './pages/p11.jsx';
import { SwissSpecSheet } from './pages/p1.jsx';
import { OpsTonight } from './pages/p2.jsx';
import { PosterEditorial } from './pages/p3.jsx';
import { WorkshopBench } from './pages/p4.jsx';
import { CalmGallery } from './pages/p5.jsx';
// iterations on 7/10
import { It12Dashboard, It12Game } from './pages/p12.jsx';
import { It13Dashboard, It13Game } from './pages/p13.jsx';
import { It14Dashboard, It14Game } from './pages/p14.jsx';
import { It15Dashboard, It15Game } from './pages/p15.jsx';
import { It16Dashboard, It16Game } from './pages/p16.jsx';
import { It17Dashboard, It17Game } from './pages/p17.jsx';
import { It18Dashboard, It18Game } from './pages/p18.jsx';
import { It19Dashboard, It19Game } from './pages/p19.jsx';
import { It20Dashboard, It20Game } from './pages/p20.jsx';
import { It21Dashboard, It21Game } from './pages/p21.jsx';
import { LibraryView1626 } from './pages/library.jsx';

const PAGES = {
  1: SwissSpecSheet,
  2: OpsTonight,
  3: PosterEditorial,
  4: WorkshopBench,
  5: CalmGallery,
  6: CinematicDashboard,
  '6-game': CinematicGame,
  7: GalaxyDashboard,
  '7-game': GalaxyGame,
  8: ProDashboard,
  '8-game': ProGame,
  9: EditorialDashboard,
  '9-game': EditorialGame,
  10: UtilityDashboard,
  '10-game': UtilityGame,
  11: GlassDashboard,
  '11-game': GlassGame,
  12: It12Dashboard,
  '12-game': It12Game,
  13: It13Dashboard,
  '13-game': It13Game,
  14: It14Dashboard,
  '14-game': It14Game,
  15: It15Dashboard,
  '15-game': It15Game,
  16: It16Dashboard,
  '16-game': It16Game,
  17: It17Dashboard,
  '17-game': It17Game,
  18: It18Dashboard,
  '18-game': It18Game,
  19: It19Dashboard,
  '19-game': It19Game,
  20: It20Dashboard,
  '20-game': It20Game,
  21: It21Dashboard,
  '21-game': It21Game,
  'library': LibraryView1626,
};

export function App() {
  // accept both "/7" and "/7-game" and "/7/game" forms
  const path = window.location.pathname
    .replace(/^\/+|\/+$/g, '')
    .replace('/', '-');
  const Page = PAGES[path] || PAGES[path.split('-')[0]] || (path === '' ? PAGES['library'] : PAGES[1]);
  return <Page />;
}