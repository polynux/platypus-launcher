import React from 'react';
import './index.css';

const DESIGNS = [
  { n: 1, name: 'Graphite', line: 'The brief played straight: Zed’s docking, Linear’s density. The launch command lights up as you edit settings.' },
  { n: 2, name: 'Timetable', line: 'The library as a timetable with sortable columns; your play sessions drawn as a 14-day schedule.' },
  { n: 3, name: 'Cinemascope', line: 'Hero art letterboxed at 2.39:1, the one colour on screen sampled from it. Inspector docks as a tool rail.' },
  { n: 4, name: 'Source', line: 'Olive, etched, matte, after Steam’s 2004 interface. The status strip opens into a working console (press `).' },
  { n: 5, name: 'Monograph', line: 'An alphabetical index for a spine and the title set big across the seam between art and action strip.' },
];

const ROUND2 = [
  { n: 6, name: 'Studio', line: '/1’s layout made warmer: art capsules in the list, the accent sampled from each game, a filter menu that holds any number of stores, Basics and Advanced settings.' },
  { n: 7, name: 'Monograph II', line: '/5 pushed further: covers in the index, the masthead washed in the banner’s colour, the box art sitting on the seam. Palette and dialogs turn to the same paper.' },
  { n: 8, name: 'Lounge', line: 'For everyone: big covers, “Continue playing”, art fading into a room tinted by the game, and settings in plain words with Advanced one tap away.' },
  { n: 9, name: 'Atlas', line: 'Stores live on a vertical rail that grows with every plugin you add; shift-click to combine. Launch-from becomes a dropdown so it scales too.' },
  { n: 10, name: 'Duotone', line: 'The whole shell is cut from one hue of the game’s art, four depths of it, and changes as you move through the library.' },
];

const ROUND3 = [
  { n: 11, name: 'Studio II', line: 'Stores as a labelled column with Installed, Downloads and Needs attention above them. Cover and title from /7. Settings open in their own window with Everyday and Advanced sections.' },
  { n: 12, name: 'Lounge II', line: 'Store chips that wrap for any number of stores, an All / Installed / Downloads switch, and a callout only when games need you. Settings rise as a sheet over the page.' },
  { n: 13, name: 'Monograph III', line: 'One-line store picker that opens into a checklist. Play sits on the cover’s sleeve. Settings arrive as a sheet of paper with a table of contents.' },
  { n: 14, name: 'Atlas II', line: 'Places and stores in one bar across the top, scrolling as stores are added. A Tune button opens quick tiles, and All settings opens the full drawer.' },
  { n: 15, name: 'Duotone II', line: 'The intense game colour of /10 at desktop shapes. View tiles with counts, a store checklist, and settings that work like a phone’s: a list of topics, one page each.' },
];

const ROUND4 = [
  { n: 16, name: 'Studio III', line: '/11 with a sidebar that collapses to an icon rail (Ctrl B). Under Play, the real store page: screenshots with thumbnails, the full description with its pictures and clips, achievements.' },
  { n: 17, name: 'Lounge III', line: 'The Material look of /12, but settings are a section of the game page with a category list, as in a desktop app. A sticky bar keeps Play in reach while you scroll.' },
  { n: 18, name: 'Monograph IV', line: 'The store page set as an article with a margin of pictures and achievements. Settings open on a blurred pane over the page, with the banner colour only as an accent.' },
  { n: 19, name: 'Atlas III', line: 'The top source bar of /14. Tune is now a desktop flyout: presets, four switches and the compatibility layer. Store page, achievements and activity in tabs.' },
  { n: 20, name: 'Duotone III', line: 'The intense hue of /15 with the store list folded by default and a flat game list. Full store page with a large screenshot viewer.' },
];

const FINAL = [
  { n: 21, name: 'Atlas final', line: '/19 with the screenshots as a contact sheet in the right column, like /18. A switch in the top bar picks how far the game colour goes: accents only, or the whole app as in /20.' },
];

export function Index() {
  return (
    <main className="ix">
      <h1>Platypus, readings of one brief</h1>
      <p className="ix-lead">Same library, same behaviour, different worlds. Every screen is interactive: arrows or j/k move through the library, Ctrl K opens the command palette, Ctrl I toggles the inspector, Enter plays.</p>
      <ol>
        {DESIGNS.map((d) => (
          <li key={d.n}>
            <a href={`/${d.n}`}><span className="ix-n">/{d.n}</span><span className="ix-name">{d.name}</span><span className="ix-line">{d.line}</span></a>
          </li>
        ))}
      </ol>
      <h2 className="ix-h2">Round two, after feedback</h2>
      <ol>
        {ROUND2.map((d) => (
          <li key={d.n}>
            <a href={`/${d.n}`}><span className="ix-n">/{d.n}</span><span className="ix-name">{d.name}</span><span className="ix-line">{d.line}</span></a>
          </li>
        ))}
      </ol>
      <h2 className="ix-h2">Round three: store selection and settings</h2>
      <ol>
        {ROUND3.map((d) => (
          <li key={d.n}>
            <a href={`/${d.n}`}><span className="ix-n">/{d.n}</span><span className="ix-name">{d.name}</span><span className="ix-line">{d.line}</span></a>
          </li>
        ))}
      </ol>
      <h2 className="ix-h2">Round four: store pages, achievements, settings that fit a desktop</h2>
      <ol>
        {ROUND4.map((d) => (
          <li key={d.n}>
            <a href={`/${d.n}`}><span className="ix-n">/{d.n}</span><span className="ix-name">{d.name}</span><span className="ix-line">{d.line}</span></a>
          </li>
        ))}
      </ol>
      <h2 className="ix-h2">Final</h2>
      <ol>
        {FINAL.map((d) => (
          <li key={d.n}>
            <a href={`/${d.n}`}><span className="ix-n">/{d.n}</span><span className="ix-name">{d.name}</span><span className="ix-line">{d.line}</span></a>
          </li>
        ))}
      </ol>
      <p className="ix-foot">Art and store pages from Steam (snapshot). Library, sessions, saves and settings are illustrative.</p>
    </main>
  );
}
